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
	RegisterTool(&MenuLister{})
}

// MenuListResponse 菜单列表响应结构
//
// 设计原理：
// 1. 完整数据：返回完整的菜单对象，包含所有字段和关联数据
// 2. 统计信息：提供总数便于快速了解系统规模
// 3. 描述信息：Description字段说明数据用途，帮助AI理解如何使用
//
// 为什么返回完整的SysBaseMenu对象：
// - 完整性：包含所有菜单信息，无需额外查询
// - 关联数据：通过Preload加载Parameters和MenuBtn，提供完整视图
// - 前端需求：前端路由配置需要完整的菜单信息
//
// 好处：
// - 一次性获取：避免多次查询，提高效率
// - 数据一致性：所有菜单信息来自同一查询，保证一致性
// - AI友好：完整的数据帮助AI做出正确决策
type MenuListResponse struct {
	Success     bool                  `json:"success"`     // 是否成功
	Message     string                `json:"message"`     // 操作结果消息
	Menus       []system.SysBaseMenu  `json:"menus"`       // 菜单列表（包含完整信息）
	TotalCount  int                   `json:"totalCount"`  // 总数量
	Description string                `json:"description"` // 数据描述（帮助AI理解）
}

// MenuLister 菜单列表工具
//
// 设计原理（查询工具模式）：
// 1. 只读操作：不修改任何数据，只查询和返回信息
// 2. 数据完整性：使用Preload加载关联数据，提供完整视图
// 3. 无状态设计：空结构体，保证线程安全
//
// 为什么需要这个工具：
// - AI决策支持：AI需要知道系统中已有哪些菜单，避免重复创建
// - 前端路由配置：前端需要完整的菜单信息来配置路由
// - 系统分析：帮助了解系统的菜单结构和页面组织
type MenuLister struct{}

// New 创建菜单列表工具的元数据定义
//
// 设计原理：
// 1. 功能说明：详细描述工具返回的数据结构和用途，帮助AI理解何时使用
// 2. 占位参数：使用_placeholder参数避免MCP协议的JSON schema校验失败
// 3. 使用场景：在描述中说明多个使用场景，提高AI的使用准确性
//
// 为什么需要占位参数：
// - MCP协议要求：某些MCP实现要求工具至少有一个参数
// - 兼容性：避免因参数为空导致的协议校验错误
// - 不影响功能：占位参数不影响实际功能，只是满足协议要求
//
// 描述中的关键信息：
// - 数据内容：说明返回完整的菜单树形结构
// - 使用场景：列出多个典型使用场景，帮助AI判断何时调用
// - 数据结构：说明包含路由配置、元数据、参数和按钮等信息
func (m *MenuLister) New() mcp.Tool {
	return mcp.NewTool("list_all_menus",
		mcp.WithDescription(`获取系统中所有菜单信息，包括菜单树结构、路由信息、组件路径等，用于前端编写vue-router时正确跳转

**功能说明：**
- 返回完整的菜单树形结构
- 包含路由配置信息（path、name、component）
- 包含菜单元数据（title、icon、keepAlive等）
- 包含菜单参数和按钮配置
- 支持父子菜单关系展示

**使用场景：**
- 前端路由配置：获取所有菜单信息用于配置vue-router
- 菜单权限管理：了解系统中所有可用的菜单项
- 导航组件开发：构建动态导航菜单
- 系统架构分析：了解系统的菜单结构和页面组织`),
mcp.WithString("_placeholder",
			mcp.Description("占位符，防止json schema校验失败"),
		),	
	)
}

// Handle 处理菜单列表请求
//
// 设计原理：
// 1. 错误处理：使用IsError标记错误响应，而不是直接返回error，便于AI处理
// 2. 数据完整性：返回完整的菜单对象，包含所有关联数据
// 3. 忽略参数：使用_表示不使用context和request参数，因为这是查询操作
//
// 处理流程：
// 1. 查询菜单：从数据库获取所有菜单，包含关联数据
// 2. 构建响应：封装为MenuListResponse结构
// 3. 序列化：转换为JSON格式返回
//
// 为什么错误时返回IsError而不是error：
// - AI友好：AI可以解析响应中的IsError字段判断是否成功
// - 统一格式：所有响应都使用相同的JSON格式，便于处理
// - 部分成功：即使查询失败，也可以返回错误信息给AI
//
// 好处：
// - 可调试性：详细的错误信息便于定位问题
// - 一致性：统一的响应格式便于AI理解
// - 完整性：包含所有菜单信息，无需额外查询
func (m *MenuLister) Handle(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 获取所有基础菜单
	// 设计考虑：使用getAllMenus方法封装查询逻辑，便于维护和测试
	allMenus, err := m.getAllMenus()
	if err != nil {
		// 错误处理：记录日志并返回错误响应
		global.GVA_LOG.Error("获取菜单列表失败", zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("获取菜单列表失败: %v", err),
				},
			},
			IsError: true, // 标记为错误响应，便于AI识别
		}, nil
	}

	// 构建返回结果
	// 设计考虑：提供描述信息帮助AI理解数据用途
	response := MenuListResponse{
		Success:     true,
		Message:     "获取菜单列表成功",
		Menus:       allMenus,
		TotalCount:  len(allMenus),
		Description: "系统中所有菜单信息的标准列表，包含路由配置和组件信息",
	}

	// 序列化响应：使用Indent格式化，提高可读性
	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		// 序列化失败处理：记录日志并返回错误响应
		global.GVA_LOG.Error("序列化菜单响应失败", zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("序列化响应失败: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// 返回MCP标准格式的响应
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(responseJSON),
			},
		},
	}, nil
}

// getAllMenus 获取所有基础菜单
//
// 设计原理：
// 1. 排序查询：按sort字段排序，保证菜单顺序正确
// 2. 预加载关联：使用Preload加载Parameters和MenuBtn，避免N+1查询问题
// 3. 完整数据：返回完整的菜单对象，包含所有字段和关联数据
//
// 为什么使用Preload：
// - 性能优化：一次性加载所有关联数据，避免多次查询（N+1问题）
// - 数据完整性：确保返回的菜单对象包含所有必要信息
// - 查询效率：减少数据库往返次数，提高性能
//
// 为什么按sort排序：
// - 显示顺序：菜单通常按sort字段排序显示
// - 一致性：固定的排序顺序便于前端处理
// - 可读性：有序的数据便于人类阅读和理解
//
// 好处：
// - 性能：预加载避免N+1查询问题，提高查询效率
// - 完整性：包含所有菜单信息，无需额外查询
// - 准确性：直接从数据库查询，数据准确可靠
func (m *MenuLister) getAllMenus() ([]system.SysBaseMenu, error) {
	var menus []system.SysBaseMenu
	// 查询所有菜单，按sort排序，并预加载关联数据
	// 设计考虑：
	// - Order("sort")：按排序字段排序，保证菜单顺序
	// - Preload("Parameters")：预加载路由参数，避免后续查询
	// - Preload("MenuBtn")：预加载菜单按钮，避免后续查询
	err := global.GVA_DB.Order("sort").Preload("Parameters").Preload("MenuBtn").Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

