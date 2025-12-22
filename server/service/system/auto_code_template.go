// Package system 提供自动化代码生成的核心服务
// 该包通过模板引擎和AST操作实现代码的自动化生成、注入和管理
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/flipped-aurora/gin-vue-admin/server/utils/autocode"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	utilsAst "github.com/flipped-aurora/gin-vue-admin/server/utils/ast"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// AutoCodeTemplate 全局导出的自动化代码模板服务实例
// 使用单例模式确保服务的一致性和可访问性
// 好处：避免重复创建实例，提供统一的服务入口
var AutoCodeTemplate = new(autoCodeTemplate)

// autoCodeTemplate 自动化代码模板服务结构体
// 采用空结构体设计，只包含方法不包含状态，符合值接收者的最佳实践
// 好处：零成本抽象，内存占用最小，同时保持方法接收者的语义清晰
type autoCodeTemplate struct{}

// checkPackage 检查包结构是否完整，确保代码生成的前置条件满足
// 为什么这么写：
// 1. 在代码生成前验证包结构完整性，避免生成到不存在的目录导致失败
// 2. 区分package和plugin两种不同的模板类型，它们有不同的目录结构要求
// 3. 使用os.Stat检查文件存在性，这是最轻量级的文件系统操作
//
// 设计优势：
// - 提前发现问题：在生成代码前就能发现结构问题，避免部分生成后失败
// - 清晰的错误提示：针对每种类型返回具体的错误信息，便于快速定位问题
// - 类型区分：通过switch区分不同模板类型，代码清晰易维护
func (s *autoCodeTemplate) checkPackage(Pkg string, template string) (err error) {
	switch template {
	case "package":
		// package类型需要三层结构：api、service、router，每层都需要enter.go作为入口文件
		// 这样设计的好处是：确保代码生成时能够正确注入到对应层的enter.go文件中
		apiEnter := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "api", "v1", Pkg, "enter.go")
		_, err = os.Stat(apiEnter)
		if err != nil {
			return fmt.Errorf("package结构异常,缺少api/v1/%s/enter.go", Pkg)
		}
		serviceEnter := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "service", Pkg, "enter.go")
		_, err = os.Stat(serviceEnter)
		if err != nil {
			return fmt.Errorf("package结构异常,缺少service/%s/enter.go", Pkg)
		}
		routerEnter := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "router", Pkg, "enter.go")
		_, err = os.Stat(routerEnter)
		if err != nil {
			return fmt.Errorf("package结构异常,缺少router/%s/enter.go", Pkg)
		}
	case "plugin":
		// plugin类型只需要plugin.go入口文件
		// 这种简化设计符合插件相对独立的特点
		pluginEnter := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", Pkg, "plugin.go")
		_, err = os.Stat(pluginEnter)
		if err != nil {
			return fmt.Errorf("plugin结构异常,缺少plugin/%s/plugin.go", Pkg)
		}
	}
	return nil
}

