package system

import (
	"context"
	"os"
	"path/filepath"
	"text/template"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/autocode"
)

// CreateMcp 根据用户提供的 MCP Tool 信息，从模板文件自动生成 MCP Tool 的 Go 代码文件
//
// 为什么使用模板化代码生成？
// 1. 代码一致性：确保所有生成的 MCP Tool 都遵循相同的代码结构和规范
// 2. 减少重复：避免手动编写相似的样板代码，提高开发效率
// 3. 易于维护：模板集中管理，修改模板即可统一更新所有生成的代码风格
// 4. 标准化：生成的代码符合 MCP (Model Context Protocol) 工具的标准接口规范
//
// 参数说明：
//   - ctx: 上下文对象，用于控制请求的生命周期（虽然当前未使用，但保留以支持未来可能的取消操作和超时控制）
//   - info: MCP Tool 的配置信息，包含工具名称、描述、参数定义、响应类型等
//
// 返回值说明：
//   - toolFilePath: 生成的工具文件的完整路径，便于调用方知道文件生成位置，可用于后续操作（如文件检查、清理等）
//   - err: 错误信息，如果生成过程中出现任何错误（模板解析失败、文件创建失败、模板执行失败等）
//
// 设计优势：
// 1. 配置驱动：路径配置集中在 global.GVA_CONFIG 中，便于部署时调整
// 2. 命名规范：自动将驼峰命名转为下划线命名，符合 Go 语言文件命名规范
// 3. 模板函数：使用自定义模板函数（如 title），增强模板表达能力
// 4. 资源管理：使用 defer 确保文件句柄正确关闭，避免资源泄露
func (s *autoCodeTemplate) CreateMcp(ctx context.Context, info request.AutoMcpTool) (toolFilePath string, err error) {
	// 构建模板文件路径
	// 为什么使用 filepath.Join 而不是字符串拼接？
	// 1. 跨平台兼容：自动处理不同操作系统的路径分隔符（Windows 用 \，Unix 用 /）
	// 2. 路径规范化：自动处理多余的分隔符和相对路径，避免路径错误
	// 3. 代码可读性：语义清晰，明确表达这是在构建路径
	// 4. 配置灵活性：路径由配置决定，支持不同的项目结构和部署方式
	mcpTemplatePath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "resource", "mcp", "tools.tpl")
	// 构建生成文件的存放目录路径
	// 集中存放的好处：所有 MCP Tool 文件在同一目录，便于管理和查找
	mcpToolPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "mcp")

	// 声明模板变量
	// 为什么使用 *template.Template 而不是 template.Template？
	// - template.Template 是一个较大的结构体，使用指针可以避免值拷贝，提高性能
	// - 指针语义更清晰，表示这是一个可被修改的模板实例
	var files *template.Template

	// 获取模板文件名（不含路径）
	// 为什么需要提取文件名？
	// - template.New() 需要模板名称用于错误报告和调试，使用文件名作为标识更直观
	// - 如果直接使用完整路径，错误信息中会显示冗长的路径，不利于阅读
	templateName := filepath.Base(mcpTemplatePath)

	// 创建并解析模板
	// 为什么分步骤（New -> Funcs -> ParseFiles）而不是一次性完成？
	// 1. 灵活性：可以在解析前注入自定义函数，扩展模板能力
	// 2. 可测试性：每个步骤独立，便于单元测试
	// 3. 错误定位：每个步骤的错误更具体，便于调试
	//
	// autocode.GetTemplateFuncMap() 的作用：
	// - 提供模板中使用的自定义函数（如 title、camelCase 等）
	// - 统一管理模板函数，避免重复定义
	// - 支持复杂的字符串转换和数据格式化
	files, err = template.New(templateName).Funcs(autocode.GetTemplateFuncMap()).ParseFiles(mcpTemplatePath)
	if err != nil {
		// 提前返回错误，避免后续操作浪费资源
		// 为什么使用命名返回值？
		// - Go 语言特性，可以直接 return 而不需要显式返回 err
		// - 代码更简洁，但要注意保持 err 的一致性
		return
	}

	// 将工具名称从驼峰命名转换为下划线命名
	// 为什么需要转换？
	// 1. Go 语言文件命名约定：通常使用下划线分隔的小写字母（snake_case）
	// 2. 与项目风格一致：保持代码库中文件命名的统一性
	// 3. 文件系统兼容性：某些文件系统对大小写不敏感，下划线命名更安全
	// 4. 可读性：下划线分隔的命名在文件列表中更易于区分单词
	// 示例：UserManagementTool -> user_management_tool.go
	fileName := utils.HumpToUnderscore(info.Name)

	// 构建完整的文件路径
	// 为什么在这里构建路径而不是在文件创建时？
	// - 路径可以在文件创建前进行验证和调整
	// - 便于返回给调用方，告知文件生成位置
	toolFilePath = filepath.Join(mcpToolPath, fileName+".go")

	// 创建目标文件
	// os.Create 的行为：
	// - 如果文件不存在，创建新文件
	// - 如果文件已存在，会截断（清空）现有文件
	// 为什么直接覆盖而不是检查文件是否存在？
	// - 允许重新生成工具代码，支持代码更新场景
	// - 简化逻辑，让用户明确知道文件会被覆盖
	f, err := os.Create(toolFilePath)
	if err != nil {
		// 文件创建失败可能的原因：
		// - 目录不存在（需要先创建目录）
		// - 权限不足
		// - 磁盘空间不足
		return
	}
	// 使用 defer 确保文件句柄在函数返回前关闭
	// 为什么使用 defer 而不是在最后手动关闭？
	// 1. 防止遗漏：即使函数中间有 return，defer 也会执行
	// 2. 资源安全：避免文件句柄泄露，确保系统资源正确释放
	// 3. 代码简洁：不需要在每个返回点都写 Close()
	defer f.Close()

	// 执行模板，将数据填充到模板中并写入文件
	// Execute 的作用：
	// - 将 info 结构体的数据填充到模板的占位符中（如 {{.Name}}）
	// - 执行模板中定义的控制逻辑（如 {{range}}、{{if}} 等）
	// - 将生成的文本内容直接写入文件，无需额外的缓冲区
	//
	// 为什么直接写入文件而不是先写到内存？
	// 1. 内存效率：对于可能较大的代码文件，直接写入避免占用过多内存
	// 2. 实时反馈：写入过程可以立即看到文件内容
	// 3. 简单直接：减少中间步骤，降低出错概率
	err = files.Execute(f, info)
	if err != nil {
		// 模板执行失败的可能原因：
		// - 模板语法错误
		// - 数据结构与模板不匹配
		// - 自定义函数调用失败
		return
	}

	// 返回生成的文件路径和错误信息（如果有）
	// 命名返回值的好处：可以只写 return，Go 会自动返回已赋值的变量
	return

}
