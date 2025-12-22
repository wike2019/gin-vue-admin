package mcpTool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/mark3labs/mcp-go/mcp"
)

// 注册工具
// 设计思路：使用init函数在包加载时自动注册工具，无需手动调用
// 好处：简化工具注册流程，确保工具在系统启动时就被注册，避免遗漏
func init() {
	RegisterTool(&GVAAnalyzer{})
}

// GVAAnalyzer GVA分析器 - 用于分析当前功能是否需要创建独立的package和module
//
// 设计目的：
// 1. 系统状态分析：扫描当前系统中已存在的包和模块，为AI提供上下文信息
// 2. 智能决策支持：帮助AI判断是否需要创建新包或复用现有包
// 3. 系统清理：自动检测并清理空包，保持系统整洁
// 4. 预设计模块发现：扫描文件系统中的预设计模块，供AI参考使用
//
// 核心价值：
// - 避免重复创建：通过分析现有资源，避免创建重复的包和模块
// - 提高效率：AI可以基于现有资源做出更智能的决策
// - 系统维护：自动清理无用资源，保持代码库整洁
type GVAAnalyzer struct{}

// AnalyzeRequest 分析请求结构体
type AnalyzeRequest struct {
	Requirement string `json:"requirement" binding:"required"` // 用户需求描述
}

// AnalyzeResponse 分析响应结构体
type AnalyzeResponse struct {
	ExistingPackages   []PackageInfo           `json:"existingPackages"`   // 现有包信息
	PredesignedModules []PredesignedModuleInfo `json:"predesignedModules"` // 预设计模块信息
	Dictionaries       []DictionaryPre         `json:"dictionaries"`       // 字典信息
	CleanupInfo        *CleanupInfo            `json:"cleanupInfo"`        // 清理信息（如果有）
}

// ModuleInfo 模块信息
type ModuleInfo struct {
	ModuleName  string   `json:"moduleName"`  // 模块名称
	PackageName string   `json:"packageName"` // 包名
	Template    string   `json:"template"`    // 模板类型
	StructName  string   `json:"structName"`  // 结构体名称
	TableName   string   `json:"tableName"`   // 表名
	Description string   `json:"description"` // 描述
	FilePaths   []string `json:"filePaths"`   // 相关文件路径
}

// PackageInfo 包信息
type PackageInfo struct {
	PackageName string `json:"packageName"` // 包名
	Template    string `json:"template"`    // 模板类型
	Label       string `json:"label"`       // 标签
	Desc        string `json:"desc"`        // 描述
	Module      string `json:"module"`      // 模块
	IsEmpty     bool   `json:"isEmpty"`     // 是否为空包
}

// PredesignedModuleInfo 预设计模块信息
type PredesignedModuleInfo struct {
	ModuleName  string   `json:"moduleName"`  // 模块名称
	PackageName string   `json:"packageName"` // 包名
	Template    string   `json:"template"`    // 模板类型
	FilePaths   []string `json:"filePaths"`   // 文件路径列表
	Description string   `json:"description"` // 描述
}

// CleanupInfo 清理信息
type CleanupInfo struct {
	DeletedPackages []string `json:"deletedPackages"` // 已删除的包
	DeletedModules  []string `json:"deletedModules"`  // 已删除的模块
	CleanupMessage  string   `json:"cleanupMessage"`  // 清理消息
}

// New 创建GVA分析器工具
func (g *GVAAnalyzer) New() mcp.Tool {
	return mcp.NewTool("gva_analyze",
		mcp.WithDescription("返回当前系统中有效的包和模块信息，并分析用户需求是否需要创建新的包、模块和字典。同时检查并清理空包，确保系统整洁。"),
		mcp.WithString("requirement",
			mcp.Description("用户需求描述，用于分析是否需要创建新的包和模块"),
			mcp.Required(),
		),
	)
}

