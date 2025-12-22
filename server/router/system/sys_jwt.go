package system

import (
	"github.com/gin-gonic/gin"
)

// JwtRouter JWT路由结构体
// 为什么使用结构体：
//   - 遵循面向对象设计原则，将路由初始化逻辑封装在结构体方法中
//   - 与项目中其他路由模块（如 UserRouter、MenuRouter）保持一致的代码风格
//   - 便于后续扩展，如果需要添加JWT相关的其他路由，可以在同一个结构体中扩展
//
// 好处：
//  1. 代码组织清晰：每个功能模块的路由独立管理，职责单一
//  2. 易于维护：修改JWT相关路由时，只需关注这一个文件
//  3. 符合Go语言最佳实践：使用结构体方法组织相关功能
//  4. 便于测试：可以针对JwtRouter结构体编写单元测试
type JwtRouter struct{}

// InitJwtRouter 初始化JWT相关路由
// 参数 Router：路由组，通常是 "/api/v1" 下的子路由组
// 为什么这么写：
//  1. 统一路由管理：将JWT相关的所有路由集中在一个方法中初始化，便于查找和维护
//  2. 路由分组：使用 Router.Group("jwt") 创建路由组，所有JWT相关接口都以 "/jwt" 为前缀
//  3. 代码块组织：使用大括号包裹路由定义，使代码结构更清晰，便于阅读
//
// 好处：
//  1. 模块化设计：JWT功能独立，不影响其他模块的路由配置
//  2. 统一前缀：所有JWT接口都有 "/jwt" 前缀，符合RESTful API设计规范
//  3. 易于扩展：后续添加JWT相关接口时，只需在此方法中添加即可
//  4. 便于中间件管理：如果需要为JWT路由组统一添加中间件，只需在Group后调用Use方法
func (s *JwtRouter) InitJwtRouter(Router *gin.RouterGroup) {
	// 创建JWT路由组，所有JWT相关的接口都会以 "/jwt" 为前缀
	// 例如：POST /api/v1/jwt/jsonInBlacklist
	// 为什么使用路由分组：
	//   - 统一管理JWT相关接口，代码组织更清晰
	//   - 便于统一添加中间件（如权限验证、日志记录等）
	//   - 符合RESTful API设计规范，接口路径更有语义
	jwtRouter := Router.Group("jwt")
	{
		// jsonInBlacklist：将JWT token加入黑名单
		// 为什么需要JWT黑名单机制：
		//   1. 安全退出：用户退出登录时，需要使当前token失效，防止token被恶意使用
		//   2. 强制下线：管理员可以强制某个用户下线，通过将token加入黑名单实现
		//   3. 安全防护：当检测到token泄露或异常时，可以立即将token加入黑名单
		//   4. 权限撤销：当用户权限被撤销时，需要使已签发的token失效
		// 为什么使用POST方法：
		//   - 这是一个有副作用的操作（修改黑名单状态），应该使用POST而不是GET
		//   - 符合RESTful API设计规范：POST用于创建资源或执行操作
		// 好处：
		//   1. 安全性：即使token未过期，加入黑名单后也无法使用，提高系统安全性
		//   2. 灵活性：支持主动失效token，不依赖token的过期时间
		//   3. 可控性：管理员可以精确控制哪些token失效，实现细粒度的访问控制
		jwtRouter.POST("jsonInBlacklist", jwtApi.JsonInBlacklist) // jwt加入黑名单
	}
}
