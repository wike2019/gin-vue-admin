package system

import (
	"context"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	common "github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/ast"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/autocode"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// AutoCodePackage 自动代码生成包服务实例
// 使用单例模式，确保全局只有一个服务实例，便于统一管理和调用
var AutoCodePackage = new(autoCodePackage)

// autoCodePackage 自动代码生成包服务结构体
// 该服务负责管理代码生成模板包，支持两种模板类型：
// 1. package: 系统包模板，用于生成标准业务模块代码
// 2. plugin: 插件模板，用于生成插件化业务代码
// 好处：通过模板化生成代码，可以保证代码风格统一，减少重复工作，提高开发效率
type autoCodePackage struct{}

// Create 创建包信息并生成代码
// 该函数是整个代码生成流程的核心入口，负责验证、创建数据库记录和生成代码文件
//
// 设计思路：
// 1. 前置验证：在生成代码前进行严格验证，避免生成无效代码
// 2. 事务保证：使用数据库事务确保数据一致性和原子性操作
// 3. 模板驱动：通过模板文件生成代码，保证代码风格统一
// 4. AST注入：对于需要注入到现有文件的代码，使用AST操作确保语法正确
//
// 好处：
// - 事务保证：如果代码生成失败，数据库记录也会回滚，避免数据不一致
// - 验证前置：提前发现错误，避免生成无效代码后再回滚
// - 模板复用：通过模板可以快速生成符合项目规范的代码
//
// @author: [piexlmax](https://github.com/piexlmax)
// @author: [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodePackage) Create(ctx context.Context, info *request.SysAutoCodePackageCreate) error {
	// 使用switch进行多重条件验证，比多个if更清晰
	// 好处：集中处理所有验证逻辑，易于维护和扩展
	switch {
	case info.Template == "":
		return errors.New("模板不能为空!")
	case info.Template == "page":
		// page模板是表单生成器专用，不能用于包生成
		// 这样设计可以避免模板类型混淆，保证功能边界清晰
		return errors.New("page为表单生成器!")
	case info.PackageName == "":
		return errors.New("PackageName不能为空!")
	case token.IsKeyword(info.PackageName):
		// 检查包名是否为Go关键字，避免编译错误
		// 好处：提前发现命名冲突，避免生成无法编译的代码
		return errors.Errorf("%s为go的关键字!", info.PackageName)
	case info.Template == "package":
		// 保护系统核心包名，防止覆盖系统关键代码
		// 好处：保护系统稳定性，避免误操作导致系统崩溃
		if info.PackageName == "system" || info.PackageName == "example" {
			return errors.New("不能使用已保留的package name")
		}
	default:
		break
	}
	// 检查包名和模板组合是否已存在，避免重复创建
	// 使用组合键（package_name + template）确保唯一性
	// 好处：允许同名包使用不同模板，同时防止完全重复的记录
	if !errors.Is(global.GVA_DB.Where("package_name = ? and template = ?", info.PackageName, info.Template).First(&model.SysAutoCodePackage{}).Error, gorm.ErrRecordNotFound) {
		return errors.New("存在相同PackageName")
	}
	create := info.Create()
	// 使用事务确保数据库操作和文件生成的原子性
	// 好处：如果文件生成失败，数据库记录会自动回滚，保证数据一致性
	return global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先创建数据库记录，如果失败则直接回滚
		err := tx.Create(&create).Error
		if err != nil {
			return errors.Wrap(err, "创建失败!")
		}
		// 生成代码生成所需的数据结构
		code := info.AutoCode()
		// 解析模板，返回三个映射：
		// - code: 普通模板文件映射（模板路径 -> 生成文件路径）
		// - asts: AST注入映射（需要注入到现有文件的代码）
		// - creates: 需要创建的文件映射（模板路径 -> 生成文件路径）
		// 好处：分离普通生成和AST注入，便于分别处理
		_, asts, creates, err := s.templates(ctx, create, code, true)
		if err != nil {
			return err
		}
		// 处理普通模板文件生成
		// key: 模板文件的绝对路径，value: 生成文件的绝对路径
		// 好处：通过映射关系，可以批量处理多个文件，代码更简洁
		for key, value := range creates {
			// 使用Go标准库的text/template解析模板文件
			// filepath.Base(key)作为模板名称，避免路径冲突
			// autocode.GetTemplateFuncMap()提供自定义模板函数，增强模板功能
			// 好处：支持复杂的模板逻辑，如字符串处理、格式化等
			var files *template.Template
			files, err = template.New(filepath.Base(key)).Funcs(autocode.GetTemplateFuncMap()).ParseFiles(key)
			if err != nil {
				return errors.Wrapf(err, "[filepath:%s]读取模版文件失败!", key)
			}
			// 创建目标文件的目录结构
			// os.ModePerm (0777) 确保目录可读写，适合开发环境
			// 好处：自动创建多层目录，避免手动创建目录的繁琐
			err = os.MkdirAll(filepath.Dir(value), os.ModePerm)
			if err != nil {
				return errors.Wrapf(err, "[filepath:%s]创建文件夹失败!", value)
			}
			// 创建目标文件
			var file *os.File
			file, err = os.Create(value)
			if err != nil {
				return errors.Wrapf(err, "[filepath:%s]创建文件夹失败!", value)
			}
			// 执行模板渲染，将数据填充到模板中生成最终代码
			// 好处：模板化生成保证代码风格统一，易于维护
			err = files.Execute(file, code)
			_ = file.Close()
			if err != nil {
				return errors.Wrapf(err, "[filepath:%s]生成失败!", value)
			}
			fmt.Printf("[template:%s][filepath:%s]生成成功!\n", key, value)
		}
		// 处理AST注入：将生成的代码注入到现有文件中
		// key格式: "文件路径=>AST类型"，通过"=>"分隔符便于解析
		// 好处：使用分隔符比结构体更灵活，易于扩展新的AST类型
		for key, value := range asts {
			keys := strings.Split(key, "=>")
			if len(keys) == 2 {
				// 只处理需要注入的特定类型，其他类型可能只需要创建文件
				// 好处：区分不同类型的处理逻辑，避免不必要的AST操作
				switch keys[1] {
				case ast.TypePluginInitializeV2, ast.TypePackageApiEnter, ast.TypePackageRouterEnter, ast.TypePackageServiceEnter:
					// 解析现有文件为AST
					file, _ := value.Parse("", nil)
					if file != nil {
						// 注入新代码到AST中
						// 好处：使用AST确保注入的代码语法正确，避免字符串拼接导致的语法错误
						err = value.Injection(file)
						if err != nil {
							return err
						}
						// 格式化并写回文件
						// 好处：自动格式化代码，保证代码风格统一
						err = value.Format("", nil, file)
						if err != nil {
							return err
						}
					}
					fmt.Printf("[type:%s]注入成功!\n", key)
				}
			}
		}
		return nil
	})
}

