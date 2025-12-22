package mcpTool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 注册工具到MCP工具注册表
// 
// 设计原理：
// - 利用Go的包初始化机制，在包导入时自动注册工具
// - 无需手动调用，简化工具的使用和集成
func init() {
	RegisterTool(&DictionaryQuery{})
}

// DictionaryPre 字典预览结构（用于简化查询）
//
// 设计原理：
// - 轻量级结构：只包含核心字段，用于快速预览
// - 减少数据传输：不需要完整字典信息时使用此结构
type DictionaryPre struct {
	Type string `json:"type"` // 字典名（英）
	Desc string `json:"desc"` // 描述
}

// DictionaryInfo 字典信息结构
//
// 设计原理：
// 1. 完整信息：包含字典的所有基本信息（ID、名称、类型、状态、描述）
// 2. 关联数据：Details字段包含字典的所有选项值，提供完整视图
// 3. 状态管理：Status使用指针类型，支持nil值表示未设置状态
//
// 为什么Status使用指针类型：
// - 三态支持：nil表示未设置，true/false表示启用/禁用
// - 数据库兼容：GORM支持指针类型处理NULL值
// - 灵活性：可以区分"未设置"和"禁用"两种状态
//
// 好处：
// - 完整性：一次查询获取字典及其所有选项值
// - 可读性：结构化的数据便于AI理解和使用
// - 扩展性：可以添加更多字段而不破坏现有结构
type DictionaryInfo struct {
	ID      uint                   `json:"id"`      // 字典ID
	Name    string                 `json:"name"`    // 字典名（中）
	Type    string                 `json:"type"`    // 字典名（英）
	Status  *bool                  `json:"status"`  // 状态（指针类型支持nil）
	Desc    string                 `json:"desc"`    // 描述
	Details []DictionaryDetailInfo `json:"details"` // 字典详情（选项值列表）
}

// DictionaryDetailInfo 字典详情信息结构
//
// 设计原理：
// 1. 选项值：Label用于展示，Value用于存储，Extend用于扩展信息
// 2. 状态管理：Status使用指针类型，支持启用/禁用控制
// 3. 排序支持：Sort字段支持自定义排序，便于控制选项显示顺序
//
// 字段说明：
// - Label: 用户界面显示的文本（如"启用"）
// - Value: 数据库中存储的值（如"1"）
// - Extend: 扩展信息，可以存储额外的配置数据
// - Status: 启用状态，支持禁用某个选项
// - Sort: 排序号，数字越小越靠前
//
// 好处：
// - 灵活性：支持禁用某个选项而不删除
// - 可扩展：Extend字段可以存储任意扩展信息
// - 可排序：Sort字段支持自定义显示顺序
type DictionaryDetailInfo struct {
	ID     uint   `json:"id"`     // 详情ID
	Label  string `json:"label"`  // 展示值（用户看到的文本）
	Value  string `json:"value"`  // 字典值（数据库中存储的值）
	Extend string `json:"extend"` // 扩展值（额外的配置信息）
	Status *bool  `json:"status"` // 启用状态（指针类型支持nil）
	Sort   int    `json:"sort"`   // 排序标记（数字越小越靠前）
}

// DictionaryQueryResponse 字典查询响应结构
//
// 设计原理：
// 1. 统一响应格式：使用标准的success/message格式，便于错误处理
// 2. 统计信息：Total字段提供总数，便于快速了解规模
// 3. 完整数据：Dictionaries包含所有字典及其详情，提供完整视图
//
// 好处：
// - 可操作性：AI可以根据返回的字典信息选择合适的字典类型
// - 可调试性：详细的响应信息便于定位问题
// - 完整性：包含所有字典信息，无需额外查询
type DictionaryQueryResponse struct {
	Success      bool             `json:"success"`      // 是否成功
	Message      string           `json:"message"`     // 操作结果消息
	Total        int              `json:"total"`        // 总数量
	Dictionaries []DictionaryInfo `json:"dictionaries"` // 字典列表
}

// DictionaryQuery 字典查询工具
//
// 设计原理（查询工具模式）：
// 1. 只读操作：不修改任何数据，只查询和返回信息
// 2. 灵活查询：支持按类型查询、过滤禁用项、只返回详情等多种查询方式
// 3. 无状态设计：空结构体，保证线程安全
//
// 为什么需要这个工具：
// - AI决策支持：AI需要知道系统中可用的字典类型，避免创建重复字典
// - 代码生成：生成代码时需要知道字典的选项值，用于生成下拉选择等组件
// - 系统分析：帮助了解系统的字典结构和可用选项
type DictionaryQuery struct{}

