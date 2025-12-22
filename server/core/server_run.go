package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// server 接口定义
// 为什么定义接口而不是直接使用 http.Server？
// - 接口抽象使代码更灵活，便于测试和扩展
// - 可以轻松替换为其他实现了相同接口的服务器实现
// - 符合依赖倒置原则，依赖抽象而非具体实现
type server interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

// initServer 启动服务并实现优雅关闭
//
// 设计思路：
// 1. 在 goroutine 中启动服务器，避免阻塞主线程
// 2. 主线程监听系统信号，实现优雅关闭
// 3. 使用 context.WithTimeout 设置关闭超时，防止无限等待
//
// 为什么需要优雅关闭？
// - 数据完整性：正在处理的请求可以完成，避免数据丢失
// - 用户体验：不会突然中断用户操作
// - 资源清理：可以正确关闭数据库连接、释放资源
// - 容器化部署：Kubernetes 等平台依赖优雅关闭机制
//
// 参数说明：
// - address: 服务器监听地址（如 ":8888"）
// - router: Gin 路由引擎，处理所有 HTTP 请求
// - readTimeout: 读取请求头的超时时间，防止慢客户端占用连接
// - writeTimeout: 写入响应的超时时间，防止响应时间过长
func initServer(address string, router *gin.Engine, readTimeout, writeTimeout time.Duration) {
	// 创建 HTTP 服务器实例
	// 为什么使用 http.Server 而不是直接调用 router.Run()？
	// - 更细粒度的控制：可以设置超时、最大请求头大小等参数
	// - 支持优雅关闭：http.Server.Shutdown() 提供了标准的优雅关闭机制
	// - 生产环境最佳实践：符合 Go 官方推荐的服务器启动方式
	srv := &http.Server{
		Addr:           address,           // 监听地址
		Handler:        router,            // 请求处理器（Gin 路由）
		ReadTimeout:    readTimeout,       // 读取超时：防止客户端长时间不发送数据
		WriteTimeout:   writeTimeout,      // 写入超时：防止响应时间过长
		MaxHeaderBytes: 1 << 20,          // 最大请求头大小：1MB，防止恶意大请求头攻击
	}

	// 在独立的 goroutine 中启动服务器
	// 为什么使用 goroutine？
	// - 非阻塞：主线程可以继续执行，监听系统信号
	// - 并发处理：服务器在后台运行，主线程处理关闭逻辑
	// - 标准模式：这是 Go 中实现优雅关闭的标准做法
	go func() {
		// 启动服务器
		// 为什么判断 err != http.ErrServerClosed？
		// - http.ErrServerClosed 是正常关闭时的错误，不应该视为失败
		// - 只有其他错误（端口占用、权限不足等）才需要记录和退出
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("listen: %s\n", err)
			zap.L().Error("server启动失败", zap.Error(err))
			os.Exit(1) // 启动失败直接退出，因为服务无法正常运行
		}
	}()

	// 创建信号通道，用于接收系统中断信号
	// 为什么使用缓冲通道（容量为1）？
	// - 确保信号不会丢失：即使没有立即读取，信号也能被缓存
	// - 避免阻塞：发送信号的操作不会因为通道满而阻塞
	quit := make(chan os.Signal, 1)

	// 注册要监听的系统信号
	// kill (无参数) 默认发送 syscall.SIGTERM - 正常终止信号，容器编排工具常用
	// kill -2 发送 syscall.SIGINT - 中断信号，Ctrl+C 触发
	// kill -9 发送 syscall.SIGKILL - 强制终止，无法被捕获，所以不需要添加
	// 为什么监听这两个信号？
	// - SIGTERM：Kubernetes、Docker 等容器平台发送此信号进行优雅关闭
	// - SIGINT：开发环境 Ctrl+C 触发，方便本地调试
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞等待信号
	// 主线程在这里等待，直到收到关闭信号
	<-quit
	zap.L().Info("关闭WEB服务...")

	// 创建带超时的上下文
	// 为什么设置 5 秒超时？
	// - 防止无限等待：如果某些请求处理时间过长，最多等待 5 秒
	// - 平衡用户体验和关闭速度：给正在处理的请求足够时间完成，但不会无限等待
	// - 容器化部署要求：Kubernetes 默认给 Pod 30 秒优雅关闭时间，5 秒是合理的
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// 确保取消函数被调用，释放资源
	// defer 确保即使发生 panic，cancel 也会被执行
	defer cancel()

	// 优雅关闭服务器
	// Shutdown 会：
	// 1. 停止接受新请求
	// 2. 等待正在处理的请求完成（或超时）
	// 3. 关闭所有空闲连接
	// 为什么使用 Fatal 而不是 Error？
	// - 关闭失败是严重错误，服务可能处于不一致状态
	// - Fatal 会记录日志并退出程序，避免服务处于半关闭状态
	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Fatal("WEB服务关闭异常", zap.Error(err))
	}

	zap.L().Info("WEB服务已关闭")
}
