package system

// MCP (Model Context Protocol) 工具管理API层
// 设计说明：
// 1. MCP协议：使用MCP协议与AI模型交互，支持工具调用
// 2. 客户端管理：创建MCP客户端连接，管理工具列表和调用
// 3. 测试功能：提供工具测试接口，便于调试和验证
// 4. 配置生成：自动生成MCP服务器配置，便于集成
// 5. 好处：支持AI集成、工具管理、易于测试

import (
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/mcp/client"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCP 创建MCP工具
// 设计说明：
// 1. 工具创建：根据配置创建MCP工具文件
// 2. Context传递：传递context支持超时控制
// 3. 路径返回：返回工具文件路径，便于后续使用
// 4. 好处：支持超时控制、路径明确、便于管理
// @Tags      mcp
// @Summary   自动McpTool
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.AutoMcpTool  true  "创建自动代码"
// @Success   200   {string}  string                 "{"success":true,"data":{},"msg":"创建成功"}"
// @Router    /autoCode/mcp [post]
func (a *AutoCodeTemplateApi) MCP(c *gin.Context) {
	var info request.AutoMcpTool
	err := c.ShouldBindJSON(&info)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	// 创建MCP工具，返回工具文件路径
	toolFilePath, err := autoCodeTemplateService.CreateMcp(c.Request.Context(), info)
	if err != nil {
		response.FailWithMessage("创建失败", c)
		global.GVA_LOG.Error(err.Error())
		return
	}
	// 返回工具文件路径，便于后续使用和管理
	response.OkWithMessage("创建成功,MCP Tool路径:"+toolFilePath, c)
}

// MCPList 获取MCP工具列表
// 设计说明：
// 1. 客户端连接：创建MCP客户端连接，查询可用工具
// 2. 配置生成：自动生成MCP服务器配置，便于前端集成
// 3. 资源管理：使用defer确保客户端连接关闭，防止资源泄漏
// 4. 好处：工具发现、配置自动生成、资源管理完善
// @Tags      mcp
// @Summary   自动McpTool
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.AutoMcpTool  true  "创建自动代码"
// @Success   200   {string}  string                 "{"success":true,"data":{},"msg":"创建成功"}"
// @Router    /autoCode/mcpList [post]
func (a *AutoCodeTemplateApi) MCPList(c *gin.Context) {
	// 构建MCP服务器URL
	baseUrl := fmt.Sprintf("http://127.0.0.1:%d%s", global.GVA_CONFIG.System.Addr, global.GVA_CONFIG.MCP.SSEPath)

	// 创建MCP客户端
	testClient, err := client.NewClient(baseUrl, "testClient", "v1.0.0", global.GVA_CONFIG.MCP.Name)
	if err != nil {
		response.FailWithMessage("创建MCP客户端失败:"+err.Error(), c)
		return
	}
	defer testClient.Close()
	toolsRequest := mcp.ListToolsRequest{}

	// 查询可用工具列表
	list, err := testClient.ListTools(c.Request.Context(), toolsRequest)

	if err != nil {
		response.FailWithMessage("创建失败", c)
		global.GVA_LOG.Error(err.Error())
		return
	}

	// 生成MCP服务器配置，便于前端集成
	mcpServerConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			global.GVA_CONFIG.MCP.Name: map[string]string{
				"url": baseUrl,
			},
		},
	}
	response.OkWithData(gin.H{
		"mcpServerConfig": mcpServerConfig,
		"list":            list,
	}, c)
}

// MCPTest 测试MCP工具调用
// 设计说明：
// 1. 内联结构体：使用匿名结构体定义请求参数，避免定义单独的结构体文件
// 2. 参数验证：使用binding标签进行参数验证，保证参数完整性
// 3. 连接初始化：先初始化MCP连接，再调用工具，符合MCP协议规范
// 4. 错误处理：每个步骤都有详细的错误处理，便于问题定位
// 5. 好处：代码简洁、协议规范、错误处理完善
// @Tags      mcp
// @Summary   测试McpTool
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      object  true  "调用MCP Tool的参数"
// @Success   200   {object}  response.Response  "{"success":true,"data":{},"msg":"测试成功"}"
// @Router    /autoCode/mcpTest [post]
func (a *AutoCodeTemplateApi) MCPTest(c *gin.Context) {
	// 使用匿名结构体定义请求参数，避免创建单独的文件
	// 好处：代码更紧凑，参数定义和使用在一起
	var testRequest struct {
		Name      string                 `json:"name" binding:"required"`      // 工具名称
		Arguments map[string]interface{} `json:"arguments" binding:"required"` // 工具参数
	}

	// 绑定JSON请求体，binding标签自动进行参数验证
	if err := c.ShouldBindJSON(&testRequest); err != nil {
		response.FailWithMessage("参数解析失败:"+err.Error(), c)
		return
	}

	// 创建MCP客户端连接
	baseUrl := fmt.Sprintf("http://127.0.0.1:%d%s", global.GVA_CONFIG.System.Addr, global.GVA_CONFIG.MCP.SSEPath)
	testClient, err := client.NewClient(baseUrl, "testClient", "v1.0.0", global.GVA_CONFIG.MCP.Name)
	if err != nil {
		response.FailWithMessage("创建MCP客户端失败:"+err.Error(), c)
		return
	}
	defer testClient.Close()

	ctx := c.Request.Context()

	// 初始化MCP连接，这是MCP协议要求的步骤
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "testClient",
		Version: "v1.0.0",
	}

	_, err = testClient.Initialize(ctx, initRequest)
	if err != nil {
		response.FailWithMessage("初始化MCP连接失败:"+err.Error(), c)
		return
	}

	// 构建工具调用请求
	request := mcp.CallToolRequest{}
	request.Params.Name = testRequest.Name
	request.Params.Arguments = testRequest.Arguments

	// 调用MCP工具
	result, err := testClient.CallTool(ctx, request)
	if err != nil {
		response.FailWithMessage("工具调用失败:"+err.Error(), c)
		return
	}

	// 检查返回结果是否为空
	if len(result.Content) == 0 {
		response.FailWithMessage("工具未返回任何内容", c)
		return
	}

	// 返回工具执行结果
	response.OkWithData(result.Content, c)
}
