package utils

import (
	"net"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ClearToken 清除客户端cookie中的token
// 设计思路：在用户登出、token失效、异地登录等场景下需要清除客户端的token
// 好处：
// 1. 防止客户端继续使用已失效的token进行请求
// 2. 支持主动登出功能，提升安全性
// 3. 通过设置maxAge为-1使cookie立即过期，浏览器会自动删除
func ClearToken(c *gin.Context) {
	// 从请求的Host中提取主机名或IP地址
	// 使用SplitHostPort分离主机和端口（如"localhost:8080" -> "localhost", "8080"）
	// 好处：正确处理带端口的Host，避免设置cookie时包含端口号导致的问题
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		// 如果分离失败（可能Host不包含端口），直接使用原始Host
		// 好处：兼容不同格式的Host，提高代码健壮性
		host = c.Request.Host
	}

	// 判断host是否为IP地址
	// 设计思路：Cookie的Domain属性对IP地址和域名的处理方式不同
	// 好处：
	// 1. IP地址作为Domain时浏览器可能会拒绝，设置为空字符串更安全
	// 2. 域名需要明确指定Domain，确保cookie在正确的域名下生效（如支持子域名）
	// 3. 开发环境常用localhost或IP，生产环境用域名，这样写能同时兼容两种场景
	if net.ParseIP(host) != nil {
		// IP地址：Domain设为空字符串，浏览器会自动处理
		// maxAge=-1：立即过期；Path="/"：全站生效；HttpOnly=false, Secure=false：适合开发环境
		c.SetCookie("x-token", "", -1, "/", "", false, false)
	} else {
		// 域名：明确指定Domain，确保cookie在正确的域名下生效
		// 好处：支持子域名共享cookie（如设置Domain为".example.com"，子域名也可以访问）
		c.SetCookie("x-token", "", -1, "/", host, false, false)
	}
}

// SetToken 将token设置到客户端cookie中
// 设计思路：在用户登录成功后或token刷新后，需要将token存储到客户端的cookie中
// 好处：
// 1. 浏览器会自动携带cookie，无需前端手动处理token存储和传递
// 2. 配合GetToken的fallback机制，支持Header和Cookie两种方式，提高兼容性
// 3. 通过maxAge参数控制token有效期，实现自动过期
func SetToken(c *gin.Context, token string, maxAge int) {
	// 从请求的Host中提取主机名或IP地址
	// 使用SplitHostPort分离主机和端口
	// 好处：正确处理带端口的Host，避免cookie设置异常
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		// 分离失败时使用原始Host
		host = c.Request.Host
	}

	// 根据host类型（IP或域名）设置不同的Domain属性
	// 设计思路：Cookie的Domain属性对IP和域名的处理规则不同，需要区别对待
	// 好处：确保cookie能在各种环境下正常工作（开发环境的localhost/IP，生产环境的域名）
	if net.ParseIP(host) != nil {
		// IP地址：Domain设为空字符串，避免浏览器拒绝
		// maxAge：token剩余有效期（秒），浏览器会在过期后自动删除cookie
		c.SetCookie("x-token", token, maxAge, "/", "", false, false)
	} else {
		// 域名：设置Domain为当前域名，确保cookie在正确的域名下生效
		// 好处：支持同域名下的多路径访问，提高用户体验
		c.SetCookie("x-token", token, maxAge, "/", host, false, false)
	}
}

// GetToken 从请求中获取token，支持Header和Cookie两种方式
// 设计思路：优先从Header获取，如果没有则从Cookie获取，体现了灵活性和兼容性
// 好处：
//  1. 优先Header方式：适合API调用场景，前端可以灵活控制token传递，不受cookie限制
//  2. Cookie fallback：适合传统Web应用，浏览器自动携带，简化前端实现
//  3. 自动刷新机制：从Cookie获取后如果token仍然有效，会重新写入cookie刷新过期时间
//     这样可以延长用户会话时间，提升用户体验（用户在活跃期间不会因为token过期而被迫重新登录）
func GetToken(c *gin.Context) string {
	// 第一步：优先从请求头中获取token
	// 设计思路：Header方式更灵活，不受cookie的SameSite、Domain等限制，适合API调用
	// 好处：前端可以将token存储在localStorage或sessionStorage中，手动添加到请求头
	token := c.Request.Header.Get("x-token")

	// 第二步：如果Header中没有token，尝试从Cookie中获取
	// 设计思路：提供fallback机制，兼容不同场景下的token传递方式
	// 好处：支持浏览器自动携带cookie的场景，减少前端代码复杂度
	if token == "" {
		j := NewJWT()
		token, _ = c.Cookie("x-token")

		// 第三步：验证从Cookie获取的token是否有效
		// 设计思路：在重新写入cookie之前先验证token的有效性
		// 好处：只对有效的token进行刷新操作，避免将无效token写入cookie
		claims, err := j.ParseToken(token)
		if err != nil {
			// token解析失败（可能已过期或格式错误），记录错误但不阻止请求继续
			// 好处：给上层调用者处理错误的机会，保持代码的灵活性
			global.GVA_LOG.Error("重新写入cookie token失败,未能成功解析token,请检查请求头是否存在x-token且claims是否为规定结构")
			return token // 返回原始token（可能是空字符串或无效token），让调用者判断
		}

		// 第四步：重新设置cookie，刷新token的过期时间
		// 设计思路：计算token剩余有效时间，更新cookie的maxAge
		// 好处：
		// 1. 延长用户会话时间，用户在活跃期间不需要频繁登录
		// 2. 确保cookie的过期时间与token的过期时间同步
		// 3. 防止cookie过期而token未过期的情况
		SetToken(c, token, int(claims.ExpiresAt.Unix()-time.Now().Unix()))
	}
	return token
}

