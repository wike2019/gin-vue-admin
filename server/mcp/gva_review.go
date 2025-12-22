package mcpTool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// GVAReviewer GVA代码审查工具
// 
// 设计目的：
// 1. 代码质量检查：验证生成的代码是否满足用户需求
// 2. 完整性分析：检查是否遗漏了关联关系、交互功能等
// 3. 优化建议：提供针对性的代码优化建议
// 4. 生成Prompt：为AI生成优化代码的指导Prompt
//
// 核心价值：
// - 质量保证：确保生成的代码满足需求
// - 发现遗漏：识别缺失的功能和关联关系
// - 指导优化：提供具体的优化建议和Prompt
// - 流程闭环：在代码生成后提供审查，形成完整的开发流程
type GVAReviewer struct{}

// init 注册工具
// 设计思路：使用init函数自动注册，确保工具可用
// 好处：简化注册流程，避免遗漏
func init() {
	RegisterTool(&GVAReviewer{})
}

// ReviewRequest 审查请求结构
type ReviewRequest struct {
	UserRequirement string   `json:"userRequirement"` // 经过requirement_analyze后的用户需求
	GeneratedFiles  []string `json:"generatedFiles"`  // gva_execute创建的文件列表
}

// ReviewResponse 审查响应结构
type ReviewResponse struct {
	Success          bool   `json:"success"`          // 是否审查成功
	Message          string `json:"message"`          // 审查结果消息
	AdjustmentPrompt string `json:"adjustmentPrompt"` // 调整代码的提示
	ReviewDetails    string `json:"reviewDetails"`    // 详细的审查结果
}

// New 创建GVA代码审查工具
func (g *GVAReviewer) New() mcp.Tool {
	return mcp.NewTool("gva_review",
		mcp.WithDescription(`**GVA代码审查工具 - 在gva_execute调用后使用**

**核心功能：**
- 接收经过requirement_analyze处理的用户需求和gva_execute生成的文件列表
- 分析生成的代码是否满足用户的原始需求
- 检查是否涉及到关联、交互等复杂功能
- 如果代码不满足需求，提供调整建议和新的prompt

**使用场景：**
- 在gva_execute成功执行后调用
- 用于验证生成的代码是否完整满足用户需求
- 检查模块间的关联关系是否正确实现
- 发现缺失的交互功能或业务逻辑

**工作流程：**
1. 接收用户原始需求和生成的文件列表
2. 分析需求中的关键功能点
3. 检查生成的文件是否覆盖所有功能
4. 识别缺失的关联关系、交互功能等
5. 生成调整建议和新的开发prompt

**输出内容：**
- 审查结果和是否需要调整
- 详细的缺失功能分析
- 针对性的代码调整建议
- 可直接使用的开发prompt

**重要提示：**
- 本工具专门用于代码质量审查，不执行实际的代码修改
- 重点关注模块间关联、用户交互、业务流程完整性
- 提供的调整建议应该具体可执行`),
		mcp.WithString("userRequirement",
			mcp.Description("经过requirement_analyze处理后的用户需求描述，包含详细的功能要求和字段信息"),
			mcp.Required(),
		),
		mcp.WithString("generatedFiles",
			mcp.Description("gva_execute创建的文件列表，JSON字符串格式，包含所有生成的后端和前端文件路径"),
			mcp.Required(),
		),
	)
}

// Handle 处理审查请求
// 
// 设计思路：
// 1. 参数验证：严格验证输入参数，确保数据格式正确
// 2. 简化分析：不进行复杂的代码分析，直接生成优化Prompt
// 3. 结构化输出：返回结构化的审查结果，便于AI使用
//
// 为什么简化分析：
// - 代码分析需要读取文件内容，复杂度高
// - AI可以基于需求和文件列表自行分析
// - 生成优化Prompt让AI进行深度分析更灵活
//
// 好处：
// 1. 性能：避免读取大量文件，提高响应速度
// 2. 灵活性：让AI根据具体情况进行分析，更智能
// 3. 可扩展性：后续可以增强分析逻辑，不影响现有流程
func (g *GVAReviewer) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 步骤1：获取并验证用户需求
	// 为什么需要用户需求：用于对比生成的代码是否满足需求
	// 好处：提供审查的基准，确保审查有针对性
	userRequirementData, ok := request.GetArguments()["userRequirement"]
	if !ok {
		return nil, errors.New("参数错误：userRequirement 必须提供")
	}

	userRequirement, ok := userRequirementData.(string)
	if !ok {
		return nil, errors.New("参数错误：userRequirement 必须是字符串类型")
	}

	// 步骤2：获取并验证生成的文件列表
	// 为什么需要文件列表：知道生成了哪些文件，便于分析完整性
	// 好处：可以检查是否遗漏了必要的文件
	generatedFilesData, ok := request.GetArguments()["generatedFiles"]
	if !ok {
		return nil, errors.New("参数错误：generatedFiles 必须提供")
	}

	generatedFilesStr, ok := generatedFilesData.(string)
	if !ok {
		return nil, errors.New("参数错误：generatedFiles 必须是JSON字符串")
	}

	// 步骤3：解析JSON字符串为字符串数组
	// 为什么使用JSON字符串：MCP协议中复杂类型需要序列化传输
	// 好处：支持传输数组类型，保持协议兼容性
	var generatedFiles []string
	err := json.Unmarshal([]byte(generatedFilesStr), &generatedFiles)
	if err != nil {
		return nil, fmt.Errorf("解析generatedFiles失败: %v", err)
	}

	// 验证文件列表不为空
	// 为什么需要验证：空列表无法进行审查
	// 好处：提前发现无效输入，提供清晰的错误信息
	if len(generatedFiles) == 0 {
		return nil, errors.New("参数错误：generatedFiles 不能为空")
	}

	// 步骤4：生成调整提示
	// 设计思路：不进行复杂的代码分析，直接生成优化Prompt
	// 为什么这样设计：
	// 1. 性能考虑：避免读取大量文件进行分析
	// 2. 灵活性：让AI根据具体情况进行分析更智能
	// 3. 可扩展性：后续可以增强分析逻辑
	// 好处：快速响应，让AI进行深度分析
	adjustmentPrompt := g.generateAdjustmentPrompt(userRequirement, generatedFiles)

	// 构建简化的审查详情
	reviewDetails := fmt.Sprintf("📋 **代码审查报告**\n\n **用户原始需求：**\n%s\n\n **已生成文件数量：** %d\n\n **建议进行代码优化和完善**", userRequirement, len(generatedFiles))

	// 构建审查结果
	reviewResult := &ReviewResponse{
		Success:          true,
		Message:          "代码审查完成",
		AdjustmentPrompt: adjustmentPrompt,
		ReviewDetails:    reviewDetails,
	}

	// 序列化响应
	responseJSON, err := json.MarshalIndent(reviewResult, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化审查结果失败: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("代码审查结果：\n\n%s", string(responseJSON))),
		},
	}, nil
}