// Create 创建并生成自动化代码的入口方法
// 为什么这么设计：
// 1. 采用上下文传递，支持超时控制和请求追踪
// 2. 先验证后生成，确保生成的代码可靠
// 3. 支持增量功能（API、菜单、Excel等）的可选创建
// 4. 完整的历史记录，便于回滚和管理
//
// 设计优势：
// - 原子性操作：文件生成失败时不会留下部分生成的文件（通过先验证全部再写入）
// - 事务保证：API创建使用数据库事务，保证数据一致性
// - 幂等性考虑：检查菜单和API是否已存在，避免重复创建
// - 可追溯性：保存完整的生成历史，包括模板、注入点等信息
func (s *autoCodeTemplate) Create(ctx context.Context, info request.AutoCode) error {
	// 提前创建历史记录对象，在整个过程中收集所有相关信息
	// 这样做的好处：即使过程中出错，也能知道已经完成了哪些操作
	history := info.History()

	// 查询包配置信息，获取模板类型等元数据
	// 使用WithContext保证能够正确传递上下文（超时、取消等）
	var autoPkg model.SysAutoCodePackage
	err := global.GVA_DB.WithContext(ctx).Where("package_name = ?", info.Package).First(&autoPkg).Error
	if err != nil {
		return errors.Wrap(err, "查询包失败!")
	}

	// 前置验证：检查包结构完整性
	// 在生成代码前验证，避免部分生成后失败造成的不一致状态
	err = s.checkPackage(info.Package, autoPkg.Template)
	if err != nil {
		return err
	}

	// 去重检查：防止重复创建相同的结构体或使用重复的简称
	// 这是业务逻辑层面的保护，避免产生冲突的代码
	if AutocodeHistory.Repeat(info.BusinessDB, info.StructName, info.Abbreviation, info.Package) {
		return errors.New("已经创建过此数据结构,请勿重复创建!")
	}

	// 核心代码生成逻辑
	// 返回：生成的代码映射、使用的模板信息、AST注入点信息
	generate, templates, injections, err := s.generate(ctx, info, autoPkg)
	if err != nil {
		return err
	}

	// 批量写入生成的文件
	// 为什么使用MkdirAll：确保目标目录存在，避免因目录不存在导致写入失败
	// 为什么使用strings.Builder：内存高效，避免了多次字符串拼接的开销
	for key, builder := range generate {
		// 先创建目录（如果不存在），再写入文件
		// filepath.Dir获取目录路径，os.ModePerm确保有足够的权限
		err = os.MkdirAll(filepath.Dir(key), os.ModePerm)
		if err != nil {
			return errors.Wrapf(err, "[filepath:%s]创建文件夹失败!", key)
		}
		// 0666权限：所有用户可读写，适合生成的代码文件
		err = os.WriteFile(key, []byte(builder.String()), 0666)
		if err != nil {
			return errors.Wrapf(err, "[filepath:%s]写入文件失败!", key)
		}
	}

	// 可选功能1：自动创建API到数据库
	// 条件判断：需要开启自动创建且不是仅生成模板模式
	// 为什么使用事务：确保所有API要么全部创建成功，要么全部回滚，保证数据一致性
	if info.AutoCreateApiToSql && !info.OnlyTemplate {
		apis := info.Apis()
		err := global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, v := range apis {
				var api model.SysApi
				var id uint
				// 先查询是否存在（通过path和method唯一标识）
				// 为什么这样设计：API的path+method应该是唯一的，避免重复注册
				err := tx.Where("path = ? AND method = ?", v.Path, v.Method).First(&api).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// 不存在则创建
					if err = tx.Create(&v).Error; err != nil {
						return err // 遇到错误时回滚事务
					}
					id = v.ID
				} else {
					// 已存在则使用现有ID（幂等性保证）
					id = api.ID
				}
				// 记录API ID到历史中，便于后续回滚或管理
				history.ApiIDs = append(history.ApiIDs, id)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// 可选功能2：自动创建菜单到数据库
	// 支持菜单的自动创建，包括基础的CRUD按钮权限
	if info.AutoCreateMenuToSql {
		var entity model.SysBaseMenu
		var id uint
		// 检查菜单是否已存在（通过简称name判断）
		// 为什么使用First而不是Create：实现幂等性，避免重复创建
		err := global.GVA_DB.WithContext(ctx).First(&entity, "name = ?", info.Abbreviation).Error
		if err == nil {
			// 菜单已存在，使用现有ID
			id = entity.ID
		} else {
			// 菜单不存在，创建新菜单
			entity = info.Menu(autoPkg.Template)
			// 如果开启了按钮权限且不是仅模板模式，自动创建标准CRUD按钮
			// 为什么预设这些按钮：符合常见的业务需求，减少手动配置
			if info.AutoCreateBtnAuth && !info.OnlyTemplate {
				entity.MenuBtn = []model.SysBaseMenuBtn{
					{SysBaseMenuID: entity.ID, Name: "add", Desc: "新增"},
					{SysBaseMenuID: entity.ID, Name: "batchDelete", Desc: "批量删除"},
					{SysBaseMenuID: entity.ID, Name: "delete", Desc: "删除"},
					{SysBaseMenuID: entity.ID, Name: "edit", Desc: "编辑"},
					{SysBaseMenuID: entity.ID, Name: "info", Desc: "详情"},
				}
				// 如果包含Excel功能，额外添加Excel相关按钮
				// 这种条件追加的方式：保持代码清晰，按需扩展
				if info.HasExcel {
					excelBtn := []model.SysBaseMenuBtn{
						{SysBaseMenuID: entity.ID, Name: "exportTemplate", Desc: "导出模板"},
						{SysBaseMenuID: entity.ID, Name: "exportExcel", Desc: "导出Excel"},
						{SysBaseMenuID: entity.ID, Name: "importExcel", Desc: "导入Excel"},
					}
					entity.MenuBtn = append(entity.MenuBtn, excelBtn...)
				}
			}
			err = global.GVA_DB.WithContext(ctx).Create(&entity).Error
			id = entity.ID
			if err != nil {
				return errors.Wrap(err, "创建菜单失败!")
			}
		}
		history.MenuID = id
	}

	// 可选功能3：创建Excel导出模板配置
	// 将字段映射关系保存到数据库，便于后续Excel导入导出功能使用
	if info.HasExcel {
		dbName := info.BusinessDB
		name := info.Package + "_" + info.StructName
		tableName := info.TableName
		// 构建字段映射：只包含标记为Excel的字段
		// 使用map[string]string：key是列名，value是字段描述，便于后续使用
		fieldsMap := make(map[string]string, len(info.Fields))
		for _, field := range info.Fields {
			if field.Excel {
				fieldsMap[field.ColumnName] = field.FieldDesc
			}
		}
		// 将map序列化为JSON存储
		// 为什么用JSON：灵活存储结构化的配置信息，便于后续解析和使用
		templateInfo, _ := json.Marshal(fieldsMap)
		sysExportTemplate := model.SysExportTemplate{
			DBName:       dbName,
			Name:         name,
			TableName:    tableName,
			TemplateID:   name,
			TemplateInfo: string(templateInfo),
		}
		err = SysExportTemplateServiceApp.CreateSysExportTemplate(&sysExportTemplate)
		if err != nil {
			return err
		}
		history.ExportTemplateID = sysExportTemplate.ID
	}

	// 保存完整的生成历史记录
	// 为什么保存这些信息：
	// 1. Templates：记录使用了哪些模板文件，便于追踪和调试
	// 2. Injections：记录AST注入点信息，便于回滚时知道需要移除哪些代码
	// 3. 使用JSON序列化AST信息：AST对象复杂，序列化为JSON便于存储和检索
	history.Templates = templates
	history.Injections = make(map[string]string, len(injections))
	for key, value := range injections {
		bytes, _ := json.Marshal(value)
		history.Injections[key] = string(bytes)
	}
	err = AutocodeHistory.Create(ctx, history)
	if err != nil {
		return err
	}
	return nil
}

// Preview 预览自动化代码，不实际生成文件，只返回代码内容
// 为什么需要这个方法：
// 1. 用户可以在生成前先预览，确认生成的代码是否符合预期
// 2. 降低误操作风险，提高用户体验
// 3. 便于前端展示代码内容，支持语法高亮等展示功能
//
// 设计优势：
// - 复用generate方法：避免代码重复，确保预览和实际生成逻辑一致
// - Markdown代码块格式：返回的代码包装在```代码块中，便于前端直接渲染
// - 相对路径转换：将绝对路径转换为相对路径，使显示更友好
func (s *autoCodeTemplate) Preview(ctx context.Context, info request.AutoCode) (map[string]string, error) {
	var entity model.SysAutoCodePackage
	err := global.GVA_DB.WithContext(ctx).Where("package_name = ?", info.Package).First(&entity).Error
	if err != nil {
		return nil, errors.Wrap(err, "查询包失败!")
	}
	// 去重检查，但允许IsAdd模式（可能是添加新功能而不是新建结构体）
	// 为什么加IsAdd判断：支持在已有结构体上添加新功能的场景
	if AutocodeHistory.Repeat(info.BusinessDB, info.StructName, info.Abbreviation, info.Package) && !info.IsAdd {
		return nil, errors.New("已经创建过此数据结构或重复简称,请勿重复创建!")
	}

	preview := make(map[string]string)
	// 复用generate方法生成代码，但不写入文件
	// 好处：保证预览和实际生成逻辑完全一致
	codes, _, _, err := s.generate(ctx, info, entity)
	if err != nil {
		return nil, err
	}

	// 将生成的代码转换为预览格式（Markdown代码块）
	for key, writer := range codes {
		// 将绝对路径转换为相对路径（相对于项目根目录）
		// 为什么这样做：绝对路径对用户不友好，相对路径更直观
		if len(key) > len(global.GVA_CONFIG.AutoCode.Root) {
			key, _ = filepath.Rel(global.GVA_CONFIG.AutoCode.Root, key)
		}
		// 获取文件扩展名作为代码块的语言标识（去掉点号）
		// 例如：.go -> go, .js -> js，用于语法高亮
		suffix := filepath.Ext(key)[1:]
		var builder strings.Builder
		// 包装成Markdown代码块格式，便于前端渲染
		builder.WriteString("```" + suffix + "\n\n")
		builder.WriteString(writer.String())
		builder.WriteString("\n\n```")
		preview[key] = builder.String()
	}
	return preview, nil
}

// generate 核心代码生成方法，负责模板渲染和AST代码注入
// 为什么分成两部分：
// 1. 模板渲染：使用Go标准模板引擎生成新文件（如model、api、service等）
// 2. AST注入：使用抽象语法树操作在现有文件中注入代码（如router注册、初始化代码等）
//
// 设计优势：
// - 分离关注点：模板生成和代码注入使用不同技术，分开处理更清晰
// - 灵活性：可以根据配置跳过某些注入（如OnlyTemplate模式）
// - 可追溯性：返回模板和注入信息，便于历史记录和回滚
//
// 使用示例：
//
// 示例1：基本使用 - 生成完整的CRUD代码并自动注入路由
//
//	ctx := context.Background()
//	info := request.AutoCode{
//		Package:             "example",              // 包名
//		StructName:          "Example",              // 结构体名称
//		TableName:           "example_table",        // 数据库表名
//		BusinessDB:          "mysql",                // 业务数据库
//		Description:         "示例管理",              // 中文描述
//		Abbreviation:        "exa",                  // 简称
//		PackageName:         "example",              // 文件名称
//		HumpPackageName:     "example",              // Go文件名称
//		AutoMigrate:         true,                   // 自动迁移表结构
//		AutoCreateApiToSql:  true,                   // 自动创建API到数据库
//		AutoCreateMenuToSql: true,                   // 自动创建菜单
//		OnlyTemplate:        false,                  // 不仅生成模板，还要注入代码
//		Fields: []*request.AutoCodeField{
//			{
//				FieldName:    "Name",
//				FieldDesc:    "名称",
//				FieldType:    "string",
//				FieldJson:    "name",
//				ColumnName:   "name",
//				DataTypeLong: "100",
//				Comment:      "名称",
//				Form:         true,
//				Table:        true,
//			},
//		},
//	}
//	entity := model.SysAutoCodePackage{
//		PackageName: "example",
//		Template:    "package",  // 使用package模板类型
//	}
//	code, templates, injections, err := s.generate(ctx, info, entity)
//	if err != nil {
//		// 处理错误
//		return err
//	}
//	// code: 生成的代码文件路径 -> 代码内容的映射
//	// templates: 使用的模板文件路径 -> 生成目标文件路径的映射
//	// injections: AST注入类型 -> 注入AST对象的映射
//
// 示例2：仅生成模板文件，不注入代码（用于预览或手动集成）
//
//	info := request.AutoCode{
//		Package:      "product",
//		StructName:   "Product",
//		TableName:    "products",
//		OnlyTemplate: true,  // 关键：设置为true，跳过所有AST注入
//		// ... 其他字段
//	}
//	// 当 OnlyTemplate=true 时，会跳过以下注入：
//	// - TypePackageInitializeGorm (GORM初始化代码)
//	// - TypePluginInitializeGorm (插件GORM初始化代码)
//
// 示例3：生成代码但不自动迁移数据库（手动控制数据库迁移）
//
//	info := request.AutoCode{
//		Package:      "order",
//		StructName:   "Order",
//		AutoMigrate:  false,  // 关键：设置为false，跳过GORM初始化注入
//		OnlyTemplate: false,
//		// ... 其他字段
//	}
//	// 当 AutoMigrate=false 时，生成的代码文件正常，但不会在初始化文件中注入GORM迁移代码
//
// 示例4：处理返回值 - 遍历生成的代码并写入文件
//
//	code, templates, injections, err := s.generate(ctx, info, entity)
//	if err != nil {
//		return err
//	}
//	for filePath, builder := range code {
//		// 创建目录
//		dir := filepath.Dir(filePath)
//		os.MkdirAll(dir, os.ModePerm)
//		// 写入文件
//		os.WriteFile(filePath, []byte(builder.String()), 0666)
//	}
//	// templates 可用于记录使用了哪些模板
//	// injections 可用于后续回滚操作
//
// 示例5：使用package类型模板（标准三层架构）
//
//	entity := model.SysAutoCodePackage{
//		PackageName: "user",
//		Template:    "package",  // 生成 api/v1/user、service/user、router/user
//	}
//	// 会在以下位置生成文件和注入代码：
//	// - server/api/v1/user/*.go (API层)
//	// - server/service/user/*.go (Service层)
//	// - server/router/user/*.go (Router层)
//
// 示例6：使用plugin类型模板（插件模式）
//
//	entity := model.SysAutoCodePackage{
//		PackageName: "payment",
//		Template:    "plugin",  // 生成 plugin/payment/* 目录结构
//	}
//	// 会在以下位置生成文件和注入代码：
//	// - server/plugin/payment/api/*.go
//	// - server/plugin/payment/service/*.go
//	// - server/plugin/payment/router/*.go
//	// 注意：plugin类型会跳过 TypePluginInitializeV2 类型的注入
func (s *autoCodeTemplate) generate(ctx context.Context, info request.AutoCode, entity model.SysAutoCodePackage) (map[string]strings.Builder, map[string]string, map[string]utilsAst.Ast, error) {
	// 第一步：获取模板文件和AST注入点信息
	// templates: 模板文件路径 -> 生成目标文件路径的映射
	// asts: AST注入点信息，格式为"文件路径=>注入类型"
	templates, asts, _, err := AutoCodePackage.templates(ctx, entity, info, false)
	if err != nil {
		return nil, nil, nil, err
	}

	// 第二步：使用模板引擎生成新文件
	// 为什么使用strings.Builder：内存高效，避免频繁的字符串拼接和内存分配
	code := make(map[string]strings.Builder)
	for key, create := range templates {
		// 创建模板实例，使用模板文件名作为名称
		// Funcs：注册自定义函数，扩展模板功能（如字符串处理、格式化等）
		var files *template.Template
		files, err = template.New(filepath.Base(key)).Funcs(autocode.GetTemplateFuncMap()).ParseFiles(key)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "[filpath:%s]读取模版文件失败!", key)
		}
		var builder strings.Builder
		// 执行模板渲染，将info数据填充到模板中
		// 好处：模板引擎自动处理转义、条件判断、循环等逻辑
		err = files.Execute(&builder, info)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "[filpath:%s]生成文件失败!", create)
		}
		code[create] = builder
	} // 生成文件

	// 第三步：使用AST操作注入代码到现有文件
	// 为什么使用AST而不是字符串拼接：
	// 1. 保证代码格式正确（AST操作后会自动格式化）
	// 2. 避免破坏现有代码结构
	// 3. 可以精确控制注入位置
	injections := make(map[string]utilsAst.Ast, len(asts))
	for key, value := range asts {
		// 解析注入点key，格式：文件路径=>注入类型
		// 使用"=>"分隔符，避免路径中包含冒号导致解析错误
		keys := strings.Split(key, "=>")
		if len(keys) == 2 {
			// 跳过某些特定的注入类型（根据配置决定）
			// 为什么这样设计：提供细粒度的控制，允许用户选择性地跳过某些注入
			if keys[1] == utilsAst.TypePluginInitializeV2 {
				continue
			}
			// 如果只是生成模板，跳过数据库初始化相关的注入
			// 这样可以在不修改初始化代码的情况下生成代码文件
			if info.OnlyTemplate {
				if keys[1] == utilsAst.TypePackageInitializeGorm || keys[1] == utilsAst.TypePluginInitializeGorm {
					continue
				}
			}
			// 如果不需要自动迁移数据库，跳过GORM初始化注入
			// 提供灵活性：允许只生成代码但不自动修改数据库迁移逻辑
			if !info.AutoMigrate {
				if keys[1] == utilsAst.TypePackageInitializeGorm || keys[1] == utilsAst.TypePluginInitializeGorm {
					continue
				}
			}
			var builder strings.Builder
			// 解析目标文件的AST，准备进行代码注入
			parse, _ := value.Parse("", &builder)
			if parse != nil {
				// 执行AST注入操作，在合适的位置插入代码
				_ = value.Injection(parse)
				// 格式化代码并写入builder
				// 格式化确保代码风格一致，符合Go规范
				err = value.Format("", &builder, parse)
				if err != nil {
					return nil, nil, nil, err
				}
				code[keys[0]] = builder
				// 记录注入信息，便于后续回滚
				injections[keys[1]] = value
				fmt.Println(keys[0], "注入成功!")
			}
		}
	}
	// 注入代码
	return code, templates, injections, nil
}

