package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
)

// LoadTls 返回一个 Gin 中间件，用于强制 HTTPS 连接并处理 TLS/SSL 安全相关配置
//
// 设计目的：
// 1. 安全性：强制所有 HTTP 请求重定向到 HTTPS，防止中间人攻击和数据泄露
// 2. 标准化：统一处理 TLS/SSL 相关的安全头和安全策略
// 3. 易用性：通过中间件方式，只需在路由中 use 一下即可启用 HTTPS 保护
//
// 使用方式：
// 在 router 初始化时调用 router.Use(middleware.LoadTls()) 即可启用
//
// 工作原理：
// - 使用 unrolled/secure 库来处理 HTTP 安全相关的操作
// - SSLRedirect: true 表示所有 HTTP 请求都会被重定向到 HTTPS
// - SSLHost 指定 HTTPS 服务的主机和端口
// - 如果处理过程中出现错误，会中断请求处理，避免不安全连接继续执行
//
// 好处和意义：
// 1. 自动化安全：无需在每个路由中单独处理 HTTPS 重定向，统一管理
// 2. 防止配置遗漏：通过中间件确保所有请求都经过 HTTPS 检查
// 3. 代码解耦：安全逻辑与业务逻辑分离，便于维护和测试
// 4. 性能优化：在请求处理早期进行安全检查，避免不必要的业务逻辑执行
// 5. 符合安全最佳实践：遵循 OWASP 等安全标准，强制使用加密连接
func LoadTls() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 创建 secure 中间件实例，配置 HTTPS 安全选项
		// 为什么每次请求都创建新实例？
		// - secure 库的设计要求每个请求使用新的实例来处理
		// - 这样可以确保每个请求都有独立的安全上下文
		// - 虽然有一定性能开销，但保证了安全处理的正确性
		middleware := secure.New(secure.Options{
			// SSLRedirect: true 表示强制 HTTPS 重定向
			// 当检测到 HTTP 请求时，会自动重定向到 HTTPS
			// 这是防止明文传输的关键配置
			SSLRedirect: true,
			// SSLHost 指定 HTTPS 服务的主机地址和端口
			// 注意：生产环境应该使用实际的域名，而不是 localhost
			// 例如：SSLHost: "example.com:443" 或通过环境变量配置
			SSLHost: "localhost:443",
		})

		// 执行安全处理，包括：
		// - 检查请求协议（HTTP/HTTPS）
		// - 设置安全相关的 HTTP 头（如 HSTS）
		// - 执行 HTTPS 重定向（如果需要）
		// - 处理其他安全策略
		err := middleware.Process(c.Writer, c.Request)

		if err != nil {
			// 如果安全处理过程中出现错误，立即中断请求处理
			// 为什么直接返回而不继续？
			// - 安全是第一优先级，如果安全检查失败，不应该继续处理请求
			// - 避免在安全状态不确定的情况下执行业务逻辑
			// - 防止潜在的安全漏洞被利用
			// 注意：生产环境应该使用日志库（如 zap）记录错误，而不是 fmt.Println
			fmt.Println(err)
			return
		}

		// 安全检查通过后，继续执行后续的中间件和路由处理
		// c.Next() 是 Gin 框架的核心机制，用于将控制权传递给下一个处理器
		c.Next()
	}
}
