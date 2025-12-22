// Package global 提供全局变量管理，用于存储和访问应用的核心组件。
// 
// 设计思想：
// 1. 单例模式：通过包级别的全局变量，确保整个应用共享同一份核心资源（数据库连接、Redis连接等）
// 2. 集中管理：将所有全局状态集中在一个包中，便于统一管理和维护
// 3. 线程安全：使用读写锁保护共享资源的并发访问，避免数据竞争
//
// 为什么使用全局变量而不是依赖注入：
// - 简化访问：任何地方都可以通过 global.GVA_DB 直接访问，无需层层传递
// - 减少参数传递：避免在每个函数签名中传递大量依赖参数
// - 初始化时机：应用启动时初始化一次，后续直接使用，符合数据库连接池等资源的使用模式
//
// 注意事项：
// - 全局变量必须在应用启动时正确初始化，否则会导致运行时错误
// - 并发访问需要使用提供的线程安全方法，或使用 lock 保护
package global

import (
	"fmt"
	"github.com/mark3labs/mcp-go/server"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/qmgo"

	"github.com/flipped-aurora/gin-vue-admin/server/utils/timer"
	"github.com/songzhibin97/gkit/cache/local_cache"

	"golang.org/x/sync/singleflight"

	"go.uber.org/zap"

	"github.com/flipped-aurora/gin-vue-admin/server/config"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

var (
	// GVA_DB 默认的数据库连接实例。
	// 使用 GORM 作为 ORM 框架，提供数据库操作能力。
	// 全局单例设计可以避免重复创建连接，复用连接池资源，提高性能。
	GVA_DB *gorm.DB

	// GVA_DBList 多数据库连接映射表，key 为数据库名称，value 为对应的数据库连接。
	// 支持多数据库场景：主从分离、分库分表、不同业务使用不同数据库等。
	// 使用 map 结构便于按名称快速查找对应的数据库连接。
	// 注意：访问此 map 需要使用 lock 保护，或使用提供的 GetGlobalDBByDBName 等方法。
	GVA_DBList map[string]*gorm.DB

	// GVA_REDIS 默认的 Redis 客户端实例。
	// Redis 作为缓存和会话存储，全局单例可以复用连接，减少连接开销。
	// UniversalClient 接口支持单机、哨兵、集群等多种部署模式。
	GVA_REDIS redis.UniversalClient

	// GVA_REDISList 多 Redis 连接映射表，key 为 Redis 名称，value 为对应的 Redis 客户端。
	// 支持多 Redis 实例场景：读写分离、不同业务使用不同 Redis、缓存和会话存储分离等。
	// 通过名称区分不同的 Redis 实例，实现灵活的配置和管理。
	GVA_REDISList map[string]redis.UniversalClient

	// GVA_MONGO MongoDB 客户端实例。
	// 使用 Qmgo（Go 的 MongoDB 驱动），提供文档数据库操作能力。
	// 支持需要非关系型数据存储的场景。
	GVA_MONGO *qmgo.QmgoClient

	// GVA_CONFIG 应用配置信息。
	// 在应用启动时从配置文件加载，存储数据库、Redis、服务器等所有配置。
	// 全局访问配置信息，避免在多个地方重复读取配置文件。
	GVA_CONFIG config.Server

	// GVA_VP Viper 配置管理器的实例。
	// Viper 支持多种配置格式（YAML、JSON、TOML等）和环境变量。
	// 全局变量便于在运行时动态读取和监听配置变化。
	GVA_VP *viper.Viper

	// GVA_LOG 日志记录器实例，使用 zap 高性能日志库。
	// zap 是 Uber 开源的日志库，性能优异，适合高并发场景。
	// 全局单例确保整个应用使用统一的日志格式和配置，便于日志管理和分析。
	// 注意：已废弃旧的 oplogging.Logger，改用 zap.Logger
	GVA_LOG *zap.Logger

	// GVA_Timer 定时任务管理器，用于执行周期性任务。
	// 使用接口类型 timer.Timer，便于替换不同的实现。
	// 初始化时创建新的定时任务实例，支持任务的添加、删除、暂停、恢复等操作。
	// 常见用途：定时清理、数据同步、统计报表生成等。
	GVA_Timer timer.Timer = timer.NewTimerTask()

	// GVA_Concurrency_Control 并发控制组，使用 singleflight 防止缓存击穿。
	// singleflight 确保对于相同的 key，同时只有一个请求执行，其他请求等待结果。
	// 好处：
	// 1. 防止缓存击穿：多个请求同时查询不存在的 key 时，只执行一次数据库查询
	// 2. 减少重复计算：相同的计算任务只执行一次
	// 3. 降低后端压力：避免大量并发请求打到数据库或外部服务
	GVA_Concurrency_Control = &singleflight.Group{}

	// GVA_ROUTERS 存储所有注册的路由信息。
	// 用于路由查询、调试、API 文档生成等场景。
	// gin.RoutesInfo 提供了路由的方法、路径、处理器等信息。
	GVA_ROUTERS gin.RoutesInfo

	// GVA_ACTIVE_DBNAME 当前活跃的数据库名称。
	// 使用指针类型 *string，nil 表示未设置或使用默认数据库。
	// 在多数据库场景下，标识当前操作使用的是哪个数据库。
	GVA_ACTIVE_DBNAME *string

	// GVA_MCP_SERVER MCP (Model Context Protocol) 服务器实例。
	// MCP 是用于 AI 模型上下文管理的协议，提供模型相关的服务能力。
	GVA_MCP_SERVER *server.MCPServer

	// BlackCache 黑名单缓存，使用本地内存缓存。
	// 常用于存储需要快速访问的黑名单数据，如：JWT token 黑名单、IP 黑名单等。
	// 本地缓存相比 Redis 访问更快，但仅限单机使用，适合不要求分布式一致性的场景。
	BlackCache local_cache.Cache

	// lock 读写锁，用于保护 GVA_DBList 等共享资源的并发访问。
	// 使用 RWMutex 而非 Mutex 的好处：
	// 1. 允许多个 goroutine 同时读取（RLock），提高并发读性能
	// 2. 写操作（Lock）时独占，保证数据一致性
	// 3. 适合读多写少的场景，数据库连接列表通常是初始化后只读
	lock sync.RWMutex
)

// GetGlobalDBByDBName 通过名称从数据库列表中获取对应的数据库连接。
//
// 参数：
//   - dbname: 数据库名称，作为 GVA_DBList 的 key
//
// 返回值：
//   - *gorm.DB: 对应的数据库连接实例，如果不存在则返回 nil
//
// 设计说明：
// 1. 使用 RLock（读锁）保护并发访问，允许多个 goroutine 同时读取
// 2. 使用 defer 确保锁一定会被释放，避免死锁
// 3. 返回 nil 而不是 panic，让调用方决定如何处理不存在的数据库
// 4. 适用于需要可选数据库访问的场景，调用方需要检查返回值是否为 nil
func GetGlobalDBByDBName(dbname string) *gorm.DB {
	lock.RLock()
	defer lock.RUnlock()
	return GVA_DBList[dbname]
}

// MustGetGlobalDBByDBName 通过名称获取数据库连接，如果不存在则触发 panic。
//
// 参数：
//   - dbname: 数据库名称，作为 GVA_DBList 的 key
//
// 返回值：
//   - *gorm.DB: 对应的数据库连接实例，保证非 nil
//
// 设计说明：
// 1. "Must" 前缀表示此函数在失败时会 panic，这是 Go 的命名约定
// 2. 使用 panic 快速失败，适用于数据库连接必须存在的场景
// 3. 如果数据库未初始化，立即暴露问题，避免后续更隐蔽的错误
// 4. 适合在应用启动检查、关键路径中使用，确保数据库连接可用
// 5. 与 GetGlobalDBByDBName 的区别：Must 版本保证非 nil，普通版本可能返回 nil
func MustGetGlobalDBByDBName(dbname string) *gorm.DB {
	lock.RLock()
	defer lock.RUnlock()
	db, ok := GVA_DBList[dbname]
	if !ok || db == nil {
		panic("db no init")
	}
	return db
}

// GetRedis 通过名称从 Redis 列表中获取对应的 Redis 客户端实例。
//
// 参数：
//   - name: Redis 实例名称，作为 GVA_REDISList 的 key
//
// 返回值：
//   - redis.UniversalClient: 对应的 Redis 客户端实例，保证非 nil
//
// 设计说明：
// 1. 与 MustGetGlobalDBByDBName 类似，使用 panic 确保 Redis 连接存在
// 2. Redis 通常是关键依赖（缓存、会话存储），未初始化应该立即暴露问题
// 3. 使用 fmt.Sprintf 提供详细的错误信息，包含 Redis 名称，便于调试
// 4. 注意：函数内部变量名 redis 会遮蔽包名，但在此函数作用域内不影响使用
// 5. 不检查 GVA_REDISList 是否为 nil，假设在调用前已正确初始化
func GetRedis(name string) redis.UniversalClient {
	redis, ok := GVA_REDISList[name]
	if !ok || redis == nil {
		panic(fmt.Sprintf("redis `%s` no init", name))
	}
	return redis
}