// GetClaims 从请求中解析JWT token并返回Claims信息
// 设计思路：将token解析逻辑封装成独立函数，便于复用和统一错误处理
// 好处：
// 1. 代码复用：避免在每个需要claims的地方重复解析token的代码
// 2. 统一错误处理：集中处理token解析错误，记录日志便于问题排查
// 3. 返回错误：让调用者根据错误决定如何处理，保持代码灵活性
func GetClaims(c *gin.Context) (*systemReq.CustomClaims, error) {
	// 获取token（可能来自Header或Cookie）
	token := GetToken(c)

	// 创建JWT解析器实例
	j := NewJWT()

	// 解析token获取claims
	// 设计思路：将token字符串解析为结构化的Claims对象，包含用户ID、权限等信息
	// 好处：方便后续从claims中提取用户信息，避免重复解析
	claims, err := j.ParseToken(token)
	if err != nil {
		// 解析失败时记录错误日志，但不中断程序执行
		// 好处：便于排查token相关问题（过期、格式错误、签名验证失败等）
		global.GVA_LOG.Error("从Gin的Context中获取从jwt解析信息失败, 请检查请求头是否存在x-token且claims是否为规定结构")
	}

	// 返回claims和错误，让调用者决定如何处理
	// 好处：调用者可以根据业务需求决定是返回默认值还是中断处理
	return claims, err
}

// GetUserID 从Gin的Context中获取从jwt解析出来的用户ID
// 设计思路：采用两层查找策略，优先从Context中获取（性能更好），如果不存在则从token解析
// 好处：
// 1. 性能优化：JWT中间件已经解析过token并将claims存入Context，直接获取避免重复解析
// 2. 容错机制：如果Context中没有claims（可能是中间件未执行），则主动解析token
// 3. 返回类型安全：返回uint类型的用户ID，0表示获取失败或用户不存在
func GetUserID(c *gin.Context) uint {
	// 第一步：尝试从Context中获取claims（JWT中间件已经解析并存储）
	// 设计思路：Context是Gin框架提供的请求上下文，中间件解析token后会存储claims
	// 好处：避免重复解析token，提高性能，特别是在需要多次获取用户信息的场景下
	if claims, exists := c.Get("claims"); !exists {
		// Context中没有claims，主动解析token获取
		// 设计思路：提供fallback机制，即使中间件未执行也能获取用户信息
		// 好处：提高代码的容错性和灵活性
		if cl, err := GetClaims(c); err != nil {
			// 解析失败，返回0作为默认值
			// 好处：返回零值避免panic，调用者可以通过判断返回值是否为0来判断是否成功
			return 0
		} else {
			// 解析成功，返回用户ID
			return cl.BaseClaims.ID
		}
	} else {
		// Context中存在claims，直接使用类型断言获取
		// 设计思路：类型断言比重新解析token快得多，是性能优化的关键
		// 好处：减少CPU计算和字符串操作，提升响应速度
		waitUse := claims.(*systemReq.CustomClaims)
		return waitUse.BaseClaims.ID
	}
}

// GetUserUuid 从Gin的Context中获取从jwt解析出来的用户UUID
// 设计思路：与GetUserID相同的两层查找策略，优先从Context获取，fallback到token解析
// 好处：
// 1. 性能优化：优先使用Context中已解析的claims，避免重复解析token
// 2. 统一的设计模式：与GetUserID等函数保持一致的实现方式，代码风格统一
// 3. 类型安全：返回uuid.UUID类型，空UUID表示获取失败
// 注意：UUID是用户的全局唯一标识，常用于分布式系统中的用户标识
func GetUserUuid(c *gin.Context) uuid.UUID {
	// 优先从Context获取claims（性能更好）
	if claims, exists := c.Get("claims"); !exists {
		// Context中没有，则解析token获取
		if cl, err := GetClaims(c); err != nil {
			// 解析失败，返回空UUID（零值）
			return uuid.UUID{}
		} else {
			return cl.UUID
		}
	} else {
		// 直接从Context获取，性能最优
		waitUse := claims.(*systemReq.CustomClaims)
		return waitUse.UUID
	}
}

