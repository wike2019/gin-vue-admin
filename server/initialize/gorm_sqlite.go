package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/initialize/internal"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// GormSqlite 初始化Sqlite数据库
// 使用全局配置 global.GVA_CONFIG.Sqlite 进行初始化
// 设计好处：
// 1. 简化调用：无需传入参数，直接使用应用全局配置，适合单数据库场景
// 2. 配置集中管理：所有配置统一在 global.GVA_CONFIG 中管理，便于维护
func GormSqlite() *gorm.DB {
	s := global.GVA_CONFIG.Sqlite
	return initSqliteDatabase(s)
}

// GormSqliteByConfig 初始化Sqlite数据库通过传入配置
// 设计好处：
// 1. 灵活性：允许传入自定义配置，支持多数据库实例或动态配置场景
// 2. 可测试性：测试时可以传入测试配置，不依赖全局配置
// 3. 代码复用：与 GormSqlite() 共用 initSqliteDatabase 辅助函数，避免代码重复
func GormSqliteByConfig(s config.Sqlite) *gorm.DB {
	return initSqliteDatabase(s)
}

// initSqliteDatabase 初始化Sqlite数据库辅助函数
// 设计模式：提取公共逻辑到辅助函数
// 好处：
// 1. DRY原则：避免 GormSqlite 和 GormSqliteByConfig 重复实现相同逻辑
// 2. 单一职责：初始化逻辑集中在一个函数，修改时只需改一处
// 3. 易于维护：后续添加日志、监控等功能只需修改此函数
func initSqliteDatabase(s config.Sqlite) *gorm.DB {
	// 参数校验：数据库名为空时返回 nil，避免无效连接
	// 好处：提前失败，避免后续连接错误，提供清晰的错误信号
	if s.Dbname == "" {
		return nil
	}

	// 获取通用数据库配置（日志级别、慢查询阈值等）
	general := s.GeneralDB
	
	// 使用 GORM 打开 SQLite 数据库连接
	// sqlite.Open() 使用 glebarez/sqlite 驱动，这是一个纯 Go 实现的 SQLite 驱动
	// 好处：无需 CGO，跨平台兼容性好，部署简单
	// internal.Gorm.Config(general) 提供统一的 GORM 配置（日志、命名策略等）
	// 好处：所有数据库使用相同的 GORM 配置策略，保证行为一致性
	if db, err := gorm.Open(sqlite.Open(s.Dsn()), internal.Gorm.Config(general)); err != nil {
		// 连接失败时 panic，因为数据库连接是应用启动的必要条件
		// 好处：快速失败，避免应用在无数据库连接的情况下运行
		panic(err)
	} else {
		// 获取底层 *sql.DB 对象，用于设置连接池参数
		sqlDB, _ := db.DB()
		
		// 设置最大空闲连接数：连接池中保持的空闲连接数量
		// 好处：保持一定数量的空闲连接，减少频繁建立/关闭连接的开销，提高性能
		// 注意：SQLite 是文件数据库，连接池的作用相对较小，但仍有助于管理并发访问
		sqlDB.SetMaxIdleConns(s.MaxIdleConns)
		
		// 设置最大打开连接数：同时打开的最大连接数
		// 好处：限制并发连接数，防止数据库文件被过多连接锁定
		// 注意：SQLite 对并发写入支持有限，合理设置此值可以避免数据库锁定问题
		sqlDB.SetMaxOpenConns(s.MaxOpenConns)
		
		return db
	}
}
