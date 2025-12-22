package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// AuthorityRouter 角色权限路由结构体
//
// 设计说明：
// 1. 为什么使用结构体？
//   - 遵循面向对象设计原则，将角色权限相关的路由初始化逻辑封装在结构体方法中
//   - 便于代码组织和维护，角色权限相关的所有路由都集中在同一个结构体中
//   - 符合 Go 语言的最佳实践，提高代码的可读性和可维护性
//   - 便于后续扩展，如果需要添加路由相关的配置或状态，可以直接在结构体中添加字段
//
// 2. 为什么使用空结构体？
//   - 这个结构体只用于方法绑定，不需要存储任何状态数据
//   - 空结构体不占用内存空间，性能最优（零内存占用）
//   - 符合 Go 语言的惯用法，简洁高效
//
// 3. 好处：
//   - 代码结构清晰：角色权限相关的所有路由定义独立，便于查找和维护
//   - 易于扩展：未来如果需要添加角色权限相关的其他路由，只需在此方法中扩展
//   - 统一管理：所有角色权限相关的路由都在一个地方，避免路由分散
//   - 符合单一职责原则：每个路由结构体只负责一个功能模块的路由定义
type AuthorityRouter struct{}

// InitAuthorityRouter 初始化角色权限路由信息
//
// 设计说明：
// 1. 双路由组设计（核心设计理念）：
//   - authorityRouter: 带有 OperationRecord() 中间件的路由组，用于需要记录操作日志的接口
//   - authorityRouterWithoutRecord: 不带有操作记录中间件的路由组，用于不需要记录操作日志的接口
//
// 2. 为什么创建两个路由组？
//   原因：不同类型的操作对日志记录的需求不同
//   - 写操作（增删改）：需要详细的操作日志用于审计追踪、问题排查、安全监控
//     * 创建角色、删除角色、更新角色等操作会改变系统状态，必须记录
//     * 操作日志包含：请求参数、响应内容、执行时间、操作用户、客户端IP等
//     * 用于审计合规要求，便于追踪谁在什么时候做了什么操作
//   - 读操作（查询）：通常不需要记录操作日志
//     * 查询操作不会改变系统状态，通常不会引起问题
//     * 查询操作频率高，如果全部记录会产生大量日志，浪费存储空间
//     * 查询操作可能涉及敏感数据，过度记录可能带来安全风险
//
// 3. 这样设计的好处：
//   - 性能优化：避免对查询操作进行不必要的日志记录，减少数据库写入压力
//   - 存储优化：只记录关键操作，节省数据库存储空间
//   - 可维护性：代码逻辑清晰，明确区分需要记录和不需要记录的操作
//   - 安全性：减少敏感数据的日志记录，降低数据泄露风险
//   - 灵活性：不同操作可以灵活选择是否需要记录日志，便于后续调整
//
// 4. 代码块分组（{}）的作用：
//   - 代码组织：将相关的路由注册逻辑分组，提高可读性
//   - 逻辑分离：第一个代码块包含所有写操作（需要记录），第二个代码块包含所有读操作（不需要记录）
//   - 便于维护：每个代码块代表一类操作，便于后续添加新路由时选择正确的路由组
//   - 便于注释：可以对整个代码块进行统一说明，说明该组路由的特点
//
// 5. 路由路径设计：
//   - 所有接口都使用 "authority" 作为路径前缀，统一管理角色权限相关接口
//   - 路径示例：/api/v1/authority/createAuthority（具体路径取决于父路由组）
//   - 符合 RESTful API 设计规范，接口命名清晰直观
//
// @param Router *gin.RouterGroup 父路由组，通常是 PrivateGroup（需要认证的私有路由组）
func (s *AuthorityRouter) InitAuthorityRouter(Router *gin.RouterGroup) {
	// 创建带操作记录中间件的路由组
	// 作用：为所有写操作（增删改）添加操作日志记录功能
	// OperationRecord() 中间件会记录：请求参数、响应内容、执行时间、操作用户、客户端IP等信息
	// 为什么在路由组级别添加：该路由组下的所有路由都需要记录操作日志，统一添加避免重复代码
	// 好处：集中管理，如需修改日志记录策略，只需修改此处即可
	authorityRouter := Router.Group("authority").Use(middleware.OperationRecord())

	// 创建不带操作记录中间件的路由组
	// 作用：为读操作（查询）提供路由，不记录操作日志
	// 为什么需要单独的路由组：查询操作不需要记录日志，使用独立的路由组避免不必要的性能开销
	// 注意：虽然名称不同，但路径前缀都是 "authority"，通过不同的路由变量实现功能分离
	authorityRouterWithoutRecord := Router.Group("authority")

	// 写操作路由组：所有会改变系统状态的操作都需要记录操作日志
	// 设计原则：增删改操作必须记录，用于审计追踪和问题排查
	{
		// 创建角色接口
		// 路径：POST /authority/createAuthority
		// 说明：创建新的角色，会改变系统状态，需要记录操作日志
		// 为什么使用 POST：创建操作属于写操作，使用 POST 符合 RESTful 规范
		authorityRouter.POST("createAuthority", authorityApi.CreateAuthority) // 创建角色

		// 删除角色接口
		// 路径：POST /authority/deleteAuthority
		// 说明：删除指定角色，会改变系统状态，需要记录操作日志
		// 为什么使用 POST 而不是 DELETE：保持接口设计一致性，POST 更灵活，便于传递复杂参数
		authorityRouter.POST("deleteAuthority", authorityApi.DeleteAuthority) // 删除角色

		// 更新角色接口
		// 路径：PUT /authority/updateAuthority
		// 说明：更新角色信息，会改变系统状态，需要记录操作日志
		// 为什么使用 PUT：更新操作使用 PUT 更符合 RESTful 规范，语义更清晰
		authorityRouter.PUT("updateAuthority", authorityApi.UpdateAuthority) // 更新角色

		// 拷贝角色接口
		// 路径：POST /authority/copyAuthority
		// 说明：基于现有角色创建新角色，会改变系统状态（创建新角色），需要记录操作日志
		// 为什么使用 POST：这是创建操作的变体，使用 POST 更合适
		authorityRouter.POST("copyAuthority", authorityApi.CopyAuthority) // 拷贝角色

		// 设置角色数据权限接口
		// 路径：POST /authority/setDataAuthority
		// 说明：设置角色的数据权限范围，会改变系统状态，需要记录操作日志
		// 为什么需要记录：数据权限是重要的安全配置，必须记录变更历史用于审计
		authorityRouter.POST("setDataAuthority", authorityApi.SetDataAuthority) // 设置角色资源权限
	}

	// 读操作路由组：查询操作不需要记录操作日志
	// 设计原则：查询操作不改变系统状态，通常不需要记录，避免日志过多
	{
		// 获取角色列表接口
		// 路径：POST /authority/getAuthorityList
		// 说明：查询角色列表，不改变系统状态，不需要记录操作日志
		// 为什么不需要记录：查询操作频繁且不会改变数据，记录会产生大量冗余日志
		// 好处：减少数据库写入压力，节省存储空间，提高查询性能
		// 为什么使用 POST 而不是 GET：可能需要传递复杂的查询条件（分页、筛选等），POST 更灵活
		authorityRouterWithoutRecord.POST("getAuthorityList", authorityApi.GetAuthorityList) // 获取角色列表
	}
}
