package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
)

// AutoCodeService 自动代码生成服务
// 这是一个无状态的服务结构体，作为服务入口点，负责协调自动代码生成的各个组件
type AutoCodeService struct{}

// Database 数据库元数据操作接口
// 定义了获取数据库元数据的统一规范，屏蔽不同数据库之间的实现差异
//
// 设计意义：
// 1. 接口抽象：通过接口定义统一的操作规范，上层业务代码无需关心具体是哪种数据库
// 2. 多态支持：不同的数据库实现（MySQL、PostgreSQL、SQL Server等）都实现相同的接口
// 3. 依赖倒置：业务代码依赖抽象（接口），而非具体实现，符合SOLID原则中的依赖倒置原则
//
// 接口方法说明：
//   - GetDB: 获取数据库服务器中的所有数据库列表
//   - GetTables: 获取指定数据库中的所有表列表
//   - GetColumn: 获取指定表中所有列的详细信息（字段名、类型、注释、主键标识等）
//
// 好处：
//   - 可扩展性：新增数据库支持只需实现该接口，无需修改现有业务代码
//   - 可测试性：可以使用 Mock 实现进行单元测试
//   - 代码复用：相同的数据获取逻辑可以复用，减少重复代码
type Database interface {
	GetDB(businessDB string) (data []response.Db, err error)
	GetTables(businessDB string, dbName string) (data []response.Table, err error)
	GetColumn(businessDB string, tableName string, dbName string) (data []response.Column, err error)
}

// Database 根据业务数据库标识返回对应的数据库操作实现（工厂方法）
//
// 参数说明：
//   - businessDB: 业务数据库连接别名
//   - 空字符串 ""：使用系统默认数据库连接，根据 global.GVA_CONFIG.System.DbType 确定数据库类型
//   - 非空字符串：从 global.GVA_CONFIG.DBList 中查找对应别名的数据库配置，使用其类型
//
// 设计模式：工厂方法模式（Factory Method Pattern）
// 根据配置动态创建和返回不同的数据库实现实例，实现策略选择
//
// 为什么这样设计：
//
//  1. 策略模式：根据数据库类型选择不同的实现策略
//     不同的数据库（MySQL、PostgreSQL、SQL Server、Oracle、SQLite）有不同的元数据查询语法
//     通过接口抽象和工厂方法，可以在运行时动态选择正确的实现
//
//  2. 多数据源支持：
//     - businessDB == ""：使用默认数据源，适用于单数据库场景
//     - businessDB != ""：从 DBList 中选择指定的业务数据源，支持多租户、多业务场景
//     这种设计让系统能够同时管理多个不同类型的数据库连接
//
//  3. 配置驱动：
//     数据库类型的选择完全由配置决定，无需修改代码即可切换数据库类型
//     提高了系统的灵活性和可配置性
//
//  4. 统一入口：
//     无论使用哪种数据库，都通过同一个方法获取实现，保持了API的一致性
//
// 返回逻辑说明：
//
//   - 当 businessDB 为空时：
//     根据 global.GVA_CONFIG.System.DbType 配置选择对应的数据库实现
//     如果配置的数据库类型不在支持列表中，默认返回 MySQL 实现（向后兼容）
//
//   - 当 businessDB 不为空时：
//     遍历 global.GVA_CONFIG.DBList 查找匹配的数据库配置
//     根据配置的 Type 字段选择对应的数据库实现
//     如果未找到匹配的配置，默认返回 MySQL 实现
//
// 好处：
//  1. 解耦：业务代码与具体数据库实现解耦，更换数据库类型只需修改配置
//  2. 可扩展：新增数据库类型支持只需：
//     a) 实现 Database 接口（如 sys_auto_code_mysql.go）
//     b) 创建对应的全局实例变量（如 AutoCodeMysql）
//     c) 在此方法中添加对应的 case 分支
//  3. 单一职责：每种数据库的实现独立在各自的文件中，职责清晰
//  4. 易于维护：修改某个数据库的实现不会影响其他数据库
//  5. 类型安全：编译期就能发现接口实现不完整的问题
//  6. 性能优化：返回的是单例实例（如 AutoCodeMysql），避免重复创建对象
//
// 使用示例：
//
//	// 获取默认数据库的实现
//	dbService := autoCodeService.Database("")
//	databases, err := dbService.GetDB("")
//
//	// 获取指定业务数据库的实现
//	dbService := autoCodeService.Database("tenant1")
//	tables, err := dbService.GetTables("tenant1", "mydb")
func (autoCodeService *AutoCodeService) Database(businessDB string) Database {
	// 场景1：使用默认数据库连接（单数据源场景）
	if businessDB == "" {
		// 根据系统配置的默认数据库类型选择对应的实现
		switch global.GVA_CONFIG.System.DbType {
		case "mysql":
			return AutoCodeMysql
		case "pgsql":
			return AutoCodePgsql
		case "mssql":
			return AutoCodeMssql
		case "oracle":
			return AutoCodeOracle
		case "sqlite":
			return AutoCodeSqlite
		default:
			// 默认返回 MySQL 实现，保证向后兼容性
			// 即使配置了未知的数据库类型，系统仍能正常运行
			return AutoCodeMysql
		}
	} else {
		// 场景2：使用指定的业务数据库连接（多数据源场景）
		// 遍历配置的数据库连接列表，查找匹配的业务数据库
		for _, info := range global.GVA_CONFIG.DBList {
			if info.AliasName == businessDB {
				// 找到匹配的数据库配置，根据其类型选择对应的实现
				switch info.Type {
				case "mysql":
					return AutoCodeMysql
				case "mssql":
					return AutoCodeMssql
				case "pgsql":
					return AutoCodePgsql
				case "oracle":
					return AutoCodeOracle
				case "sqlite":
					return AutoCodeSqlite
				default:
					// 如果数据库类型不在支持列表中，默认返回 MySQL 实现
					return AutoCodeMysql
				}
			}
		}
		// 如果未找到匹配的业务数据库配置，默认返回 MySQL 实现
		// 这样可以避免返回 nil，保证调用方不会出现空指针异常
		return AutoCodeMysql
	}
}