// New 创建字典查询工具的元数据定义
//
// 设计原理：
// 1. 灵活查询：支持多种查询方式（按类型、过滤禁用、只返回详情）
// 2. 参数可选：所有参数都是可选的，提供默认行为
// 3. 功能说明：详细描述工具用途，帮助AI理解何时使用
//
// 参数设计考虑：
// - dictType可选：不提供时返回所有字典，提供时精确查询
// - includeDisabled默认false：大多数场景只需要启用的字典
// - detailsOnly默认false：大多数场景需要完整的字典信息
//
// 为什么支持多种查询方式：
// - 性能优化：按类型查询可以减少数据传输
// - 场景适配：不同场景需要不同的数据粒度
// - 灵活性：支持各种查询需求，提高工具适用性
//
// 好处：
// - AI友好：清晰的参数描述帮助AI正确调用工具
// - 性能：支持精确查询，减少不必要的数据传输
// - 灵活性：多种查询方式满足不同需求
func (d *DictionaryQuery) New() mcp.Tool {
	return mcp.NewTool("query_dictionaries",
		mcp.WithDescription("查询系统中所有的字典和字典属性，用于AI生成逻辑时了解可用的字典选项"),
		mcp.WithString("dictType",
			mcp.Description("可选：指定字典类型进行精确查询，如果不提供则返回所有字典"),
		),
		mcp.WithBoolean("includeDisabled",
			mcp.Description("是否包含已禁用的字典和字典项，默认为false（只返回启用的）"),
		),
		mcp.WithBoolean("detailsOnly",
			mcp.Description("是否只返回字典详情信息（不包含字典基本信息），默认为false"),
		),
	)
}

