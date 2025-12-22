package mcpTool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

// 注册工具到MCP工具注册表
// 
// 设计原理：
// - 利用Go的包初始化机制，在包导入时自动注册工具
// - 注释说明注册将在enter.go中统一处理，便于理解工具注册流程
func init() {
	// 注册工具将在enter.go中统一处理
	RegisterTool(&ApiLister{})
}

// ApiInfo API信息结构
//
// 设计原理：
// 1. 统一格式：数据库API和gin路由API使用相同结构，便于统一处理
// 2. 来源标识：通过Source字段区分API来源，便于AI判断是否需要创建新API
// 3. 可选字段：使用omitempty标签，避免返回空值，减少响应体积
//
// 字段设计考虑：
// - ID: 仅数据库API有ID，gin路由API没有，使用omitempty避免返回0值
// - Path/Method: 所有API都有，作为核心标识
// - Description/ApiGroup: 仅数据库API有，gin路由需要AI自行推断
// - Source: 明确标识来源，帮助AI理解API的状态（已注册 vs 仅路由）
//
// 好处：
// - 可比较性：相同结构的API便于比较和去重
// - AI友好：清晰的来源标识帮助AI做出正确决策
// - 扩展性：可以添加更多字段而不破坏现有结构
type ApiInfo struct {
	ID          uint   `json:"id,omitempty"`          // 数据库ID（仅数据库API有）
	Path        string `json:"path"`                  // API路径
	Description string `json:"description,omitempty"` // API描述
	ApiGroup    string `json:"apiGroup,omitempty"`    // API组
	Method      string `json:"method"`                // HTTP方法
	Source      string `json:"source"`                // 来源：database 或 gin
}

// ApiListResponse API列表响应结构
//
// 设计原理：
// 1. 分类返回：将数据库API和gin路由API分开返回，便于AI理解不同来源
// 2. 统计信息：提供总数便于快速了解系统规模
// 3. 统一格式：使用标准的success/message格式，便于错误处理
//
// 为什么分开返回databaseApis和ginApis：
// - 语义清晰：AI可以明确知道哪些API已注册，哪些仅存在于路由中
// - 决策支持：帮助AI判断是否需要调用create_api工具创建新API
// - 问题诊断：便于发现路由和数据库不一致的问题
//
// 好处：
// - 可操作性：AI可以根据来源决定后续操作
// - 可调试性：分开的数据便于定位问题
// - 完整性：提供所有API信息，不遗漏任何路由
type ApiListResponse struct {
	Success      bool      `json:"success"`      // 是否成功
	Message      string    `json:"message"`      // 操作结果消息
	DatabaseApis []ApiInfo `json:"databaseApis"` // 数据库中的API（已注册，有完整元数据）
	GinApis      []ApiInfo `json:"ginApis"`      // gin框架中的API（仅路由，无元数据）
	TotalCount   int       `json:"totalCount"`   // 总数量（便于快速了解规模）
}

// ApiLister API列表工具
//
// 设计原理（查询工具模式）：
// 1. 只读操作：不修改任何数据，只查询和返回信息
// 2. 数据聚合：从多个数据源（数据库、gin路由）聚合信息
// 3. 无状态设计：空结构体，保证线程安全
//
// 为什么需要这个工具：
// - AI决策支持：AI需要知道系统中已有哪些API，避免重复创建
// - 系统分析：帮助了解系统的API结构和覆盖情况
// - 一致性检查：可以发现路由和数据库不一致的问题
type ApiLister struct{}

// New 创建API列表工具的元数据定义
//
// 设计原理：
// 1. 功能说明：详细描述工具返回的数据结构和用途，帮助AI理解何时使用
// 2. 占位参数：使用_placeholder参数避免MCP协议的JSON schema校验失败
// 3. 使用指导：在描述中说明如何使用返回的数据，提高AI的使用准确性
//
// 为什么需要占位参数：
// - MCP协议要求：某些MCP实现要求工具至少有一个参数
// - 兼容性：避免因参数为空导致的协议校验错误
// - 不影响功能：占位参数不影响实际功能，只是满足协议要求
//
// 描述中的关键信息：
// - 数据分类：明确说明返回两组数据（databaseApis和ginApis）
// - 使用场景：说明何时需要创建新API，何时使用现有API
// - 数据结构：说明ginApis需要AI自行推断业务含义
func (a *ApiLister) New() mcp.Tool {
	return mcp.NewTool("list_all_apis",
		mcp.WithDescription(`获取系统中所有的API接口，分为两组：

**功能说明：**
- 返回数据库中已注册的API列表
- 返回gin框架中实际注册的路由API列表
- 帮助前端判断是使用现有API还是需要创建新的API,如果api在前端未使用且需要前端调用的时候，请到api文件夹下对应模块的js中添加方法并暴露给当前业务调用

**返回数据结构：**
- databaseApis: 数据库中的API记录（包含ID、描述、分组等完整信息）
- ginApis: gin路由中的API（仅包含路径和方法），需要AI根据路径自行揣摩路径的业务含义，例如：/api/user/:id 表示根据用户ID获取用户信息`),
		mcp.WithString("_placeholder",
			mcp.Description("占位符，防止json schema校验失败"),
		),	
	)
}

