package initialize

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// initRedisClient 根据配置创建并初始化 Redis 客户端
// 设计意义：
// 1. 统一封装：将 Redis 客户端的创建逻辑封装在一个函数中，避免代码重复
// 2. 模式切换：通过配置灵活支持单例模式和集群模式，无需修改代码
// 3. 连接验证：创建后立即进行 Ping 测试，确保连接可用，避免运行时才发现连接问题
// 4. 接口统一：返回 redis.UniversalClient 接口类型，可以同时兼容单例和集群客户端
//
// 参数：
//   - redisCfg: Redis 配置信息，包含地址、密码、数据库编号等
//
// 返回值：
//   - redis.UniversalClient: Redis 客户端实例（接口类型，兼容单例和集群）
//   - error: 连接失败时返回错误
func initRedisClient(redisCfg config.Redis) (redis.UniversalClient, error) {
	var client redis.UniversalClient

	// 根据配置选择 Redis 部署模式
	// 为什么使用条件判断而不是统一接口？
	// - 单例模式和集群模式的配置参数不同（单例用 Addr+DB，集群用 ClusterAddrs）
	// - 集群模式下不需要指定 DB（集群不支持多数据库）
	// - 通过配置驱动，可以在不同环境（开发/生产）使用不同的部署方式
	if redisCfg.UseCluster {
		// 集群模式：适用于高可用、高并发的生产环境
		// 好处：
		// 1. 高可用性：单个节点故障不影响整体服务
		// 2. 水平扩展：可以动态增加节点提升性能
		// 3. 数据分片：自动将数据分布到多个节点，提升存储容量
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    redisCfg.ClusterAddrs, // 集群节点地址列表，客户端会自动发现所有节点
			Password: redisCfg.Password,     // 集群密码（所有节点使用相同密码）
		})
	} else {
		// 单例模式：适用于开发、测试或小规模生产环境
		// 好处：
		// 1. 简单易用：配置简单，适合快速开发
		// 2. 资源占用少：单个实例，资源消耗低
		// 3. 支持多数据库：可以通过 DB 参数选择不同的逻辑数据库（0-15）
		client = redis.NewClient(&redis.Options{
			Addr:     redisCfg.Addr,     // 单例模式下的服务器地址:端口
			Password: redisCfg.Password, // Redis 密码（如果设置了的话）
			DB:       redisCfg.DB,       // 数据库编号，用于逻辑隔离（0-15）
		})
	}

	// 创建客户端后立即进行连接测试
	// 为什么要在初始化时 Ping？
	// 1. Fail-Fast 原则：启动时发现问题比运行时发现更好，避免服务启动后才发现 Redis 不可用
	// 2. 配置验证：确保配置的地址、密码等信息正确
	// 3. 网络检查：验证网络连通性，避免后续操作时才发现网络问题
	// 4. 日志记录：记录连接状态，便于运维监控和问题排查
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		// 连接失败时记录详细的错误信息，包括 Redis 实例名称和具体错误
		// 使用结构化日志（zap）便于日志收集和分析
		global.GVA_LOG.Error("redis connect ping failed, err:", zap.String("name", redisCfg.Name), zap.Error(err))
		return nil, err
	}

	// 连接成功时记录日志，确认连接状态
	// 包含实例名称和 Ping 响应，便于确认是哪个 Redis 实例连接成功
	global.GVA_LOG.Info("redis connect ping response:", zap.String("name", redisCfg.Name), zap.String("pong", pong))
	return client, nil
}

// Redis 初始化默认的单个 Redis 客户端
// 设计意义：
// 1. 简化使用：大多数场景只需要一个 Redis 实例，提供简单的初始化函数
// 2. 全局访问：将客户端存储到 global.GVA_REDIS，方便全局访问
// 3. 启动保障：使用 panic 确保 Redis 必须可用，避免服务启动后才发现问题
//
// 使用场景：
// - 缓存数据（如 JWT 黑名单、验证码等）
// - 会话存储
// - 分布式锁
// - 消息队列
//
// 为什么使用 panic 而不是返回错误？
// - Redis 通常是核心依赖，如果不可用，服务无法正常运行
// - 启动时失败比运行时失败更容易发现和修复
// - 符合 Go 的 Fail-Fast 设计哲学
func Redis() {
	redisClient, err := initRedisClient(global.GVA_CONFIG.Redis)
	if err != nil {
		// 初始化失败直接 panic，确保问题在启动时被发现
		// 这比在运行时才发现 Redis 不可用要好得多
		panic(err)
	}
	// 将客户端存储到全局变量，供其他模块使用
	global.GVA_REDIS = redisClient
}

// RedisList 初始化多个 Redis 客户端列表
// 设计意义：
// 1. 多实例支持：支持同时连接多个 Redis 实例，每个实例有独立的名称
// 2. 职责分离：不同的 Redis 实例可以用于不同的业务场景（缓存、会话、消息队列等）
// 3. 灵活配置：通过配置可以动态添加或移除 Redis 实例，无需修改代码
//
// 使用场景：
// - 多点登录：需要为每个用户存储多个设备的登录信息
// - 多租户系统：不同租户使用不同的 Redis 实例
// - 读写分离：读操作和写操作使用不同的 Redis 实例
// - 业务隔离：不同业务模块使用独立的 Redis 实例，避免相互影响
//
// 为什么使用 map 存储？
// - 通过名称快速查找对应的 Redis 客户端
// - 支持动态添加和删除 Redis 实例
// - 代码中使用 global.GetRedis(name) 可以方便地获取指定名称的客户端
//
// 为什么每个实例初始化失败都 panic？
// - 如果配置了 Redis 列表，说明这些实例都是必需的
// - 启动时确保所有配置的 Redis 实例都可用，避免运行时错误
func RedisList() {
	// 使用 map 存储多个 Redis 客户端，key 为配置中的 name，value 为客户端实例
	// 好处：通过名称快速查找，支持动态管理多个 Redis 连接
	redisMap := make(map[string]redis.UniversalClient)

	// 遍历配置中的所有 Redis 实例配置
	// 为什么使用循环而不是单独初始化？
	// - 配置驱动的设计，可以动态添加多个 Redis 实例
	// - 代码简洁，避免为每个实例写重复的初始化代码
	// - 易于扩展，新增 Redis 实例只需修改配置文件
	for _, redisCfg := range global.GVA_CONFIG.RedisList {
		client, err := initRedisClient(redisCfg)
		if err != nil {
			// 任何一个 Redis 实例初始化失败都会导致服务启动失败
			// 这确保了所有配置的 Redis 实例在服务启动时都是可用的
			panic(err)
		}
		// 使用配置中的 name 作为 key，便于后续通过名称查找对应的客户端
		// 注意：name 必须在配置中唯一，否则会覆盖之前的客户端
		redisMap[redisCfg.Name] = client
	}

	// 将初始化好的 Redis 客户端映射存储到全局变量
	// 其他模块可以通过 global.GetRedis(name) 获取指定名称的客户端
	global.GVA_REDISList = redisMap
}
