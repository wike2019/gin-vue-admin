package middleware

import (
	"errors"
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/golang-jwt/jwt/v5"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/gin-gonic/gin"
)

// JWTAuth JWT认证中间件
// 返回值类型为 gin.HandlerFunc，这是Gin框架中间件的标准模式
// 好处：符合Gin的中间件设计规范，可以灵活地在路由中注册使用，支持链式调用
// 设计思路：通过返回函数的方式，可以在注册中间件时进行配置，同时保持中间件的可复用性
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 第一步：获取Token
		// 设计思路：优先从请求头 x-token 获取，如果不存在则从Cookie中获取
		// 好处：
		// 1. 支持多种token传递方式（Header和Cookie），提高兼容性
		// 2. Header方式适合API调用，Cookie方式适合浏览器自动携带
		// 3. 前端可以将token存储在localStorage（通过Header传递）或Cookie中，灵活选择
		// 注意：前端需要与后端协商token存储方式和过期时间，可以约定刷新令牌机制或重新登录策略
		token := utils.GetToken(c)

		// 第二步：验证Token是否存在
		// 设计思路：在解析token之前先检查是否存在，避免无效的解析操作
		// 好处：提前返回，减少不必要的计算，提高性能
		if token == "" {
			response.NoAuth("未登录或非法访问，请登录", c)
			c.Abort() // 终止后续中间件和路由处理器的执行
			return
		}

		// 第三步：检查Token是否在黑名单中
		// 设计思路：即使token格式正确且未过期，如果被加入黑名单（如用户登出、异地登录等），也应该拒绝访问
		// 好处：
		// 1. 支持主动撤销token，增强安全性
		// 2. 可以处理用户登出、密码修改、账户被禁用等场景
		// 3. 使用缓存（BlackCache）检查，性能开销小
		// 4. 清除客户端token，防止后续请求继续使用无效token
		if isBlacklist(token) {
			response.NoAuth("您的帐户异地登陆或令牌失效", c)
			utils.ClearToken(c) // 清除客户端cookie中的token
			c.Abort()
			return
		}

		// 第四步：创建JWT解析器并解析Token
		// 设计思路：每次请求都创建新的JWT实例，确保使用最新的签名密钥
		// 好处：支持动态更新签名密钥，提高安全性
		j := utils.NewJWT()

		// 解析token，提取其中包含的用户信息（claims）
		// claims包含：用户ID、用户名、UUID、权限ID、过期时间等信息
		claims, err := j.ParseToken(token)
		if err != nil {
			// 特殊处理：token过期的情况
			// 设计思路：区分不同类型的错误，给用户更明确的提示
			// 好处：提升用户体验，明确告知token已过期，需要重新登录
			if errors.Is(err, utils.TokenExpired) {
				response.NoAuth("登录已过期，请重新登录", c)
				utils.ClearToken(c) // 清除过期的token
				c.Abort()
				return
			}
			// 其他错误：token格式错误、签名无效、尚未生效等
			// 好处：统一处理所有token解析错误，保证安全性
			response.NoAuth(err.Error(), c)
			utils.ClearToken(c)
			c.Abort()
			return
		}

		// 第五步：可选的用户状态检查（已注释）
		// 设计思路：检查用户是否被禁用或删除，如果被禁用则将token加入黑名单
		// 为什么注释掉：
		// 1. 每次请求都查询数据库会消耗性能，影响响应速度
		// 2. 可以通过其他机制（如定期检查、事件通知）来处理用户状态变更
		// 3. 如果确实需要实时检查，可以启用此代码，但建议添加缓存机制
		// 好处：如果启用，可以确保被禁用的用户立即无法访问系统
		//if user, err := userService.FindUserByUuid(claims.UUID.String()); err != nil || user.Enable == 2 {
		//	_ = jwtService.JsonInBlacklist(system.JwtBlacklist{Jwt: token})
		//	response.FailWithDetailed(gin.H{"reload": true}, err.Error(), c)
		//	c.Abort()
		//}

		// 第六步：将解析后的claims存入Gin的Context
		// 设计思路：后续的路由处理器和中间件可以直接从context中获取用户信息，无需重复解析token
		// 好处：
		// 1. 避免重复解析token，提高性能
		// 2. 统一的数据访问方式，简化业务代码
		// 3. 可以在后续中间件和处理器中通过 c.Get("claims") 获取用户信息
		c.Set("claims", claims)

		// 第七步：Token自动刷新机制
		// 设计思路：当token即将过期时（剩余时间小于BufferTime），自动生成新token
		// BufferTime（缓冲时间）的作用：
		// 1. 在token过期前提前刷新，避免用户在使用过程中突然被要求重新登录
		// 2. 给前端足够的时间来更新token，提升用户体验
		// 3. 在缓冲时间内，旧token和新token都有效，但前端只保留新token
		// 好处：
		// 1. 无感知刷新，用户无需频繁登录
		// 2. 提高安全性，定期更新token
		// 3. 减少因token过期导致的请求失败
		if claims.ExpiresAt.Unix()-time.Now().Unix() < claims.BufferTime {
			// 计算新的过期时间（从当前时间开始，重新计算完整的过期时长）
			dr, _ := utils.ParseDuration(global.GVA_CONFIG.JWT.ExpiresTime)
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(dr))

			// 使用旧token创建新token
			// CreateTokenByOldToken 内部使用了并发控制（归并回源），避免同一token被并发刷新多次
			// 好处：防止并发请求时生成多个不同的新token，保证token的唯一性
			newToken, _ := j.CreateTokenByOldToken(token, *claims)
			newClaims, _ := j.ParseToken(newToken)

			// 将新token和新过期时间通过响应头返回给前端
			// 设计思路：通过Header传递，前端可以拦截响应并更新本地存储的token
			// 好处：前端可以自动更新token，无需用户感知
			c.Header("new-token", newToken)
			c.Header("new-expires-at", strconv.FormatInt(newClaims.ExpiresAt.Unix(), 10))

			// 同时更新Cookie中的token
			// 好处：支持Cookie方式的token存储，浏览器会自动管理
			utils.SetToken(c, newToken, int(dr.Seconds()/60))

			// 如果启用了多点登录检测（UseMultipoint）
			// 设计思路：将新token存入Redis，以用户名作为key
			// 好处：
			// 1. 支持单点登录（SSO）：同一用户只能有一个有效token
			// 2. 可以追踪用户的活跃token
			// 3. 支持异地登录检测：新登录会使旧token失效
			if global.GVA_CONFIG.System.UseMultipoint {
				// 记录新的活跃jwt到Redis，key为用户名，value为新token
				_ = utils.SetRedisJWT(newToken, newClaims.Username)
			}
		}

		// 第八步：继续执行后续的中间件和路由处理器
		// 设计思路：认证通过后，允许请求继续处理
		// 好处：符合中间件链式调用的设计模式
		c.Next()

		// 第九步：在响应返回前，再次检查并设置新token到响应头
		// 设计思路：在c.Next()之后处理，确保即使后续中间件修改了响应头，也能正确返回新token
		// 为什么需要这一步：
		// 1. 某些中间件可能会清除或修改响应头
		// 2. 确保新token能够正确返回给前端
		// 3. 如果在上面的逻辑中已经设置了新token，这里会再次确认
		// 好处：保证token刷新机制的可靠性，确保前端能够获取到新token
		if newToken, exists := c.Get("new-token"); exists {
			c.Header("new-token", newToken.(string))
		}
		if newExpiresAt, exists := c.Get("new-expires-at"); exists {
			c.Header("new-expires-at", newExpiresAt.(string))
		}
	}
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: IsBlacklist
//@description: 判断JWT是否在黑名单内部
//@param: jwt string
//@return: bool

// isBlacklist 检查token是否在黑名单中
// 设计思路：使用内存缓存（BlackCache）快速检查token是否被拉黑
// 好处：
// 1. 性能优异：内存查找速度快，O(1)时间复杂度
// 2. 支持主动撤销token：当用户登出、修改密码、被禁用时，可以将token加入黑名单
// 3. 安全性高：即使token格式正确且未过期，只要在黑名单中就拒绝访问
// 4. 实现简单：使用缓存库的Get方法即可判断，无需查询数据库
// 注意：BlackCache应该是全局共享的缓存实例，支持跨请求的token黑名单管理
func isBlacklist(jwt string) bool {
	_, ok := global.BlackCache.Get(jwt)
	return ok
}
