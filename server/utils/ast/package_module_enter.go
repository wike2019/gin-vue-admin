package ast

import (
	"go/ast"
	"go/token"
	"io"
)

// PackageModuleEnter 模块化入口，用于在包级别的入口文件中自动注入模块相关的代码
//
// 设计目的：
// 1. 实现模块化架构：通过 AST 操作自动管理模块的注册和引用，避免手动维护代码
// 2. 代码生成自动化：在代码生成过程中自动注入必要的导入、结构体字段和变量声明
// 3. 支持回滚操作：可以安全地移除注入的代码，保证代码的可逆性
//
// 模块命名规则：
// ModuleName := PackageName.AppName.GroupName.ServiceName
// 例如：serviceUser = service.Service.User
// 这种命名方式保证了模块名称的唯一性和可读性，便于在代码中快速定位
//
// 设计模式：
// - 通过嵌入 Base 结构体实现代码复用（组合优于继承）
// - 采用模板方法模式，只需实现特定的 Injection 和 Rollback 逻辑
// - 支持相对路径和绝对路径的自动转换，提高代码的可移植性
//
// 使用场景：
// - 代码生成工具：自动生成模块入口代码
// - 插件系统：动态注册和卸载模块
// - 重构工具：批量修改模块引用关系
type PackageModuleEnter struct {
	Base                // 嵌入基础结构体，复用 Parse、Format 等通用方法，避免代码重复
	Type         Type   // 类型，用于区分不同的注入场景（如 TypePackageApiModuleEnter、TypePackageServiceModuleEnter）
	Path         string // 文件路径（绝对路径），目标文件的完整路径
	ImportPath   string // 导包路径，需要注入的 import 语句路径
	RelativePath string // 相对路径，提供路径的另一种表示方式，便于跨平台和可移植性
	StructName   string // 结构体名称，将作为结构体字段的类型名（例如："User"）
	AppName      string // 应用名称，用于构建选择器表达式（例如："Service"）
	GroupName    string // 分组名称，用于构建选择器表达式（例如："Group"）
	ModuleName   string // 模块名称，用于创建全局变量名（例如："serviceUser"）
	PackageName  string // 包名，用于构建选择器表达式的根包（例如："service"）
	ServiceName  string // 服务名称，用于构建选择器表达式的最终选择器（例如："User"）
}

