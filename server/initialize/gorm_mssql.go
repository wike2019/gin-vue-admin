package initialize

/*
 * @Author: 逆光飞翔 191180776@qq.com
 * @Date: 2022-12-08 17:25:49
 * @LastEditors: 逆光飞翔 191180776@qq.com
 * @LastEditTime: 2022-12-08 18:00:00
 * @FilePath: \server\initialize\gorm_mssql.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

import (
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/initialize/internal"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

// GormMssql 初始化Mssql数据库
// 使用全局配置 global.GVA_CONFIG.Mssql 进行初始化
// 设计好处：
// 1. 简化调用：无需传入参数，直接使用应用全局配置，适合单数据库场景
// 2. 配置集中管理：所有配置统一在 global.GVA_CONFIG 中管理，便于维护
// Author [LouisZhang](191180776@qq.com)
func GormMssql() *gorm.DB {
	m := global.GVA_CONFIG.Mssql
	
	// 参数校验：数据库名为空时返回 nil，避免无效连接
	// 好处：提前失败，避免后续连接错误，提供清晰的错误信号
	if m.Dbname == "" {
		return nil
	}
	
	// SQL Server 驱动配置
	mssqlConfig := sqlserver.Config{
		DSN:               m.Dsn(), // DSN data source name，包含连接所需的所有信息（主机、端口、数据库名、用户名、密码等）
		DefaultStringSize: 191,     // string 类型字段的默认长度
		// 设置为 191 的原因：与 MySQL 保持一致，避免字符串字段过长导致索引问题
		// 191 是一个常用的默认值，适合大多数业务场景
	}
	
	// 获取通用数据库配置（日志级别、慢查询阈值等）
	general := m.GeneralDB
	
	// 使用 GORM 打开 SQL Server 数据库连接
	// internal.Gorm.Config(general) 提供统一的 GORM 配置（日志、命名策略等）
	// 好处：所有数据库使用相同的 GORM 配置策略，保证行为一致性
	// 注意：连接失败时返回 nil 而不是 panic，这是与 GormMssqlByConfig 的区别
	// 好处：允许调用者检查返回值，决定是否继续执行（适合可选数据库场景）
	if db, err := gorm.Open(sqlserver.New(mssqlConfig), internal.Gorm.Config(general)); err != nil {
		return nil
	} else {
		// 设置表选项：指定存储引擎（SQL Server 中对应的是不同的存储格式或索引类型）
		// 好处：统一表结构，确保所有表使用相同的存储配置，便于管理和优化
		db.InstanceSet("gorm:table_options", "ENGINE="+m.Engine)
		
		// 获取底层 *sql.DB 对象，用于设置连接池参数
		sqlDB, _ := db.DB()
		
		// 设置最大空闲连接数：连接池中保持的空闲连接数量
		// 好处：保持一定数量的空闲连接，减少频繁建立/关闭连接的开销，提高性能
		// 注意：SQL Server 连接建立有一定成本，合理设置空闲连接数可以提升性能
		sqlDB.SetMaxIdleConns(m.MaxIdleConns)
		
		// 设置最大打开连接数：同时打开的最大连接数
		// 好处：限制并发连接数，防止数据库连接过多导致资源耗尽
		// 注意：需要根据 SQL Server 数据库服务器配置和应用并发量合理设置
		sqlDB.SetMaxOpenConns(m.MaxOpenConns)
		
		return db
	}
}

// GormMssqlByConfig 初始化Mssql数据库通过传入配置
// 设计好处：
// 1. 灵活性：允许传入自定义配置，支持多数据库实例或动态配置场景
// 2. 可测试性：测试时可以传入测试配置，不依赖全局配置
// 3. 错误处理：连接失败时 panic，表明这是必需配置，必须成功
// 注意：与 GormMssql() 的区别在于错误处理方式（panic vs return nil）
// 好处：明确表达配置的必需性，快速失败原则
func GormMssqlByConfig(m config.Mssql) *gorm.DB {
	// 参数校验：数据库名为空时返回 nil，避免无效连接
	// 好处：提前失败，避免后续连接错误，提供清晰的错误信号
	if m.Dbname == "" {
		return nil
	}
	
	// SQL Server 驱动配置
	mssqlConfig := sqlserver.Config{
		DSN:               m.Dsn(), // DSN data source name，包含连接所需的所有信息（主机、端口、数据库名、用户名、密码等）
		DefaultStringSize: 191,     // string 类型字段的默认长度
		// 设置为 191 的原因：与 MySQL 保持一致，避免字符串字段过长导致索引问题
		// 191 是一个常用的默认值，适合大多数业务场景
	}
	
	// 获取通用数据库配置（日志级别、慢查询阈值等）
	general := m.GeneralDB
	
	// 使用 GORM 打开 SQL Server 数据库连接
	// internal.Gorm.Config(general) 提供统一的 GORM 配置（日志、命名策略等）
	// 好处：所有数据库使用相同的 GORM 配置策略，保证行为一致性
	// 注意：连接失败时 panic，因为数据库连接是应用启动的必要条件
	// 好处：快速失败，避免应用在无数据库连接的情况下运行
	if db, err := gorm.Open(sqlserver.New(mssqlConfig), internal.Gorm.Config(general)); err != nil {
		panic(err)
	} else {
		// 设置表选项：硬编码为 InnoDB（这里可能是历史遗留，SQL Server 不使用 InnoDB）
		// 注意：SQL Server 不使用 MySQL 的存储引擎概念，此设置可能无效或用于兼容性
		// 建议：可以考虑使用配置中的 m.Engine 而不是硬编码
		db.InstanceSet("gorm:table_options", "ENGINE=InnoDB")
		
		// 获取底层 *sql.DB 对象，用于设置连接池参数
		sqlDB, _ := db.DB()
		
		// 设置最大空闲连接数：连接池中保持的空闲连接数量
		// 好处：保持一定数量的空闲连接，减少频繁建立/关闭连接的开销，提高性能
		sqlDB.SetMaxIdleConns(m.MaxIdleConns)
		
		// 设置最大打开连接数：同时打开的最大连接数
		// 好处：限制并发连接数，防止数据库连接过多导致资源耗尽
		sqlDB.SetMaxOpenConns(m.MaxOpenConns)
		
		return db
	}
}