// Delete 删除包记录
// 根据ID删除单个包记录
// 设计原因：提供精确删除功能，适用于UI中的单条删除操作
// 好处：操作简单直接，错误处理清晰
//
// @author: [piexlmax](https://github.com/piexlmax)
// @author: [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodePackage) Delete(ctx context.Context, info common.GetById) error {
	err := global.GVA_DB.WithContext(ctx).Delete(&model.SysAutoCodePackage{}, info.Uint()).Error
	if err != nil {
		return errors.Wrap(err, "删除失败!")
	}
	return nil
}

// DeleteByNames 根据包名批量删除包记录
// 设计原因：支持批量操作，提高删除效率
// 使用场景：清理多个包、批量同步时删除不存在的包等
// 好处：
// - 批量操作减少数据库交互次数，提高性能
// - 空数组检查避免无效查询，节省资源
// - 使用IN查询，SQL简洁高效
//
// @author: [piexlmax](https://github.com/piexlmax)
// @author: [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodePackage) DeleteByNames(ctx context.Context, names []string) error {
	// 空数组直接返回，避免无效的数据库查询
	// 好处：提前返回，节省资源，代码更健壮
	if len(names) == 0 {
		return nil
	}
	// 使用IN查询批量删除，比循环删除效率更高
	// 好处：一次SQL操作完成批量删除，减少数据库压力
	err := global.GVA_DB.WithContext(ctx).Where("package_name IN ?", names).Delete(&model.SysAutoCodePackage{}).Error
	if err != nil {
		return errors.Wrap(err, "删除失败!")
	}
	return nil
}

// All 获取所有包，并自动同步文件系统与数据库
//
// 核心功能：
// 1. 自动发现：扫描文件系统中的包（service和plugin目录）
// 2. 自动同步：将文件系统中存在但数据库中不存在的包自动添加到数据库
// 3. 自动清理：删除数据库中存在但文件系统中不存在的包记录
//
// 设计原因：
// - 支持手动创建包目录后自动识别，无需手动添加数据库记录
// - 保持数据库与文件系统的一致性，避免数据不同步
// - 自动检测插件结构完整性，提供友好的提示信息
//
// 好处：
// - 提高开发效率：开发者只需创建目录，系统自动识别
// - 数据一致性：自动同步机制保证数据库和文件系统一致
// - 容错性强：即使手动删除目录，系统也能自动清理数据库记录
//
// @author: [piexlmax](https://github.com/piexlmax)
// @author: [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodePackage) All(ctx context.Context) (entities []model.SysAutoCodePackage, err error) {
	// 分别收集server包和plugin包，便于分类处理
	// 好处：逻辑清晰，便于后续分别处理不同类型的包
	server := make([]model.SysAutoCodePackage, 0)
	plugin := make([]model.SysAutoCodePackage, 0)
	// 使用filepath.Join构建路径，跨平台兼容性好
	// 好处：自动处理不同操作系统的路径分隔符，代码更健壮
	serverPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "service")
	pluginPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin")
	// 读取service目录，发现所有server包
	serverDir, err := os.ReadDir(serverPath)
	if err != nil {
		return nil, errors.Wrap(err, "读取service文件夹失败!")
	}
	// 读取plugin目录，发现所有plugin包
	pluginDir, err := os.ReadDir(pluginPath)
	if err != nil {
		return nil, errors.Wrap(err, "读取plugin文件夹失败!")
	}
	// 遍历service目录，发现所有server包
	// 只处理目录，忽略文件
	// 好处：自动识别所有已存在的包，无需手动配置
	for i := 0; i < len(serverDir); i++ {
		if serverDir[i].IsDir() {
			// 为每个发现的包创建标准化的包信息
			// 好处：统一包信息格式，便于后续处理
			serverPackage := model.SysAutoCodePackage{
				PackageName: serverDir[i].Name(),
				Template:    "package",
				Label:       serverDir[i].Name() + "包",
				Desc:        "系统自动读取" + serverDir[i].Name() + "包",
				Module:      global.GVA_CONFIG.AutoCode.Module,
			}
			server = append(server, serverPackage)
		}
	}
	// 遍历plugin目录，发现所有plugin包
	// plugin包需要验证结构完整性，确保符合v2插件标准
	for i := 0; i < len(pluginDir); i++ {
		if pluginDir[i].IsDir() {
			// 定义v2插件必须包含的目录结构
			// 使用map便于快速查找和删除已找到的目录
			// 好处：通过删除操作，最终剩余的key就是缺失的目录，逻辑清晰
			dirNameMap := map[string]bool{
				"api":        true,
				"config":     true,
				"initialize": true,
				"plugin":     true,
				"router":     true,
				"service":    true,
			}
			dir, e := os.ReadDir(filepath.Join(pluginPath, pluginDir[i].Name()))
			if e != nil {
				return nil, errors.Wrap(err, "读取plugin文件夹失败!")
			}
			// 检查插件目录是否包含所有必需的子目录
			// 通过删除已找到的目录，最终dirNameMap中剩余的就是缺失的目录
			// 好处：一次遍历完成检查，效率高，逻辑清晰
			for k := 0; k < len(dir); k++ {
				if dir[k].IsDir() {
					if ok := dirNameMap[dir[k].Name()]; ok {
						delete(dirNameMap, dir[k].Name())
					}
				}
			}

			// 根据结构完整性生成不同的描述信息
			// 好处：给用户明确的提示，帮助识别插件是否符合标准
			var desc string
			if len(dirNameMap) == 0 {
				// 完全符合标准结构
				desc = "系统自动读取" + pluginDir[i].Name() + "插件，使用前请确认是否为v2版本插件"
			} else {
				// 缺少某些结构，生成警告描述
				// 收集缺失的目录名，生成友好的提示信息
				var missingDirs []string
				for dirName := range dirNameMap {
					missingDirs = append(missingDirs, dirName)
				}
				desc = fmt.Sprintf("系统自动读取，但是缺少 %s 结构，不建议自动化和mcp使用", strings.Join(missingDirs, "、"))
			}

			pluginPackage := model.SysAutoCodePackage{
				PackageName: pluginDir[i].Name(),
				Template:    "plugin",
				Label:       pluginDir[i].Name() + "插件",
				Desc:        desc,
				Module:      global.GVA_CONFIG.AutoCode.Module,
			}
			plugin = append(plugin, pluginPackage)
		}
	}

	// 从数据库获取所有已存在的包记录
	err = global.GVA_DB.WithContext(ctx).Find(&entities).Error
	if err != nil {
		return nil, errors.Wrap(err, "获取所有包失败!")
	}
	// 将数据库记录转换为map，便于快速查找
	// 好处：O(1)查找复杂度，比遍历数组效率高
	entitiesMap := make(map[string]model.SysAutoCodePackage)
	for i := 0; i < len(entities); i++ {
		entitiesMap[entities[i].PackageName] = entities[i]
	}
	// 找出文件系统中存在但数据库中不存在的包（需要新增）
	createEntity := []model.SysAutoCodePackage{}
	for i := 0; i < len(server); i++ {
		// 使用map查找，避免嵌套循环，提高效率
		if _, ok := entitiesMap[server[i].PackageName]; !ok {
			if server[i].Template == "package" {
				createEntity = append(createEntity, server[i])
			}
		}
	}
	for i := 0; i < len(plugin); i++ {
		if _, ok := entitiesMap[plugin[i].PackageName]; !ok {
			if plugin[i].Template == "plugin" {
				createEntity = append(createEntity, plugin[i])
			}
		}
	}

	// 批量创建新发现的包记录
	// 好处：批量操作减少数据库交互，提高性能
	if len(createEntity) > 0 {
		err = global.GVA_DB.WithContext(ctx).Create(&createEntity).Error
		if err != nil {
			return nil, errors.Wrap(err, "同步失败!")
		}
		// 将新创建的记录添加到返回结果中
		entities = append(entities, createEntity...)
	}

	// 处理数据库存在但实体文件不存在的情况 - 删除数据库中对应的数据
	// 设计原因：保持数据库与文件系统的一致性，避免"僵尸"记录
	// 好处：自动清理无效记录，保证数据准确性
	existingPackageNames := make(map[string]bool)
	// 收集所有文件系统中存在的包名
	for i := 0; i < len(server); i++ {
		existingPackageNames[server[i].PackageName] = true
	}
	for i := 0; i < len(plugin); i++ {
		existingPackageNames[plugin[i].PackageName] = true
	}

	// 找出需要删除的数据库记录（数据库中存在但文件系统中不存在）
	deleteEntityIDs := []uint{}
	for i := 0; i < len(entities); i++ {
		if !existingPackageNames[entities[i].PackageName] {
			deleteEntityIDs = append(deleteEntityIDs, entities[i].ID)
		}
	}

	// 删除数据库中不存在文件的记录
	// 好处：自动清理，无需手动维护，保证数据一致性
	if len(deleteEntityIDs) > 0 {
		err = global.GVA_DB.WithContext(ctx).Delete(&model.SysAutoCodePackage{}, deleteEntityIDs).Error
		if err != nil {
			return nil, errors.Wrap(err, "删除不存在的包记录失败!")
		}
		// 从返回结果中移除已删除的记录
		// 好处：返回结果只包含有效的记录，避免返回已删除的数据
		filteredEntities := []model.SysAutoCodePackage{}
		for i := 0; i < len(entities); i++ {
			if existingPackageNames[entities[i].PackageName] {
				filteredEntities = append(filteredEntities, entities[i])
			}
		}
		entities = filteredEntities
	}

	return entities, nil
}

