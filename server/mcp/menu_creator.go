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
func init() {
	RegisterTool(&MenuCreator{})
}

// MenuCreateRequest 菜单创建请求结构
//
// 设计原理：
// 1. 完整配置：包含前端路由所需的所有配置项，支持复杂的菜单场景
// 2. 树形结构：通过ParentId支持菜单的层级关系，构建菜单树
// 3. 扩展性：包含Parameters和MenuBtn，支持动态路由和按钮权限
//
// 字段分类说明：
// - 路由相关：Path、Name、Component（Vue Router必需）
// - 显示相关：Title、Icon、Hidden、Sort（菜单展示控制）
// - 功能相关：KeepAlive、DefaultMenu、CloseTab、ActiveName（页面行为控制）
// - 结构相关：ParentId（菜单层级）、Parameters（路由参数）、MenuBtn（按钮权限）
//
// 为什么需要这么多字段：
// - 前端路由系统复杂：需要支持多种路由场景和页面行为
// - 权限管理：MenuBtn用于按钮级权限控制
// - 用户体验：KeepAlive、CloseTab等提升用户体验
//
// 好处：
// - 一次性配置：所有菜单相关配置在一个请求中完成
// - 灵活性：支持各种复杂的菜单场景
// - 可维护性：结构化的配置便于管理和修改
type MenuCreateRequest struct {
	ParentId    uint                   `json:"parentId"`    // 父菜单ID，0表示根菜单
	Path        string                 `json:"path"`        // 路由path
	Name        string                 `json:"name"`        // 路由name
	Hidden      bool                   `json:"hidden"`      // 是否在列表隐藏
	Component   string                 `json:"component"`   // 对应前端文件路径
	Sort        int                    `json:"sort"`        // 排序标记
	Title       string                 `json:"title"`       // 菜单名
	Icon        string                 `json:"icon"`        // 菜单图标
	KeepAlive   bool                   `json:"keepAlive"`   // 是否缓存
	DefaultMenu bool                   `json:"defaultMenu"` // 是否是基础路由
	CloseTab    bool                   `json:"closeTab"`    // 自动关闭tab
	ActiveName  string                 `json:"activeName"`  // 高亮菜单
	Parameters  []MenuParameterRequest `json:"parameters"`  // 路由参数
	MenuBtn     []MenuButtonRequest    `json:"menuBtn"`     // 菜单按钮
}

// MenuParameterRequest 菜单参数请求结构
//
// 设计原理：
// 1. 路由参数支持：支持Vue Router的params和query两种参数类型
// 2. 动态路由：通过参数实现动态路由，如 /user/:id
// 3. 参数预设：Value字段可以预设参数值，用于默认路由
//
// 为什么需要这个结构：
// - Vue Router支持：Vue Router需要参数配置才能正确路由
// - 动态页面：支持根据参数显示不同内容的页面
// - 默认值：可以预设参数值，提供更好的用户体验
type MenuParameterRequest struct {
	Type  string `json:"type"`  // 参数类型：params或query
	Key   string `json:"key"`   // 参数key
	Value string `json:"value"` // 参数值
}

// MenuButtonRequest 菜单按钮请求结构
//
// 设计原理：
// 1. 按钮权限：每个按钮可以单独配置权限，实现细粒度权限控制
// 2. 按钮元数据：Name用于权限标识，Desc用于界面展示
// 3. 权限管理：与RBAC系统集成，支持按钮级权限控制
//
// 为什么需要按钮权限：
// - 细粒度控制：不同角色可能对同一页面有不同的操作权限
// - 安全性：防止未授权操作，提升系统安全性
// - 灵活性：可以根据业务需求灵活配置按钮权限
type MenuButtonRequest struct {
	Name string `json:"name"` // 按钮名称（用于权限标识）
	Desc string `json:"desc"` // 按钮描述（用于界面展示）
}

// MenuCreateResponse 菜单创建响应结构
//
// 设计原理：
// 1. 统一响应格式：与API创建工具保持一致，便于统一处理
// 2. 关键信息：返回MenuID、Name、Path等关键信息，便于后续操作
// 3. 状态反馈：Success和Message提供操作结果反馈
//
// 好处：
// - 可追溯：通过MenuID可以追踪创建的菜单记录
// - 可调试：详细的Message帮助定位问题
// - 可扩展：可以添加更多字段而不破坏现有代码
type MenuCreateResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 操作结果消息
	MenuID  uint   `json:"menuId"`  // 创建的菜单记录ID
	Name    string `json:"name"`    // 路由name（用于确认）
	Path    string `json:"path"`    // 路由path（用于确认）
}

