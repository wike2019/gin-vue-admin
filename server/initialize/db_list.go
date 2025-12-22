package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// sys 常量定义系统数据库的别名标识
// 为什么使用常量而不是硬编码字符串？
// - 避免魔法字符串：统一管理数据库别名，减少拼写错误
// - 易于维护：如果需要修改系统数据库别名，只需修改一处
// - 提高可读性：代码意图更清晰，表明这是系统数据库的特殊标识
const sys = "system"

// DBList 初始化多数据库连接列表
//
// 设计思路：
// 1. 支持多数据库配置：允许在配置文件中定义多个数据库连接
// 2. 按需初始化：只初始化未禁用的数据库配置
// 3. 统一管理：使用 map 结构统一管理所有数据库连接
// 4. 向后兼容：保持 GVA_DB 变量以兼容旧版本代码
//
// 为什么使用 map[string]*gorm.DB 存储数据库连接？
// - 快速查找：通过别名（AliasName）快速定位对应的数据库连接，时间复杂度 O(1)
// - 灵活扩展：支持动态添加多个数据库，无需修改代码结构
// - 业务隔离：不同业务模块可以使用不同的数据库，实现数据隔离
// - 易于管理：通过别名区分不同用途的数据库（如：system、business、log等）
func DBList() {
	// 创建数据库连接映射表
	// key: 数据库别名（AliasName），value: GORM 数据库连接实例
	dbMap := make(map[string]*gorm.DB)

	// 遍历配置中的所有数据库配置项
	// 为什么遍历配置而不是直接连接？
	// - 配置驱动：通过配置文件灵活控制数据库连接，无需重新编译代码
	// - 环境适配：不同环境（开发/测试/生产）可以使用不同的数据库配置
	for _, info := range global.GVA_CONFIG.DBList {
		// 跳过被禁用的数据库配置
		// 为什么需要 Disable 选项？
		// - 临时禁用：可以临时禁用某个数据库而不删除配置，方便调试和故障排查
		// - 环境切换：在不同环境中可以快速启用/禁用特定数据库
		// - 配置复用：同一配置文件可以在不同场景下使用，通过 Disable 控制
		if info.Disable {
			continue
		}

		// 根据数据库类型选择对应的初始化函数
		// 为什么使用 switch 而不是 if-else？
		// - 性能优化：switch 语句在某些情况下比 if-else 链更高效
		// - 代码清晰：每种数据库类型独立处理，逻辑更清晰
		// - 易于扩展：添加新的数据库类型只需添加新的 case 分支
		switch info.Type {
		case "mysql":
			// MySQL 数据库初始化
			// 为什么使用 GormMysqlByConfig 而不是直接调用 GormMysql？
			// - 配置复用：GeneralDB 结构体统一了不同数据库的通用配置
			// - 代码复用：避免为每个数据库类型重复编写相同的配置处理逻辑
			// - 类型安全：通过 config.Mysql 类型确保配置的正确性
			dbMap[info.AliasName] = GormMysqlByConfig(config.Mysql{GeneralDB: info.GeneralDB})
		case "mssql":
			// SQL Server 数据库初始化
			dbMap[info.AliasName] = GormMssqlByConfig(config.Mssql{GeneralDB: info.GeneralDB})
		case "pgsql":
			// PostgreSQL 数据库初始化
			dbMap[info.AliasName] = GormPgSqlByConfig(config.Pgsql{GeneralDB: info.GeneralDB})
		case "oracle":
			// Oracle 数据库初始化
			dbMap[info.AliasName] = GormOracleByConfig(config.Oracle{GeneralDB: info.GeneralDB})
		default:
			// 跳过不支持的数据库类型
			// 为什么不抛出错误而是静默跳过？
			// - 容错性：配置错误不会导致整个应用启动失败
			// - 灵活性：允许配置文件中存在未使用的数据库配置项
			// - 渐进式迁移：可以逐步添加新数据库类型支持，不影响现有功能
			continue
		}
	}

	// 向后兼容处理：设置系统默认数据库连接
	// 为什么需要这个特殊判断？
	// - 向后兼容：旧版本代码直接使用 global.GVA_DB，需要保持兼容性
	// - 平滑迁移：从单数据库版本迁移到多数据库版本时，现有代码无需修改
	// - 默认行为：如果没有指定数据库别名，使用系统数据库作为默认数据库
	//
	// 为什么使用 map 查找而不是直接赋值？
	// - 安全访问：使用 ok 模式检查 key 是否存在，避免空指针异常
	// - 条件判断：只有当系统数据库存在时才设置 GVA_DB，避免覆盖已有连接
	if sysDB, ok := dbMap[sys]; ok {
		global.GVA_DB = sysDB
	}

	// 将初始化完成的数据库映射表保存到全局变量
	// 为什么保存到全局变量？
	// - 全局访问：其他模块可以通过 global.GVA_DBList 访问所有数据库连接
	// - 统一管理：所有数据库连接集中管理，便于监控和维护
	// - 动态切换：支持运行时根据业务需求切换不同的数据库连接
	global.GVA_DBList = dbMap
}

func CloseDBList() {

	for _, item := range global.GVA_DBList {
		db, _ := item.DB()
		err := db.Close()
		if err != nil {
			global.GVA_LOG.Error("关闭数据库连接失败!", zap.Error(err))
		}
	}
}
