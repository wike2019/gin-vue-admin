package middleware

import (
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

// CasbinHandler Casbin RBAC权限控制中间件
// 设计思路：基于角色的访问控制（RBAC），在JWT认证之后进行细粒度的权限验证
// 为什么需要这个中间件？
// 1. JWT认证只能验证用户身份，无法判断用户是否有权限访问特定资源
// 2. 不同角色的用户应该有不同的访问权限（如管理员可以删除用户，普通用户不能）
// 3. 需要在路由层面统一进行权限检查，避免在每个handler中重复编写权限验证代码
//
// 好处：
// 1. 统一权限管理：所有权限规则集中在Casbin策略中，便于维护和修改
// 2. 细粒度控制：可以精确控制每个角色对每个资源的操作权限（GET、POST、PUT、DELETE等）
// 3. 代码复用：避免在每个业务handler中重复编写权限验证逻辑
// 4. 安全性：在请求到达业务逻辑之前就进行权限拦截，防止未授权访问
// 5. 灵活性：权限规则存储在数据库中，可以动态修改，无需重启服务
// 6. 可扩展性：支持复杂的权限模型（如角色继承、资源层级等）
//
// 执行时机：
// - 在JWT认证中间件之后执行（router.go中：JWTAuth -> CasbinHandler）
// - 只有通过JWT认证的用户才会进入此中间件进行权限检查
// - 公共路由（PublicGroup）不经过此中间件，私有路由（PrivateGroup）必须经过
func CasbinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 第一步：从请求中获取用户Claims信息
		// 设计思路：JWT中间件已经解析token并将claims存入Context，这里直接获取即可
		// 为什么使用GetClaims而不是从Context获取？
		// - GetClaims内部会优先从Context获取，如果不存在则解析token
		// - 提供fallback机制，即使JWT中间件未执行也能获取用户信息
		// - 代码更简洁，统一使用GetClaims获取用户信息
		// 注意：这里忽略错误，因为如果JWT认证失败，请求不会到达这里
		waitUse, _ := utils.GetClaims(c)

		// 第二步：获取请求的完整路径
		// 设计思路：从请求URL中提取路径，用于权限匹配
		// 例如：/api/v1/user/list -> 完整路径包含路由前缀
		path := c.Request.URL.Path

		// 第三步：去除路由前缀，获取实际的资源路径
		// 设计思路：系统可能配置了路由前缀（如/api/v1），但权限规则中存储的是去除前缀后的路径
		// 为什么需要去除前缀？
		// - 权限规则通常只关心业务路径，不关心API版本或路由前缀
		// - 例如：/api/v1/user/list 和 /api/v2/user/list 在权限上应该视为同一个资源
		// - 简化权限规则：权限表中只需要存储 /user/list，而不是 /api/v1/user/list
		// 好处：
		// 1. 权限规则更简洁：不包含技术层面的前缀信息
		// 2. 版本兼容：API版本升级时，权限规则不需要修改
		// 3. 统一管理：不同环境（开发、测试、生产）使用相同的前缀，权限规则可以复用
		obj := strings.TrimPrefix(path, global.GVA_CONFIG.System.RouterPrefix)

		// 第四步：获取HTTP请求方法
		// 设计思路：不同的HTTP方法代表不同的操作类型，需要分别控制权限
		// 为什么需要区分HTTP方法？
		// - GET：查询操作，通常权限要求较低
		// - POST：创建操作，需要创建权限
		// - PUT：更新操作，需要更新权限
		// - DELETE：删除操作，通常需要较高权限
		// 好处：
		// 1. 细粒度控制：可以允许用户查询数据，但不允许修改或删除
		// 2. 安全性：防止用户通过修改HTTP方法绕过权限检查
		// 3. RESTful规范：符合REST API的设计原则，不同操作对应不同方法
		act := c.Request.Method

		// 第五步：获取用户的角色ID（AuthorityId），转换为字符串作为Casbin的主体（subject）
		// 设计思路：Casbin使用字符串作为主体标识，而AuthorityId是uint类型，需要转换
		// 为什么使用角色ID而不是用户ID？
		// - RBAC模型基于角色，而不是基于用户：用户属于某个角色，角色拥有权限
		// - 好处：
		//   1. 权限管理更简单：只需要管理角色的权限，不需要为每个用户单独配置
		//   2. 可扩展性：新增用户时只需分配角色，权限自动继承
		//   3. 灵活性：可以动态调整角色权限，所有该角色的用户权限自动更新
		//   4. 性能优化：权限规则数量 = 角色数 × 资源数，而不是用户数 × 资源数
		// - 例如：100个用户，10个角色，100个资源
		//   基于用户：最多需要 100 × 100 = 10,000 条规则
		//   基于角色：最多需要 10 × 100 = 1,000 条规则
		sub := strconv.Itoa(int(waitUse.AuthorityId))

		// 第六步：获取Casbin执行器实例
		// 设计思路：使用单例模式获取Casbin实例，避免重复创建
		// 为什么使用GetCasbin而不是直接创建？
		// - GetCasbin内部使用sync.Once确保只创建一次实例
		// - Casbin实例包含策略缓存，重复创建会导致缓存失效，影响性能
		// - 好处：
		//   1. 性能优化：策略缓存可以加速权限检查
		//   2. 资源节约：避免重复加载策略到内存
		//   3. 一致性：所有请求使用同一个Casbin实例，保证权限检查的一致性
		e := utils.GetCasbin()

		// 第七步：执行权限检查
		// 设计思路：使用Casbin的Enforce方法检查(sub, obj, act)三元组是否在策略中允许
		// 参数说明：
		// - sub（subject）：主体，这里是用户的角色ID
		// - obj（object）：对象，这里是资源路径（去除前缀后）
		// - act（action）：操作，这里是HTTP方法
		// 返回值：
		// - success：true表示有权限，false表示无权限
		// - err：错误信息，这里忽略错误，因为权限检查失败是正常情况（用户可能确实没有权限）
		// 为什么忽略错误？
		// - Casbin的Enforce方法在权限检查失败时不会返回error，只会返回success=false
		// - error通常表示Casbin配置错误或系统异常，这种情况应该在上层处理
		// - 在中间件中，我们主要关心权限检查的结果，而不是Casbin本身的错误
		success, _ := e.Enforce(sub, obj, act)

		// 第八步：权限检查失败时的处理
		// 设计思路：如果用户没有权限，返回错误响应并中止请求处理
		// 为什么使用c.Abort()？
		// - Abort()会阻止后续的中间件和handler执行，确保请求不会继续处理
		// - 即使某个handler没有检查权限，也不会执行，提高安全性
		// - 好处：
		//   1. 安全性：确保无权限的请求不会到达业务逻辑
		//   2. 性能：提前终止请求，避免不必要的业务处理
		//   3. 一致性：所有无权限的请求都返回相同的错误格式
		if !success {
			// 返回统一的错误响应格式
			// 设计思路：使用response.FailWithDetailed返回标准化的错误响应
			// 好处：
			// 1. 前端可以统一处理权限错误，提供友好的提示
			// 2. 错误信息清晰，便于问题排查
			// 3. 符合RESTful API的错误响应规范
			response.FailWithDetailed(gin.H{}, "权限不足", c)
			// 中止请求处理，阻止后续中间件和handler执行
			c.Abort()
			return
		}

		// 第九步：权限检查通过，继续处理请求
		// 设计思路：调用c.Next()将控制权传递给下一个中间件或handler
		// 为什么需要c.Next()？
		// - Gin中间件必须调用c.Next()才能继续执行后续的中间件和handler
		// - 如果不调用c.Next()，请求会在此处停止，不会到达业务逻辑
		// - 好处：
		//   1. 符合Gin框架的设计模式
		//   2. 确保请求能够正常流转到业务逻辑
		//   3. 支持中间件链式执行
		c.Next()
	}
}