// Parse 解析 Go 源文件并返回 AST 文件节点
//
// 路径处理逻辑说明（为什么需要这样处理）：
// 1. 支持灵活的路径输入方式：
//   - 如果 filename 为空，需要根据已有信息推断文件路径
//   - 优先使用 RelativePath（相对路径），因为相对路径具有更好的可移植性
//     可以在不同环境（开发、测试、生产）中正常工作，不受工作目录影响
//
// 2. 路径映射的建立和维护：
//   - 如果 RelativePath 为空，则使用 Path（绝对路径）并计算相对路径
//     这样可以在首次调用时建立路径映射关系，后续可以使用相对路径
//   - 如果已有 RelativePath，则反向计算绝对路径，确保路径一致性
//     这样可以在不同环境下都能正确找到文件
//
// 3. 为什么需要维护两种路径？
//   - 绝对路径：用于实际的文件操作，确保能准确找到文件
//   - 相对路径：用于配置和序列化，便于跨平台和版本控制
//
// 好处：
// - 提高 API 的易用性：调用方可以使用绝对路径或相对路径，方法会自动处理
// - 自动维护路径映射：避免路径不一致导致的错误
// - 通过委托给 Base.Parse 实现代码复用，遵循 DRY 原则
// - 支持跨平台：相对路径在不同操作系统上都能正常工作
//
// 使用示例：
//
//	示例1：首次调用，使用绝对路径
//	  parser := &PackageModuleEnter{
//	      Path: "/path/to/file/enter.go",
//	  }
//	  file, err := parser.Parse("", nil)  // filename 为空，自动使用 Path
//
//	示例2：后续调用，使用相对路径
//	  parser := &PackageModuleEnter{
//	      RelativePath: "server/api/v1/enter.go",
//	  }
//	  file, err := parser.Parse("", nil)  // 自动转换为绝对路径
func (a *PackageModuleEnter) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	if filename == "" {
		if a.RelativePath == "" {
			// 如果没有相对路径，使用绝对路径并计算相对路径
			// 这样可以在首次调用时建立路径映射关系
			filename = a.Path
			a.RelativePath = a.Base.RelativePath(a.Path)
			return a.Base.Parse(filename, writer)
		}
		// 如果已有相对路径，转换为绝对路径
		// 这样可以在不同环境下都能正确找到文件
		a.Path = a.Base.AbsolutePath(a.RelativePath)
		filename = a.Path
	}
	// 委托给 Base.Parse 实现实际的解析逻辑
	// 这样避免了重复实现解析代码，遵循 DRY 原则
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作，用于撤销 Injection 所做的修改
//
// 设计思路：
// 1. 需要回滚两种类型的注入：
//   - 结构体字段：从 Group（如 ApiGroup、ServiceGroup）结构体中移除字段
//   - 变量声明：移除 var 声明中的模块变量（如 var serviceUser = ...）
//
// 2. 为什么需要这样复杂的遍历？
//   - AST 是树形结构，需要逐层遍历才能找到目标节点
//   - 使用类型断言（type assertion）来识别不同类型的 AST 节点
//   - 需要同时处理结构体字段和变量声明两种不同的注入类型
//
// 3. 回滚顺序的重要性：
//   - 先回滚结构体字段，再回滚变量声明
//   - 如果变量声明所在的 var 块变空，需要删除整个 var 声明
//   - 删除空的 var 声明后，还需要回滚对应的 import 语句
//
// 4. 为什么需要删除空的 var() 声明？
//   - 空的 var() 声明会影响后续的注入逻辑
//   - 因为空的 var 块中没有 *ast.ValueSpec，会导致 Injection 方法识别不到变量
//   - 删除空的 var 声明可以保持代码的整洁性
//
// 5. TypePackageServiceModuleEnter 的特殊处理：
//   - 对于服务模块入口类型，不需要回滚变量声明
//   - 这是因为服务模块可能有不同的注入策略
//
// 好处：
// - 保证代码的可逆性：可以安全地撤销注入操作
// - 保持代码整洁：删除空的声明块，避免产生无效代码
// - 支持多次回滚：可以安全地多次调用，不会产生副作用
//
// 注意事项：
// - 使用切片操作删除元素时需要注意索引的变化
// - 删除 var 声明时需要同时回滚对应的 import
//
// 使用示例：
//
//	示例1：回滚 API 模块的结构体字段和变量声明
//	  // 假设目标文件 server/api/v1/enter.go 内容如下：
//	  // package v1
//	  // import "gin-vue-admin/server/service"
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  //     User  // 需要回滚的字段
//	  // }
//	  // var apiUser = service.Service.Group.User  // 需要回滚的变量
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      Path:         "/path/to/server/api/v1/enter.go",
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      AppName:      "Service",
//	      GroupName:    "Group",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Rollback(file)
//	  _ = enter.Format("", nil, file)
//
//	  // 回滚后的文件内容：
//	  // package v1
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  //     // User 字段已被删除
//	  // }
//	  // // apiUser 变量和 import 语句已被删除
//
//	示例2：回滚 var 块中的变量，保留其他变量
//	  // 假设目标文件内容如下：
//	  // var (
//	  //     apiSystem = service.Service.Group.System
//	  //     apiUser = service.Service.Group.User  // 需要回滚的变量
//	  // )
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Rollback(file)
//
//	  // 回滚后的 var 块：
//	  // var (
//	  //     apiSystem = service.Service.Group.System
//	  //     // apiUser 已被删除，但 apiSystem 保留
//	  // )
//
//	示例3：回滚后删除空的 var 声明块
//	  // 假设目标文件内容如下：
//	  // var (
//	  //     apiUser = service.Service.Group.User  // 唯一的变量
//	  // )
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Rollback(file)
//
//	  // 回滚后：空的 var 块会被完全删除
//	  // var 声明块和对应的 import 语句都会被删除
//
//	示例4：服务模块回滚（只回滚结构体字段，不回滚变量）
//	  // TypePackageServiceModuleEnter 类型只回滚结构体字段
//	  // 假设目标文件内容如下：
//	  // type ServiceGroup struct {
//	  //     SystemService
//	  //     User  // 需要回滚的字段
//	  // }
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageServiceModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      AppName:      "Service",
//	      GroupName:    "Group",
//	      ModuleName:   "serviceUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Rollback(file)
//
//	  // 回滚后：只删除结构体字段，不会处理变量声明
//	  // type ServiceGroup struct {
//	  //     SystemService
//	  //     // User 字段已被删除
//	  // }
//
//	示例5：多次回滚的安全性（幂等性）
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Rollback(file)  // 第一次回滚，删除代码
//	  _ = enter.Rollback(file)  // 第二次回滚，不会报错，因为已经删除
//	  _ = enter.Rollback(file)  // 第三次回滚，仍然安全
//
//	  // 结果：无论调用多少次，都不会产生错误或副作用
//
//	示例6：回滚最后一个声明时的特殊处理
//	  // 如果 var 声明是文件中的最后一个声明，删除时需要特殊处理
//	  // 假设文件内容如下：
//	  // package v1
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  // }
//	  // var apiUser = service.Service.Group.User  // 最后一个声明
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Rollback(file)
//
//	  // 回滚后：最后一个声明被正确删除
//	  // package v1
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  // }
func (a *PackageModuleEnter) Rollback(file *ast.File) error {
	// 遍历文件的所有声明（declarations）
	for i := 0; i < len(file.Decls); i++ {
		v1, o1 := file.Decls[i].(*ast.GenDecl)
		if o1 {
			// 遍历通用声明中的所有规格（specs）
			for j := 0; j < len(v1.Specs); j++ {
				// 检查是否是类型声明（type declaration）
				v2, o2 := v1.Specs[j].(*ast.TypeSpec)
				if o2 {
					// 检查是否是目标 Group 类型（如 ApiGroup、ServiceGroup）
					if v2.Name.Name != a.Type.Group() {
						continue
					}
					// 检查是否是结构体类型
					v3, o3 := v2.Type.(*ast.StructType)
					if o3 {
						// 遍历结构体字段，查找并删除目标字段
						for k := 0; k < len(v3.Fields.List); k++ {
							v4, o4 := v3.Fields.List[k].Type.(*ast.Ident)
							// 如果字段类型匹配，则删除该字段
							if o4 && v4.Name == a.StructName {
								// 使用切片操作删除字段：保留前面的元素，跳过当前元素，保留后面的元素
								v3.Fields.List = append(v3.Fields.List[:k], v3.Fields.List[k+1:]...)
							}
						}
					}
					continue
				}
				// 对于服务模块入口类型，不需要回滚变量声明
				if a.Type == TypePackageServiceModuleEnter {
					continue
				}
				// 检查是否是值声明（value declaration，即 var 声明中的变量）
				v3, o3 := v1.Specs[j].(*ast.ValueSpec)
				if o3 {
					// 如果变量名匹配，则删除该变量声明
					if len(v3.Names) == 1 && v3.Names[0].Name == a.ModuleName {
						// 使用切片操作删除变量声明
						v1.Specs = append(v1.Specs[:j], v1.Specs[j+1:]...)
					}
				}
				// 如果 var 声明块变空了，需要删除整个 var 声明
				if v1.Tok == token.VAR && len(v1.Specs) == 0 {
					// 回滚对应的 import 语句
					_ = NewImport(a.ImportPath).Rollback(file)
					// 检查是否是最后一个声明
					if i == len(file.Decls)-1 {
						// 如果是最后一个，删除最后一个元素
						// 注意：这里使用切片操作删除最后一个元素，而不是 append
						file.Decls = file.Decls[:i]
						break
					}
					// 删除空的 var 声明
					// 空的 var() 如果不删除则会影响后续的注入变量，因为识别不到 *ast.ValueSpec
					file.Decls = append(file.Decls[:i], file.Decls[i+1:]...)
				}
			}
		}
	}
	return nil
}