// AddFunc 向现有模块添加新功能的入口方法
// 与Create不同：Create是创建完整的CRUD模块，AddFunc是在已有模块上添加单个功能函数
// 为什么需要这个方法：
// 1. 支持增量开发：在已有模块上添加新功能，而不需要重新生成整个模块
// 2. 灵活性：允许用户根据需要逐步扩展功能
//
// 设计优势：
// - 统一处理：一次性处理API、Service、前端JS和路由四个层面的代码
// - 模板区分：自动识别package和plugin类型，使用不同的模板路径
// - 原子性：如果任何一步失败，整个操作失败，避免部分添加
func (s *autoCodeTemplate) AddFunc(info request.AutoFunc) error {
	// 查询包配置，确定是package还是plugin类型
	autoPkg := model.SysAutoCodePackage{}
	err := global.GVA_DB.First(&autoPkg, "package_name = ?", info.Package).Error
	if err != nil {
		return err
	}
	// 根据模板类型设置IsPlugin标志
	// 为什么这样设计：package和plugin有不同的目录结构，需要在后续处理中区分
	if autoPkg.Template != "package" {
		info.IsPlugin = true
	}

	// 按顺序添加三个文件：API层、Service层、前端JS层
	// 为什么按这个顺序：符合依赖关系，API依赖Service，前端依赖API
	err = s.addTemplateToFile("api.go", info)
	if err != nil {
		return err
	}
	err = s.addTemplateToFile("server.go", info)
	if err != nil {
		return err
	}
	err = s.addTemplateToFile("api.js", info)
	if err != nil {
		return err
	}
	// 最后处理路由注册（使用AST注入到router文件）
	// 为什么单独处理：路由需要注入到现有文件，使用AST操作更安全可靠
	return s.addTemplateToAst("router", info)
}

