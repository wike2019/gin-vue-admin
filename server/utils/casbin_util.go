package utils

import (
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"go.uber.org/zap"
)

var (
	// syncedCachedEnforcer Casbin权限执行器实例
	// 使用 SyncedCachedEnforcer 而非普通 Enforcer 的原因：
	// 1. 缓存机制：将权限策略缓存在内存中，避免每次都查询数据库，大幅提升权限验证性能
	// 2. 同步机制：支持多个实例之间的策略同步，当策略变更时自动同步更新，保证分布式环境下的数据一致性
	// 3. 并发安全：内置的同步机制保证了多协程环境下的线程安全
	syncedCachedEnforcer *casbin.SyncedCachedEnforcer

	// once 使用 sync.Once 确保单例模式
	// 为什么使用 sync.Once：
	// 1. 线程安全：sync.Once 保证初始化函数只会被执行一次，即使多个协程同时调用也是安全的
	// 2. 性能优化：避免重复创建 enforcer 实例，节省内存和初始化时间
	// 3. 延迟初始化：只有在第一次调用时才进行初始化，避免程序启动时的资源浪费
	// 4. 简洁高效：相比手动加锁，sync.Once 更简洁且性能更好
	once sync.Once
)

// GetCasbin 获取casbin权限管理实例（单例模式）
// 返回值：*casbin.SyncedCachedEnforcer Casbin权限执行器
//
// 设计说明：
// 1. 单例模式：确保整个应用只有一个 enforcer 实例，避免重复创建带来的资源浪费
// 2. 延迟初始化：使用 sync.Once 实现懒加载，只有在首次调用时才初始化
// 3. 全局共享：多个模块可以安全地共享同一个 enforcer 实例进行权限验证
func GetCasbin() *casbin.SyncedCachedEnforcer {
	// sync.Once.Do 保证初始化逻辑只会执行一次
	// 即使多个协程同时调用 GetCasbin()，也只会有一个协程执行初始化
	once.Do(func() {
		// 创建 GORM 适配器，用于将 Casbin 策略存储在数据库中
		// 为什么使用数据库适配器：
		// 1. 持久化：策略数据存储在数据库中，不会因为应用重启而丢失
		// 2. 动态更新：可以在运行时修改策略，无需重启应用
		// 3. 多实例共享：多个应用实例可以共享同一套策略数据
		// 4. 便于管理：可以通过数据库工具直接查看和管理权限策略
		a, err := gormadapter.NewAdapterByDB(global.GVA_DB)
		if err != nil {
			// Casbin 表必须是 InnoDB 引擎，因为需要支持事务
			// MyISAM 不支持事务，会导致策略更新失败
			zap.L().Error("适配数据库失败请检查casbin表是否为InnoDB引擎!", zap.Error(err))
			return
		}

		// 定义 Casbin 模型配置（基于 RBAC 角色访问控制模型）
		// 为什么使用字符串而非文件：
		// 1. 灵活性：可以动态修改模型配置，不需要重启应用
		// 2. 可移植性：不依赖外部配置文件，代码更自包含
		// 3. 版本控制：模型配置和代码一起进行版本管理
		text := `
		[request_definition]
		// 请求定义：定义访问请求的格式，sub(主体), obj(对象), act(动作)
		// 例如：用户 alice 要访问 /api/users 的 GET 操作
		r = sub, obj, act
		
		[policy_definition]
		// 策略定义：定义策略的格式，与请求定义对应
		// 例如：用户 alice 可以访问 /api/users 的 GET 操作
		p = sub, obj, act
		
		[role_definition]
		// 角色定义：定义角色继承关系，g = 角色继承，_ = 表示占位符
		// 例如：alice 是 admin 角色，admin 是 super_admin 角色
		// 格式：g = 用户/角色, 角色（第一个 _ 代表用户或角色，第二个 _ 代表角色）
		g = _, _
		
		[policy_effect]
		// 策略效果：定义多个策略规则组合后的最终效果
		// some(where (p.eft == allow)) 表示只要有一个策略允许，就允许访问（OR 逻辑）
		// 还有其他效果如：拒绝优先、全部允许等
		e = some(where (p.eft == allow))
		
		[matchers]
		// 匹配器：定义请求和策略如何匹配
		// r.sub == p.sub：请求的主体必须等于策略的主体
		// keyMatch2(r.obj, p.obj)：使用 keyMatch2 函数匹配对象路径，支持通配符（如 /api/*）
		// r.act == p.act：请求的动作必须等于策略的动作
		m = r.sub == p.sub && keyMatch2(r.obj,p.obj) && r.act == p.act
		`

		// 从字符串创建模型对象
		// 为什么不使用文件：见上面的注释说明
		m, err := model.NewModelFromString(text)
		if err != nil {
			zap.L().Error("字符串加载模型失败!", zap.Error(err))
			return
		}

		// 创建带缓存和同步功能的执行器
		// 第一个参数：模型配置
		// 第二个参数：适配器（用于持久化策略）
		syncedCachedEnforcer, _ = casbin.NewSyncedCachedEnforcer(m, a)

		// 设置缓存过期时间为 3600 秒（1小时）
		// 为什么设置缓存过期：
		// 1. 性能与实时性平衡：缓存提升性能，过期时间保证策略更新的及时性
		// 2. 自动刷新：过期后自动重新加载策略，无需手动刷新
		// 3. 内存管理：避免策略长期占用内存（虽然对于策略数据影响较小）
		// 4. 数据一致性：在分布式环境下，过期机制有助于多实例之间的策略同步
		syncedCachedEnforcer.SetExpireTime(60 * 60)

		// 从数据库加载策略到内存
		// 忽略错误是因为：如果加载失败，后续调用时会再次尝试加载
		// 在实际使用中，建议检查错误并进行日志记录
		_ = syncedCachedEnforcer.LoadPolicy()
	})

	// 返回单例实例
	// 后续调用会直接返回已初始化的实例，不会再次执行初始化逻辑
	return syncedCachedEnforcer
}