// Injection 注入操作，用于向 AST 中注入模块相关的代码
//
// 注入内容：
// 1. Import 语句：注入必要的包导入
// 2. 结构体字段：向 Group 结构体（如 ApiGroup、ServiceGroup）中添加字段
// 3. 变量声明：创建模块变量（如 var serviceUser = service.Service.User）
//
// 设计思路：
// 1. 幂等性保证：
//   - 使用 hasValue 和 hasVariables 标志来避免重复注入
//   - 检查结构体字段是否已存在，避免重复添加
//   - 检查变量是否已声明，避免重复声明
//
// 2. 为什么需要检查 hasValue 和 hasVariables？
//   - hasVariables：标识文件中是否存在 var 声明块
//   - hasValue：标识目标变量是否已经声明
//   - 这两个标志的组合决定了注入策略：
//   - 如果存在 var 块但变量未声明：在现有 var 块中添加变量
//   - 如果不存在 var 块：创建新的 var 声明块
//
// 3. 结构体字段注入逻辑：
//   - 查找目标 Group 类型（如 ApiGroup、ServiceGroup）
//   - 检查字段是否已存在，避免重复添加
//   - 如果不存在，则添加新字段
//
// 4. 变量声明注入逻辑：
//   - 优先在现有的 var 块中添加变量（保持代码结构）
//   - 如果不存在 var 块，则创建新的 var 声明块
//   - 变量值使用选择器表达式构建：PackageName.AppName.GroupName.ServiceName
//
// 5. TypePackageServiceModuleEnter 的特殊处理：
//   - 对于服务模块入口类型，设置 hasValue = true
//   - 这是因为服务模块可能有不同的注入策略，不需要变量声明
//
// 6. 选择器表达式的构建：
//   - 构建多级选择器：PackageName.AppName.GroupName.ServiceName
//   - 例如：service.Service.Group.User
//   - 使用嵌套的 SelectorExpr 来表示多级选择
//
// 好处：
// - 幂等性：可以安全地多次调用，不会产生重复代码
// - 智能注入：自动判断注入位置，保持代码结构
// - 避免重复：检查现有代码，只注入不存在的部分
// - 代码整洁：优先使用现有的 var 块，避免创建多余的声明
//
// 注意事项：
// - 空的 var() 声明会被识别为不存在 var 块（hasVariables = false）
// - 这是因为空的 var 块中没有 *ast.ValueSpec，无法识别
//
// 使用示例：
//
//	示例1：注入 API 模块到 ApiGroup 结构体
//	  // 假设目标文件 server/api/v1/enter.go 内容如下：
//	  // package v1
//	  // import "gin-vue-admin/server/service"
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  // }
//	  // var apiUser = service.Service.Group.User
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      Path:         "/path/to/server/api/v1/enter.go",
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      AppName:      "Service",
//	      GroupName:    "Group",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Injection(file)
//	  _ = enter.Format("", nil, file)
//
//	  // 注入后的文件内容：
//	  // package v1
//	  // import "gin-vue-admin/server/service"
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  //     User  // 新增的字段
//	  // }
//	  // var apiUser = service.Service.Group.User  // 新增的变量
//
//	示例2：在现有 var 块中添加变量
//	  // 假设目标文件已有 var 块：
//	  // var (
//	  //     apiSystem = service.Service.Group.System
//	  // )
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      AppName:      "Service",
//	      GroupName:    "Group",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Injection(file)
//
//	  // 注入后的 var 块：
//	  // var (
//	  //     apiSystem = service.Service.Group.System
//	  //     apiUser = service.Service.Group.User  // 在现有 var 块中添加
//	  // )
//
//	示例3：创建新的 var 声明块
//	  // 假设目标文件没有 var 块，只有类型声明：
//	  // package v1
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  // }
//
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      AppName:      "Service",
//	      GroupName:    "Group",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Injection(file)
//
//	  // 注入后会创建新的 var 块：
//	  // package v1
//	  // type ApiGroup struct {
//	  //     SystemApi
//	  //     User
//	  // }
//	  // var apiUser = service.Service.Group.User  // 新创建的 var 块
//
//	示例4：服务模块注入（不创建变量声明）
//	  // TypePackageServiceModuleEnter 类型不会创建变量声明
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageServiceModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      AppName:      "Service",
//	      GroupName:    "Group",
//	      ModuleName:   "serviceUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Injection(file)
//
//	  // 只会注入结构体字段，不会创建变量：
//	  // type ServiceGroup struct {
//	  //     SystemService
//	  //     User  // 只添加字段，不创建变量
//	  // }
//
//	示例5：幂等性保证（多次调用不会重复注入）
//	  enter := &PackageModuleEnter{
//	      Type:         TypePackageApiModuleEnter,
//	      ImportPath:   "gin-vue-admin/server/service",
//	      StructName:   "User",
//	      AppName:      "Service",
//	      GroupName:    "Group",
//	      ModuleName:   "apiUser",
//	      PackageName:  "service",
//	      ServiceName:  "User",
//	  }
//	  file, _ := enter.Parse("", nil)
//	  _ = enter.Injection(file)  // 第一次调用，注入代码
//	  _ = enter.Injection(file)  // 第二次调用，不会重复注入
//	  _ = enter.Injection(file)  // 第三次调用，仍然不会重复注入
//
//	  // 结果：无论调用多少次，都只会注入一次
func (a *PackageModuleEnter) Injection(file *ast.File) error {
	// 首先注入 import 语句
	// 使用 NewImport 创建导入操作器，并调用其 Injection 方法
	// 忽略返回值是因为 import 注入失败不影响后续操作
	_ = NewImport(a.ImportPath).Injection(file)

	// 标志变量，用于判断注入状态
	var hasValue bool     // 标识目标变量是否已经声明
	var hasVariables bool // 标识文件中是否存在 var 声明块

	// 遍历文件的所有声明
	for i := 0; i < len(file.Decls); i++ {
		v1, o1 := file.Decls[i].(*ast.GenDecl)
		if o1 {
			// 检查是否是 var 声明块
			if v1.Tok == token.VAR {
				hasVariables = true
			}
			// 遍历通用声明中的所有规格
			for j := 0; j < len(v1.Specs); j++ {
				// 对于服务模块入口类型，设置 hasValue = true
				// 这是因为服务模块可能有不同的注入策略，不需要变量声明
				if a.Type == TypePackageServiceModuleEnter {
					hasValue = true
				}
				// 检查是否是类型声明
				v2, o2 := v1.Specs[j].(*ast.TypeSpec)
				if o2 {
					// 检查是否是目标 Group 类型
					if v2.Name.Name != a.Type.Group() {
						continue
					}
					// 检查是否是结构体类型
					v3, o3 := v2.Type.(*ast.StructType)
					if o3 {
						// 检查结构体字段是否已存在
						var hasStruct bool
						for k := 0; k < len(v3.Fields.List); k++ {
							v4, o4 := v3.Fields.List[k].Type.(*ast.Ident)
							if o4 && v4.Name == a.StructName {
								hasStruct = true
							}
						}
						// 如果字段不存在，则添加新字段
						if !hasStruct {
							// 创建新的字段，字段类型为 StructName
							field := &ast.Field{Type: &ast.Ident{Name: a.StructName}}
							v3.Fields.List = append(v3.Fields.List, field)
						}
					}
					continue
				}
				// 检查是否是值声明（变量声明）
				v3, o3 := v1.Specs[j].(*ast.ValueSpec)
				if o3 {
					hasVariables = true
					// 如果变量名匹配，说明变量已存在
					if len(v3.Names) == 1 && v3.Names[0].Name == a.ModuleName {
						hasValue = true
					}
				}
				// 如果 var 声明块是空的，则视为不存在 var 块
				// 说明是空 var()，空的 var 块中没有 *ast.ValueSpec，无法识别
				if v1.Tok == token.VAR && len(v1.Specs) == 0 {
					hasVariables = false
				}
				// 如果存在 var 块但变量未声明，则在现有 var 块中添加变量
				if hasVariables && !hasValue {
					// 构建选择器表达式：PackageName.AppName.GroupName.ServiceName
					// 例如：service.Service.Group.User
					spec := &ast.ValueSpec{
						Names: []*ast.Ident{{Name: a.ModuleName}},
						Values: []ast.Expr{
							&ast.SelectorExpr{
								X: &ast.SelectorExpr{
									X: &ast.SelectorExpr{
										X:   &ast.Ident{Name: a.PackageName},
										Sel: &ast.Ident{Name: a.AppName},
									},
									Sel: &ast.Ident{Name: a.GroupName},
								},
								Sel: &ast.Ident{Name: a.ServiceName},
							},
						},
					}
					// 在现有 var 块中添加变量声明
					v1.Specs = append(v1.Specs, spec)
					hasValue = true
				}
			}
		}
	}
	// 如果既没有变量声明，也没有 var 块，则创建新的 var 声明块
	if !hasValue && !hasVariables {
		// 创建新的 var 声明块
		decl := &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{{Name: a.ModuleName}},
					Values: []ast.Expr{
						// 构建选择器表达式：PackageName.AppName.GroupName.ServiceName
						&ast.SelectorExpr{
							X: &ast.SelectorExpr{
								X: &ast.SelectorExpr{
									X:   &ast.Ident{Name: a.PackageName},
									Sel: &ast.Ident{Name: a.AppName},
								},
								Sel: &ast.Ident{Name: a.GroupName},
							},
							Sel: &ast.Ident{Name: a.ServiceName},
						},
					},
				},
			},
		}
		// 将新的 var 声明块添加到文件声明列表
		file.Decls = append(file.Decls, decl)
	}
	return nil
}