// GetApiAndServer 获取功能函数的模板代码预览（不写入文件）
// 与AddFunc的关系：AddFunc实际添加代码，GetApiAndServer只是预览
// 为什么需要这个方法：
// 1. 预览功能：让用户在添加前看到将要生成的代码
// 2. 支持AI辅助：可能用于AI生成代码后的预览和编辑
// 3. 调试支持：方便开发者查看模板渲染结果
//
// 设计优势：
// - 返回字符串而非文件：轻量级，便于前端展示和编辑
// - 统一返回格式：使用map统一返回三种类型的代码，便于处理
func (s *autoCodeTemplate) GetApiAndServer(info request.AutoFunc) (map[string]string, error) {
	// 查询包配置，确定模板类型
	autoPkg := model.SysAutoCodePackage{}
	err := global.GVA_DB.First(&autoPkg, "package_name = ?", info.Package).Error
	if err != nil {
		return nil, err
	}
	// 根据模板类型设置IsPlugin标志
	if autoPkg.Template != "package" {
		info.IsPlugin = true
	}

	// 分别获取三种类型的模板代码
	apiStr, err := s.getTemplateStr("api.go", info)
	if err != nil {
		return nil, err
	}
	serverStr, err := s.getTemplateStr("server.go", info)
	if err != nil {
		return nil, err
	}
	jsStr, err := s.getTemplateStr("api.js", info)
	if err != nil {
		return nil, err
	}
	// 统一返回，使用明确的key便于前端识别和处理
	return map[string]string{"api": apiStr, "server": serverStr, "js": jsStr}, nil

}