// generateAdjustmentPrompt 生成调整代码的提示
// 
// 设计思路：构建结构化的Prompt，包含需求、文件列表和优化指导
// 为什么生成Prompt而不是直接分析：
// 1. 灵活性：让AI根据具体情况进行分析，更智能
// 2. 可扩展性：可以轻松添加新的优化指导
// 3. 性能：避免读取大量文件进行分析
//
// Prompt结构：
// 1. 用户原始需求：提供审查的基准
// 2. 生成的文件列表：让AI知道有哪些文件
// 3. 优化指导：提供具体的优化方向和注意事项
//
// 好处：
// 1. 完整性：包含所有必要信息，便于AI理解
// 2. 指导性：提供明确的优化方向，避免AI盲目分析
// 3. 可操作性：包含具体的操作建议，便于执行
func (g *GVAReviewer) generateAdjustmentPrompt(userRequirement string, generatedFiles []string) string {
	var prompt strings.Builder

	// 构建Prompt标题和用户需求
	// 为什么包含用户需求：让AI知道原始需求，便于对比分析
	// 好处：提供审查的基准，确保优化方向正确
	prompt.WriteString("🔧 **代码调整指导 Prompt：**\n\n")
	prompt.WriteString(fmt.Sprintf("**用户的原始需求为：** %s\n\n", userRequirement))
	
	// 列出生成的文件
	// 为什么列出文件：让AI知道生成了哪些文件，便于分析完整性
	// 好处：可以检查是否遗漏了必要的文件
	prompt.WriteString("**经过GVA生成后的文件有如下内容：**\n")
	for _, file := range generatedFiles {
		prompt.WriteString(fmt.Sprintf("- %s\n", file))
	}
	prompt.WriteString("\n")

	// 提供优化指导
	// 为什么提供指导：明确优化方向，避免AI盲目分析
	// 好处：
	// 1. 全面性：涵盖代码质量、关联关系、交互功能等各个方面
	// 2. 规范性：强调遵循GVA框架规范
	// 3. 操作性：提供具体的操作建议（如使用menu_lister、api_lister等）
	prompt.WriteString("**请帮我优化和完善代码，确保：**\n")
	prompt.WriteString("1. 代码完全满足用户的原始需求\n")
	prompt.WriteString("2. 完善模块间的关联关系，确保数据一致性\n")
	prompt.WriteString("3. 实现所有必要的用户交互功能\n")
	prompt.WriteString("4. 保持代码的完整性和可维护性\n")
	prompt.WriteString("5. 遵循GVA框架的开发规范和最佳实践\n")
	prompt.WriteString("6. 确保前后端功能完整对接\n")
	prompt.WriteString("7. 添加必要的错误处理和数据验证\n\n")
	
	// 提供具体的操作指导
	// 为什么需要这些指导：告诉AI如何使用其他MCP工具
	// 好处：
	// 1. 工具使用：指导AI正确使用menu_lister、api_lister等工具
	// 2. 避免错误：提醒不要随意删除import，避免破坏代码
	// 3. 完整性：指导如何创建缺失的菜单和API
	prompt.WriteString("8. 如果需要vue路由跳转，请使用 menu_lister获取完整路由表，并且路由跳转使用 router.push({\"name\":从menu_lister中获取的name})\n\n")
	prompt.WriteString("9. 如果当前所有的vue页面内容无法满足需求，则自行书写vue文件，并且调用 menu_creator创建菜单记录\n\n")
	prompt.WriteString("10. 如果需要API调用，请使用 api_lister获取api表，根据需求调用对应接口\n\n")
	prompt.WriteString("11. 如果当前所有API无法满足则自行书写接口，补全前后端代码，并使用 api_creator创建api记录\n\n")
	prompt.WriteString("12. 无论前后端都不要随意删除import的内容\n\n")
	prompt.WriteString("**请基于用户需求和现有文件，提供完整的代码优化方案。**")

	return prompt.String()
}
