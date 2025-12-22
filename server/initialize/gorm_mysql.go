package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/initialize/internal"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// GormMysql 初始化Mysql数据库
// 使用全局配置 global.GVA_CONFIG.Mysql 进行初始化
// 设计好处：
// 1. 简化调用：无需传入参数，直接使用应用全局配置，适合单数据库场景
// 2. 配置集中管理：所有配置统一在 global.GVA_CONFIG 中管理，便于维护
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [ByteZhou-2018](https://github.com/ByteZhou-2018)
func GormMysql() *gorm.DB {
	m := global.GVA_CONFIG.Mysql
	return initMysqlDatabase(m)
}

// GormMysqlByConfig 通过传入配置初始化Mysql数据库
// 设计好处：
// 1. 灵活性：允许传入自定义配置，支持多数据库实例或动态配置场景
// 2. 可测试性：测试时可以传入测试配置，不依赖全局配置
// 3. 代码复用：与 GormMysql() 共用 initMysqlDatabase 辅助函数，避免代码重复
func GormMysqlByConfig(m config.Mysql) *gorm.DB {
	return initMysqlDatabase(m)
}

// initMysqlDatabase 初始化Mysql数据库的辅助函数
// 设计模式：提取公共逻辑到辅助函数
// 好处：
// 1. DRY原则：避免 GormMysql 和 GormMysqlByConfig 重复实现相同逻辑
// 2. 单一职责：初始化逻辑集中在一个函数，修改时只需改一处
// 3. 易于维护：后续添加日志、监控等功能只需修改此函数
func initMysqlDatabase(m config.Mysql) *gorm.DB {
	// 参数校验：数据库名为空时返回 nil，避免无效连接
	// 好处：提前失败，避免后续连接错误，提供清晰的错误信号
	if m.Dbname == "" {
		return nil
	}

	// MySQL 驱动配置
	mysqlConfig := mysql.Config{
		DSN:                       m.Dsn(), // DSN data source name，包含连接所需的所有信息（主机、端口、数据库名、用户名、密码等）
		DefaultStringSize:         191,     // string 类型字段的默认长度
		// 设置为 191 的原因：MySQL 5.7 之前，utf8mb4 字符集下索引键最大长度为 767 字节
		// 191 * 4 字节（utf8mb4 最大字符长度）= 764 字节，留有余量避免索引创建失败
		SkipInitializeWithVersion: false,   // 根据版本自动配置：让 GORM 根据 MySQL 版本自动优化配置
		// 好处：不同 MySQL 版本可能有不同的特性，自动适配可以充分利用版本特性
	}
	
	// 获取通用数据库配置（日志级别、慢查询阈值等）
	general := m.GeneralDB
	
	// 使用 GORM 打开数据库连接
	// internal.Gorm.Config(general) 提供统一的 GORM 配置（日志、命名策略等）
	// 好处：所有数据库使用相同的 GORM 配置策略，保证行为一致性
	if db, err := gorm.Open(mysql.New(mysqlConfig), internal.Gorm.Config(general)); err != nil {
		// 连接失败时 panic，因为数据库连接是应用启动的必要条件
		// 好处：快速失败，避免应用在无数据库连接的情况下运行
		panic(err)
	} else {
		// 设置表选项：指定存储引擎（如 InnoDB、MyISAM）
		// 好处：统一表结构，确保所有表使用相同的存储引擎，便于管理和优化
		db.InstanceSet("gorm:table_options", "ENGINE="+m.Engine)
		
		// 获取底层 *sql.DB 对象，用于设置连接池参数
		sqlDB, _ := db.DB()
		
		// 设置最大空闲连接数：连接池中保持的空闲连接数量
		// 好处：保持一定数量的空闲连接，减少频繁建立/关闭连接的开销，提高性能
		sqlDB.SetMaxIdleConns(m.MaxIdleConns)
		
		// 设置最大打开连接数：同时打开的最大连接数
		// 好处：限制并发连接数，防止数据库连接过多导致资源耗尽
		// 注意：需要根据数据库服务器配置和应用并发量合理设置
		sqlDB.SetMaxOpenConns(m.MaxOpenConns)
		
		return db
	}
}
