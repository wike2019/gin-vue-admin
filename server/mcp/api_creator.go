package mcpTool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

// 注册工具到MCP工具注册表
// 
// 设计原理：
// - 利用Go的包初始化机制，在包导入时自动注册工具
// - 无需手动调用，简化工具的使用和集成
// - 符合Go语言的惯用法，减少样板代码
func init() {
	RegisterTool(&ApiCreator{})
}

// ApiCreateRequest API创建请求结构
//
// 设计考虑：
// 1. 字段设计：包含API的核心元数据，用于权限管理和路由注册
// 2. JSON标签：使用小驼峰命名，符合前端和API交互的常见约定
// 3. 简洁性：只包含必要字段，避免过度设计
//
// 字段说明：
// - Path: API路径，用于路由匹配和权限控制，如 /user/create
// - Description: 中文描述，用于权限管理界面展示，提升用户体验
// - ApiGroup: API分组，便于按业务模块管理权限，支持批量授权
// - Method: HTTP方法，支持RESTful API设计，默认POST
type ApiCreateRequest struct {
	Path        string `json:"path"`        // API路径
	Description string `json:"description"` // API中文描述
	ApiGroup    string `json:"apiGroup"`    // API组
	Method      string `json:"method"`      // HTTP方法
}

// ApiCreateResponse API创建响应结构
//
// 设计原理：
// 1. 统一响应格式：所有API创建操作返回相同结构，便于客户端统一处理
// 2. 详细反馈：包含成功状态、消息、ID等信息，便于后续操作和调试
// 3. 幂等性支持：返回创建的API ID，支持重复调用时的去重判断
//
// 好处：
// - 可追溯：通过ApiID可以追踪创建的API记录
// - 可调试：详细的Message帮助定位问题
// - 可扩展：可以添加更多字段而不破坏现有代码
type ApiCreateResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 操作结果消息
	ApiID   uint   `json:"apiId"`   // 创建的API记录ID
	Path    string `json:"path"`    // API路径（用于确认）
	Method  string `json:"method"`  // HTTP方法（用于确认）
}

// ApiCreator API创建工具
//
// 设计原理（工具模式）：
// 1. 单一职责：专门负责API记录的创建，职责清晰
// 2. 无状态设计：结构体为空，所有状态通过参数传递，保证线程安全
// 3. 接口实现：实现McpTool接口，符合依赖倒置原则
//
// 为什么使用空结构体：
// - 内存效率：空结构体不占用内存空间（0字节）
// - 语义清晰：表示这是一个工具类，不需要存储状态
// - 线程安全：无状态设计天然支持并发调用
type ApiCreator struct{}

// New 创建API创建工具的元数据定义
//
// 设计原理：
// 1. 元数据驱动：通过工具定义描述工具的能力和参数，AI可以根据描述自动调用
// 2. 参数验证：使用Required()标记必填参数，在调用前进行验证，提前发现错误
// 3. 默认值支持：为method提供默认值"POST"，减少调用方的参数传递
// 4. 批量操作支持：通过apis参数支持批量创建，提高效率
//
// 为什么返回mcp.Tool而不是直接处理：
// - 分离关注点：元数据定义与业务逻辑分离，便于工具发现和文档生成
// - 动态注册：MCP服务器可以根据元数据动态注册工具，无需硬编码
// - AI友好：AI可以通过元数据理解工具用途，自动选择合适的工具
//
// 参数设计考虑：
// - path/description/apiGroup必填：这些是API的核心标识，缺少会导致权限管理混乱
// - method可选且默认POST：大多数管理后台API使用POST，提供默认值减少冗余
// - apis支持批量：批量操作减少网络往返，提高性能
func (a *ApiCreator) New() mcp.Tool {
	return mcp.NewTool("create_api",
		mcp.WithDescription(`创建后端API记录，用于AI编辑器自动添加API接口时自动创建对应的API权限记录。

**重要限制：**
- 当使用gva_auto_generate工具且needCreatedModules=true时，模块创建会自动生成API权限，不应调用此工具
- 仅在以下情况使用：1) 单独创建API（不涉及模块创建）；2) AI编辑器自动添加API；3) router下的文件产生路径变化时`),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("API路径，如：/user/create"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("API中文描述，如：创建用户"),
		),
		mcp.WithString("apiGroup",
			mcp.Required(),
			mcp.Description("API组名称，用于分类管理，如：用户管理"),
		),
		mcp.WithString("method",
			mcp.Description("HTTP方法"),
			mcp.DefaultString("POST"),
		),
		mcp.WithString("apis",
			mcp.Description("批量创建API的JSON字符串，格式：[{\"path\":\"/user/create\",\"description\":\"创建用户\",\"apiGroup\":\"用户管理\",\"method\":\"POST\"}]"),
		),
	)
}