// Handle 处理分析请求
func (g *GVAAnalyzer) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 解析请求参数
	requirementStr, ok := request.GetArguments()["requirement"].(string)
	if !ok || requirementStr == "" {
		return nil, errors.New("参数错误：requirement 必须是非空字符串")
	}

	// 创建分析请求
	analyzeReq := AnalyzeRequest{
		Requirement: requirementStr,
	}

	// 执行分析逻辑
	response, err := g.performAnalysis(ctx, analyzeReq)
	if err != nil {
		return nil, fmt.Errorf("分析失败: %v", err)
	}

	// 序列化响应
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("序列化响应失败: %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(responseJSON)),
		},
	}, nil
}

// performAnalysis 执行分析逻辑
//
// 设计思路：采用分步骤处理的方式，每个步骤职责明确
// 好处：
// 1. 可维护性：每个步骤独立，便于理解和修改
// 2. 容错性：单个步骤失败不影响其他步骤
// 3. 可扩展性：新增分析步骤时不影响现有逻辑
//
// 处理流程：
// 1. 获取系统状态（包、历史记录）
// 2. 清理空包（保持系统整洁）
// 3. 扫描预设计模块（发现可用资源）
// 4. 构建分析结果（为AI提供决策依据）
func (g *GVAAnalyzer) performAnalysis(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	// 步骤1：获取数据库中的包信息
	// 为什么先获取包信息：包是模块的组织单位，需要先了解包的结构
	// 好处：为后续的空包检测和模块扫描提供基础数据
	var packages []model.SysAutoCodePackage
	if err := global.GVA_DB.Find(&packages).Error; err != nil {
		return nil, fmt.Errorf("获取包信息失败: %v", err)
	}

	// 步骤2：获取历史记录
	// 为什么需要历史记录：历史记录包含模块创建信息，用于关联空包和模块
	// 好处：可以准确识别哪些模块属于已删除的包，避免数据不一致
	var histories []model.SysAutoCodeHistory
	if err := global.GVA_DB.Find(&histories).Error; err != nil {
		return nil, fmt.Errorf("获取历史记录失败: %v", err)
	}

	// 步骤3：检查空包并进行清理
	// 设计思路：在分析前先清理，确保返回的数据都是有效的
	// 好处：
	// 1. 保持系统整洁：自动清理无用资源
	// 2. 提高分析准确性：避免基于无效数据做决策
	// 3. 减少存储占用：及时清理空文件夹
	cleanupInfo := &CleanupInfo{
		DeletedPackages: []string{},
		DeletedModules:  []string{},
	}

	var validPackages []model.SysAutoCodePackage
	var emptyPackageHistoryIDs []uint

	// 遍历所有包，检查是否为空
	// 为什么使用遍历：需要逐个检查每个包的实际状态
	// 好处：精确识别空包，避免误删有内容的包
	for _, pkg := range packages {
		// 检查包文件夹是否为空
		// 为什么检查文件系统：数据库记录可能不准确，需要验证实际文件
		// 好处：确保清理的是真正无用的包，避免误删
		isEmpty, err := g.isPackageFolderEmpty(pkg.PackageName, pkg.Template)
		if err != nil {
			// 使用Warn而不是Error：单个包检查失败不影响整体流程
			// 好处：提高容错性，部分失败不影响其他包的检查
			global.GVA_LOG.Warn(fmt.Sprintf("检查包 %s 是否为空时出错: %v", pkg.PackageName, err))
			continue
		}

		if isEmpty {
			// 删除空包文件夹
			// 为什么先删除文件夹：确保文件系统状态与数据库一致
			// 好处：避免留下孤立的文件夹，保持文件系统整洁
			if err := g.removeEmptyPackageFolder(pkg.PackageName, pkg.Template); err != nil {
				global.GVA_LOG.Warn(fmt.Sprintf("删除空包文件夹 %s 失败: %v", pkg.PackageName, err))
			} else {
				cleanupInfo.DeletedPackages = append(cleanupInfo.DeletedPackages, pkg.PackageName)
			}

			// 删除数据库记录
			// 为什么删除数据库记录：保持数据库与文件系统的一致性
			// 好处：避免数据库中存在无效引用，导致后续操作出错
			if err := global.GVA_DB.Delete(&pkg).Error; err != nil {
				global.GVA_LOG.Warn(fmt.Sprintf("删除包数据库记录 %s 失败: %v", pkg.PackageName, err))
			}

			// 收集相关的历史记录ID
			// 为什么收集历史记录ID：后续需要清理这些历史记录
			// 好处：批量删除历史记录，提高效率，避免数据不一致
			for _, history := range histories {
				if history.Package == pkg.PackageName {
					emptyPackageHistoryIDs = append(emptyPackageHistoryIDs, history.ID)
					cleanupInfo.DeletedModules = append(cleanupInfo.DeletedModules, history.StructName)
				}
			}
		} else {
			// 保留有效包
			// 为什么单独收集：后续分析只基于有效包
			// 好处：确保分析结果的准确性，避免基于无效数据做决策
			validPackages = append(validPackages, pkg)
		}
	}

	// 步骤5：清理空包相关的历史记录和脏历史记录
	// 设计思路：先收集需要删除的历史记录ID，然后批量删除
	// 好处：
	// 1. 性能优化：批量删除比逐个删除效率高
	// 2. 数据一致性：确保历史记录与包状态一致
	// 3. 避免孤立数据：清理无效的历史记录，防止后续操作出错
	var dirtyHistoryIDs []uint
	for _, history := range histories {
		// 检查是否为空包相关的历史记录
		// 为什么需要检查：历史记录可能引用已删除的包
		// 好处：准确识别需要清理的历史记录，避免误删有效数据
		for _, emptyID := range emptyPackageHistoryIDs {
			if history.ID == emptyID {
				dirtyHistoryIDs = append(dirtyHistoryIDs, history.ID)
				break
			}
		}
	}

	// 批量删除脏历史记录
	// 为什么使用批量删除：提高删除效率，减少数据库操作次数
	// 好处：性能更好，特别是在需要删除大量记录时
	if len(dirtyHistoryIDs) > 0 {
		if err := global.GVA_DB.Delete(&model.SysAutoCodeHistory{}, "id IN ?", dirtyHistoryIDs).Error; err != nil {
			global.GVA_LOG.Warn(fmt.Sprintf("删除脏历史记录失败: %v", err))
		} else {
			global.GVA_LOG.Info(fmt.Sprintf("成功删除 %d 条脏历史记录", len(dirtyHistoryIDs)))
		}

		// 清理相关的API和菜单记录
		// 为什么需要清理：API和菜单可能引用已删除的模块
		// 好处：保持系统完整性，避免出现无效的API和菜单项
		if err := g.cleanupRelatedApiAndMenus(dirtyHistoryIDs); err != nil {
			global.GVA_LOG.Warn(fmt.Sprintf("清理相关API和菜单记录失败: %v", err))
		}
	}

	// 步骤6：扫描预设计模块
	// 设计思路：扫描文件系统中的.go文件，发现已存在但未在数据库中注册的模块
	// 好处：
	// 1. 发现可用资源：找到可以复用的模块，避免重复创建
	// 2. 完整性检查：发现文件系统与数据库不一致的情况
	// 3. 为AI提供参考：让AI知道有哪些模块已经存在
	predesignedModules, err := g.scanPredesignedModules()
	if err != nil {
		// 使用Warn并设置空列表：扫描失败不影响主流程
		// 好处：提高容错性，即使扫描失败也能返回其他分析结果
		global.GVA_LOG.Warn(fmt.Sprintf("扫描预设计模块失败: %v", err))
		predesignedModules = []PredesignedModuleInfo{} // 设置为空列表，不影响主流程
	}

	// 步骤7：过滤掉与已删除包相关的模块
	// 设计思路：确保返回的模块都是有效的，不包含已删除包中的模块
	// 好处：
	// 1. 数据准确性：只返回有效的模块信息
	// 2. 避免混淆：防止AI基于无效模块做决策
	// 3. 一致性保证：确保返回结果与清理操作一致
	filteredModules := []PredesignedModuleInfo{}
	for _, module := range predesignedModules {
		isDeleted := false
		// 检查模块是否属于已删除的包
		// 为什么需要检查：预设计模块可能属于已删除的包
		// 好处：确保返回的模块都是有效的
		for _, deletedPkg := range cleanupInfo.DeletedPackages {
			if module.PackageName == deletedPkg {
				isDeleted = true
				break
			}
		}
		if !isDeleted {
			filteredModules = append(filteredModules, module)
		}
	}

	// 8. 构建分析结果消息
	var analysisMessage strings.Builder
	if len(cleanupInfo.DeletedPackages) > 0 || len(cleanupInfo.DeletedModules) > 0 {
		analysisMessage.WriteString("**系统清理完成**\n\n")
		if len(cleanupInfo.DeletedPackages) > 0 {
			analysisMessage.WriteString(fmt.Sprintf("- 删除了 %d 个空包: %s\n", len(cleanupInfo.DeletedPackages), strings.Join(cleanupInfo.DeletedPackages, ", ")))
		}
		if len(cleanupInfo.DeletedModules) > 0 {
			analysisMessage.WriteString(fmt.Sprintf("- 删除了 %d 个相关模块: %s\n", len(cleanupInfo.DeletedModules), strings.Join(cleanupInfo.DeletedModules, ", ")))
		}
		analysisMessage.WriteString("\n")
		cleanupInfo.CleanupMessage = analysisMessage.String()
	}

	analysisMessage.WriteString(" **分析结果**\n\n")
	analysisMessage.WriteString(fmt.Sprintf("- **现有包数量**: %d\n", len(validPackages)))
	analysisMessage.WriteString(fmt.Sprintf("- **预设计模块数量**: %d\n\n", len(filteredModules)))

	// 9. 转换包信息
	existingPackages := make([]PackageInfo, len(validPackages))
	for i, pkg := range validPackages {
		existingPackages[i] = PackageInfo{
			PackageName: pkg.PackageName,
			Template:    pkg.Template,
			Label:       pkg.Label,
			Desc:        pkg.Desc,
			Module:      pkg.Module,
			IsEmpty:     false, // 已经过滤掉空包
		}
	}

	dictionaries := []DictionaryPre{} // 这里可以根据需要填充字典信息
	err = global.GVA_DB.Table("sys_dictionaries").Find(&dictionaries, "deleted_at is null").Error
	if err != nil {
		global.GVA_LOG.Warn(fmt.Sprintf("获取字典信息失败: %v", err))
		dictionaries = []DictionaryPre{} // 设置为空列表，不影响主流程
	}

	// 10. 构建响应
	response := &AnalyzeResponse{
		ExistingPackages:   existingPackages,
		PredesignedModules: filteredModules,
		Dictionaries:       dictionaries,
	}

	return response, nil
}

