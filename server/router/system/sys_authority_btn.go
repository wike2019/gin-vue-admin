package system

import (
	"github.com/gin-gonic/gin"
)

// AuthorityBtnRouter 权限按钮路由结构体
// 为什么使用结构体而不是直接定义函数？
// - 设计模式：采用面向对象的设计，将路由相关的逻辑封装在结构体中
// - 代码组织：结构体可以包含多个方法，便于扩展和维护
// - 一致性：与项目中其他路由文件保持统一的代码风格（如 AuthorityRouter、MenuRouter 等）
// - 可扩展性：如果未来需要为路由添加配置或状态，结构体更容易扩展
type AuthorityBtnRouter struct{}

// AuthorityBtnRouterApp 权限按钮路由的全局实例
// 为什么使用全局实例？
// - 单例模式：确保整个应用中只有一个路由实例，避免重复注册
// - 便于调用：在路由初始化时（router/enter.go）可以直接通过这个实例调用初始化方法
// - 统一管理：所有路由实例都在 enter.go 中统一管理，便于维护和查找
// - 符合 Go 语言习惯：Go 中常用包级变量来提供单例实例
var AuthorityBtnRouterApp = new(AuthorityBtnRouter)

// InitAuthorityBtnRouterRouter 初始化权限按钮相关的路由
// 参数说明：
//   - Router: gin.RouterGroup，父级路由组，用于创建子路由组
//
// 为什么使用路由组（Router.Group）？
// - 路由分组：将相关的路由组织在一起，统一管理路径前缀和中间件
// - 代码复用：可以为整个路由组统一添加中间件，避免在每个路由上重复添加
// - 路径管理：所有权限按钮相关的接口都使用 "/authorityBtn" 前缀，保持 RESTful 风格
// - 维护性：如果需要修改路径前缀或添加中间件，只需要修改一处
func (s *AuthorityBtnRouter) InitAuthorityBtnRouterRouter(Router *gin.RouterGroup) {
	// 为什么注释掉带操作记录的 router？
	// - 操作记录中间件（OperationRecord）会记录每次请求的详细信息到数据库
	// - 权限按钮相关的接口（查询、设置、检查）属于高频操作，可能被频繁调用
	// - 如果记录所有操作，会产生大量日志数据，影响数据库性能和存储空间
	// - 这些接口通常不需要审计追踪，或者可以通过其他方式（如应用日志）记录
	// - 性能优化：避免每次请求都执行数据库写入操作，提高接口响应速度
	// authorityRouter := Router.Group("authorityBtn").Use(middleware.OperationRecord())

	// 创建不带操作记录的路由组
	// 为什么命名为 authorityRouterWithoutRecord？
	// - 明确表达意图：这个路由组不使用操作记录中间件
	// - 与注释掉的代码形成对比，说明设计决策的原因
	// - 便于后续维护：如果未来需要添加操作记录，可以清楚地知道哪些路由需要修改
	authorityRouterWithoutRecord := Router.Group("authorityBtn")
	{
		// 为什么使用代码块（{}）？
		// - 代码组织：将相关的路由注册逻辑分组，提高代码可读性
		// - 作用域管理：代码块可以限制变量的作用域，避免命名冲突
		// - 逻辑清晰：明确表示这些路由属于同一个功能模块
		// - 便于维护：如果需要修改路由组配置，只需要修改代码块外的定义

		// 获取权限按钮列表接口
		// 为什么使用 POST 而不是 GET？
		// - 查询条件可能较复杂：权限按钮查询可能需要传递多个参数（如角色ID、菜单ID等）
		// - POST 请求体可以传递复杂的数据结构，而 GET 的查询参数有长度限制
		// - 安全性：某些查询条件可能包含敏感信息，POST 请求体不会出现在 URL 中
		// - 项目统一风格：项目中其他查询接口也使用 POST，保持 API 设计的一致性
		authorityRouterWithoutRecord.POST("getAuthorityBtn", authorityBtnApi.GetAuthorityBtn)

		// 设置权限按钮接口
		// 功能：为指定角色设置权限按钮的可见性
		// 为什么需要这个接口？
		// - 细粒度权限控制：不同角色可能看到不同的按钮（如编辑、删除等）
		// - 动态配置：管理员可以根据业务需求动态调整按钮权限
		// - 安全性：控制用户界面上按钮的显示，防止未授权操作
		authorityRouterWithoutRecord.POST("setAuthorityBtn", authorityBtnApi.SetAuthorityBtn)

		// 检查是否可以删除权限按钮接口
		// 为什么需要单独的检查接口？
		// - 业务规则验证：删除权限按钮前需要检查是否被其他角色使用
		// - 用户体验：前端可以先调用此接口检查，避免提交后才发现无法删除
		// - 数据完整性：防止删除正在使用的权限按钮，导致数据不一致
		// - 减少无效请求：提前验证可以减少不必要的删除操作请求
		authorityRouterWithoutRecord.POST("canRemoveAuthorityBtn", authorityBtnApi.CanRemoveAuthorityBtn)
	}
}