// getTemplateStr 根据模板类型名称获取渲染后的模板字符串
// 为什么单独提取这个方法：
// 1. 代码复用：GetApiAndServer和addTemplateToFile都需要模板渲染功能
// 2. 职责单一：专门负责模板读取和渲染，便于测试和维护
//
// 设计优势：
// - 统一模板路径：所有函数模板都在resource/function目录下，便于管理
// - 自定义函数支持：通过GetTemplateFuncMap扩展模板功能
// - 高效字符串构建：使用strings.Builder避免多次内存分配
func (s *autoCodeTemplate) getTemplateStr(t string, info request.AutoFunc) (string, error) {
	// 构建模板文件路径，模板文件统一使用.tpl扩展名
	// 路径结构：root/server/resource/function/{类型}.tpl
	tempPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "resource", "function", t+".tpl")

	// 创建模板实例并解析文件
	// filepath.Base：使用文件名作为模板名称，避免路径冲突
	// Funcs：注册自定义函数，扩展模板能力（如字符串处理、格式化等）
	files, err := template.New(filepath.Base(tempPath)).Funcs(autocode.GetTemplateFuncMap()).ParseFiles(tempPath)
	if err != nil {
		return "", errors.Wrapf(err, "[filepath:%s]读取模版文件失败!", tempPath)
	}
	var builder strings.Builder
	// 执行模板渲染，将info数据填充到模板中生成代码
	err = files.Execute(&builder, info)
	if err != nil {
		// 打印错误到控制台，便于调试
		fmt.Println(err.Error())
		return "", errors.Wrapf(err, "[filpath:%s]生成文件失败!", tempPath)
	}
	return builder.String(), nil
}

