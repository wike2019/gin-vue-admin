package middleware

import (
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/gin-gonic/gin"
)

// Cors 直接放行所有跨域请求并放行所有 OPTIONS 方法
//
// 设计说明：
//
//  1. 为什么使用函数返回HandlerFunc：这是Gin中间件的标准模式，允许在注册时进行配置，
//     同时保持中间件的灵活性和可测试性。返回闭包函数可以访问外部作用域的变量。
//
//  2. 为什么直接使用Origin头：浏览器在跨域请求时会自动添加Origin头，直接使用它
//     可以动态响应任何来源的请求，适用于开发环境或需要支持多个前端域名的场景。
//
// 3. 好处：
//   - 简单高效：无需维护白名单，减少配置复杂度
//   - 开发友好：适合开发环境快速迭代，避免频繁配置
//   - 灵活性高：自动适配所有前端域名，无需修改后端代码
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取请求方法和来源域名
		// 为什么需要获取这些信息：
		// - method用于判断是否为预检请求（OPTIONS）
		// - origin用于设置CORS响应头，告诉浏览器允许该来源的跨域请求
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")

		// 设置CORS响应头 - Access-Control-Allow-Origin
		// 为什么直接使用origin值：动态响应请求来源，实现"允许所有来源"的效果
		// 好处：无需硬编码域名列表，自动适配所有前端应用
		c.Header("Access-Control-Allow-Origin", origin)

		// 设置允许的请求头
		// 为什么包含这些特定头：
		// - Content-Type: 前端需要发送JSON等格式数据
		// - Authorization/Token/X-Token: 认证相关，前端需要携带token
		// - X-CSRF-Token: 防止CSRF攻击
		// - X-User-Id: 业务相关的自定义头
		// 好处：明确声明允许的自定义头，避免浏览器拦截请求
		c.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token,X-Token,X-User-Id")

		// 设置允许的HTTP方法
		// 为什么包含这些方法：RESTful API常用的CRUD操作
		// - POST: 创建资源
		// - GET: 查询资源
		// - OPTIONS: 预检请求（必须包含）
		// - DELETE: 删除资源
		// - PUT: 更新资源
		// 好处：明确声明API支持的操作，提高安全性
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS,DELETE,PUT")

		// 设置暴露给前端的响应头
		// 为什么需要这个：默认情况下，浏览器只暴露基本响应头给JavaScript
		// 这里暴露了New-Token和New-Expires-At，用于token刷新机制
		// 好处：前端可以读取这些自定义头，实现无感知的token刷新
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type, New-Token, New-Expires-At")

		// 允许携带凭证（cookies、authorization headers等）
		// 为什么设置为true：前端需要发送认证信息（如cookies、token）
		// 注意：当Allow-Credentials为true时，Access-Control-Allow-Origin不能为"*"
		// 这就是为什么上面使用具体的origin值而不是"*"
		// 好处：支持基于cookie的会话管理和token认证
		c.Header("Access-Control-Allow-Credentials", "true")

		// 放行所有OPTIONS方法（预检请求）
		// 为什么需要特殊处理OPTIONS：
		// 1. 浏览器在发送复杂跨域请求前会先发送OPTIONS预检请求
		// 2. 预检请求不需要执行实际业务逻辑，只需要返回CORS头即可
		// 3. 返回204 No Content表示请求成功但无内容返回，这是预检请求的标准响应
		// 好处：避免预检请求触发不必要的业务逻辑，提高性能
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return // 提前返回，不执行后续中间件和路由处理
		}

		// 处理请求：继续执行后续中间件和路由处理器
		// 为什么需要c.Next()：这是Gin中间件的核心机制，调用它才会继续执行
		// 后续的中间件和最终的路由处理函数
		c.Next()
	}
}

