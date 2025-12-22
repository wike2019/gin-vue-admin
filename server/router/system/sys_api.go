package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// ApiRouter API路由结构体
// 为什么使用空结构体？
// - Go 语言中，如果结构体只用于方法接收器，不需要存储任何状态，使用空结构体最节省内存
// - 空结构体占用 0 字节内存，比普通结构体更高效
// - 语义清晰：表明这个结构体只用于组织方法，不存储数据
// 好处：内存效率高，代码简洁，符合 Go 语言最佳实践
type ApiRouter struct{}

// InitApiRouter 初始化 API 路由
// 为什么接收两个路由组参数（Router 和 RouterPub）？
// - Router: 私有路由组，需要 JWT 认证和 Casbin 权限检查，用于需要身份验证的接口
// - RouterPub: 公共路由组，不需要认证，用于公开访问的接口
// - 好处：灵活支持不同安全级别的接口，统一管理路由注册逻辑
//
// 为什么这样设计路由分组？
// 1. 安全性：区分需要认证和不需要认证的接口，避免安全漏洞
// 2. 性能：公共接口不需要经过认证中间件，减少处理开销
// 3. 可维护性：路由分组清晰，便于理解和维护
// 4. 可扩展性：新增接口时可以根据安全需求选择合适的路由组
func (s *ApiRouter) InitApiRouter(Router *gin.RouterGroup, RouterPub *gin.RouterGroup) {
	// 创建带操作记录的路由组
	// 为什么需要操作记录中间件？
	// - 操作记录（OperationRecord）会记录请求的详细信息：请求参数、响应内容、执行时间、用户ID等
	// - 用于审计追踪：记录谁在什么时候执行了什么操作，满足合规要求
	// - 用于问题排查：当出现问题时，可以通过操作日志快速定位原因
	// - 用于性能分析：通过执行时间分析接口性能瓶颈
	// 好处：
	//   1. 完整的操作审计，满足安全合规要求
	//   2. 便于问题排查和性能优化
	//   3. 支持操作回滚和数据分析
	// 注意：操作记录会增加数据库写入，对于高频查询接口可能影响性能
	apiRouter := Router.Group("api").Use(middleware.OperationRecord())

	// 创建不带操作记录的路由组（但仍需要认证）
	// 为什么需要这个路由组？
	// - 某些接口（如查询列表）调用频率很高，如果每次都记录操作日志会产生大量数据库写入
	// - 查询操作通常不需要详细的审计记录，只需要记录关键操作（增删改）
	// - 性能优化：避免高频查询接口产生过多的日志记录，减轻数据库压力
	// 好处：
	//   1. 减少数据库写入压力，提高系统性能
	//   2. 降低存储成本，只记录关键操作
	//   3. 保持查询接口的快速响应
	apiRouterWithoutRecord := Router.Group("api")

	// 创建公共路由组（不需要认证，也不需要操作记录）
	// 为什么使用 RouterPub？
	// - 某些接口需要在未认证状态下访问（如刷新 Casbin 权限缓存）
	// - 这些接口通常是系统内部调用或特殊场景使用
	// - 安全考虑：公共路由组应该只包含必要的、安全的接口
	// 好处：
	//   1. 支持系统内部调用，不需要额外的认证开销
	//   2. 灵活性：某些特殊场景下可以绕过认证
	//   3. 性能：减少认证中间件的处理时间
	apiPublicRouterWithoutRecord := RouterPub.Group("api")

	// 第一组：需要操作记录的管理接口
	// 为什么使用代码块（{}）分组？
	// - 代码组织：将相关的路由注册逻辑分组，提高可读性
	// - 作用域管理：代码块可以限制变量的作用域，避免命名冲突
	// - 逻辑清晰：每个代码块代表一个功能模块，便于理解和维护
	// - 便于注释：可以针对每个代码块添加说明，解释为什么这样分组
	{
		// 这些接口都是管理操作（增删改查、同步等），需要记录操作日志
		// 为什么需要记录？
		// - 这些操作会改变系统状态，必须记录谁在什么时候做了什么
		// - 便于审计和问题排查，满足合规要求
		// - 支持操作回滚和数据分析
		apiRouter.GET("getApiGroups", apiRouterApi.GetApiGroups)          // 获取路由组
		apiRouter.GET("syncApi", apiRouterApi.SyncApi)                    // 同步Api
		apiRouter.POST("ignoreApi", apiRouterApi.IgnoreApi)               // 忽略Api
		apiRouter.POST("enterSyncApi", apiRouterApi.EnterSyncApi)         // 确认同步Api
		apiRouter.POST("createApi", apiRouterApi.CreateApi)               // 创建Api
		apiRouter.POST("deleteApi", apiRouterApi.DeleteApi)               // 删除Api
		apiRouter.POST("getApiById", apiRouterApi.GetApiById)             // 获取单条Api消息
		apiRouter.POST("updateApi", apiRouterApi.UpdateApi)               // 更新api
		apiRouter.DELETE("deleteApisByIds", apiRouterApi.DeleteApisByIds) // 删除选中api
	}

	// 第二组：不需要操作记录的查询接口
	// 为什么这些接口不需要操作记录？
	// - 这些是查询接口（getAllApis、getApiList），调用频率通常很高
	// - 查询操作不会改变系统状态，不需要详细的审计记录
	// - 性能考虑：避免高频查询产生大量日志，减轻数据库压力
	// 好处：
	//   1. 提高查询性能，减少日志写入开销
	//   2. 降低存储成本，只记录关键操作
	//   3. 保持系统响应速度
	{
		apiRouterWithoutRecord.POST("getAllApis", apiRouterApi.GetAllApis) // 获取所有api
		apiRouterWithoutRecord.POST("getApiList", apiRouterApi.GetApiList) // 获取Api列表
	}

	// 第三组：公共接口（不需要认证）
	// 为什么 freshCasbin 放在公共路由组？
	// - 刷新 Casbin 权限缓存是系统内部操作，可能需要在不认证状态下调用
	// - 某些场景下（如系统初始化、定时任务）可能需要刷新权限缓存
	// - 安全考虑：这个接口应该添加额外的保护措施（如 IP 白名单、内部调用验证等）
	// 好处：
	//   1. 支持系统内部调用，不需要认证开销
	//   2. 灵活性：支持多种调用场景
	// 注意：生产环境应该添加额外的安全保护，避免被恶意调用
	{
		apiPublicRouterWithoutRecord.GET("freshCasbin", apiRouterApi.FreshCasbin) // 刷新casbin权限
	}
}