// Handle 处理API列表请求
//
// 设计原理：
// 1. 容错处理：即使某个数据源失败，也返回部分结果，提高可用性
// 2. 错误返回：使用标准响应格式返回错误，而不是直接返回error，便于AI处理
// 3. 数据聚合：从两个独立的数据源获取数据，统一返回
// 4. 忽略参数：使用_表示不使用context和request参数，因为这是查询操作
//
// 处理流程：
// 1. 获取数据库API：从数据库查询已注册的API
// 2. 获取gin路由API：从gin路由表获取所有路由
// 3. 聚合结果：将两组数据合并返回
// 4. 错误处理：每个步骤独立处理错误，不相互影响
//
// 为什么错误时返回nil error：
// - AI友好：AI可以解析响应中的success字段判断是否成功
// - 部分成功：即使一个数据源失败，另一个数据源的结果仍然有用
// - 统一格式：所有响应都使用相同的JSON格式，便于处理
//
// 好处：
// - 高可用性：部分失败不影响整体功能
// - 可调试性：详细的错误信息便于定位问题
// - 一致性：统一的响应格式便于AI理解
func (a *ApiLister) Handle(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	// 获取数据库中的API
	// 设计考虑：优先获取数据库API，因为这是更完整的信息源
	databaseApis, err := a.getDatabaseApis()
	if err != nil {
		// 错误处理：记录日志并返回错误响应，但不中断流程
		// 设计考虑：即使数据库查询失败，gin路由信息仍然有价值
		global.GVA_LOG.Error("获取数据库API失败", zap.Error(err))
		errorResponse := ApiListResponse{
			Success: false,
			Message: "获取数据库API失败: " + err.Error(),
		}
		resultJSON, _ := json.Marshal(errorResponse)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(resultJSON),
				},
			},
		}, nil
	}

	// 获取gin路由中的API
	// 设计考虑：从gin路由表获取，包含所有已注册的路由（包括被忽略的）
	ginApis, err := a.getGinApis()
	if err != nil {
		// 错误处理：即使gin路由获取失败，数据库API信息仍然有用
		global.GVA_LOG.Error("获取gin路由API失败", zap.Error(err))
		errorResponse := ApiListResponse{
			Success: false,
			Message: "获取gin路由API失败: " + err.Error(),
		}
		resultJSON, _ := json.Marshal(errorResponse)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(resultJSON),
				},
			},
		}, nil
	}

	// 构建响应
	// 设计考虑：聚合两个数据源的结果，提供完整的API视图
	response := ApiListResponse{
		Success:      true,
		Message:      "获取API列表成功",
		DatabaseApis: databaseApis,
		GinApis:      ginApis,
		TotalCount:   len(databaseApis) + len(ginApis),
	}

	// 记录日志：使用结构化日志记录统计信息
	// 设计考虑：便于监控和调试，了解系统的API规模
	global.GVA_LOG.Info("API列表获取成功",
		zap.Int("数据库API数量", len(databaseApis)),
		zap.Int("gin路由API数量", len(ginApis)),
		zap.Int("总数量", response.TotalCount))

	// 序列化响应：使用标准JSON格式
	resultJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("序列化结果失败: %v", err)
	}

	// 返回MCP标准格式的响应
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}

// getDatabaseApis 获取数据库中的所有API
//
// 设计原理：
// 1. 排序查询：按api_group和path排序，便于阅读和分析
// 2. 数据转换：将数据库模型转换为统一的ApiInfo格式
// 3. 来源标记：明确标记来源为"database"，便于AI区分
//
// 为什么使用Order排序：
// - 可读性：按分组和路径排序，便于人类阅读
// - 一致性：固定的排序顺序便于比较和去重
// - 分组管理：按api_group排序便于按业务模块查看
//
// 好处：
// - 完整性：获取所有已注册的API，包括完整的元数据
// - 准确性：直接从数据库查询，数据准确可靠
// - 结构化：包含ID、描述、分组等完整信息
func (a *ApiLister) getDatabaseApis() ([]ApiInfo, error) {
	var apis []system.SysApi
	// 查询所有API并按分组和路径排序
	// 设计考虑：使用Model()明确指定模型，Order()指定排序规则
	err := global.GVA_DB.Model(&system.SysApi{}).Order("api_group ASC, path ASC").Find(&apis).Error
	if err != nil {
		return nil, err
	}

	// 转换为ApiInfo格式
	// 设计考虑：统一数据格式，便于后续处理和AI理解
	var result []ApiInfo
	for _, api := range apis {
		result = append(result, ApiInfo{
			ID:          api.ID,
			Path:        api.Path,
			Description: api.Description,
			ApiGroup:    api.ApiGroup,
			Method:      api.Method,
			Source:      "database", // 明确标记来源
		})
	}

	return result, nil
}

// getGinApis 获取gin路由中的所有API（包含被忽略的API）
//
// 设计原理：
// 1. 路由遍历：从全局路由表获取所有已注册的路由
// 2. 最小信息：只包含path和method，因为gin路由没有其他元数据
// 3. 来源标记：明确标记来源为"gin"，便于AI区分
//
// 为什么需要获取gin路由API：
// - 完整性：有些路由可能没有在数据库中注册，但仍在使用
// - 一致性检查：可以发现路由和数据库不一致的问题
// - AI决策：帮助AI了解所有可用的路由，避免创建重复路由
//
// 设计考虑：
// - 包含被忽略的API：global.GVA_ROUTERS包含所有路由，包括被忽略的
// - 最小数据：gin路由只有path和method，其他字段为空
// - 快速查询：直接从内存中的路由表获取，无需数据库查询
//
// 好处：
// - 全面性：获取所有路由，不遗漏任何API
// - 性能：内存查询，速度快
// - 实时性：反映当前实际注册的路由状态
func (a *ApiLister) getGinApis() ([]ApiInfo, error) {
	// 从gin路由信息中获取所有API
	// 设计考虑：global.GVA_ROUTERS在系统启动时已填充所有路由信息
	var result []ApiInfo
	for _, route := range global.GVA_ROUTERS {
		result = append(result, ApiInfo{
			Path:   route.Path,
			Method: route.Method,
			Source: "gin", // 明确标记来源，AI需要知道这是路由而非数据库记录
		})
	}

	return result, nil
}
