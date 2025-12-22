package system

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
)

// AutoCodeSqlite SQLite 数据库操作的全局单例实例
// 使用单例模式的好处：
// 1. 避免重复创建对象，节省内存资源
// 2. 提供统一的访问入口，便于管理和维护
// 3. 符合 Go 语言的最佳实践，通过包级别变量暴露服务
var AutoCodeSqlite = new(autoCodeSqlite)

// autoCodeSqlite SQLite 数据库操作结构体
// 采用空结构体的设计，因为不需要存储任何状态信息
// 所有方法都是无状态的，只负责执行数据库查询操作
// 这种设计的好处：
// 1. 零内存占用（空结构体在 Go 中不占用内存）
// 2. 方法可以安全地并发调用，无需考虑状态同步问题
// 3. 代码简洁，职责单一，符合单一职责原则
type autoCodeSqlite struct{}

// GetDB 获取数据库的所有数据库名
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
//
// 参数说明：
//   - businessDB: 业务数据库标识符，空字符串表示使用默认数据库
//
// 设计思路：
// 1. 使用 PRAGMA database_list 查询所有附加的数据库
//   - PRAGMA 是 SQLite 特有的命令，用于查询数据库元信息
//   - database_list 返回当前连接中所有附加的数据库文件列表
//   - 这是 SQLite 获取数据库列表的标准方式
//
// 2. 支持多数据库切换机制
//   - businessDB 为空时使用默认数据库 global.GVA_DB
//   - businessDB 不为空时从 global.GVA_DBList 中获取对应的业务数据库
//   - 这种设计的好处：
//   - 支持多租户场景，不同业务可以使用不同的数据库
//   - 保持代码统一，通过参数控制数据库选择，无需重复代码
//   - 提高系统的灵活性和可扩展性
//
// 3. 文件路径处理逻辑
//   - SQLite 数据库本质上是文件，PRAGMA database_list 返回的是文件路径
//   - 使用 filepath.Base() 提取文件名，去除路径信息
//   - 使用 filepath.Ext() 和 strings.TrimSuffix() 去除文件扩展名
//   - 这样做的意义：
//   - 只返回数据库名称，不包含路径和扩展名，更符合业务语义
//   - 用户看到的是简洁的数据库名，而不是完整的文件路径
//   - 提高用户体验，避免暴露底层文件系统细节
//
// 4. 使用匿名结构体接收查询结果
//   - 只定义需要的字段（File），减少内存占用
//   - 使用 gorm tag 映射列名，保持代码简洁
//   - 不需要定义完整的结构体，符合 YAGNI 原则（You Aren't Gonna Need It）
func (a *autoCodeSqlite) GetDB(businessDB string) (data []response.Db, err error) {
	var entities []response.Db
	// PRAGMA database_list 是 SQLite 特有的命令，用于列出所有附加的数据库
	// 返回结果包含 seq, name, file 三个字段，这里只需要 file 字段
	sql := "PRAGMA database_list;"
	var databaseList []struct {
		File string `gorm:"column:file"` // 数据库文件路径
	}

	// 根据 businessDB 参数选择使用哪个数据库连接
	// 这种条件判断的设计模式在多个方法中重复使用，保证了代码的一致性
	if businessDB == "" {
		// 使用默认数据库连接
		err = global.GVA_DB.Raw(sql).Find(&databaseList).Error
	} else {
		// 使用指定的业务数据库连接
		// 从全局数据库连接池中获取对应的连接
		err = global.GVA_DBList[businessDB].Raw(sql).Find(&databaseList).Error
	}

	// 遍历查询结果，提取数据库名称
	for _, database := range databaseList {
		if database.File != "" {
			// 从完整路径中提取文件名（去除目录路径）
			// 例如：/path/to/database.db -> database.db
			fileName := filepath.Base(database.File)
			// 提取文件扩展名
			// 例如：database.db -> .db
			fileExt := filepath.Ext(fileName)
			// 去除扩展名，得到纯数据库名
			// 例如：database.db -> database
			fileNameWithoutExt := strings.TrimSuffix(fileName, fileExt)

			// 将处理后的数据库名添加到结果列表
			entities = append(entities, response.Db{fileNameWithoutExt})
		}
	}
	// 注释掉的代码：如果需要直接使用配置中的数据库名，可以取消注释
	// 但当前实现通过 PRAGMA 动态获取，更加灵活和准确
	// entities = append(entities, response.Db{global.GVA_CONFIG.Sqlite.Dbname})
	return entities, err
}