// Format 格式化 AST 并写回文件
//
// 设计思路：
// 1. 路径处理：
//   - 如果 filename 为空，使用结构体中的 Path（绝对路径）
//   - 这样可以确保格式化操作能正确找到目标文件
//
// 2. 为什么需要格式化？
//   - AST 操作后，代码结构可能不够规范（如缩进、换行等）
//   - 使用 go/format 包可以自动格式化代码，符合 Go 代码规范
//   - 格式化后的代码更易读，符合团队编码规范
//
// 3. 委托给 Base.Format：
//   - Base.Format 实现了通用的格式化逻辑
//   - 通过委托实现代码复用，避免重复实现
//   - 保持格式化逻辑的一致性
//
// 好处：
// - 代码复用：使用 Base.Format 的通用实现，避免重复代码
// - 自动格式化：确保生成的代码符合 Go 代码规范
// - 路径处理：自动处理路径为空的情况，提高 API 的易用性
//
// 使用场景：
// - 代码生成后需要格式化，确保代码可读性
// - 代码重构后需要格式化，保持代码风格一致
func (a *PackageModuleEnter) Format(filename string, writer io.Writer, file *ast.File) error {
	// 如果 filename 为空，使用结构体中的 Path（绝对路径）
	// 这样可以确保格式化操作能正确找到目标文件
	if filename == "" {
		filename = a.Path
	}
	// 委托给 Base.Format 实现实际的格式化逻辑
	// Base.Format 会使用 go/format 包格式化代码，并写回文件
	return a.Base.Format(filename, writer, file)
}