// CorsByRules 按照配置处理跨域请求
//
// 设计说明：
// 1. 为什么需要这个函数：提供更灵活的CORS策略，支持生产环境的安全需求
//   - allow-all模式：开发环境使用，方便快速开发
//   - strict-whitelist模式：生产环境使用，只允许配置的域名访问，提高安全性
//   - 其他模式：可以根据业务需求扩展
//
// 2. 为什么在函数开始就判断模式：
//   - 如果模式是allow-all，直接复用Cors()函数，避免重复代码
//   - 这是策略模式的体现，根据配置选择不同的处理策略
//   - 好处：代码复用，逻辑清晰，易于维护
func CorsByRules() gin.HandlerFunc {
	// 放行全部模式：直接使用Cors()函数
	// 为什么这样设计：
	// - 代码复用：避免重复实现相同的逻辑
	// - 一致性：两种模式使用相同的CORS头设置逻辑
	// - 维护性：如果需要修改CORS行为，只需修改Cors()函数
	if global.GVA_CONFIG.Cors.Mode == "allow-all" {
		return Cors()
	}

	// 白名单模式：根据配置的域名列表进行验证
	return func(c *gin.Context) {
		// 检查当前请求的origin是否在白名单中
		// 为什么需要检查：生产环境需要限制允许访问的域名，防止未授权访问
		// 好处：提高安全性，防止恶意网站调用API
		whitelist := checkCors(c.GetHeader("origin"))

		// 通过检查, 添加请求头
		// 为什么只在通过检查时添加：只有白名单中的域名才允许跨域访问
		// 好处：精确控制哪些域名可以访问API，符合最小权限原则
		if whitelist != nil {
			// 使用配置中的值而不是直接使用origin
			// 为什么：配置中可能包含通配符或特定格式，需要按配置返回
			c.Header("Access-Control-Allow-Origin", whitelist.AllowOrigin)
			c.Header("Access-Control-Allow-Headers", whitelist.AllowHeaders)
			c.Header("Access-Control-Allow-Methods", whitelist.AllowMethods)
			c.Header("Access-Control-Expose-Headers", whitelist.ExposeHeaders)

			// 根据配置决定是否允许凭证
			// 为什么需要条件判断：某些场景可能不需要携带凭证，提高安全性
			// 好处：灵活配置，适应不同的安全需求
			if whitelist.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		// 严格白名单模式且未通过检查，直接拒绝处理请求
		// 为什么需要这个判断：
		// 1. strict-whitelist模式要求严格验证，未通过验证的请求应该被拒绝
		// 2. 但需要排除健康检查接口（/health），因为监控系统可能从不同域名访问
		// 3. 只允许GET方法的健康检查，避免被滥用
		// 好处：
		// - 严格模式提供最高安全性
		// - 健康检查接口的例外保证监控系统正常工作
		// - 防止恶意请求访问API
		if whitelist == nil && global.GVA_CONFIG.Cors.Mode == "strict-whitelist" && !(c.Request.Method == "GET" && c.Request.URL.Path == "/health") {
			c.AbortWithStatus(http.StatusForbidden)
			return // 拒绝请求，不继续处理
		} else {
			// 非严格白名单模式，无论是否通过检查均放行所有 OPTIONS 方法
			// 为什么这样设计：
			// 1. 预检请求（OPTIONS）本身不执行业务逻辑，相对安全
			// 2. 即使不在白名单中，也允许预检请求，让浏览器正常完成CORS检查流程
			// 3. 实际的业务请求仍然会被白名单检查拦截（如果配置了严格模式）
			// 好处：
			// - 避免预检请求被误拦截，导致前端CORS错误信息不明确
			// - 保持浏览器CORS机制的正常工作流程
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return // 预检请求直接返回，不继续处理
			}
		}

		// 处理请求：继续执行后续中间件和路由处理器
		// 只有通过白名单检查（或非严格模式）的请求才会执行到这里
		c.Next()
	}
}

// checkCors 检查当前请求的origin是否在白名单中
//
// 参数说明：
// - currentOrigin: 当前请求的Origin头值，由浏览器自动添加
//
// 返回值说明：
// - *config.CORSWhitelist: 如果找到匹配的白名单配置，返回该配置的指针
// - nil: 如果未找到匹配项，返回nil
//
// 设计说明：
// 1. 为什么使用线性查找（for循环）：
//   - 白名单通常数量不多（一般几个到几十个域名）
//   - 线性查找实现简单，代码可读性好
//   - 对于小规模数据，线性查找的性能足够好，无需使用map等复杂数据结构
//   - 好处：代码简单，易于理解和维护
//
// 2. 为什么返回指针而不是值：
//   - 避免复制整个配置结构体，提高性能
//   - 返回nil可以明确表示"未找到"，比返回空值更语义化
//   - 好处：性能更好，语义更清晰
//
// 3. 为什么需要精确匹配：
//   - 安全性考虑：只允许配置中明确列出的域名
//   - 防止域名欺骗：避免子域名或相似域名被误允许
//   - 好处：提高安全性，防止未授权访问
//
// 扩展建议：
// 如果需要支持通配符域名（如 *.example.com），可以在这里添加匹配逻辑
func checkCors(currentOrigin string) *config.CORSWhitelist {
	// 遍历配置中的跨域白名单，寻找匹配项
	// 为什么使用range遍历：
	// - 简洁明了，Go语言的标准遍历方式
	// - 自动处理索引和值，代码更清晰
	for _, whitelist := range global.GVA_CONFIG.Cors.Whitelist {
		// 精确匹配当前请求的origin和配置中的AllowOrigin
		// 为什么使用精确匹配：
		// - 安全性：只允许完全匹配的域名
		// - 可预测性：行为明确，不会有意外的匹配
		// - 好处：安全可靠，避免误匹配
		if currentOrigin == whitelist.AllowOrigin {
			// 找到匹配项，返回配置的指针
			// 注意：返回whitelist的地址，而不是值，避免复制
			return &whitelist
		}
	}
	// 未找到匹配项，返回nil
	// 调用方可以通过判断nil来确定是否在白名单中
	return nil
}