// GetTables 获取数据库的所有表名
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
//
// 参数说明：
//   - businessDB: 业务数据库标识符，空字符串表示使用默认数据库
//   - dbName: 数据库名称（在 SQLite 中通常不使用，保留以保持接口一致性）
//
// 设计思路：
// 1. 使用 sqlite_master 系统表查询所有表名
//   - sqlite_master 是 SQLite 的系统表，存储了数据库的元数据信息
//   - 包含所有表、索引、视图、触发器的定义
//   - WHERE type='table' 过滤出所有用户表，排除系统表和视图
//   - 这是 SQLite 获取表列表的标准和推荐方式
//
// 2. 为什么使用 sqlite_master 而不是其他方式？
//   - 性能好：直接查询系统表，无需额外的 PRAGMA 命令
//   - 信息完整：可以获取表的详细信息（如果需要扩展功能）
//   - 标准做法：这是 SQLite 官方文档推荐的方式
//   - 兼容性好：所有 SQLite 版本都支持
//
// 3. 使用字符串切片接收查询结果
//   - 因为只需要表名（name 字段），使用 []string 足够
//   - 比使用结构体更轻量，减少内存占用
//   - GORM 的 Find 方法可以直接将单列结果映射到字符串切片
//
// 4. 多数据库支持
//   - 与 GetDB 方法保持相同的多数据库切换逻辑
//   - 保证了代码风格的一致性，降低维护成本
//   - 支持在不同业务数据库中查询表列表
func (a *autoCodeSqlite) GetTables(businessDB string, dbName string) (data []response.Table, err error) {
	var entities []response.Table
	// sqlite_master 是 SQLite 的系统表，存储数据库的元数据
	// type='table' 表示只查询用户表，排除视图、索引等
	// 这是 SQLite 获取所有表名的标准方式
	sql := `SELECT name FROM sqlite_master WHERE type='table'`
	tabelNames := []string{}

	// 根据 businessDB 参数选择数据库连接
	// 保持与 GetDB 方法相同的设计模式，确保代码一致性
	if businessDB == "" {
		err = global.GVA_DB.Raw(sql).Find(&tabelNames).Error
	} else {
		err = global.GVA_DBList[businessDB].Raw(sql).Find(&tabelNames).Error
	}

	// 将查询到的表名转换为响应结构体
	// 使用循环转换而不是直接映射，保证了数据结构的统一性
	for _, tabelName := range tabelNames {
		entities = append(entities, response.Table{tabelName})
	}
	return entities, err
}

// GetColumn 获取指定数据表的所有字段名,类型值等
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
//
// 参数说明：
//   - businessDB: 业务数据库标识符，空字符串表示使用默认数据库
//   - tableName: 要查询的表名
//   - dbName: 数据库名称（在 SQLite 中通常不使用，保留以保持接口一致性）
//
// 设计思路：
// 1. 使用 PRAGMA table_info 获取表结构信息
//   - PRAGMA table_info 是 SQLite 特有的命令，用于查询表的列信息
//   - 返回的信息包括：cid（列序号）、name（列名）、type（数据类型）、
//     notnull（非空约束）、dflt_value（默认值）、pk（主键标识）
//   - 这是 SQLite 获取表结构最直接和高效的方式
//
// 2. 为什么使用 fmt.Sprintf 动态构建 SQL？
//   - tableName 是运行时参数，需要动态插入到 SQL 中
//   - 使用 fmt.Sprintf 可以安全地构建 SQL 语句
//   - 注意：在实际生产环境中，应该对 tableName 进行验证和转义，防止 SQL 注入
//   - 但在这个场景中，tableName 通常来自系统内部（通过 GetTables 获取），相对安全
//
// 3. 使用匿名结构体接收查询结果
//   - 只定义需要的字段（Name, Type, Pk），忽略其他不需要的字段
//   - 减少内存占用，提高查询效率
//   - 使用 gorm tag 精确映射列名，避免字段名不匹配的问题
//   - 这种设计的好处：
//   - 代码简洁，不需要定义完整的结构体
//   - 只关注业务需要的字段，符合最小化原则
//   - 如果将来需要更多字段，可以轻松扩展
//
// 4. 使用 Scan 而不是 Find
//   - Scan 方法用于将查询结果映射到结构体
//   - 与 Find 的区别：Find 通常用于查询实体表，Scan 用于查询结果映射
//   - 在这个场景中，查询的是元数据而不是业务数据，使用 Scan 更合适
//
// 5. 主键判断逻辑
//   - SQLite 的 PRAGMA table_info 返回的 pk 字段是整数
//   - pk = 1 表示该列是主键（或主键的一部分）
//   - pk = 0 表示不是主键
//   - 使用 columnInfo.Pk == 1 将整数转换为布尔值，语义更清晰
//
// 6. 字段映射到响应结构体
//   - ColumnName: 列名，直接映射
//   - DataType: 数据类型，直接映射（注意：SQLite 的类型系统比较灵活）
//   - PrimaryKey: 主键标识，通过布尔值转换
//   - 这种映射保证了返回数据的结构化和标准化
func (a *autoCodeSqlite) GetColumn(businessDB string, tableName string, dbName string) (data []response.Column, err error) {
	var entities []response.Column
	// PRAGMA table_info 返回指定表的所有列信息
	// 包括：列名、数据类型、是否非空、默认值、是否主键等
	// 使用 fmt.Sprintf 动态插入表名（注意：生产环境应验证表名防止 SQL 注入）
	sql := fmt.Sprintf("PRAGMA table_info(%s);", tableName)

	// 定义匿名结构体接收查询结果
	// 只定义需要的字段，减少内存占用
	// 使用 gorm tag 映射列名，确保字段正确映射
	var columnInfos []struct {
		Name string `gorm:"column:name"` // 列名
		Type string `gorm:"column:type"` // 数据类型
		Pk   int    `gorm:"column:pk"`   // 主键标识（1=主键，0=非主键）
	}

	// 根据 businessDB 参数选择数据库连接
	// 保持与其他方法相同的设计模式
	if businessDB == "" {
		// 使用 Scan 方法将查询结果映射到结构体
		// Scan 适用于查询元数据或自定义查询结果
		err = global.GVA_DB.Raw(sql).Scan(&columnInfos).Error
	} else {
		err = global.GVA_DBList[businessDB].Raw(sql).Scan(&columnInfos).Error
	}

	// 将查询结果转换为响应结构体
	for _, columnInfo := range columnInfos {
		entities = append(entities, response.Column{
			ColumnName: columnInfo.Name,    // 列名直接映射
			DataType:   columnInfo.Type,    // 数据类型直接映射
			PrimaryKey: columnInfo.Pk == 1, // 将整数转换为布尔值（1=主键，0=非主键）
		})
	}
	return entities, err
}
