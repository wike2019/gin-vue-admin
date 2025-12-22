package mcpTool

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// McpTool 定义了MCP工具必须实现的接口
//
// 设计原理：
// 1. 接口抽象：通过接口定义统一的工具契约，所有工具必须实现 Handle 和 New 方法
// 2. 多态支持：不同的工具实现可以有不同的行为，但都遵循相同的接口规范
// 3. 依赖倒置：框架依赖于抽象接口而非具体实现，降低耦合度
//
// 好处：
// - 解耦：框架代码与具体工具实现解耦，修改工具实现不影响框架
// - 可扩展：新增工具只需实现此接口，无需修改现有代码（开闭原则）
// - 可测试：可以轻松创建 mock 对象进行单元测试
type McpTool interface {
	// Handle 处理工具调用请求
	// ctx: 上下文，用于传递取消信号、超时等控制信息
	// request: MCP工具调用请求，包含工具名称、参数等信息
	// 返回: 工具执行结果和可能的错误
	Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

	// New 创建工具的注册信息
	// 返回: mcp.Tool 对象，包含工具的名称、描述、参数定义等元数据
	//
	// 设计考虑：
	// - 每次调用返回新的实例，保证元数据的独立性和线程安全
	// - 将元数据获取与业务逻辑分离，便于工具信息的统一管理
	New() mcp.Tool
}

// toolRegister 工具注册表，用于存储所有已注册的工具
//
// 设计原理（注册表模式）：
// 1. 集中管理：所有工具实例统一存储在 map 中，以工具名称为 key
// 2. 延迟注册：工具可以在包初始化时（init函数）自行注册，无需手动管理注册顺序
// 3. 单例访问：全局唯一注册表，确保工具的唯一性和一致性
//
// 为什么使用 map[string]McpTool：
// - string 作为 key：使用工具名称（mcp.Tool.Name），便于快速查找和管理
// - McpTool 作为 value：存储工具实例，支持运行时调用 Handle 方法
//
// 好处：
// - 自动发现：通过包导入机制自动收集所有工具，无需手动列举
// - 低耦合：工具注册与框架初始化分离，工具可以在任何时候注册
// - 高效查找：map 的 O(1) 时间复杂度，快速定位工具
var toolRegister = make(map[string]McpTool)

// RegisterTool 供工具在 init 函数中调用，将自己注册到工具注册表中
//
// 参数：
//
//	tool: 实现了 McpTool 接口的工具实例
//
// 设计原理：
// 1. 包级别注册：利用 Go 的包初始化机制，工具在包导入时自动注册
// 2. 名称提取：通过 tool.New() 获取工具元数据，提取名称作为注册 key
// 3. 幂等性：同一工具可以多次注册，后注册的会覆盖先注册的（虽然通常只注册一次）
//
// 使用场景：
//
//	// 在某个工具包中
//	func init() {
//	    RegisterTool(&MyTool{})
//	}
//
// 好处：
// - 自动化：工具导入即注册，无需手动调用注册逻辑
// - 简洁：工具开发者只需实现接口并调用一次 RegisterTool
// - 灵活性：支持动态注册，工具可以在运行时决定是否注册
func RegisterTool(tool McpTool) {
	mcpTool := tool.New()
	toolRegister[mcpTool.Name] = tool
}

// RegisterAllTools 将所有已注册的工具批量注册到 MCP 服务器中
//
// 参数：
//
//	mcpServer: MCP 服务器实例，用于接收工具注册
//
// 设计原理：
// 1. 批量处理：一次性将所有工具注册到服务器，简化初始化流程
// 2. 统一入口：所有工具的注册通过同一个函数完成，便于管理和调试
// 3. 延迟绑定：工具先注册到本地注册表，服务器启动时再统一绑定
//
// 执行流程：
//  1. 遍历 toolRegister 中的所有工具
//  2. 为每个工具调用 New() 获取元数据
//  3. 将工具元数据和处理器注册到 MCP 服务器
//
// 为什么分离 RegisterTool 和 RegisterAllTools：
// - 职责分离：RegisterTool 负责工具收集，RegisterAllTools 负责服务器绑定
// - 时机控制：工具可以在任何时间注册，但服务器绑定在特定时机进行
// - 解耦设计：工具注册不依赖服务器实例，服务器绑定不关心工具注册细节
//
// 好处：
// - 统一管理：所有工具的服务器注册在一个地方完成，便于监控和日志记录
// - 性能优化：批量注册比逐个注册更高效
// - 错误隔离：可以在这里统一处理注册失败的情况，避免单个工具影响整体
func RegisterAllTools(mcpServer *server.MCPServer) {
	for _, tool := range toolRegister {
		mcpServer.AddTool(tool.New(), tool.Handle)
	}
}