// Templates 获取所有可用的模板文件夹
//
// 功能：扫描resource目录，返回所有可用的代码生成模板
// 设计原因：动态发现模板，支持扩展新模板而无需修改代码
// 好处：
// - 可扩展性：添加新模板只需在resource目录创建文件夹
// - 灵活性：支持多种代码生成场景（package、plugin等）
// - 维护性：模板与代码分离，便于维护和更新
//
// 过滤规则：
// - page: 表单生成器专用，不用于包生成
// - function: 函数生成器专用
// - preview: 预览功能专用
// - mcp: MCP生成器专用
// 好处：通过过滤，只返回适用于包生成的模板，避免混淆
//
// @author: [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodePackage) Templates(ctx context.Context) ([]string, error) {
	templates := make([]string, 0)
	// 读取resource目录，该目录包含所有代码生成模板
	entries, err := os.ReadDir("resource")
	if err != nil {
		return nil, errors.Wrap(err, "读取模版文件夹失败!")
	}
	// 遍历目录，筛选出可用的模板
	for i := 0; i < len(entries); i++ {
		if entries[i].IsDir() {
			// 跳过特殊用途的模板目录，这些不是包生成模板
			// 好处：明确区分不同类型的模板，避免误用
			if entries[i].Name() == "page" {
				continue
			} // page 为表单生成器
			if entries[i].Name() == "function" {
				continue
			} // function 为函数生成器
			if entries[i].Name() == "preview" {
				continue
			} // preview 为预览代码生成器的代码
			if entries[i].Name() == "mcp" {
				continue
			} // mcp 为mcp生成器的代码
			// 将符合条件的模板目录添加到结果中
			templates = append(templates, entries[i].Name())
		}
	}
	return templates, nil
}