// isPackageFolderEmpty 检查包文件夹是否为空
//
// 设计思路：
// 1. 根据模板类型确定不同的路径结构（plugin和package路径不同）
// 2. 先检查文件夹是否存在，不存在视为空
// 3. 递归检查是否有.go文件，没有.go文件视为空
//
// 为什么这样设计：
// - plugin和package的目录结构不同，需要分别处理
// - 只检查.go文件：因为只有.go文件才是有效的代码文件
// - 递归检查：确保子目录中的文件也被检测到
//
// 好处：
// 1. 准确性：准确判断包是否真的为空
// 2. 灵活性：支持不同的包类型（plugin和package）
// 3. 完整性：递归检查确保不遗漏子目录中的文件
func (g *GVAAnalyzer) isPackageFolderEmpty(packageName, template string) (bool, error) {
	// 根据模板类型确定基础路径
	// 为什么需要区分：plugin和package的目录结构完全不同
	// plugin: server/plugin/packageName/
	// package: server/api/v1/packageName/
	// 好处：准确找到对应类型的包目录，避免误判
	var basePath string
	if template == "plugin" {
		basePath = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", packageName)
	} else {
		basePath = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "api", "v1", packageName)
	}

	// 检查文件夹是否存在
	// 为什么先检查存在性：不存在的文件夹肯定为空
	// 好处：快速处理不存在的文件夹，避免不必要的递归检查
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return true, nil // 文件夹不存在，视为空
	} else if err != nil {
		return false, err // 其他错误（如权限问题）需要返回错误
	}

	// 递归检查是否有.go文件
	// 为什么只检查.go文件：只有Go代码文件才是有效的模块文件
	// 好处：忽略其他类型的文件（如配置文件、文档等），只关注实际代码
	return g.hasGoFilesRecursive(basePath)
}