// MenuCreator 菜单创建工具
//
// 设计原理（工具模式）：
// 1. 单一职责：专门负责菜单记录的创建，职责清晰
// 2. 无状态设计：结构体为空，所有状态通过参数传递，保证线程安全
// 3. 接口实现：实现McpTool接口，符合依赖倒置原则
//
// 为什么使用空结构体：
// - 内存效率：空结构体不占用内存空间（0字节）
// - 语义清晰：表示这是一个工具类，不需要存储状态
// - 线程安全：无状态设计天然支持并发调用
type MenuCreator struct{}

// New 创建菜单创建工具的元数据定义
//
// 设计原理：
// 1. 元数据驱动：通过工具定义描述工具的能力和参数，AI可以根据描述自动调用
// 2. 参数验证：使用Required()标记必填参数，在调用前进行验证，提前发现错误
// 3. 默认值支持：为常用参数提供默认值，减少调用方的参数传递
// 4. JSON参数：复杂结构（parameters、menuBtn）使用JSON字符串传递，简化参数定义
//
// 参数设计考虑：
// - 必填参数：path、name、component、title是菜单的核心标识，必须提供
// - 默认值：parentId默认为0（根菜单）、sort默认为1、icon默认为"menu"
// - JSON参数：parameters和menuBtn是数组，使用JSON字符串便于传递
//
// 为什么使用JSON字符串传递复杂结构：
// - MCP协议限制：MCP工具参数不支持复杂嵌套结构
// - 灵活性：JSON格式便于扩展，可以添加更多字段
// - 兼容性：字符串类型在所有MCP实现中都支持
//
// 好处：
// - AI友好：清晰的参数描述帮助AI正确调用工具
// - 易用性：默认值减少参数传递，提高易用性
// - 扩展性：JSON格式便于后续添加更多配置项
func (m *MenuCreator) New() mcp.Tool {
	return mcp.NewTool("create_menu",
		mcp.WithDescription(`创建前端菜单记录，用于AI编辑器自动添加前端页面时自动创建对应的菜单项。

**重要限制：**
- 当使用gva_auto_generate工具且needCreatedModules=true时，模块创建会自动生成菜单项，不应调用此工具
- 仅在以下情况使用：1) 单独创建菜单（不涉及模块创建）；2) AI编辑器自动添加前端页面时`),
		mcp.WithNumber("parentId",
			mcp.Description("父菜单ID，0表示根菜单"),
			mcp.DefaultNumber(0),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("路由path，如：userList"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("路由name，用于Vue Router，如：userList"),
		),
		mcp.WithBoolean("hidden",
			mcp.Description("是否在菜单列表中隐藏"),
		),
		mcp.WithString("component",
			mcp.Required(),
			mcp.Description("对应的前端Vue组件路径，如：view/user/list.vue"),
		),
		mcp.WithNumber("sort",
			mcp.Description("菜单排序号，数字越小越靠前"),
			mcp.DefaultNumber(1),
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("菜单显示标题"),
		),
		mcp.WithString("icon",
			mcp.Description("菜单图标名称"),
			mcp.DefaultString("menu"),
		),
		mcp.WithBoolean("keepAlive",
			mcp.Description("是否缓存页面"),
		),
		mcp.WithBoolean("defaultMenu",
			mcp.Description("是否是基础路由"),
		),
		mcp.WithBoolean("closeTab",
			mcp.Description("是否自动关闭tab"),
		),
		mcp.WithString("activeName",
			mcp.Description("高亮菜单名称"),
		),
		mcp.WithString("parameters",
			mcp.Description("路由参数JSON字符串，格式：[{\"type\":\"params\",\"key\":\"id\",\"value\":\"1\"}]"),
		),
		mcp.WithString("menuBtn",
			mcp.Description("菜单按钮JSON字符串，格式：[{\"name\":\"add\",\"desc\":\"新增\"}]"),
		),
	)
}

// Handle 处理菜单创建请求
//
// 设计原理：
// 1. 参数解析：分必填和可选参数处理，必填参数立即验证，可选参数提供默认值
// 2. 类型转换：MCP协议中数字是float64，需要转换为int/uint类型
// 3. JSON解析：复杂结构从JSON字符串解析，支持数组和嵌套结构
// 4. 数据转换：将请求结构转换为数据库模型，保持数据结构一致性
//
// 处理流程：
// 1. 参数提取：从MCP请求中提取所有参数
// 2. 参数验证：验证必填参数是否存在且有效
// 3. 默认值处理：为可选参数设置默认值
// 4. JSON解析：解析parameters和menuBtn的JSON字符串
// 5. 模型构建：构建SysBaseMenu模型对象
// 6. 服务调用：通过service层创建菜单
// 7. 结果返回：返回创建结果和菜单ID
//
// 为什么需要类型转换：
// - MCP协议：JSON数字在Go中解析为float64
// - 类型安全：明确转换为目标类型，避免类型错误
// - 数据一致性：确保数据库存储的数据类型正确
//
// 好处：
// - 容错性：参数验证提前发现错误，避免无效的数据库操作
// - 灵活性：支持部分参数，使用默认值填充缺失参数
// - 可维护性：清晰的参数处理逻辑便于理解和修改
func (m *MenuCreator) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 解析请求参数
	args := request.GetArguments()

	// 必需参数验证
	// 设计考虑：path、name、component、title是菜单的核心标识，缺少会导致菜单无法正常使用
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, errors.New("path 参数是必需的")
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, errors.New("name 参数是必需的")
	}

	component, ok := args["component"].(string)
	if !ok || component == "" {
		return nil, errors.New("component 参数是必需的")
	}

	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, errors.New("title 参数是必需的")
	}

	// 可选参数处理
	// 设计原理：所有可选参数都有默认值，如果未提供则使用默认值
	// 类型转换：MCP协议中数字是float64，需要转换为Go的uint/int类型
	
	// 父菜单ID：默认为0表示根菜单
	// 设计考虑：大多数菜单是根菜单，默认值0减少参数传递
	parentId := uint(0)
	if val, ok := args["parentId"].(float64); ok {
		parentId = uint(val)
	}

	// 是否隐藏：默认为false，菜单默认显示
	hidden := false
	if val, ok := args["hidden"].(bool); ok {
		hidden = val
	}

	// 排序号：默认为1，数字越小越靠前
	// 设计考虑：大多数菜单使用默认排序即可
	sort := 1
	if val, ok := args["sort"].(float64); ok {
		sort = int(val)
	}

	// 图标：默认为"menu"，提供通用图标
	icon := "menu"
	if val, ok := args["icon"].(string); ok && val != "" {
		icon = val
	}

	// 页面缓存：默认为false，大多数页面不需要缓存
	keepAlive := false
	if val, ok := args["keepAlive"].(bool); ok {
		keepAlive = val
	}

	// 基础路由：默认为false，大多数菜单不是基础路由
	defaultMenu := false
	if val, ok := args["defaultMenu"].(bool); ok {
		defaultMenu = val
	}

	// 自动关闭tab：默认为false，tab默认不自动关闭
	closeTab := false
	if val, ok := args["closeTab"].(bool); ok {
		closeTab = val
	}

	// 高亮菜单：默认为空字符串，表示不特殊高亮
	activeName := ""
	if val, ok := args["activeName"].(string); ok {
		activeName = val
	}

	// 解析路由参数（JSON字符串）
	// 设计原理：parameters是数组结构，使用JSON字符串传递，解析后转换为数据库模型
	// 为什么使用JSON：MCP协议不支持复杂嵌套结构，JSON字符串是通用解决方案
	var parameters []system.SysBaseMenuParameter
	if parametersStr, ok := args["parameters"].(string); ok && parametersStr != "" {
		var paramReqs []MenuParameterRequest
		// JSON解析：将JSON字符串解析为请求结构数组
		if err := json.Unmarshal([]byte(parametersStr), &paramReqs); err != nil {
			return nil, fmt.Errorf("parameters 参数格式错误: %v", err)
		}
		// 数据转换：将请求结构转换为数据库模型
		// 设计考虑：保持数据结构一致性，便于数据库存储
		for _, param := range paramReqs {
			parameters = append(parameters, system.SysBaseMenuParameter{
				Type:  param.Type,
				Key:   param.Key,
				Value: param.Value,
			})
		}
	}

	// 解析菜单按钮（JSON字符串）
	// 设计原理：menuBtn是数组结构，用于按钮级权限控制
	// 为什么需要按钮权限：不同角色可能对同一页面有不同的操作权限
	var menuBtn []system.SysBaseMenuBtn
	if menuBtnStr, ok := args["menuBtn"].(string); ok && menuBtnStr != "" {
		var btnReqs []MenuButtonRequest
		// JSON解析：将JSON字符串解析为请求结构数组
		if err := json.Unmarshal([]byte(menuBtnStr), &btnReqs); err != nil {
			return nil, fmt.Errorf("menuBtn 参数格式错误: %v", err)
		}
		// 数据转换：将请求结构转换为数据库模型
		for _, btn := range btnReqs {
			menuBtn = append(menuBtn, system.SysBaseMenuBtn{
				Name: btn.Name,
				Desc: btn.Desc,
			})
		}
	}

	// 构建菜单对象
	// 设计原理：
	// 1. 模型映射：将请求参数映射到数据库模型，保持数据结构一致性
	// 2. 嵌套结构：Meta字段包含菜单的元数据，Parameters和MenuBtn是关联数据
	// 3. 完整性：包含所有菜单配置，确保菜单功能完整
	//
	// 为什么使用Meta嵌套结构：
	// - 数据组织：将菜单元数据（显示相关）与路由数据（功能相关）分离
	// - 模型设计：符合数据库模型设计，便于数据存储和查询
	// - 可维护性：清晰的数据结构便于理解和修改
	menu := system.SysBaseMenu{
		ParentId:  parentId,
		Path:      path,
		Name:      name,
		Hidden:    hidden,
		Component: component,
		Sort:      sort,
		Meta: system.Meta{
			Title:       title,
			Icon:        icon,
			KeepAlive:   keepAlive,
			DefaultMenu: defaultMenu,
			CloseTab:    closeTab,
			ActiveName:  activeName,
		},
		Parameters: parameters,
		MenuBtn:    menuBtn,
	}

	// 创建菜单
	// 设计原理：
	// 1. 服务层调用：通过service层创建，而不是直接操作数据库，符合分层架构
	// 2. 业务逻辑封装：service层可能包含额外的业务逻辑（如权限检查、日志记录）
	// 3. 错误处理：创建失败立即返回错误，避免无效操作
	//
	// 为什么使用service层而不是直接操作数据库：
	// - 业务逻辑封装：service层可能包含去重逻辑、权限检查等
	// - 代码复用：避免重复实现创建逻辑
	// - 易于测试：可以mock service层进行单元测试
	menuService := service.ServiceGroupApp.SystemServiceGroup.MenuService
	err := menuService.AddBaseMenu(menu)
	if err != nil {
		return nil, fmt.Errorf("创建菜单失败: %v", err)
	}

	// 获取创建的菜单ID
	// 设计考虑：AddBaseMenu可能不返回ID，需要额外查询
	// 使用name和path作为唯一标识查询，因为这两个字段组合应该是唯一的
	var createdMenu system.SysBaseMenu
	err = global.GVA_DB.Where("name = ? AND path = ?", name, path).First(&createdMenu).Error
	if err != nil {
		// 查询失败不影响整体流程，只记录警告
		// 设计考虑：菜单已创建成功，只是获取ID失败，不影响功能
		global.GVA_LOG.Warn("获取创建的菜单ID失败", zap.Error(err))
	}

	// 构建响应
	// 设计考虑：返回成功状态、消息和关键信息（MenuID、Name、Path）
	// 便于调用方了解创建结果和后续操作
	response := &MenuCreateResponse{
		Success: true,
		Message: fmt.Sprintf("成功创建菜单 %s", title),
		MenuID:  createdMenu.ID,
		Name:    name,
		Path:    path,
	}

	// JSON序列化：使用Indent格式化，提高可读性
	// 设计考虑：虽然会增加一些字节，但可读性对调试和日志记录很重要
	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化结果失败: %v", err)
	}

	// 返回MCP标准格式的响应
	// 设计原理：
	// 1. 标准格式：使用mcp.CallToolResult和mcp.TextContent，符合MCP协议
	// 2. 文本内容：使用TextContent类型，便于AI理解和展示
	// 3. 格式化输出：包含标题和JSON，提高可读性
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("菜单创建结果：\n\n%s", string(resultJSON)),
			},
		},
	}, nil
}