// templates 解析模板目录结构，生成代码文件映射和AST注入映射
//
// 这是代码生成的核心逻辑函数，负责：
// 1. 递归遍历模板目录结构
// 2. 识别不同类型的模板文件（普通模板、AST注入模板）
// 3. 根据模板类型和包类型生成正确的目标文件路径
// 4. 区分package和plugin的不同处理逻辑
//
// 返回值：
// - code: 普通模板文件映射（模板路径 -> 生成文件路径），直接通过模板渲染生成
// - asts: AST注入映射（文件路径+类型 -> AST对象），需要注入到现有文件中
// - creates: 需要创建的文件映射（模板路径 -> 生成文件路径），包含所有需要生成的文件
//
// 设计原因：
// - 分离普通生成和AST注入，便于分别处理
// - 支持复杂的目录结构，灵活适应不同模板组织方式
// - 区分package和plugin，处理不同的代码组织方式
//
// 好处：
// - 代码复用：通过模板复用代码生成逻辑
// - 灵活扩展：支持添加新的模板类型和目录结构
// - 类型安全：通过AST注入确保代码语法正确
//
// 示例1：生成package类型的用户管理模块
//
//	entity := model.SysAutoCodePackage{
//		Template:    "package",
//		PackageName: "user",
//		Desc:        "用户管理包",
//		Label:       "用户管理",
//	}
//	info := request.AutoCode{
//		StructName:      "User",
//		HumpPackageName: "user",
//		PackageName:     "user",
//		Abbreviation:    "user",
//		GenerateServer:  true,
//		GenerateWeb:     true,
//		BusinessDB:      "mysql",
//	}
//	code, asts, creates, err := s.templates(ctx, entity, info, true)
//
//	// 返回结果示例：
//	// code: {
//	//   "resource/package/server/api/user_api.tpl": "server/api/v1/user/user.go",
//	//   "resource/package/server/router/user_router.tpl": "server/router/user/user.go",
//	//   "resource/package/server/service/user_service.tpl": "server/service/user/user.go",
//	//   "resource/package/server/model/user_model.tpl": "server/model/user/user.go",
//	// }
//	// asts: {
//	//   "server/api/v1/enter.go=>TypePackageApiEnter": PackageApiEnter{...},
//	//   "server/api/v1/user/enter.go=>TypePackageApiModuleEnter": PackageApiModuleEnter{...},
//	//   "server/router/enter.go=>TypePackageRouterEnter": PackageRouterEnter{...},
//	//   "server/router/user/enter.go=>TypePackageRouterModuleEnter": PackageRouterModuleEnter{...},
//	//   "server/initialize/router_biz.go=>TypePackageInitializeRouter": PackageInitializeRouter{...},
//	//   "server/service/user/enter.go=>TypePackageServiceEnter": PackageServiceEnter{...},
//	//   "server/initialize/gorm_biz.go=>TypePackageInitializeGorm": PackageInitializeGorm{...},
//	// }
//	// creates: {
//	//   "resource/package/server/api/user_api.tpl": "server/api/v1/user/user.go",
//	//   "resource/package/server/api/user_api_enter.tpl": "server/api/v1/user/enter.go",
//	//   "resource/package/server/router/user_router.tpl": "server/router/user/user.go",
//	//   "resource/package/server/router/user_router_enter.tpl": "server/router/user/enter.go",
//	//   "resource/package/server/service/user_service.tpl": "server/service/user/user.go",
//	//   "resource/package/server/service/user_service_enter.tpl": "server/service/user/enter.go",
//	//   "resource/package/server/model/user_model.tpl": "server/model/user/user.go",
//	// }
//
// 示例2：生成plugin类型的订单管理插件
//
//	entity := model.SysAutoCodePackage{
//		Template:    "plugin",
//		PackageName: "order",
//		Desc:        "订单管理插件",
//		Label:       "订单管理",
//	}
//	info := request.AutoCode{
//		StructName:      "Order",
//		HumpPackageName: "order",
//		PackageName:     "order",
//		Abbreviation:    "order",
//		GenerateServer:  true,
//		GenerateWeb:     true,
//		BusinessDB:      "mysql",
//	}
//	code, asts, creates, err := s.templates(ctx, entity, info, false)
//
//	// 返回结果示例：
//	// code: {
//	//   "resource/plugin/server/api/order_api.tpl": "server/plugin/order/api/order.go",
//	//   "resource/plugin/server/router/order_router.tpl": "server/plugin/order/router/order.go",
//	//   "resource/plugin/server/service/order_service.tpl": "server/plugin/order/service/order.go",
//	//   "resource/plugin/server/model/order_model.tpl": "server/plugin/order/model/order.go",
//	// }
//	// asts: {
//	//   "server/plugin/order/router/enter.go=>TypePluginRouterEnter": PluginRouterEnter{...},
//	//   "server/plugin/order/api/enter.go=>TypePluginApiEnter": PluginApiEnter{...},
//	//   "server/plugin/order/service/enter.go=>TypePluginServiceEnter": PluginServiceEnter{...},
//	//   "server/plugin/order/initialize/gen.go=>TypePluginGen": PluginGen{...},
//	//   "server/plugin/order/initialize/gorm.go=>TypePluginInitializeGorm": PluginInitializeGorm{...},
//	//   "server/plugin/order/initialize/router.go=>TypePluginInitializeRouter": PluginInitializeRouter{...},
//	//   "server/initialize/plugin_biz_v2.go=>TypePluginInitializeV2": PluginInitializeV2{...},
//	// }
//	// creates: {
//	//   "resource/plugin/server/api/order_api.tpl": "server/plugin/order/api/order.go",
//	//   "resource/plugin/server/api/order_api_enter.tpl": "server/plugin/order/api/enter.go",
//	//   "resource/plugin/server/router/order_router.tpl": "server/plugin/order/router/order.go",
//	//   "resource/plugin/server/router/order_router_enter.tpl": "server/plugin/order/router/enter.go",
//	//   "resource/plugin/server/service/order_service.tpl": "server/plugin/order/service/order.go",
//	//   "resource/plugin/server/service/order_service_enter.tpl": "server/plugin/order/service/enter.go",
//	//   "resource/plugin/server/model/order_model.tpl": "server/plugin/order/model/order.go",
//	//   "resource/plugin/server/initialize/gen.tpl": "server/plugin/order/initialize/gen.go",
//	//   "resource/plugin/server/initialize/gorm.tpl": "server/plugin/order/initialize/gorm.go",
//	//   "resource/plugin/server/initialize/router.tpl": "server/plugin/order/initialize/router.go",
//	//   "resource/plugin/server/main.go.tpl": "server/plugin/order/main.go",
//	// }
//
// 示例3：只生成后端代码，不生成前端代码
//
//	entity := model.SysAutoCodePackage{
//		Template:    "package",
//		PackageName: "product",
//		Desc:        "产品管理包",
//		Label:       "产品管理",
//	}
//	info := request.AutoCode{
//		StructName:      "Product",
//		HumpPackageName: "product",
//		PackageName:     "product",
//		Abbreviation:    "product",
//		GenerateServer:  true,  // 生成后端代码
//		GenerateWeb:     false, // 不生成前端代码
//		BusinessDB:      "mysql",
//	}
//	code, asts, creates, err := s.templates(ctx, entity, info, true)
//
//	// 返回结果：只包含server目录下的文件，不包含web目录下的文件
//
// 示例4：处理model目录下的request子目录
//
//	// 当模板包含 model/request/xxx_request.tpl 时
//	// 会生成对应的请求结构体文件
//	// package类型: server/model/user/request/user.go
//	// plugin类型: server/plugin/order/model/request/order.go
//
// 详细处理流程说明：
//
// 1. 模板目录结构解析
//   - 模板目录路径：{AutoCode.Root}/{AutoCode.Server}/resource/{entity.Template}/
//   - 第一层目录：server（后端代码）或 web（前端代码）
//   - 第二层目录：根据功能分类（api、router、service、model、initialize等）
//   - 模板文件：必须以 .tpl 结尾，如 user_api.tpl
//
// 2. 三种文件映射的区别
//
//   - code映射：存储普通模板文件，直接通过模板引擎渲染生成目标文件
//     key: 模板文件路径，value: 生成的文件路径
//     例如：{"resource/package/server/api/user_api.tpl": "server/api/v1/user/user.go"}
//
//   - asts映射：存储需要AST注入的代码，这些代码需要注入到现有文件中
//     key: 目标文件路径 + "=>" + AST类型字符串，value: AST对象
//     例如：{"server/api/v1/enter.go=>TypePackageApiEnter": PackageApiEnter{...}}
//     使用AST注入的原因：
//
//   - 需要向现有文件添加代码（如注册新的API组到enter.go）
//
//   - 确保注入的代码语法正确
//
//   - 避免手动修改系统核心文件
//
//   - creates映射：存储所有需要创建的文件路径映射
//     key: 模板文件路径，value: 生成的文件路径
//     creates包含code中的所有映射，以及AST注入目标文件路径
//     creates用于后续实际创建文件
//
// 3. package模板 vs plugin模板的主要区别
//
//   - package模板：生成系统标准包，代码集成到主系统目录结构
//
//   - API路径：server/api/v1/{packageName}/{moduleName}.go
//
//   - Router路径：server/router/{packageName}/{moduleName}.go
//
//   - Service路径：server/service/{packageName}/{moduleName}.go
//
//   - Model路径：server/model/{packageName}/{moduleName}.go
//
//   - enter.go注入：需要注入到系统核心的enter.go文件中
//
//   - 路由初始化：需要注入到initialize/router_biz.go
//
//   - GORM初始化：需要注入到initialize/gorm_biz.go
//
//   - plugin模板：生成插件代码，代码独立在plugin目录下
//
//   - 所有代码路径：server/plugin/{packageName}/{type}/{moduleName}.go
//
//   - enter.go注入：只注入到插件自己的enter.go文件中
//
//   - 插件初始化：需要注入到initialize/plugin_biz_v2.go
//
//   - 额外目录：gen、config、initialize、plugin、response等插件特有目录
//
// 4. 关键处理逻辑详解
//
//	4.1 server目录处理
//	    - 检查info.GenerateServer标志，决定是否生成后端代码
//	    - 处理server目录下的直接文件（main.go.tpl、plugin.go.tpl）
//	      * 这些文件需要注入到initialize/plugin_biz_v2.go中
//	      * 用于插件初始化
//	    - 处理核心目录（api、router、service）
//	      * 遍历目录下的.tpl模板文件
//	      * 通过文件名识别模板类型（api、router、service、enter）
//	      * 对于enter.go文件：
//	        - package类型：需要注入到多个系统核心文件
//	        - plugin类型：只注入到插件自己的enter.go
//	      * 对于普通文件：生成到对应的目标路径
//	    - 处理插件特有目录（gen、config、initialize、plugin、response）
//	      * 只对plugin模板有效，package模板跳过
//	      * 处理插件初始化、配置等特殊文件
//	    - 处理model目录
//	      * 支持递归处理子目录（如model/request/）
//	      * package类型的model需要注入到initialize/gorm_biz.go
//	      * plugin类型的model不需要额外注入
//
//	4.2 web目录处理
//	    - 检查info.GenerateWeb标志，决定是否生成前端代码
//	    - 处理前端代码目录（api、form、view、table）
//	    - package类型和plugin类型的前端代码路径不同
//	      * package: web/{type}/{packageName}/{moduleName}/...
//	      * plugin: web/plugin/{packageName}/{type}/...
//
//	4.3 enter.go文件的特殊处理
//	    enter.go文件用于模块注册，不同类型的enter.go有不同的注入逻辑：
//
//	    * Package API enter.go注入（package类型）：
//	      - 注入到 api/v1/enter.go：注册API组
//	      - 注入到 api/v1/{package}/enter.go：注册具体API模块到组
//
//	    * Package Router enter.go注入（package类型）：
//	      - 注入到 router/enter.go：注册路由组
//	      - 注入到 router/{package}/enter.go：注册具体路由模块
//	      - 注入到 initialize/router_biz.go：初始化路由
//
//	    * Package Service enter.go注入（package类型）：
//	      - 注入到 service/{package}/enter.go：注册服务组和模块
//
//	    * Plugin enter.go注入（plugin类型）：
//	      - 只注入到插件自己的enter.go文件中（plugin/{package}/{type}/enter.go）
//	      - 插件代码自包含，不修改系统核心文件
//
//	4.4 文件路径生成规则
//	    - 使用strings.Index识别文件名中的关键字（api、router、service、enter等）
//	    - 使用filepath.Join构建跨平台兼容的路径
//	    - 根据模板类型和目录类型动态生成目标路径
//	    - 移除.tpl扩展名，生成对应的.go或.vue文件
//
//	4.5 错误处理
//	    - 文件扩展名验证：必须是.tpl文件
//	    - 文件名验证：必须包含有效标识（api、router、service等）
//	    - 目录结构验证：模板文件不能是目录
//	    - 非法模板验证：文件名不能包含所有关键字（防止命名冲突）
//
// 5. 设计优势
//   - 分离关注点：code、asts、creates三种映射分离不同类型的文件处理
//   - 灵活扩展：通过switch-case结构，易于添加新的目录类型和处理逻辑
//   - 类型安全：使用AST注入确保代码语法正确
//   - 自动化：自动完成模块注册和初始化，减少手动操作
//   - 兼容性：区分package和plugin，支持不同的代码组织方式
//
// 参数说明：
//   - ctx: 上下文对象，用于传递请求上下文信息
//   - entity: 自动代码包实体，包含模板类型、包名等信息
//   - info: 代码生成信息，包含结构体名、生成标志等
//   - isPackage: 是否为包创建模式（true=包创建，false=普通代码生成）
//
// 返回值说明：
//   - code: 普通模板文件映射（模板路径 -> 生成文件路径）
//   - asts: AST注入映射（文件路径+类型 -> AST对象）
//   - creates: 需要创建的文件映射（模板路径 -> 生成文件路径）
//   - err: 错误信息，如果处理过程中出现错误则返回
func (s *autoCodePackage) templates(ctx context.Context, entity model.SysAutoCodePackage, info request.AutoCode, isPackage bool) (code map[string]string, asts map[string]ast.Ast, creates map[string]string, err error) {
	// 初始化三个映射，分别存储不同类型的文件信息
	// 好处：分类存储，便于后续分别处理
	code = make(map[string]string)
	asts = make(map[string]ast.Ast)
	creates = make(map[string]string)
	// 构建模板目录路径
	templateDir := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "resource", entity.Template)
	templateDirs, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", templateDir)
	}
	// 遍历模板目录的第一层，根据目录名进行不同的处理
	// 好处：通过switch分类处理，逻辑清晰，易于扩展新的目录类型
	for i := 0; i < len(templateDirs); i++ {
		second := filepath.Join(templateDir, templateDirs[i].Name())
		switch templateDirs[i].Name() {
		case "server":
			// server目录包含后端服务代码模板
			// 如果用户选择不生成server代码且不是包创建，则跳过
			// 好处：支持选择性生成，节省资源，提高灵活性
			if !info.GenerateServer && !isPackage {
				break
			}
			var secondDirs []os.DirEntry
			secondDirs, err = os.ReadDir(second)
			if err != nil {
				return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", second)
			}
			// 遍历server目录下的子目录或文件
			for j := 0; j < len(secondDirs); j++ {
				// 跳过macOS系统文件，避免干扰
				// 好处：提高跨平台兼容性
				if secondDirs[j].Name() == ".DS_Store" {
					continue
				}
				three := filepath.Join(second, secondDirs[j].Name())
				// 处理server目录下的直接文件（非目录）
				// 这些通常是插件初始化相关的特殊文件
				if !secondDirs[j].IsDir() {
					ext := filepath.Ext(secondDirs[j].Name())
					// 验证文件扩展名，只处理.tpl模板文件
					// 好处：确保只处理有效的模板文件，避免误处理其他文件
					if ext != ".tpl" {
						return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版后缀!", three)
					}
					name := strings.TrimSuffix(secondDirs[j].Name(), ext)
					// 处理插件初始化文件（main.go、plugin.go）
					// 这些文件需要注入到initialize/plugin_biz_v2.go中
					// 好处：自动注册插件，无需手动修改初始化文件
					if name == "main.go" || name == "plugin.go" {
						pluginInitialize := &ast.PluginInitializeV2{
							Type:        ast.TypePluginInitializeV2,
							Path:        filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, name),
							PluginPath:  filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "initialize", "plugin_biz_v2.go"),
							ImportPath:  fmt.Sprintf(`"%s/plugin/%s"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName),
							PackageName: entity.PackageName,
						}
						// 使用"路径=>类型"作为key，便于后续识别和处理
						asts[pluginInitialize.PluginPath+"=>"+pluginInitialize.Type.String()] = pluginInitialize
						creates[three] = pluginInitialize.Path
						continue
					}
					return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", three)
				}
				// 处理api、router、service三个核心目录
				// 这些目录包含业务代码的主要模板
				switch secondDirs[j].Name() {
				case "api", "router", "service":
					var threeDirs []os.DirEntry
					threeDirs, err = os.ReadDir(three)
					if err != nil {
						return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", three)
					}
					// 遍历目录下的模板文件
					for k := 0; k < len(threeDirs); k++ {
						if threeDirs[k].Name() == ".DS_Store" {
							continue
						}
						four := filepath.Join(three, threeDirs[k].Name())
						// 模板文件不能是目录，必须是.tpl文件
						// 好处：严格验证模板结构，避免配置错误
						if threeDirs[k].IsDir() {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件夹!", four)
						}
						ext := filepath.Ext(four)
						if ext != ".tpl" {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版后缀!", four)
						}
						// 通过文件名识别模板类型
						// 使用strings.Index而不是精确匹配，支持更灵活的文件命名
						// 好处：支持如"xxx_api_enter.tpl"这样的复合命名
						api := strings.Index(threeDirs[k].Name(), "api")
						hasEnter := strings.Index(threeDirs[k].Name(), "enter")
						router := strings.Index(threeDirs[k].Name(), "router")
						service := strings.Index(threeDirs[k].Name(), "service")
						// 验证文件名必须包含至少一个有效标识
						if router == -1 && api == -1 && service == -1 && hasEnter == -1 {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", four)
						}
						// package模板的特殊处理逻辑
						// package模板用于生成系统标准包，路径结构与plugin不同
						if entity.Template == "package" {
							// 默认路径：server/{api|router|service}/{packageName}/{moduleName}.go
							create := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), entity.PackageName, info.HumpPackageName+".go")
							// API文件需要放在v1子目录下，符合RESTful API版本管理规范
							// 好处：支持API版本管理，便于后续扩展新版本
							if api != -1 {
								create = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), "v1", entity.PackageName, info.HumpPackageName+".go")
							}
							// enter.go文件需要注入到对应的enter.go中，实现模块注册
							// 好处：自动注册新模块，无需手动修改enter.go文件
							if hasEnter != -1 {
								// 识别当前处理的目录类型（api、router、service）
								isApi := strings.Index(secondDirs[j].Name(), "api")
								isRouter := strings.Index(secondDirs[j].Name(), "router")
								isService := strings.Index(secondDirs[j].Name(), "service")
								// 处理API的enter.go注入
								// 需要注入到两个地方：
								// 1. api/v1/enter.go - 注册API组
								// 2. api/v1/{package}/enter.go - 注册具体模块
								// 好处：自动建立模块间的依赖关系，保证代码结构正确
								if isApi != -1 {
									// 注入到api/v1/enter.go，注册新的API组
									packageApiEnter := &ast.PackageEnter{
										Type:              ast.TypePackageApiEnter,
										Path:              filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), "v1", "enter.go"),
										ImportPath:        fmt.Sprintf(`"%s/%s/%s/%s"`, global.GVA_CONFIG.AutoCode.Module, "api", "v1", entity.PackageName),
										StructName:        utils.FirstUpper(entity.PackageName) + "ApiGroup",
										PackageName:       entity.PackageName,
										PackageStructName: "ApiGroup",
									}
									asts[packageApiEnter.Path+"=>"+packageApiEnter.Type.String()] = packageApiEnter
									// 注入到api/v1/{package}/enter.go，注册具体模块到API组
									packageApiModuleEnter := &ast.PackageModuleEnter{
										Type:        ast.TypePackageApiModuleEnter,
										Path:        filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), "v1", entity.PackageName, "enter.go"),
										ImportPath:  fmt.Sprintf(`"%s/service"`, global.GVA_CONFIG.AutoCode.Module),
										StructName:  info.StructName + "Api",
										AppName:     "ServiceGroupApp",
										GroupName:   utils.FirstUpper(entity.PackageName) + "ServiceGroup",
										ModuleName:  info.Abbreviation + "Service",
										PackageName: "service",
										ServiceName: info.StructName + "Service",
									}
									asts[packageApiModuleEnter.Path+"=>"+packageApiModuleEnter.Type.String()] = packageApiModuleEnter
									creates[four] = packageApiModuleEnter.Path
								}
								// 处理Router的enter.go注入
								// Router需要注入到三个地方：
								// 1. router/enter.go - 注册路由组
								// 2. router/{package}/enter.go - 注册具体路由模块
								// 3. initialize/router_biz.go - 初始化路由
								// 好处：自动建立完整的路由注册链路，确保路由正确加载
								if isRouter != -1 {
									// 注入到router/enter.go，注册新的路由组
									packageRouterEnter := &ast.PackageEnter{
										Type:              ast.TypePackageRouterEnter,
										Path:              filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), "enter.go"),
										ImportPath:        fmt.Sprintf(`"%s/%s/%s"`, global.GVA_CONFIG.AutoCode.Module, secondDirs[j].Name(), entity.PackageName),
										StructName:        utils.FirstUpper(entity.PackageName),
										PackageName:       entity.PackageName,
										PackageStructName: "RouterGroup",
									}
									asts[packageRouterEnter.Path+"=>"+packageRouterEnter.Type.String()] = packageRouterEnter
									// 注入到router/{package}/enter.go，注册具体路由模块
									packageRouterModuleEnter := &ast.PackageModuleEnter{
										Type:        ast.TypePackageRouterModuleEnter,
										Path:        filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), entity.PackageName, "enter.go"),
										ImportPath:  fmt.Sprintf(`api "%s/api/v1"`, global.GVA_CONFIG.AutoCode.Module),
										StructName:  info.StructName + "Router",
										AppName:     "ApiGroupApp",
										GroupName:   utils.FirstUpper(entity.PackageName) + "ApiGroup",
										ModuleName:  info.Abbreviation + "Api",
										PackageName: "api",
										ServiceName: info.StructName + "Api",
									}
									creates[four] = packageRouterModuleEnter.Path
									asts[packageRouterModuleEnter.Path+"=>"+packageRouterModuleEnter.Type.String()] = packageRouterModuleEnter
									// 注入到initialize/router_biz.go，在系统初始化时注册路由
									// 好处：自动完成路由初始化，无需手动修改初始化文件
									packageInitializeRouter := &ast.PackageInitializeRouter{
										Type:                 ast.TypePackageInitializeRouter,
										Path:                 filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "initialize", "router_biz.go"),
										ImportPath:           fmt.Sprintf(`"%s/router"`, global.GVA_CONFIG.AutoCode.Module),
										AppName:              "RouterGroupApp",
										GroupName:            utils.FirstUpper(entity.PackageName),
										ModuleName:           entity.PackageName + "Router",
										PackageName:          "router",
										FunctionName:         "Init" + info.StructName + "Router",
										LeftRouterGroupName:  "privateGroup",
										RightRouterGroupName: "publicGroup",
									}
									asts[packageInitializeRouter.Path+"=>"+packageInitializeRouter.Type.String()] = packageInitializeRouter
								}
								// 处理Service的enter.go注入
								// Service需要注入到两个地方：
								// 1. service/{package}/enter.go - 注册服务组
								// 2. service/{package}/enter.go - 注册具体服务模块
								// 好处：自动建立服务注册关系，保证依赖注入正确
								if isService != -1 {
									path := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext))
									importPath := fmt.Sprintf(`"%s/service/%s"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName)
									// 注入到service/{package}/enter.go，注册服务组
									packageServiceEnter := &ast.PackageEnter{
										Type:              ast.TypePackageServiceEnter,
										Path:              path,
										ImportPath:        importPath,
										StructName:        utils.FirstUpper(entity.PackageName) + "ServiceGroup",
										PackageName:       entity.PackageName,
										PackageStructName: "ServiceGroup",
									}
									asts[packageServiceEnter.Path+"=>"+packageServiceEnter.Type.String()] = packageServiceEnter
									// 注入到service/{package}/enter.go，注册具体服务模块
									packageServiceModuleEnter := &ast.PackageModuleEnter{
										Type:       ast.TypePackageServiceModuleEnter,
										Path:       filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), entity.PackageName, "enter.go"),
										StructName: info.StructName + "Service",
									}
									asts[packageServiceModuleEnter.Path+"=>"+packageServiceModuleEnter.Type.String()] = packageServiceModuleEnter
									creates[four] = packageServiceModuleEnter.Path
								}
								continue
							}
							code[four] = create
							continue
						}
						// plugin模板的enter.go处理逻辑
						// plugin的enter.go需要注入到插件自己的目录结构中
						// 好处：插件代码自包含，便于插件管理和分发
						if hasEnter != -1 {
							isApi := strings.Index(secondDirs[j].Name(), "api")
							isRouter := strings.Index(secondDirs[j].Name(), "router")
							isService := strings.Index(secondDirs[j].Name(), "service")
							// 处理plugin的router enter.go
							if isRouter != -1 {
								pluginRouterEnter := &ast.PluginEnter{
									Type:            ast.TypePluginRouterEnter,
									Path:            filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext)),
									ImportPath:      fmt.Sprintf(`"%s/plugin/%s/api"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName),
									StructName:      info.StructName,
									StructCamelName: info.Abbreviation,
									ModuleName:      "api" + info.StructName,
									GroupName:       "Api",
									PackageName:     "api",
									ServiceName:     info.StructName,
								}
								asts[pluginRouterEnter.Path+"=>"+pluginRouterEnter.Type.String()] = pluginRouterEnter
								creates[four] = pluginRouterEnter.Path
							}
							if isApi != -1 {
								pluginApiEnter := &ast.PluginEnter{
									Type:            ast.TypePluginApiEnter,
									Path:            filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext)),
									ImportPath:      fmt.Sprintf(`"%s/plugin/%s/service"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName),
									StructName:      info.StructName,
									StructCamelName: info.Abbreviation,
									ModuleName:      "service" + info.StructName,
									GroupName:       "Service",
									PackageName:     "service",
									ServiceName:     info.StructName,
								}
								asts[pluginApiEnter.Path+"=>"+pluginApiEnter.Type.String()] = pluginApiEnter
								creates[four] = pluginApiEnter.Path
							}
							if isService != -1 {
								pluginServiceEnter := &ast.PluginEnter{
									Type:            ast.TypePluginServiceEnter,
									Path:            filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext)),
									StructName:      info.StructName,
									StructCamelName: info.Abbreviation,
								}
								asts[pluginServiceEnter.Path+"=>"+pluginServiceEnter.Type.String()] = pluginServiceEnter
								creates[four] = pluginServiceEnter.Path
							}
							continue
						} // enter.go
						create := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), info.HumpPackageName+".go")
						code[four] = create
					}
				// 处理插件特有的目录：gen、config、initialize、plugin、response
				// 这些目录包含插件的初始化、配置等特殊文件
				// 好处：插件可以有自己的配置和初始化逻辑，实现插件化架构
				case "gen", "config", "initialize", "plugin", "response":
					// package模板不需要这些目录，只有plugin模板需要
					// 好处：区分package和plugin的不同需求，避免生成不必要的文件
					if entity.Template == "package" {
						continue
					} // package模板不需要生成gen, config, initialize
					var threeDirs []os.DirEntry
					threeDirs, err = os.ReadDir(three)
					if err != nil {
						return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", three)
					}
					for k := 0; k < len(threeDirs); k++ {
						if threeDirs[k].Name() == ".DS_Store" {
							continue
						}
						four := filepath.Join(three, threeDirs[k].Name())
						if threeDirs[k].IsDir() {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件夹!", four)
						}
						ext := filepath.Ext(four)
						if ext != ".tpl" {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版后缀!", four)
						}
						gen := strings.Index(threeDirs[k].Name(), "gen")
						api := strings.Index(threeDirs[k].Name(), "api")
						menu := strings.Index(threeDirs[k].Name(), "menu")
						viper := strings.Index(threeDirs[k].Name(), "viper")
						plugin := strings.Index(threeDirs[k].Name(), "plugin")
						config := strings.Index(threeDirs[k].Name(), "config")
						router := strings.Index(threeDirs[k].Name(), "router")
						hasGorm := strings.Index(threeDirs[k].Name(), "gorm")
						response := strings.Index(threeDirs[k].Name(), "response")
						if gen != -1 && api != -1 && menu != -1 && viper != -1 && plugin != -1 && config != -1 && router != -1 && hasGorm != -1 && response != -1 {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", four)
						}
						if api != -1 || menu != -1 || viper != -1 || response != -1 || plugin != -1 || config != -1 {
							creates[four] = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext))
						}
						if gen != -1 {
							pluginGen := &ast.PluginGen{
								Type:        ast.TypePluginGen,
								Path:        filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext)),
								ImportPath:  fmt.Sprintf(`"%s/plugin/%s/model"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName),
								StructName:  info.StructName,
								PackageName: "model",
								IsNew:       true,
							}
							asts[pluginGen.Path+"=>"+pluginGen.Type.String()] = pluginGen
							creates[four] = pluginGen.Path
						}
						if hasGorm != -1 {
							pluginInitializeGorm := &ast.PluginInitializeGorm{
								Type:        ast.TypePluginInitializeGorm,
								Path:        filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext)),
								ImportPath:  fmt.Sprintf(`"%s/plugin/%s/model"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName),
								StructName:  info.StructName,
								PackageName: "model",
								IsNew:       true,
							}
							asts[pluginInitializeGorm.Path+"=>"+pluginInitializeGorm.Type.String()] = pluginInitializeGorm
							creates[four] = pluginInitializeGorm.Path
						}
						if router != -1 {
							pluginInitializeRouter := &ast.PluginInitializeRouter{
								Type:                 ast.TypePluginInitializeRouter,
								Path:                 filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), strings.TrimSuffix(threeDirs[k].Name(), ext)),
								ImportPath:           fmt.Sprintf(`"%s/plugin/%s/router"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName),
								AppName:              "Router",
								GroupName:            info.StructName,
								PackageName:          "router",
								FunctionName:         "Init",
								LeftRouterGroupName:  "public",
								RightRouterGroupName: "private",
							}
							asts[pluginInitializeRouter.Path+"=>"+pluginInitializeRouter.Type.String()] = pluginInitializeRouter
							creates[four] = pluginInitializeRouter.Path
						}
					}
				// 处理model目录
				// model目录包含数据模型和请求/响应结构体的模板
				// 好处：统一数据模型生成，保证模型结构一致
				case "model":
					var threeDirs []os.DirEntry
					threeDirs, err = os.ReadDir(three)
					if err != nil {
						return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", three)
					}
					for k := 0; k < len(threeDirs); k++ {
						if threeDirs[k].Name() == ".DS_Store" {
							continue
						}
						four := filepath.Join(three, threeDirs[k].Name())
						// model目录下可能有子目录（如request），需要递归处理
						if threeDirs[k].IsDir() {
							var fourDirs []os.DirEntry
							fourDirs, err = os.ReadDir(four)
							if err != nil {
								return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", four)
							}
							for l := 0; l < len(fourDirs); l++ {
								if fourDirs[l].Name() == ".DS_Store" {
									continue
								}
								five := filepath.Join(four, fourDirs[l].Name())
								if fourDirs[l].IsDir() {
									return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件夹!", five)
								}
								ext := filepath.Ext(five)
								if ext != ".tpl" {
									return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版后缀!", five)
								}
								hasRequest := strings.Index(fourDirs[l].Name(), "request")
								if hasRequest == -1 {
									return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", five)
								}
								create := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), threeDirs[k].Name(), info.HumpPackageName+".go")
								if entity.Template == "package" {
									create = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), entity.PackageName, threeDirs[k].Name(), info.HumpPackageName+".go")
								}
								code[five] = create
							}
							continue
						}
						ext := filepath.Ext(threeDirs[k].Name())
						if ext != ".tpl" {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版后缀!", four)
						}
						hasModel := strings.Index(threeDirs[k].Name(), "model")
						if hasModel == -1 {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", four)
						}
						// 默认路径：plugin/{package}/model/{module}.go
						create := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", entity.PackageName, secondDirs[j].Name(), info.HumpPackageName+".go")
						// package模板的model需要特殊处理：注入到gorm初始化文件中
						// 好处：自动注册数据模型到GORM，无需手动修改初始化文件
						if entity.Template == "package" {
							// 注入到initialize/gorm_biz.go，自动注册数据模型
							packageInitializeGorm := &ast.PackageInitializeGorm{
								Type:        ast.TypePackageInitializeGorm,
								Path:        filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "initialize", "gorm_biz.go"),
								ImportPath:  fmt.Sprintf(`"%s/model/%s"`, global.GVA_CONFIG.AutoCode.Module, entity.PackageName),
								Business:    info.BusinessDB,
								StructName:  info.StructName,
								PackageName: entity.PackageName,
								IsNew:       true,
							}
							code[four] = packageInitializeGorm.Path
							asts[packageInitializeGorm.Path+"=>"+packageInitializeGorm.Type.String()] = packageInitializeGorm
							// package模板的model路径：model/{package}/{module}.go
							create = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, secondDirs[j].Name(), entity.PackageName, info.HumpPackageName+".go")
						}
						code[four] = create
					}
				default:
					return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件夹!", three)
				}
			}
		// 处理web目录（前端代码模板）
		// web目录包含Vue.js前端代码的模板
		// 好处：前后端代码一起生成，保证接口和前端代码的一致性
		case "web":
			// 如果用户选择不生成web代码且不是包创建，则跳过
			// 好处：支持只生成后端代码，提高灵活性
			if !info.GenerateWeb && !isPackage {
				break
			}
			var secondDirs []os.DirEntry
			secondDirs, err = os.ReadDir(second)
			if err != nil {
				return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", second)
			}
			for j := 0; j < len(secondDirs); j++ {
				if secondDirs[j].Name() == ".DS_Store" {
					continue
				}
				three := filepath.Join(second, secondDirs[j].Name())
				if !secondDirs[j].IsDir() {
					return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", three)
				}
				switch secondDirs[j].Name() {
				case "api", "form", "view", "table":
					var threeDirs []os.DirEntry
					threeDirs, err = os.ReadDir(three)
					if err != nil {
						return nil, nil, nil, errors.Wrapf(err, "读取模版文件夹[%s]失败!", three)
					}
					for k := 0; k < len(threeDirs); k++ {
						if threeDirs[k].Name() == ".DS_Store" {
							continue
						}
						four := filepath.Join(three, threeDirs[k].Name())
						if threeDirs[k].IsDir() {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件夹!", four)
						}
						ext := filepath.Ext(four)
						if ext != ".tpl" {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版后缀!", four)
						}
						api := strings.Index(threeDirs[k].Name(), "api")
						form := strings.Index(threeDirs[k].Name(), "form")
						view := strings.Index(threeDirs[k].Name(), "view")
						table := strings.Index(threeDirs[k].Name(), "table")
						if api == -1 && form == -1 && view == -1 && table == -1 {
							return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", four)
						}
						if entity.Template == "package" {
							if view != -1 || table != -1 {
								formPath := filepath.Join(three, "form.vue"+ext)
								value, ok := code[formPath]
								if ok {
									value = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.WebRoot(), secondDirs[j].Name(), entity.PackageName, info.PackageName, info.PackageName+"Form"+filepath.Ext(strings.TrimSuffix(threeDirs[k].Name(), ext)))
									code[formPath] = value
								}
							}
							create := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.WebRoot(), secondDirs[j].Name(), entity.PackageName, info.PackageName, info.PackageName+filepath.Ext(strings.TrimSuffix(threeDirs[k].Name(), ext)))
							if api != -1 {
								create = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.WebRoot(), secondDirs[j].Name(), entity.PackageName, info.PackageName+filepath.Ext(strings.TrimSuffix(threeDirs[k].Name(), ext)))
							}
							code[four] = create
							continue
						}
						create := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.WebRoot(), "plugin", entity.PackageName, secondDirs[j].Name(), info.PackageName+filepath.Ext(strings.TrimSuffix(threeDirs[k].Name(), ext)))
						code[four] = create
					}
				default:
					return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件夹!", three)
				}
			}
		case "readme.txt.tpl", "readme.txt.template":
			continue
		default:
			if templateDirs[i].Name() == ".DS_Store" {
				continue
			}
			return nil, nil, nil, errors.Errorf("[filpath:%s]非法模版文件!", second)
		}
	}
	return code, asts, creates, nil
}