// Handle 处理字典查询请求
//
// 设计原理：
// 1. 参数解析：所有参数都是可选的，提供默认值
// 2. 分支处理：根据dictType是否为空，选择不同的查询策略
// 3. 数据转换：将数据库模型转换为统一的响应格式
// 4. 灵活输出：支持返回完整字典信息或只返回详情信息
//
// 处理流程：
// 1. 参数提取：从MCP请求中提取查询参数
// 2. 查询执行：根据参数选择查询单个字典或所有字典
// 3. 数据转换：将数据库模型转换为响应格式
// 4. 结果处理：根据detailsOnly参数决定返回格式
//
// 为什么支持两种查询方式：
// - 性能优化：按类型查询可以减少数据传输和查询时间
// - 场景适配：不同场景需要不同的查询粒度
// - 灵活性：支持精确查询和全量查询，满足各种需求
//
// 好处：
// - 容错性：参数验证和默认值处理，提高健壮性
// - 灵活性：支持多种查询方式，满足不同场景需求
// - 性能：支持精确查询，减少不必要的数据传输
func (d *DictionaryQuery) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	// 获取参数
	// 设计考虑：所有参数都是可选的，使用类型断言和默认值处理
	dictType := ""
	if val, ok := args["dictType"].(string); ok {
		dictType = val
	}

	// 是否包含禁用的字典：默认为false，只返回启用的
	// 设计考虑：大多数场景只需要启用的字典，默认值减少参数传递
	includeDisabled := false
	if val, ok := args["includeDisabled"].(bool); ok {
		includeDisabled = val
	}

	// 是否只返回详情：默认为false，返回完整字典信息
	// 设计考虑：大多数场景需要完整的字典信息，包括字典基本信息和详情
	detailsOnly := false
	if val, ok := args["detailsOnly"].(bool); ok {
		detailsOnly = val
	}

	// 获取字典服务
	// 设计原理：通过service层查询，而不是直接操作数据库，符合分层架构
	dictionaryService := service.ServiceGroupApp.SystemServiceGroup.DictionaryService

	var dictionaries []DictionaryInfo
	var err error

	// 分支处理：根据dictType是否为空选择不同的查询策略
	// 设计考虑：精确查询和全量查询使用不同的方法，提高查询效率
	if dictType != "" {
		// 查询指定类型的字典（精确查询）
		// 设计原理：
		// 1. 性能优化：只查询指定类型的字典，减少数据传输
		// 2. 状态过滤：根据includeDisabled参数决定是否过滤禁用项
		// 3. 服务层调用：通过service层查询，封装业务逻辑
		
		// 状态过滤：如果不需要禁用的，设置status为true
		// 设计考虑：使用指针类型，nil表示不过滤，true表示只查询启用的
		var status *bool
		if !includeDisabled {
			status = &[]bool{true}[0] // 创建指向true的指针
		}

		// 调用service层查询单个字典
		// 参数说明：dictType(字典类型), 0(不使用ID查询), status(状态过滤)
		sysDictionary, err := dictionaryService.GetSysDictionary(dictType, 0, status)
		if err != nil {
			global.GVA_LOG.Error("查询字典失败", zap.Error(err))
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf(`{"success": false, "message": "查询字典失败: %v", "total": 0, "dictionaries": []}`, err.Error())),
				},
			}, nil
		}

		// 转换为响应格式
		// 设计考虑：将数据库模型转换为统一的响应格式，便于AI理解和使用
		dictInfo := DictionaryInfo{
			ID:     sysDictionary.ID,
			Name:   sysDictionary.Name,
			Type:   sysDictionary.Type,
			Status: sysDictionary.Status,
			Desc:   sysDictionary.Desc,
		}

		// 获取字典详情
		// 设计考虑：根据includeDisabled参数决定是否包含禁用的详情项
		// 为什么需要再次过滤：service层可能已经过滤了字典本身，但详情需要单独过滤
		for _, detail := range sysDictionary.SysDictionaryDetails {
			// 如果包含禁用的，或者详情项是启用的，则添加到结果中
			if includeDisabled || (detail.Status != nil && *detail.Status) {
				dictInfo.Details = append(dictInfo.Details, DictionaryDetailInfo{
					ID:     detail.ID,
					Label:  detail.Label,
					Value:  detail.Value,
					Extend: detail.Extend,
					Status: detail.Status,
					Sort:   detail.Sort,
				})
			}
		}

		dictionaries = append(dictionaries, dictInfo)
	} else {
		// 查询所有字典（全量查询）
		// 设计原理：
		// 1. 直接数据库查询：全量查询使用数据库查询，性能更好
		// 2. 预加载关联：使用Preload一次性加载所有详情，避免N+1查询
		// 3. 条件过滤：根据includeDisabled参数动态添加过滤条件
		//
		// 为什么使用直接数据库查询而不是service层：
		// - 性能：全量查询时直接数据库查询更高效
		// - 灵活性：可以灵活控制查询条件和预加载策略
		// - 控制力：直接控制SQL查询，优化性能
		var sysDictionaries []system.SysDictionary
		db := global.GVA_DB.Model(&system.SysDictionary{})

		// 状态过滤：如果不需要禁用的，添加where条件
		if !includeDisabled {
			db = db.Where("status = ?", true)
		}

		// 预加载字典详情
		// 设计原理：
		// 1. 避免N+1查询：使用Preload一次性加载所有详情
		// 2. 条件预加载：在Preload回调中添加过滤和排序条件
		// 3. 排序保证：按sort字段排序，保证详情顺序正确
		err = db.Preload("SysDictionaryDetails", func(db *gorm.DB) *gorm.DB {
			if includeDisabled {
				// 包含禁用的：只排序，不过滤
				return db.Order("sort")
			} else {
				// 不包含禁用的：过滤并排序
				return db.Where("status = ?", true).Order("sort")
			}
		}).Find(&sysDictionaries).Error

		if err != nil {
			global.GVA_LOG.Error("查询字典列表失败", zap.Error(err))
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf(`{"success": false, "message": "查询字典列表失败: %v", "total": 0, "dictionaries": []}`, err.Error())),
				},
			}, nil
		}

		// 转换为响应格式
		for _, dict := range sysDictionaries {
			dictInfo := DictionaryInfo{
				ID:     dict.ID,
				Name:   dict.Name,
				Type:   dict.Type,
				Status: dict.Status,
				Desc:   dict.Desc,
			}

			// 获取字典详情
			for _, detail := range dict.SysDictionaryDetails {
				if includeDisabled || (detail.Status != nil && *detail.Status) {
					dictInfo.Details = append(dictInfo.Details, DictionaryDetailInfo{
						ID:     detail.ID,
						Label:  detail.Label,
						Value:  detail.Value,
						Extend: detail.Extend,
						Status: detail.Status,
						Sort:   detail.Sort,
					})
				}
			}

			dictionaries = append(dictionaries, dictInfo)
		}
	}

	// 如果只需要详情信息，则提取所有详情
	// 设计原理：
	// 1. 数据提取：从所有字典中提取详情，合并为一个列表
	// 2. 简化响应：只返回详情信息，不包含字典基本信息
	// 3. 场景适配：某些场景只需要详情信息，不需要字典基本信息
	//
	// 为什么需要这个功能：
	// - 场景需求：某些场景只需要字典选项值，不需要字典元数据
	// - 数据简化：减少响应体积，提高传输效率
	// - 灵活性：支持不同的数据粒度需求
	if detailsOnly {
		var allDetails []DictionaryDetailInfo
		// 遍历所有字典，提取所有详情项
		for _, dict := range dictionaries {
			allDetails = append(allDetails, dict.Details...)
		}

		// 构建简化响应：只包含详情信息
		response := map[string]interface{}{
			"success": true,
			"message": "查询字典详情成功",
			"total":   len(allDetails),
			"details": allDetails,
		}

		responseJSON, _ := json.Marshal(response)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(responseJSON)),
			},
		}, nil
	}

	// 构建完整响应：包含字典基本信息和详情
	// 设计考虑：大多数场景需要完整的字典信息，包括字典元数据和选项值
	response := DictionaryQueryResponse{
		Success:      true,
		Message:      "查询字典成功",
		Total:        len(dictionaries),
		Dictionaries: dictionaries,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		global.GVA_LOG.Error("序列化响应失败", zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf(`{"success": false, "message": "序列化响应失败: %v", "total": 0, "dictionaries": []}`, err.Error())),
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(responseJSON)),
		},
	}, nil
}
