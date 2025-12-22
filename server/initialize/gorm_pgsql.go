package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/initialize/internal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// GormPgSql 初始化 Postgresql 数据库
// 使用全局配置 global.GVA_CONFIG.Pgsql 进行初始化
// 设计好处：
// 1. 简化调用：无需传入参数，直接使用应用全局配置，适合单数据库场景
// 2. 配置集中管理：所有配置统一在 global.GVA_CONFIG 中管理，便于维护
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
func GormPgSql() *gorm.DB {
	p := global.GVA_CONFIG.Pgsql
	return initPgSqlDatabase(p)
}

// GormPgSqlByConfig 初始化 Postgresql 数据库 通过指定参数
// 设计好处：
// 1. 灵活性：允许传入自定义配置，支持多数据库实例或动态配置场景
// 2. 可测试性：测试时可以传入测试配置，不依赖全局配置
// 3. 代码复用：与 GormPgSql() 共用 initPgSqlDatabase 辅助函数，避免代码重复
func GormPgSqlByConfig(p config.Pgsql) *gorm.DB {
	return initPgSqlDatabase(p)
}

// initPgSqlDatabase 初始化 Postgresql 数据库的辅助函数
// 设计模式：提取公共逻辑到辅助函数
// 好处：
// 1. DRY原则：避免 GormPgSql 和 GormPgSqlByConfig 重复实现相同逻辑
// 2. 单一职责：初始化逻辑集中在一个函数，修改时只需改一处
// 3. 易于维护：后续添加日志、监控等功能只需修改此函数
func initPgSqlDatabase(p config.Pgsql) *gorm.DB {
	// 参数校验：数据库名为空时返回 nil，避免无效连接
	// 好处：提前失败，避免后续连接错误，提供清晰的错误信号
	if p.Dbname == "" {
		return nil
	}
	
	// PostgreSQL 驱动配置
	pgsqlConfig := postgres.Config{
		DSN:                  p.Dsn(), // DSN data source name，包含连接所需的所有信息（主机、端口、数据库名、用户名、密码等）
		PreferSimpleProtocol: false,   // 不使用简单协议，使用标准协议
		// 设置为 false 的好处：
		// 1. 更好的功能支持：标准协议支持更多 PostgreSQL 特性（如数组、JSON 等）
		// 2. 更好的性能：对于复杂查询，标准协议通常性能更好
		// 3. 更好的兼容性：与更多 PostgreSQL 版本和特性兼容
	}
	
	// 获取通用数据库配置（日志级别、慢查询阈值等）
	general := p.GeneralDB
	
	// 使用 GORM 打开 PostgreSQL 数据库连接
	// internal.Gorm.Config(general) 提供统一的 GORM 配置（日志、命名策略等）
	// 好处：所有数据库使用相同的 GORM 配置策略，保证行为一致性
	if db, err := gorm.Open(postgres.New(pgsqlConfig), internal.Gorm.Config(general)); err != nil {
		// 连接失败时 panic，因为数据库连接是应用启动的必要条件
		// 好处：快速失败，避免应用在无数据库连接的情况下运行
		panic(err)
	} else {
		// 获取底层 *sql.DB 对象，用于设置连接池参数
		sqlDB, _ := db.DB()
		
		// 设置最大空闲连接数：连接池中保持的空闲连接数量
		// 好处：保持一定数量的空闲连接，减少频繁建立/关闭连接的开销，提高性能
		// 注意：PostgreSQL 连接建立有一定成本，合理设置空闲连接数可以提升性能
		sqlDB.SetMaxIdleConns(p.MaxIdleConns)
		
		// 设置最大打开连接数：同时打开的最大连接数
		// 好处：限制并发连接数，防止数据库连接过多导致资源耗尽
		// 注意：需要根据 PostgreSQL 数据库服务器配置（max_connections）和应用并发量合理设置
		sqlDB.SetMaxOpenConns(p.MaxOpenConns)
		
		return db
	}
}