// GetUserAuthorityId 从Gin的Context中获取从jwt解析出来的用户角色ID
// 设计思路：采用与其他Get函数相同的两层查找策略，保证代码一致性
// 好处：
// 1. 性能优化：优先使用Context中已解析的claims，避免重复解析
// 2. 权限控制：用户角色ID用于RBAC权限控制，需要频繁获取，优化性能很重要
// 3. 零值语义：返回0表示获取失败或用户无角色，调用者可以通过判断是否为0来判断
// 应用场景：在权限验证、数据过滤、功能访问控制等场景中频繁使用
func GetUserAuthorityId(c *gin.Context) uint {
	// 优先从Context获取claims
	if claims, exists := c.Get("claims"); !exists {
		// Context中没有，解析token获取
		if cl, err := GetClaims(c); err != nil {
			return 0
		} else {
			return cl.AuthorityId
		}
	} else {
		// 直接从Context获取，避免重复解析
		waitUse := claims.(*systemReq.CustomClaims)
		return waitUse.AuthorityId
	}
}

// GetUserInfo 从Gin的Context中获取完整的用户Claims信息
// 设计思路：返回完整的Claims对象，包含用户的所有信息（ID、UUID、用户名、角色等）
// 好处：
// 1. 一次性获取所有用户信息：当需要多个用户字段时，调用一次即可，避免多次调用
// 2. 类型完整：返回完整的Claims结构，包含所有用户相关字段，扩展性好
// 3. 零值语义：返回nil表示获取失败，调用者可以通过nil检查来判断是否成功
// 应用场景：在需要获取多个用户信息的场景下使用，避免多次调用不同的Get函数
func GetUserInfo(c *gin.Context) *systemReq.CustomClaims {
	// 优先从Context获取claims
	if claims, exists := c.Get("claims"); !exists {
		// Context中没有，解析token获取完整claims
		if cl, err := GetClaims(c); err != nil {
			// 解析失败，返回nil
			return nil
		} else {
			return cl
		}
	} else {
		// 直接从Context获取，类型断言后返回
		waitUse := claims.(*systemReq.CustomClaims)
		return waitUse
	}
}

// GetUserName 从Gin的Context中获取从jwt解析出来的用户名
// 设计思路：与其他Get函数保持一致的实现模式，优先Context，fallback到token解析
// 好处：
// 1. 性能优化：优先使用Context中已解析的claims，避免重复解析token
// 2. 零值语义：返回空字符串表示获取失败，调用者可以通过判断是否为空来判断
// 3. 代码一致性：与GetUserID、GetUserUuid等函数保持相同的实现模式，便于维护
// 应用场景：在日志记录、操作审计、个性化显示等场景中使用
func GetUserName(c *gin.Context) string {
	// 优先从Context获取claims
	if claims, exists := c.Get("claims"); !exists {
		// Context中没有，解析token获取
		if cl, err := GetClaims(c); err != nil {
			// 解析失败，返回空字符串
			return ""
		} else {
			return cl.Username
		}
	} else {
		// 直接从Context获取，性能最优
		waitUse := claims.(*systemReq.CustomClaims)
		return waitUse.Username
	}
}

// LoginToken 为用户登录生成JWT token
// 设计思路：将用户信息封装到Claims中，然后生成JWT token，用于后续的身份验证
// 好处：
// 1. 统一登录逻辑：将token生成逻辑封装成函数，便于复用和维护
// 2. 信息完整：Claims包含用户的关键信息（ID、UUID、用户名、角色等），后续无需查询数据库
// 3. 无状态认证：JWT是无状态的，服务器不需要存储session，便于水平扩展
// 4. 返回值设计：返回token、claims和error，调用者可以根据error判断是否成功
// 应用场景：用户登录成功后调用此函数生成token，返回给客户端用于后续请求的认证
func LoginToken(user system.Login) (token string, claims systemReq.CustomClaims, err error) {
	// 创建JWT实例，用于生成token
	j := NewJWT()

	// 创建Claims对象，包含用户的关键信息
	// 设计思路：将用户的身份信息（ID、UUID、用户名、角色等）封装到Claims中
	// 好处：
	// 1. 后续请求可以直接从token中解析出用户信息，无需查询数据库
	// 2. 减少数据库查询，提高性能
	// 3. 支持分布式系统，无需共享session存储
	claims = j.CreateClaims(systemReq.BaseClaims{
		UUID:        user.GetUUID(),        // 用户UUID，全局唯一标识
		ID:          user.GetUserId(),      // 用户ID，数据库主键
		NickName:    user.GetNickname(),    // 用户昵称，用于显示
		Username:    user.GetUsername(),    // 用户名，用于登录和标识
		AuthorityId: user.GetAuthorityId(), // 用户角色ID，用于权限控制
	})

	// 基于Claims生成JWT token字符串
	// 设计思路：将Claims序列化并签名，生成安全的token
	// 好处：
	// 1. 签名机制确保token不被篡改
	// 2. 可以设置过期时间，提高安全性
	// 3. 客户端可以在后续请求中携带此token进行身份验证
	token, err = j.CreateToken(claims)

	// 返回token、claims和error
	// 设计思路：使用命名返回值，代码更简洁
	// 好处：调用者可以同时获取token和claims，claims可以用于设置cookie等其他操作
	return
}
