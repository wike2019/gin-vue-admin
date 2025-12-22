package config

// MCP Model Context Protocol配置结构体
// MCP是一种用于AI模型上下文管理的协议，支持AI功能的集成和扩展
// 设计目的：
// 1. AI集成：为应用提供AI能力，如代码生成、智能分析等
// 2. 上下文管理：管理AI模型的上下文信息，提高AI响应的准确性
// 3. 协议标准化：遵循MCP标准，便于与各种AI服务集成
// 4. 灵活部署：支持独立部署或集成部署，适应不同场景
type MCP struct {
	// Name MCP服务名称：标识MCP服务的名称，用于日志和监控
	Name string `mapstructure:"name" json:"name" yaml:"name"`
	// Version MCP协议版本：使用的MCP协议版本号
	// 设计目的：版本管理，确保兼容性，便于升级和维护
	Version string `mapstructure:"version" json:"version" yaml:"version"`
	// SSEPath Server-Sent Events路径：SSE（服务器推送事件）的API路径
	// SSE是一种服务器向客户端推送数据的技术，用于实时通信
	// 使用场景：AI生成内容时实时推送进度和结果
	// 示例："/api/v1/mcp/sse"
	SSEPath string `mapstructure:"sse_path" json:"sse_path" yaml:"sse_path"`
	// MessagePath 消息处理路径：处理MCP消息的API路径
	// 设计目的：定义消息接收和处理的端点，实现MCP协议的消息交互
	// 示例："/api/v1/mcp/message"
	MessagePath string `mapstructure:"message_path" json:"message_path" yaml:"message_path"`
	// UrlPrefix URL前缀：MCP相关API的统一前缀
	// 设计目的：
	// 1. 路由组织：统一管理MCP相关的路由
	// 2. 版本控制：便于API版本管理
	// 3. 路径隔离：将MCP功能与其他功能隔离
	// 示例："/api/v1/mcp"
	UrlPrefix string `mapstructure:"url_prefix" json:"url_prefix" yaml:"url_prefix"`
	// Addr 独立MCP服务端口：当Separate为true时，MCP服务监听的端口
	// 设计目的：支持MCP服务独立部署，可以单独扩展和维护
	// 使用场景：
	// - 高并发场景：MCP服务独立部署，避免影响主服务
	// - 微服务架构：MCP作为独立微服务运行
	Addr int `mapstructure:"addr" json:"addr" yaml:"addr"`
	// Separate 是否独立运行：控制MCP服务是否作为独立服务运行
	// true：独立运行，MCP服务在单独的端口上运行（使用Addr配置的端口）
	// false：集成运行，MCP功能集成在主服务中（使用主服务的端口）
	// 设计优势：
	// 1. 灵活部署：根据需求选择集成或独立部署
	// 2. 资源隔离：独立部署可以实现资源隔离，互不影响
	// 3. 扩展性：独立部署便于水平扩展MCP服务
	Separate bool `mapstructure:"separate" json:"separate" yaml:"separate"`
}
