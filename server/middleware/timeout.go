package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TimeoutMiddleware 创建超时中间件
// 入参 timeout 设置超时时间（例如：time.Second * 5）
// 使用示例 xxx.Get("path",middleware.TimeoutMiddleware(30*time.Second),HandleFunc)
//
// 设计原理：
// 1. 使用 context.WithTimeout 创建带超时控制的上下文，当超时时间到达时，ctx.Done() 会收到信号
// 2. 在独立的 goroutine 中执行后续处理器，主 goroutine 通过 select 监听多个事件
// 3. 使用 buffered channel 避免 goroutine 阻塞导致的泄漏问题
// 4. 正确处理 panic 情况，确保异常能够被正确传播和处理
//
// 好处：
// - 防止长时间运行的请求占用服务器资源
// - 提供统一的超时控制机制
// - 避免 goroutine 泄漏（通过 buffered channel 和 defer 确保）
// - 正确处理异常情况（panic 恢复和传播）
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 创建带超时的上下文
		// 为什么使用 context.WithTimeout：
		// - 自动管理超时时间，到达超时时间后 ctx.Done() 会立即返回
		// - 可以传递给下游处理器，让它们也能感知到超时并主动取消操作
		// - 使用 defer cancel() 确保资源及时释放，避免内存泄漏
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel() // 确保即使正常返回也会释放 context 资源

		// 将新的上下文绑定到请求中
		// 这样下游的处理器可以通过 c.Request.Context() 获取到带超时的上下文
		// 好处：下游代码可以使用 ctx.Done() 来检查是否超时，并主动取消耗时操作
		c.Request = c.Request.WithContext(ctx)

		// 使用 buffered channel 避免 goroutine 泄漏
		// 为什么使用 buffered channel（容量为1）：
		// - 如果使用无缓冲 channel，当主 goroutine 已经因为超时返回时，
		//   子 goroutine 尝试发送数据到 done channel 会永远阻塞，导致 goroutine 泄漏
		// - 使用 buffered channel 后，即使主 goroutine 已经返回，子 goroutine 也能成功发送数据
		// - 虽然数据不会被读取，但至少不会阻塞，goroutine 可以正常结束
		done := make(chan struct{}, 1)         // 用于通知请求处理完成
		panicChan := make(chan interface{}, 1) // 用于传递 panic 信息

		// 在独立的 goroutine 中执行后续处理器
		// 为什么需要独立的 goroutine：
		// - 主 goroutine 需要能够"中断"请求处理，如果直接调用 c.Next()，
		//   主 goroutine 会阻塞，无法响应超时事件
		// - 通过 goroutine 分离执行，主 goroutine 可以通过 select 同时监听
		//   完成信号、panic 信号和超时信号
		go func() {
			// defer 确保无论正常返回还是 panic 都能执行清理工作
			defer func() {
				// 捕获 panic，避免整个程序崩溃
				// 为什么需要处理 panic：
				// - 如果处理器中发生 panic，不捕获会导致整个程序崩溃
				// - 通过 recover 捕获后，可以选择重新抛出（让上层处理）或记录日志
				if p := recover(); p != nil {
					// 使用 select + default 的非阻塞方式发送 panic
					// 为什么使用非阻塞：
					// - 如果主 goroutine 已经因为超时返回，panicChan 的接收者已经不存在
					// - 使用 default 分支避免阻塞，即使发送失败也不会导致 goroutine 泄漏
					select {
					case panicChan <- p:
						// 成功发送 panic 信息
					default:
						// 如果 channel 已满或接收者已不存在，忽略（避免阻塞）
					}
				}
				// 发送完成信号（无论是否发生 panic）
				// 同样使用非阻塞方式，避免 goroutine 泄漏
				select {
				case done <- struct{}{}:
					// 成功发送完成信号
				default:
					// 如果 channel 已满，忽略（避免阻塞）
					// 这种情况可能发生在超时后主 goroutine 已经返回
				}
			}()
			// 执行后续的中间件和处理器
			// 注意：即使超时，这个 goroutine 仍会继续执行，但响应已经被中止
			c.Next()
		}()

		// 使用 select 同时监听多个事件
		// 为什么使用 select：
		// - 可以同时等待多个 channel，哪个先有数据就处理哪个
		// - 这是 Go 语言中实现超时控制的经典模式
		select {
		case p := <-panicChan:
			// 如果处理器中发生了 panic，重新抛出
			// 为什么重新抛出：
			// - 让 Gin 框架的 panic 恢复机制来处理，保证错误处理的统一性
			// - 如果不重新抛出，panic 会被静默吞掉，可能导致数据不一致等问题
			panic(p)
		case <-done:
			// 请求处理正常完成，直接返回
			// 此时后续的响应处理会正常进行
			return
		case <-ctx.Done():
			// 超时时间到达，中止请求处理
			// 为什么需要中止：
			// - 虽然子 goroutine 可能还在运行，但我们已经决定不再等待
			// - 通过 c.Abort() 告诉 Gin 框架停止后续处理，避免写入响应

			// 设置 Connection: close 头
			// 为什么需要这个头：
			// - 告诉客户端关闭连接，避免客户端继续等待响应
			// - 释放服务器端的连接资源，提高资源利用率
			c.Header("Connection", "close")

			// 返回 504 Gateway Timeout 错误
			// 使用 AbortWithStatusJSON 而不是直接返回：
			// - Abort 会阻止后续中间件和处理器执行
			// - 即使子 goroutine 中的 c.Next() 还在执行，也不会再写入响应
			// - 返回 JSON 格式便于前端统一处理错误
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{
				"code": 504,
				"msg":  "请求超时",
			})
			return
		}
	}
}
