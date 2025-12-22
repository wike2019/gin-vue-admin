package client

import (
	"context"
	"errors"
	mcpClient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewClient 创建并初始化MCP客户端
// 
// 设计思路：
// 1. 封装了MCP客户端的创建和初始化流程，提供统一的入口函数
// 2. 使用SSE（Server-Sent Events）协议建立长连接，支持实时双向通信
// 3. 在创建时完成所有必要的初始化步骤，确保返回的客户端立即可用
//
// 参数说明：
// - baseUrl: MCP服务器的基础URL，用于建立连接
// - name: 客户端名称，用于标识当前客户端身份
// - version: 客户端版本号，便于服务器进行版本兼容性检查
// - serverName: 期望的服务器名称，用于验证连接的是正确的服务器
//
// 返回值：
// - *mcpClient.Client: 初始化完成的MCP客户端实例
// - error: 如果初始化过程中出现任何错误，返回错误信息
//
// 设计好处：
// 1. 单一职责：专注于客户端的创建和初始化，逻辑清晰
// 2. 错误处理：在每个关键步骤都进行错误检查，确保失败时能及时返回
// 3. 服务器验证：通过serverName参数验证连接的是正确的服务器，防止误连接
// 4. 协议版本：使用LATEST_PROTOCOL_VERSION确保使用最新的协议版本，获得最佳兼容性
func NewClient(baseUrl, name, version, serverName string) (*mcpClient.Client, error) {
	// 创建SSE类型的MCP客户端
	// 为什么使用SSE：SSE支持服务器主动推送消息，适合需要实时通信的场景
	// 相比HTTP轮询，SSE减少了网络开销，提高了响应速度
	client, err := mcpClient.NewSSEMCPClient(baseUrl)
	if err != nil {
		return nil, err
	}

	// 使用Background上下文，因为这是初始化阶段，不需要超时控制
	// 好处：简化上下文管理，初始化过程通常很快，不需要额外的超时机制
	ctx := context.Background()

	// 启动客户端连接
	// 为什么需要显式启动：确保连接在初始化之前已经建立，避免后续操作失败
	// 好处：提前发现连接问题，而不是在使用时才报错
	if err := client.Start(ctx); err != nil {
		return nil, err
	}

	// 构建初始化请求
	// 设计思路：使用结构体字面量初始化，代码清晰易读
	initRequest := mcp.InitializeRequest{}
	// 使用最新协议版本，确保兼容性和功能完整性
	// 好处：自动获得协议的最新特性，无需手动指定版本号
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	// 设置客户端信息，让服务器知道是谁在连接
	// 好处：服务器可以根据客户端信息提供个性化服务或进行统计
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    name,
		Version: version,
	}

	// 发送初始化请求并获取服务器信息
	// 为什么需要初始化：MCP协议要求客户端和服务器在开始通信前进行握手
	// 好处：确保双方都支持相同的协议版本，建立可靠的通信基础
	result, err := client.Initialize(ctx, initRequest)
	if err != nil {
		return nil, err
	}
	
	// 验证服务器名称是否匹配
	// 为什么需要验证：防止连接到错误的服务器，确保安全性
	// 好处：在连接建立时就发现配置错误，避免后续操作出现意外行为
	if result.ServerInfo.Name != serverName {
		return nil, errors.New("server name mismatch")
	}
	
	// 返回初始化完成的客户端
	// 此时客户端已经完全就绪，可以安全使用
	return client, nil
}