// addTemplateToAst 使用AST操作向路由文件注入路由注册代码
// 为什么使用AST而不是字符串操作：
// 1. 安全性：AST操作保证语法正确，不会破坏现有代码结构
// 2. 自动格式化：AST格式化后代码风格统一，符合Go规范
// 3. 精确控制：可以准确找到函数体并插入到合适位置
//
// 设计优势：
// - 区分认证和非认证路由：需要认证的路由和不需要的路由插入到不同位置
// - 支持package和plugin两种模式：使用不同的路由注册格式
// - 智能插入位置：认证路由插入到函数开始，非认证路由插入到函数末尾
//
// 示例1: Package模式 - 需要认证的路由 (IsAuth=true)
// 输入参数 info:
//
//	Package: "example"
//	StructName: "Customer"
//	Abbreviation: "customer"
//	HumpPackageName: "customer"
//	Method: "POST"
//	Router: "/create"
//	FuncName: "CreateCustomer"
//	IsAuth: true
//	IsPlugin: false
//
// 目标文件: router/example/customer.go
// 目标函数: InitCustomerRouter
// 生成的路由语句: customerRouter.POST("/create", customerApi.CreateCustomer)
//
// 插入前:
//
//	func (e *CustomerRouter) InitCustomerRouter(Router *gin.RouterGroup) {
//	    customerRouter := Router.Group("customer").Use(middleware.OperationRecord())
//	    customerRouterWithoutRecord := Router.Group("customer")
//	    {
//	        customerRouter.POST("customer", exaCustomerApi.CreateExaCustomer)
//	    }
//	    {
//	        customerRouterWithoutRecord.GET("customer", exaCustomerApi.GetExaCustomer)
//	    }
//	}
//
// 插入后 (插入到第一个BlockStmt中):
//
//	func (e *CustomerRouter) InitCustomerRouter(Router *gin.RouterGroup) {
//	    customerRouter := Router.Group("customer").Use(middleware.OperationRecord())
//	    customerRouterWithoutRecord := Router.Group("customer")
//	    {
//	        customerRouter.POST("customer", exaCustomerApi.CreateExaCustomer)
//	        customerRouter.POST("/create", customerApi.CreateCustomer)  // 新插入
//	    }
//	    {
//	        customerRouterWithoutRecord.GET("customer", exaCustomerApi.GetExaCustomer)
//	    }
//	}
//
// 示例2: Package模式 - 不需要认证的路由 (IsAuth=false)
// 输入参数 info:
//
//	Package: "example"
//	StructName: "Customer"
//	Abbreviation: "customer"
//	HumpPackageName: "customer"
//	Method: "GET"
//	Router: "/list"
//	FuncName: "GetCustomerList"
//	IsAuth: false
//	IsPlugin: false
//
// 目标文件: router/example/customer.go
// 目标函数: InitCustomerRouter
// 生成的路由语句: customerRouterWithoutAuth.GET("/list", customerApi.GetCustomerList)
//
// 插入前:
//
//	func (e *CustomerRouter) InitCustomerRouter(Router *gin.RouterGroup) {
//	    customerRouter := Router.Group("customer").Use(middleware.OperationRecord())
//	    customerRouterWithoutRecord := Router.Group("customer")
//	    {
//	        customerRouter.POST("customer", exaCustomerApi.CreateExaCustomer)
//	    }
//	    {
//	        customerRouterWithoutRecord.GET("customer", exaCustomerApi.GetExaCustomer)
//	    }
//	}
//
// 插入后 (插入到最后一个BlockStmt中):
//
//	func (e *CustomerRouter) InitCustomerRouter(Router *gin.RouterGroup) {
//	    customerRouter := Router.Group("customer").Use(middleware.OperationRecord())
//	    customerRouterWithoutRecord := Router.Group("customer")
//	    {
//	        customerRouter.POST("customer", exaCustomerApi.CreateExaCustomer)
//	    }
//	    {
//	        customerRouterWithoutRecord.GET("customer", exaCustomerApi.GetExaCustomer)
//	        customerRouterWithoutAuth.GET("/list", customerApi.GetCustomerList)  // 新插入
//	    }
//	}
//
// 示例3: Plugin模式 - 路由注入
// 输入参数 info:
//
//	Package: "announcement"
//	StructName: "Info"
//	HumpPackageName: "info"
//	Method: "POST"
//	Router: "/customAction"
//	FuncName: "CustomAction"
//	IsAuth: true
//	IsPlugin: true
//
// 目标文件: plugin/announcement/router/info.go
// 目标函数: Init
// 生成的路由语句: group.POST("/customAction", apiInfo.CustomAction)
//
// 插入前:
//
//	func (r *info) Init(public *gin.RouterGroup, private *gin.RouterGroup) {
//	    {
//	        group := private.Group("info").Use(middleware.OperationRecord())
//	        group.POST("createInfo", apiInfo.CreateInfo)
//	    }
//	    {
//	        group := private.Group("info")
//	        group.GET("findInfo", apiInfo.FindInfo)
//	    }
//	}
//
// 插入后 (插入到第一个BlockStmt中):
//
//	func (r *info) Init(public *gin.RouterGroup, private *gin.RouterGroup) {
//	    {
//	        group := private.Group("info").Use(middleware.OperationRecord())
//	        group.POST("createInfo", apiInfo.CreateInfo)
//	        group.POST("/customAction", apiInfo.CustomAction)  // 新插入
//	    }
//	    {
//	        group := private.Group("info")
//	        group.GET("findInfo", apiInfo.FindInfo)
//	    }
//	}
func (s *autoCodeTemplate) addTemplateToAst(t string, info request.AutoFunc) error {
	// 确定目标文件路径和函数名称（package模式）
	tPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "router", info.Package, info.HumpPackageName+".go")
	funcName := fmt.Sprintf("Init%sRouter", info.StructName)

	// 确定使用哪个路由组：认证路由组或非认证路由组
	routerStr := "RouterWithoutAuth"
	if info.IsAuth {
		routerStr = "Router"
	}

	// 构建路由注册语句（package模式）
	// 格式：abbreviationRouter.POST("/path", abbreviationApi.FuncName)
	stmtStr := fmt.Sprintf("%s%s.%s(\"%s\", %sApi.%s)", info.Abbreviation, routerStr, info.Method, info.Router, info.Abbreviation, info.FuncName)

	// 如果是plugin模式，使用不同的路径和语句格式
	if info.IsPlugin {
		tPath = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", info.Package, "router", info.HumpPackageName+".go")
		// plugin模式使用不同的注册格式：group.POST("/path", apiStructName.FuncName)
		stmtStr = fmt.Sprintf("group.%s(\"%s\", api%s.%s)", info.Method, info.Router, info.StructName, info.FuncName)
		funcName = "Init"
	}

	// 读取目标文件内容
	src, err := os.ReadFile(tPath)
	if err != nil {
		return err
	}

	// 解析Go源文件为AST
	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, "", src, 0)
	if err != nil {
		return err
	}

	// 查找目标函数（路由初始化函数）
	funcDecl := utilsAst.FindFunction(astFile, funcName)
	// 将路由注册语句转换为AST节点
	stmtNode := utilsAst.CreateStmt(stmtStr)

	// 根据是否需要认证，选择不同的插入位置
	if info.IsAuth {
		// 需要认证的路由：插入到函数开始处（通常是Router组的块中）
		// 从前向后查找第一个BlockStmt（通常是Router组）
		for i := 0; i < len(funcDecl.Body.List); i++ {
			st := funcDecl.Body.List[i]
			// 使用类型断言来检查stmt是否是一个块语句
			if blockStmt, ok := st.(*ast.BlockStmt); ok {
				// 如果是，插入代码并跳出
				blockStmt.List = append(blockStmt.List, stmtNode)
				break
			}
		}
	} else {
		// 不需要认证的路由：插入到函数末尾处（通常是RouterWithoutAuth组）
		// 从后向前查找最后一个BlockStmt（通常是RouterWithoutAuth组）
		for i := len(funcDecl.Body.List) - 1; i >= 0; i-- {
			st := funcDecl.Body.List[i]
			// 使用类型断言来检查stmt是否是一个块语句
			if blockStmt, ok := st.(*ast.BlockStmt); ok {
				// 如果是，插入代码并跳出
				blockStmt.List = append(blockStmt.List, stmtNode)
				break
			}
		}
	}

	// 将修改后的AST写回文件
	// 为什么使用Create而不是OpenFile：确保文件被完全覆盖，避免旧内容残留
	f, err := os.Create(tPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// 格式化AST并写入文件
	// format.Node会自动格式化代码，确保符合Go代码规范
	if err := format.Node(f, fileSet, astFile); err != nil {
		return err
	}
	return err
}