// hasGoFilesRecursive 递归检查目录及其子目录中是否有.go文件
//
// 设计思路：采用递归算法，先检查当前目录，再递归检查子目录
// 为什么使用递归：
// 1. 包目录可能有嵌套结构（如model/request、model/response）
// 2. 递归可以深入所有子目录，确保检查完整
//
// 返回值设计：
// - bool: true表示目录为空（没有.go文件），false表示不为空（有.go文件）
// - error: 读取目录时的错误
//
// 好处：
// 1. 完整性：确保检查所有层级的目录
// 2. 性能：找到第一个.go文件就立即返回，不继续检查
// 3. 容错性：子目录读取失败不影响其他目录的检查
func (g *GVAAnalyzer) hasGoFilesRecursive(dirPath string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// 读取失败时返回true（视为空）和错误
		// 为什么返回true：无法读取时保守处理，不删除目录
		// 好处：避免因权限问题误删目录
		return true, err // 读取失败，返回空
	}

	// 先检查当前目录下的.go文件
	// 为什么先检查当前目录：大多数情况下.go文件在当前目录，可以快速返回
	// 好处：提高性能，避免不必要的递归调用
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return false, nil // 找到.go文件，不为空
		}
	}

	// 递归检查子目录
	// 为什么需要递归：包目录可能有嵌套结构
	// 好处：确保检查所有层级的目录，不遗漏任何.go文件
	for _, entry := range entries {
		if entry.IsDir() {
			subDirPath := filepath.Join(dirPath, entry.Name())
			isEmpty, err := g.hasGoFilesRecursive(subDirPath)
			if err != nil {
				// 忽略子目录的错误，继续检查其他目录
				// 为什么忽略：单个子目录错误不应影响整体判断
				// 好处：提高容错性，部分目录无法访问时仍能继续检查
				continue
			}
			if !isEmpty {
				return false, nil // 子目录中找到.go文件，不为空
			}
		}
	}

	return true, nil // 没有找到.go文件，为空
}