// Handle 处理API创建请求
//
// 设计原理：
// 1. 统一入口：单个和批量创建使用同一个处理函数，减少代码重复
// 2. 参数归一化：将单个参数转换为数组格式，统一后续处理逻辑
// 3. 错误提前返回：参数验证失败立即返回，避免无效的数据库操作
// 4. 上下文传递：使用context支持超时控制和取消操作
//
// 处理流程：
// 1. 参数解析：检查是批量创建（apis参数）还是单个创建（path等参数）
// 2. 参数验证：确保必填参数存在且有效
// 3. 批量处理：统一转换为数组格式，便于循环处理
// 4. 结果聚合：收集所有操作结果，提供详细的反馈信息
//
// 为什么支持两种输入方式：
// - 灵活性：单个创建适合简单场景，批量创建适合复杂场景
// - 兼容性：支持AI工具的不同调用方式
// - 效率：批量创建减少多次工具调用的开销
//
// 好处：
// - 容错性：单个失败不影响其他API的创建
// - 可追溯：详细的响应信息便于调试和审计
// - 性能：批量操作减少数据库连接次数
func (a *ApiCreator) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	var apis []ApiCreateRequest

	// 检查是否是批量创建
	// 设计考虑：优先检查批量参数，因为批量操作更高效
	// 使用类型断言确保参数类型正确，避免运行时错误
	if apisStr, ok := args["apis"].(string); ok && apisStr != "" {
		// 批量创建：解析JSON字符串为API请求数组
		// 好处：一次调用创建多个API，减少网络往返和工具调用次数
		if err := json.Unmarshal([]byte(apisStr), &apis); err != nil {
			return nil, fmt.Errorf("apis 参数格式错误: %v", err)
		}
	} else {
		// 单个API创建：从独立参数构建请求对象
		// 设计考虑：提供更直观的调用方式，适合简单场景
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.New("path 参数是必需的")
		}

		description, ok := args["description"].(string)
		if !ok || description == "" {
			return nil, errors.New("description 参数是必需的")
		}

		apiGroup, ok := args["apiGroup"].(string)
		if !ok || apiGroup == "" {
			return nil, errors.New("apiGroup 参数是必需的")
		}

		// 默认值处理：如果没有提供method，使用POST
		// 设计考虑：大多数管理后台API使用POST，提供默认值减少参数传递
		method := "POST"
		if val, ok := args["method"].(string); ok && val != "" {
			method = val
		}

		// 将单个参数转换为数组格式，统一后续处理
		apis = append(apis, ApiCreateRequest{
			Path:        path,
			Description: description,
			ApiGroup:    apiGroup,
			Method:      method,
		})
	}

	// 边界检查：确保至少有一个API需要创建
	// 设计考虑：提前验证，避免无效的循环和数据库操作
	if len(apis) == 0 {
		return nil, errors.New("没有要创建的API")
	}

	// 创建API记录
	// 设计原理：
	// 1. 服务层调用：通过service层创建，而不是直接操作数据库，符合分层架构
	// 2. 批量处理：循环处理所有API，支持部分成功部分失败
	// 3. 结果收集：记录每个API的创建结果，提供详细反馈
	//
	// 为什么使用service层而不是直接操作数据库：
	// - 业务逻辑封装：service层可能包含额外的业务逻辑（如权限检查、日志记录）
	// - 代码复用：避免重复实现创建逻辑
	// - 易于测试：可以mock service层进行单元测试
	apiService := service.ServiceGroupApp.SystemServiceGroup.ApiService
	var responses []ApiCreateResponse
	successCount := 0

	// 循环处理每个API创建请求
	// 设计考虑：即使某个API创建失败，也继续处理其他API，提高容错性
	for _, apiReq := range apis {
		// 构建数据库模型对象
		// 设计考虑：使用系统定义的SysApi模型，保证数据结构一致性
		api := system.SysApi{
			Path:        apiReq.Path,
			Description: apiReq.Description,
			ApiGroup:    apiReq.ApiGroup,
			Method:      apiReq.Method,
		}

		// 调用service层创建API
		// 好处：service层可能包含去重逻辑、权限检查等
		err := apiService.CreateApi(api)
		if err != nil {
			// 失败处理：记录日志但不中断整个流程
			// 设计考虑：使用Warn级别，因为部分失败是可接受的
			// 结构化日志：使用zap的结构化日志，便于日志分析和监控
			global.GVA_LOG.Warn("创建API失败",
				zap.String("path", apiReq.Path),
				zap.String("method", apiReq.Method),
				zap.Error(err))

			// 记录失败结果，便于调用方了解哪些API创建失败
			responses = append(responses, ApiCreateResponse{
				Success: false,
				Message: fmt.Sprintf("创建API失败: %v", err),
				Path:    apiReq.Path,
				Method:  apiReq.Method,
			})
		} else {
			// 成功处理：查询创建的API记录以获取ID
			// 设计考虑：CreateApi可能不返回ID，需要额外查询
			// 使用path和method作为唯一标识查询，因为这两个字段组合应该是唯一的
			var createdApi system.SysApi
			err = global.GVA_DB.Where("path = ? AND method = ?", apiReq.Path, apiReq.Method).First(&createdApi).Error
			if err != nil {
				// 查询失败不影响整体流程，只记录警告
				// 设计考虑：API已创建成功，只是获取ID失败，不影响功能
				global.GVA_LOG.Warn("获取创建的API ID失败", zap.Error(err))
			}

			// 记录成功结果，包含API ID便于后续操作
			responses = append(responses, ApiCreateResponse{
				Success: true,
				Message: fmt.Sprintf("成功创建API %s %s", apiReq.Method, apiReq.Path),
				ApiID:   createdApi.ID,
				Path:    apiReq.Path,
				Method:  apiReq.Method,
			})
			successCount++
		}
	}

	// 构建总体响应
	// 设计原理：
	// 1. 消息差异化：单个创建返回简单消息，批量创建返回统计信息
	// 2. 结构化数据：使用map提供结构化的响应，便于程序化处理
	// 3. 详细信息：包含总数、成功数、失败数和详细列表，满足不同需求
	//
	// 为什么使用map而不是固定结构体：
	// - 灵活性：可以动态添加字段而不影响调用方
	// - 可扩展性：便于后续添加更多统计信息
	// - 兼容性：JSON格式易于各种语言解析
	var resultMessage string
	if len(apis) == 1 {
		// 单个创建：直接返回该API的创建结果消息
		resultMessage = responses[0].Message
	} else {
		// 批量创建：提供统计信息，让调用方快速了解整体情况
		resultMessage = fmt.Sprintf("批量创建API完成，成功 %d 个，失败 %d 个", successCount, len(apis)-successCount)
	}

	// 构建结构化响应
	// 字段设计：
	// - success: 是否有任何API创建成功（用于快速判断）
	// - message: 人类可读的摘要信息
	// - totalCount/successCount/failedCount: 统计数据，便于程序化处理
	// - details: 每个API的详细结果，便于定位问题
	result := map[string]interface{}{
		"success":      successCount > 0,                    // 至少有一个成功即认为整体成功
		"message":      resultMessage,                       // 摘要消息
		"totalCount":   len(apis),                          // 总数
		"successCount": successCount,                       // 成功数
		"failedCount":  len(apis) - successCount,           // 失败数
		"details":      responses,                          // 详细结果列表
	}

	// JSON序列化：使用Indent格式化，提高可读性
	// 设计考虑：虽然会增加一些字节，但可读性对调试和日志记录很重要
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化结果失败: %v", err)
	}

	// 返回MCP标准格式的响应
	// 设计原理：
	// 1. 标准格式：使用mcp.CallToolResult和mcp.TextContent，符合MCP协议
	// 2. 文本内容：使用TextContent类型，便于AI理解和展示
	// 3. 格式化输出：包含标题和JSON，提高可读性
	//
	// 好处：
	// - 协议兼容：符合MCP标准，可以被任何MCP客户端正确处理
	// - AI友好：文本格式便于AI理解和提取信息
	// - 可读性强：格式化的JSON便于人类阅读和调试
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("API创建结果：\n\n%s", string(resultJSON)),
			},
		},
	}, nil
}
