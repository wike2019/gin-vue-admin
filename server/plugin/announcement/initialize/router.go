package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/router"
	"github.com/gin-gonic/gin"
)

// Router 初始化并注册插件的路由
// 设计模式：路由初始化模式（Route Initialization Pattern） + 中间件链模式（Middleware Chain Pattern）
// 好处：
// 1. 统一管理插件的路由注册，便于维护和扩展
// 2. 区分公开路由和私有路由，实现权限控制
// 3. 通过中间件链实现横切关注点（如认证、授权、日志等）
// 4. 使用路由前缀，避免路由冲突
// @param engine *gin.Engine Gin 引擎实例，用于注册路由
func Router(engine *gin.Engine) {
	// 创建公开路由组（不需要鉴权的接口）
	// 设计模式：路由分组模式（Route Grouping Pattern）
	// 好处：
	// 1. 使用路由前缀（RouterPrefix），统一管理所有路由
	// 2. 公开路由用于 C 端访问，无需登录即可访问
	// 3. 适用于公开信息展示，如公告列表、数据源等
	public := engine.Group(global.GVA_CONFIG.System.RouterPrefix).Group("")
	
	// 创建私有路由组（需要鉴权的接口）
	// 设计模式：路由分组模式（Route Grouping Pattern） + 中间件链模式（Middleware Chain Pattern）
	// 好处：
	// 1. 使用路由前缀（RouterPrefix），统一管理所有路由
	// 2. 通过中间件链实现权限控制：
	//    - JWTAuth(): JWT 认证中间件，验证用户身份
	//    - CasbinHandler(): Casbin 权限中间件，验证用户权限
	// 3. 所有私有路由都需要先通过认证和授权才能访问
	// 4. 中间件链式调用，按顺序执行，提高代码可读性
	private := engine.Group(global.GVA_CONFIG.System.RouterPrefix).Group("")
	private.Use(middleware.JWTAuth()).Use(middleware.CasbinHandler())
	
	// 初始化并注册公告路由
	// 设计模式：委托模式（Delegation Pattern）
	// 好处：
	// 1. 将路由注册逻辑委托给 router.Router.Info.Init 方法
	// 2. 传入 public 和 private 路由组，实现路由的灵活挂载
	// 3. 插件可以独立管理自己的路由，不影响主应用路由
	router.Router.Info.Init(public, private)
}