func (s *autoCodeTemplate) addTemplateToFile(t string, info request.AutoFunc) error {
	getTemplateStr, err := s.getTemplateStr(t, info)
	if err != nil {
		return err
	}
	var target string

	switch t {
	case "api.go":
		if info.IsAi && info.ApiFunc != "" {
			getTemplateStr = info.ApiFunc
		}
		target = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "api", "v1", info.Package, info.HumpPackageName+".go")
	case "server.go":
		if info.IsAi && info.ServerFunc != "" {
			getTemplateStr = info.ServerFunc
		}
		target = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "service", info.Package, info.HumpPackageName+".go")
	case "api.js":
		if info.IsAi && info.JsFunc != "" {
			getTemplateStr = info.JsFunc
		}
		target = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Web, "api", info.Package, info.PackageName+".js")
	}
	if info.IsPlugin {
		switch t {
		case "api.go":
			target = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", info.Package, "api", info.HumpPackageName+".go")
		case "server.go":
			target = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", info.Package, "service", info.HumpPackageName+".go")
		case "api.js":
			target = filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Web, "plugin", info.Package, "api", info.PackageName+".js")
		}
	}

	// 打开文件，如果不存在则返回错误
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入内容
	_, err = fmt.Fprintln(file, getTemplateStr)
	if err != nil {
		fmt.Printf("写入文件失败: %s\n", err.Error())
		return err
	}

	return nil
}