// removeEmptyPackageFolder 删除空包文件夹
func (g *GVAAnalyzer) removeEmptyPackageFolder(packageName, template string) error {
	var basePath string
	if template == "plugin" {
		basePath = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", packageName)
	} else {
		// 对于package类型，需要删除多个目录
		paths := []string{
			filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "api", "v1", packageName),
			filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "model", packageName),
			filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "router", packageName),
			filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "service", packageName),
		}
		for _, path := range paths {
			if err := g.removeDirectoryIfExists(path); err != nil {
				return err
			}
		}
		return nil
	}

	return g.removeDirectoryIfExists(basePath)
}

// removeDirectoryIfExists 删除目录（如果存在）
func (g *GVAAnalyzer) removeDirectoryIfExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil // 目录不存在，无需删除
	} else if err != nil {
		return err // 其他错误
	}

	return os.RemoveAll(dirPath)
}

// cleanupRelatedApiAndMenus 清理相关的API和菜单记录
func (g *GVAAnalyzer) cleanupRelatedApiAndMenus(historyIDs []uint) error {
	if len(historyIDs) == 0 {
		return nil
	}

	// 这里可以根据需要实现具体的API和菜单清理逻辑
	// 由于涉及到具体的业务逻辑，这里只做日志记录
	global.GVA_LOG.Info(fmt.Sprintf("清理历史记录ID %v 相关的API和菜单记录", historyIDs))

	// 可以调用service层的相关方法进行清理
	// 例如：service.ServiceGroupApp.SystemApiService.DeleteApisByIds(historyIDs)
	// 例如：service.ServiceGroupApp.MenuService.DeleteMenusByIds(historyIDs)

	return nil
}

