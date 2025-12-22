package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	mcpTool "github.com/flipped-aurora/gin-vue-admin/server/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// McpRun 初始化并启动MCP（Model Context Protocol）服务器
// MCP是一个标准协议，用于AI应用与外部工具/数据源进行交互
//
// 返回值: *server.SSEServer - SSE服务器实例，用于提供Server-Sent Events服务
//
// 设计思路和好处：
// 1. 统一配置管理：从全局配置读取MCP参数，便于集中管理和动态配置
// 2. 全局实例共享：将MCP服务器保存到全局变量，方便其他模块访问和使用
// 3. 工具自动注册：通过RegisterAllTools统一注册所有工具，实现插件化架构
// 4. SSE通信协议：使用Server-Sent Events实现实时双向通信，适合AI交互场景
func McpRun() *server.SSEServer {
	// 从全局配置中获取MCP相关配置
	// 好处：配置集中管理，支持通过配置文件动态调整，无需重新编译
	config := global.GVA_CONFIG.MCP

	// 创建MCP服务器核心实例
	// 传入服务器名称和版本号，用于标识和版本管理
	// 好处：标准化的服务器标识，便于客户端识别和兼容性检查
	s := server.NewMCPServer(
		config.Name,
		config.Version,
	)

	// 将MCP服务器实例保存到全局变量
	// 好处：
	// 1. 全局可访问：·其他模块（如API路由、中间件）可以直接使用该实例
	// 2. 单例模式：确保整个应用只有一个MCP服务器实例，避免资源浪费
	// 3. 生命周期管理：便于统一管理和控制服务器的生命周期
	global.GVA_MCP_SERVER = s

	// 注册所有已实现的MCP工具到服务器
	// 好处：
	// 1. 插件化架构：工具通过init函数自动注册，新增工具无需修改此代码
	// 2. 解耦设计：工具实现与服务器初始化分离，提高代码可维护性
	// 3. 统一管理：所有工具集中注册，便于管理和监控
	mcpTool.RegisterAllTools(s)

	// 创建并返回SSE（Server-Sent Events）服务器
	// SSE是一种HTTP长连接技术，适合AI交互场景的实时通信
	// 好处：
	// 1. 实时通信：支持服务器主动推送消息，适合流式AI响应
	// 2. 低延迟：相比轮询方式，SSE提供更低的延迟和更好的用户体验
	// 3. 标准化端点：通过配置化的路径和URL前缀，便于路由管理和部署
	// 4. 双向通信：支持客户端发送消息和服务器推送响应
	return server.NewSSEServer(s,
		server.WithSSEEndpoint(config.SSEPath),         // SSE事件流端点，用于接收实时推送
		server.WithMessageEndpoint(config.MessagePath), // 消息处理端点，用于接收客户端消息
		server.WithBaseURL(config.UrlPrefix))           // URL前缀，用于统一管理API路径
}
