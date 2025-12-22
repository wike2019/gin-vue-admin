package middleware

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GinRecovery 是一个 Gin 中间件，用于恢复应用程序中可能出现的 panic
//
// 设计目的：
// 1. 防止 panic 导致整个服务崩溃，提高服务的稳定性和可用性
// 2. 使用结构化日志（zap）记录 panic 信息，便于问题追踪和调试
// 3. 区分不同类型的错误，采用不同的处理策略
//
// 参数说明：
//   - stack: 是否记录堆栈跟踪信息
//   - true: 开发环境推荐，记录完整堆栈便于定位问题
//   - false: 生产环境推荐，减少日志体积，提高性能
//
// 返回值：
//   - gin.HandlerFunc: 符合 Gin 中间件规范的处理器函数
//
// 使用示例：
//
//	router.Use(middleware.GinRecovery(true))  // 开发环境
//	router.Use(middleware.GinRecovery(false)) // 生产环境
func GinRecovery(stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 使用 defer + recover 模式捕获 panic
		// 这样设计的好处：
		// 1. defer 确保无论函数如何返回，recover 都会执行
		// 2. recover 只能在 defer 函数中生效，这是 Go 语言的设计限制
		// 3. 将 panic 转换为可处理的错误，避免程序崩溃
		defer func() {
			if err := recover(); err != nil {
				// 特殊处理：检查是否为网络连接中断错误（broken pipe）
				//
				// 为什么需要特殊处理：
				// 1. "broken pipe" 和 "connection reset by peer" 是客户端提前断开连接导致的
				// 2. 这是网络通信中的常见情况，不是真正的程序错误
				// 3. 如果按普通 panic 处理，会产生大量的堆栈日志，浪费资源
				// 4. 连接已断开，无法向客户端写入 HTTP 状态码，无需尝试
				//
				// 处理策略：
				// - 仅记录错误日志，不输出堆栈信息
				// - 直接中止请求，不尝试写入响应
				var brokenPipe bool

				// 类型断言检查：逐层检查错误类型
				// net.OpError -> os.SyscallError -> 错误消息
				// 这样设计可以准确识别系统级别的网络错误
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						// 使用 ToLower 进行大小写不敏感的匹配，提高健壮性
						// 检查两种常见的连接中断错误
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				// 使用 httputil.DumpRequest 记录请求信息
				// 参数 false 表示不包含请求体，这样做的好处：
				// 1. 减少日志体积（请求体可能很大）
				// 2. 避免敏感信息泄露（如密码、token等）
				// 3. 记录请求头、方法、URL 等关键信息已足够定位问题
				httpRequest, _ := httputil.DumpRequest(c.Request, false)

				// 处理连接中断的情况
				if brokenPipe {
					// 仅记录错误日志，不输出堆栈
					// 使用结构化日志（zap）的好处：
					// 1. 便于日志分析和过滤
					// 2. 支持多种输出格式（JSON、文本等）
					// 3. 高性能，异步写入
					global.GVA_LOG.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)

					// 连接已断开，无法写入响应状态码
					// 使用 c.Error 添加错误到上下文（可能用于其他中间件处理）
					// 使用 nolint: errcheck 忽略错误，因为连接已断开
					_ = c.Error(err.(error)) // nolint: errcheck

					// 中止请求处理链，不再执行后续的中间件和处理器
					c.Abort()
					return
				}

				// 处理真正的程序 panic（非连接中断错误）
				// 根据 stack 参数决定是否记录堆栈信息
				if stack {
					// 开发环境：记录完整的堆栈跟踪信息
					// debug.Stack() 获取当前 goroutine 的堆栈信息
					// 好处：可以精确定位 panic 发生的代码位置和调用链
					global.GVA_LOG.Error("[Recovery from panic]",
						zap.Any("error", err),                      // panic 的值
						zap.String("request", string(httpRequest)), // 请求信息
						zap.String("stack", string(debug.Stack())), // 堆栈跟踪
					)
				} else {
					// 生产环境：仅记录错误和请求信息，不记录堆栈
					// 好处：
					// 1. 减少日志体积，降低存储成本
					// 2. 提高日志写入性能
					// 3. 避免泄露代码结构信息
					global.GVA_LOG.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}

				// 向客户端返回 500 内部服务器错误
				// AbortWithStatus 会中止请求并写入 HTTP 状态码
				// 这样可以：
				// 1. 告知客户端发生了服务器错误
				// 2. 保持 HTTP 协议的完整性
				// 3. 避免客户端长时间等待
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		// 继续执行后续的中间件和请求处理器
		// 如果没有发生 panic，这里的执行会正常继续
		c.Next()
	}
}