// scanPredesignedModules 扫描预设计模块
func (g *GVAAnalyzer) scanPredesignedModules() ([]PredesignedModuleInfo, error) {
	// 获取autocode配置路径
	autocodeRoot := global.GVA_CONFIG.AutoCode.Root
	if autocodeRoot == "" {
		return nil, errors.New("autocode根路径未配置")
	}

	var modules []PredesignedModuleInfo

	// 扫描plugin目录
	pluginModules, err := g.scanPluginModules(filepath.Join(autocodeRoot, global.GVA_CONFIG.AutoCode.Server, "plugin"))
	if err != nil {
		global.GVA_LOG.Warn(fmt.Sprintf("扫描plugin模块失败: %v", err))
	} else {
		modules = append(modules, pluginModules...)
	}

	// 扫描model目录
	modelModules, err := g.scanModelModules(filepath.Join(autocodeRoot, global.GVA_CONFIG.AutoCode.Server, "model"))
	if err != nil {
		global.GVA_LOG.Warn(fmt.Sprintf("扫描model模块失败: %v", err))
	} else {
		modules = append(modules, modelModules...)
	}

	return modules, nil
}

// scanPluginModules 扫描插件模块
func (g *GVAAnalyzer) scanPluginModules(pluginDir string) ([]PredesignedModuleInfo, error) {
	var modules []PredesignedModuleInfo

	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return modules, nil // 目录不存在，返回空列表
	}

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			pluginName := entry.Name()
			pluginPath := filepath.Join(pluginDir, pluginName)

			// 查找model目录
			modelDir := filepath.Join(pluginPath, "model")
			if _, err := os.Stat(modelDir); err == nil {
				// 扫描model目录下的模块
				pluginModules, err := g.scanModulesInDirectory(modelDir, pluginName, "plugin")
				if err != nil {
					global.GVA_LOG.Warn(fmt.Sprintf("扫描插件 %s 的模块失败: %v", pluginName, err))
					continue
				}
				modules = append(modules, pluginModules...)
			}
		}
	}

	return modules, nil
}

// scanModelModules 扫描模型模块
func (g *GVAAnalyzer) scanModelModules(modelDir string) ([]PredesignedModuleInfo, error) {
	var modules []PredesignedModuleInfo

	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		return modules, nil // 目录不存在，返回空列表
	}

	entries, err := os.ReadDir(modelDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			packageName := entry.Name()
			packagePath := filepath.Join(modelDir, packageName)

			// 扫描包目录下的模块
			packageModules, err := g.scanModulesInDirectory(packagePath, packageName, "package")
			if err != nil {
				global.GVA_LOG.Warn(fmt.Sprintf("扫描包 %s 的模块失败: %v", packageName, err))
				continue
			}
			modules = append(modules, packageModules...)
		}
	}

	return modules, nil
}

// scanModulesInDirectory 扫描目录中的模块
func (g *GVAAnalyzer) scanModulesInDirectory(dir, packageName, template string) ([]PredesignedModuleInfo, error) {
	var modules []PredesignedModuleInfo

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			moduleName := strings.TrimSuffix(entry.Name(), ".go")
			filePath := filepath.Join(dir, entry.Name())

			module := PredesignedModuleInfo{
				ModuleName:  moduleName,
				PackageName: packageName,
				Template:    template,
				FilePaths:   []string{filePath},
				Description: fmt.Sprintf("%s模块中的%s", packageName, moduleName),
			}
			modules = append(modules, module)
		}
	}

	return modules, nil
}
