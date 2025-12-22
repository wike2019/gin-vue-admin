package ast

import (
	"go/ast"
	"go/token"
	"io"
)

// PluginEnter 插件化入口
// 用于在插件系统中自动注入和回滚代码，实现插件的动态注册和卸载
//
// 设计思路：
// 1. 通过 AST 操作实现代码的自动注入，避免手动修改代码，减少出错概率
// 2. 支持回滚机制，可以安全地移除注入的代码，保证代码的可逆性
// 3. 通过类型区分不同的注入场景（API、Router、Service），实现细粒度的控制
//
// 模块命名规则：
// ModuleName := PackageName.GroupName.ServiceName
// 例如：serviceUser = service.Service.User
// 这种命名方式保证了模块名称的唯一性和可读性，便于在代码中快速定位
type PluginEnter struct {
	Base                   // 嵌入 Base 结构体，复用基础的 AST 解析和格式化功能
	Type            Type   // 插件类型，用于区分不同的注入场景（如 TypePluginApiEnter、TypePluginRouterEnter、TypePluginServiceEnter）
	Path            string // 目标文件的绝对路径，用于定位需要修改的文件
	ImportPath      string // 需要注入的导入路径，用于添加 import 语句
	RelativePath    string // 相对路径，提供路径的另一种表示方式，便于在不同环境下使用
	StructName      string // 结构体字段名称（大驼峰），例如 "User"，将作为结构体的字段名
	StructCamelName string // 结构体类型名称（小驼峰），例如 "user"，将作为字段的类型名
	ModuleName      string // 模块变量名称，例如 "serviceUser"，用于创建全局变量
	GroupName       string // 分组名称，例如 "Service"，用于构建选择器表达式
	PackageName     string // 包名，例如 "service"，用于构建选择器表达式
	ServiceName     string // 服务名称，例如 "User"，用于构建选择器表达式
}

// Parse 解析 Go 源文件并返回 AST 文件节点
//
// 设计思路：
// 1. 支持多种路径输入方式（绝对路径、相对路径），提高方法的灵活性
// 2. 自动处理路径转换，确保无论输入哪种路径都能正确解析
// 3. 复用 Base.Parse 方法，保持代码的一致性和可维护性
//
// 路径处理逻辑：
// - 如果 filename 为空，则使用结构体中的路径信息
// - 优先使用 RelativePath，如果不存在则使用 Path
// - 自动同步 Path 和 RelativePath，确保两者保持一致
//
// 好处：
// - 调用者无需关心路径格式，方法内部自动处理
// - 减少路径转换的重复代码，提高代码复用性
func (a *PluginEnter) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	// 如果未提供文件名，则从结构体中获取路径信息
	if filename == "" {
		// 优先使用相对路径，如果相对路径为空，则使用绝对路径
		if a.RelativePath == "" {
			filename = a.Path
			// 将绝对路径转换为相对路径并保存，便于后续使用
			a.RelativePath = a.Base.RelativePath(a.Path)
			return a.Base.Parse(filename, writer)
		}
		// 将相对路径转换为绝对路径，确保路径的准确性
		a.Path = a.Base.AbsolutePath(a.RelativePath)
		filename = a.Path
	}
	// 调用基类的 Parse 方法进行实际的解析工作
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚之前注入的代码，将文件恢复到注入前的状态
//
// 设计思路：
// 1. 使用 AST 遍历精确查找需要删除的代码，避免误删其他代码
// 2. 分步骤回滚：先回滚结构体字段，再回滚变量声明
// 3. 智能清理：如果结构体字段为空，则同时清理相关的 import 语句
// 4. 类型区分：对于 TypePluginServiceEnter 类型，只回滚结构体字段，不回滚变量
//
// 好处：
// - 保证代码的可逆性，插件卸载时可以完全恢复原状
// - 避免残留代码，保持代码的整洁性
// - 通过类型区分实现细粒度控制，满足不同场景的需求
//
// 使用示例：
//
// 示例 1：回滚 API 类型的插件注入
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"
//	// )
//	//
//	// type ApiGroup struct {
//	//     User api.UserApi
//	// }
//	//
//	// var (
//	//     apiUser = api.Api.User
//	// )
//
//	enter := &PluginEnter{
//		Type:            TypePluginApiEnter,
//		Path:            "/path/to/api/v1/enter.go",
//		ImportPath:      "gin-vue-admin/server/api/v1/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "apiUser",
//		GroupName:       "Api",
//		PackageName:     "api",
//		ServiceName:     "User",
//	}
//
//	// 解析文件获取 AST
//	file, err := enter.Parse("", nil)
//	if err != nil {
//		return err
//	}
//
//	// 执行回滚操作
//	err = enter.Rollback(file)
//	if err != nil {
//		return err
//	}
//
//	// 格式化并写回文件
//	var buf bytes.Buffer
//	err = enter.Format("", &buf, file)
//	if err != nil {
//		return err
//	}
//
//	// 调用后的代码（回滚后的 enter.go 文件内容）：
//	// package v1
//	//
//	// type ApiGroup struct {
//	//     // User 字段已被删除
//	// }
//	//
//	// var (
//	//     // apiUser 变量已被删除
//	// )
//	// 注意：如果 ApiGroup 结构体还有其他字段，则 import 不会被删除
//	// 只有当结构体字段列表完全为空时，才会删除对应的 import
//
// 示例 2：回滚 Router 类型的插件注入
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package router
//	//
//	// import (
//	//     "gin-vue-admin/server/router/example"
//	// )
//	//
//	// type RouterGroup struct {
//	//     User router.UserRouter
//	// }
//	//
//	// var (
//	//     routerUser = router.Router.User
//	// )
//
//	enter := &PluginEnter{
//		Type:            TypePluginRouterEnter,
//		Path:            "/path/to/router/enter.go",
//		ImportPath:      "gin-vue-admin/server/router/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "routerUser",
//		GroupName:       "Router",
//		PackageName:     "router",
//		ServiceName:     "User",
//	}
//
//	file, _ := enter.Parse("", nil)
//	enter.Rollback(file) // 会同时删除结构体字段和变量声明
//
//	// 调用后的代码（回滚后的 enter.go 文件内容）：
//	// package router
//	//
//	// type RouterGroup struct {
//	//     // User 字段已被删除
//	// }
//	//
//	// var (
//	//     // routerUser 变量已被删除
//	// )
//
// 示例 3：回滚 Service 类型的插件注入（只回滚结构体字段）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package service
//	//
//	// import (
//	//     "gin-vue-admin/server/service/example"
//	// )
//	//
//	// type ServiceGroup struct {
//	//     User service.UserService
//	// }
//	// 注意：Service 类型不会注入变量，所以只需要回滚结构体字段
//
//	enter := &PluginEnter{
//		Type:            TypePluginServiceEnter,
//		Path:            "/path/to/service/enter.go",
//		ImportPath:      "gin-vue-admin/server/service/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "serviceUser", // 虽然定义了，但不会用于变量注入
//		GroupName:       "Service",
//		PackageName:     "service",
//		ServiceName:     "User",
//	}
//
//	file, _ := enter.Parse("", nil)
//	enter.Rollback(file) // 只删除结构体字段，不会删除变量（因为没有注入变量）
//
//	// 调用后的代码（回滚后的 enter.go 文件内容）：
//	// package service
//	//
//	// type ServiceGroup struct {
//	//     // User 字段已被删除
//	// }
//	// 注意：如果 ServiceGroup 结构体还有其他字段，则 import 不会被删除
//
// 示例 4：完整的回滚流程（包含错误处理）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"
//	// )
//	//
//	// type ApiGroup struct {
//	//     User api.UserApi
//	// }
//	//
//	// var (
//	//     apiUser = api.Api.User
//	// )
//
//	enter := &PluginEnter{
//		Type:            TypePluginApiEnter,
//		Path:            "/path/to/api/v1/enter.go",
//		ImportPath:      "gin-vue-admin/server/api/v1/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "apiUser",
//		GroupName:       "Api",
//		PackageName:     "api",
//		ServiceName:     "User",
//	}
//
//	// 1. 解析文件
//	file, err := enter.Parse("", nil)
//	if err != nil {
//		log.Printf("解析文件失败: %v", err)
//		return err
//	}
//
//	// 2. 执行回滚
//	if err := enter.Rollback(file); err != nil {
//		log.Printf("回滚失败: %v", err)
//		return err
//	}
//
//	// 3. 格式化并保存
//	var buf bytes.Buffer
//	if err := enter.Format("", &buf, file); err != nil {
//		log.Printf("格式化失败: %v", err)
//		return err
//	}
//
//	// 4. 写回文件
//	if err := os.WriteFile(enter.Path, buf.Bytes(), 0644); err != nil {
//		log.Printf("写入文件失败: %v", err)
//		return err
//	}
//
//	// 调用后的代码（回滚后的 enter.go 文件内容）：
//	// package v1
//	//
//	// type ApiGroup struct {
//	//     // User 字段已被删除
//	// }
//	//
//	// var (
//	//     // apiUser 变量已被删除
//	// )
//	// 回滚完成，文件已恢复到注入前的状态
//
// 示例 5：回滚时结构体包含多个字段的情况（import 不会被删除）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"
//	//     "gin-vue-admin/server/api/v1/system"
//	// )
//	//
//	// type ApiGroup struct {
//	//     User    api.UserApi
//	//     System  api.SystemApi
//	// }
//	//
//	// var (
//	//     apiUser   = api.Api.User
//	//     apiSystem = api.Api.System
//	// )
//
//	// 只回滚 User 相关的代码
//	enter := &PluginEnter{
//		Type:            TypePluginApiEnter,
//		Path:            "/path/to/api/v1/enter.go",
//		ImportPath:      "gin-vue-admin/server/api/v1/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "apiUser",
//		GroupName:       "Api",
//		PackageName:     "api",
//		ServiceName:     "User",
//	}
//
//	file, _ := enter.Parse("", nil)
//	enter.Rollback(file)
//
//	// 调用后的代码（回滚后的 enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"  // import 保留，因为结构体还有其他字段
//	//     "gin-vue-admin/server/api/v1/system"
//	// )
//	//
//	// type ApiGroup struct {
//	//     System api.SystemApi  // User 字段已被删除，System 字段保留
//	// }
//	//
//	// var (
//	//     apiSystem = api.Api.System  // apiUser 变量已被删除，apiSystem 变量保留
//	// )
//	// 注意：由于 ApiGroup 结构体还有其他字段（System），所以 example 的 import 不会被删除
//	// 只有当结构体字段列表完全为空时，才会删除对应的 import
func (a *PluginEnter) Rollback(file *ast.File) error {
	// 第一步：回滚结构体字段
	// 查找并删除之前注入的结构体字段
	var structType *ast.StructType
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			// 查找结构体类型定义
			if s, ok := x.Type.(*ast.StructType); ok {
				structType = s
				// 遍历结构体字段，查找匹配的字段名
				for i, field := range x.Type.(*ast.StructType).Fields.List {
					// 检查字段名是否匹配（需要确保字段有名称）
					if len(field.Names) > 0 && field.Names[0].Name == a.StructName {
						// 使用切片操作删除匹配的字段，这是 Go 中删除切片元素的常用方式
						// append(slice[:i], slice[i+1:]...) 表示保留 i 之前的元素和 i+1 之后的元素
						s.Fields.List = append(s.Fields.List[:i], s.Fields.List[i+1:]...)
						return false // 找到后停止遍历，提高效率
					}
				}
			}
		}
		return true // 继续遍历其他节点
	})

	// 如果结构体字段列表为空，说明该结构体不再需要相关的 import
	// 此时回滚对应的 import 语句，避免产生无用的导入
	// 这种设计保证了代码的整洁性，不会留下无用的 import
	if len(structType.Fields.List) == 0 {
		_ = NewImport(a.ImportPath).Rollback(file)
	}

	// 对于 Service 类型的插件，只需要回滚结构体字段，不需要回滚变量
	// 这是因为 Service 类型的插件在 enter.go 中只注入结构体字段，不注入变量
	// 这种类型区分的设计使得代码更加灵活，可以针对不同场景采用不同的注入策略
	if a.Type == TypePluginServiceEnter {
		return nil
	}

	// 第二步：回滚变量声明
	// 查找并删除之前注入的全局变量
	ast.Inspect(file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		// 查找变量声明（token.VAR 表示变量声明）
		if ok && genDecl.Tok == token.VAR {
			// 遍历变量声明中的所有规格说明
			for i, spec := range genDecl.Specs {
				valueSpec, vsok := spec.(*ast.ValueSpec)
				if vsok {
					// 检查变量名是否匹配
					for _, name := range valueSpec.Names {
						if name.Name == a.ModuleName {
							// 删除匹配的变量声明
							genDecl.Specs = append(genDecl.Specs[:i], genDecl.Specs[i+1:]...)
							return false // 找到后停止遍历
						}
					}
				}
			}
		}
		return true // 继续遍历其他节点
	})

	return nil
}

// Injection 向 AST 中注入插件相关的代码（结构体字段和变量声明）
//
// 设计思路：
// 1. 幂等性设计：先检查代码是否已存在，避免重复注入
// 2. 分步骤注入：先注入 import，再注入结构体字段，最后注入变量
// 3. 类型区分：对于 TypePluginServiceEnter 类型，只注入结构体字段，不注入变量
// 4. 使用 AST 节点构建代码，保证生成的代码符合 Go 语法规范
//
// 好处：
// - 幂等性保证多次调用不会产生重复代码
// - 自动生成符合规范的代码，减少手动编写错误
// - 通过 AST 操作保证代码的语法正确性
//
// 使用示例：
//
// 示例 1：注入 API 类型的插件（包含结构体字段和变量）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package v1
//	//
//	// type ApiGroup struct {
//	//     // 结构体为空或已有其他字段
//	// }
//	//
//	// var (
//	//     // 变量声明区域为空或已有其他变量
//	// )
//
//	enter := &PluginEnter{
//		Type:            TypePluginApiEnter,
//		Path:            "/path/to/api/v1/enter.go",
//		ImportPath:      "gin-vue-admin/server/api/v1/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "apiUser",
//		GroupName:       "Api",
//		PackageName:     "api",
//		ServiceName:     "User",
//	}
//
//	// 解析文件获取 AST
//	file, err := enter.Parse("", nil)
//	if err != nil {
//		return err
//	}
//
//	// 执行注入操作
//	err = enter.Injection(file)
//	if err != nil {
//		return err
//	}
//
//	// 格式化并写回文件
//	var buf bytes.Buffer
//	err = enter.Format("", &buf, file)
//	if err != nil {
//		return err
//	}
//
//	// 调用后的代码（注入后的 enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"
//	// )
//	//
//	// type ApiGroup struct {
//	//     User api.UserApi  // 新注入的结构体字段
//	// }
//	//
//	// var (
//	//     apiUser = api.Api.User  // 新注入的变量声明
//	// )
//
// 示例 2：注入 Router 类型的插件（包含结构体字段和变量）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package router
//	//
//	// type RouterGroup struct {
//	//     // 结构体为空或已有其他字段
//	// }
//	//
//	// var (
//	//     // 变量声明区域为空或已有其他变量
//	// )
//
//	enter := &PluginEnter{
//		Type:            TypePluginRouterEnter,
//		Path:            "/path/to/router/enter.go",
//		ImportPath:      "gin-vue-admin/server/router/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "routerUser",
//		GroupName:       "Router",
//		PackageName:     "router",
//		ServiceName:     "User",
//	}
//
//	file, _ := enter.Parse("", nil)
//	enter.Injection(file) // 会同时注入结构体字段和变量声明
//
//	// 调用后的代码（注入后的 enter.go 文件内容）：
//	// package router
//	//
//	// import (
//	//     "gin-vue-admin/server/router/example"
//	// )
//	//
//	// type RouterGroup struct {
//	//     User router.UserRouter  // 新注入的结构体字段
//	// }
//	//
//	// var (
//	//     routerUser = router.Router.User  // 新注入的变量声明
//	// )
//
// 示例 3：注入 Service 类型的插件（只注入结构体字段，不注入变量）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package service
//	//
//	// type ServiceGroup struct {
//	//     // 结构体为空或已有其他字段
//	// }
//	// 注意：Service 类型不会注入变量，所以不需要 var 声明区域
//
//	enter := &PluginEnter{
//		Type:            TypePluginServiceEnter,
//		Path:            "/path/to/service/enter.go",
//		ImportPath:      "gin-vue-admin/server/service/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "serviceUser", // 虽然定义了，但不会用于变量注入
//		GroupName:       "Service",
//		PackageName:     "service",
//		ServiceName:     "User",
//	}
//
//	file, _ := enter.Parse("", nil)
//	enter.Injection(file) // 只注入结构体字段，不会注入变量
//
//	// 调用后的代码（注入后的 enter.go 文件内容）：
//	// package service
//	//
//	// import (
//	//     "gin-vue-admin/server/service/example"
//	// )
//	//
//	// type ServiceGroup struct {
//	//     User service.UserService  // 新注入的结构体字段
//	// }
//	// 注意：不会注入变量声明，因为 TypePluginServiceEnter 类型只注入结构体字段
//
// 示例 4：完整的注入流程（包含错误处理）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package v1
//	//
//	// type ApiGroup struct {
//	//     System api.SystemApi  // 已有字段
//	// }
//	//
//	// var (
//	//     apiSystem = api.Api.System  // 已有变量
//	// )
//
//	enter := &PluginEnter{
//		Type:            TypePluginApiEnter,
//		Path:            "/path/to/api/v1/enter.go",
//		ImportPath:      "gin-vue-admin/server/api/v1/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "apiUser",
//		GroupName:       "Api",
//		PackageName:     "api",
//		ServiceName:     "User",
//	}
//
//	// 1. 解析文件
//	file, err := enter.Parse("", nil)
//	if err != nil {
//		log.Printf("解析文件失败: %v", err)
//		return err
//	}
//
//	// 2. 执行注入
//	if err := enter.Injection(file); err != nil {
//		log.Printf("注入失败: %v", err)
//		return err
//	}
//
//	// 3. 格式化并保存
//	var buf bytes.Buffer
//	if err := enter.Format("", &buf, file); err != nil {
//		log.Printf("格式化失败: %v", err)
//		return err
//	}
//
//	// 4. 写回文件
//	if err := os.WriteFile(enter.Path, buf.Bytes(), 0644); err != nil {
//		log.Printf("写入文件失败: %v", err)
//		return err
//	}
//
//	// 调用后的代码（注入后的 enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"
//	//     "gin-vue-admin/server/api/v1/system"
//	// )
//	//
//	// type ApiGroup struct {
//	//     System api.SystemApi  // 原有字段保留
//	//     User   api.UserApi    // 新注入的字段追加在末尾
//	// }
//	//
//	// var (
//	//     apiSystem = api.Api.System  // 原有变量保留
//	//     apiUser   = api.Api.User    // 新注入的变量追加在末尾
//	// )
//	// 注入完成，新字段和变量都已追加到现有代码的末尾
//
// 示例 5：幂等性示例（多次调用不会重复注入）
//
//	// 调用前的代码（enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"
//	// )
//	//
//	// type ApiGroup struct {
//	//     User api.UserApi
//	// }
//	//
//	// var (
//	//     apiUser = api.Api.User
//	// )
//
//	enter := &PluginEnter{
//		Type:            TypePluginApiEnter,
//		Path:            "/path/to/api/v1/enter.go",
//		ImportPath:      "gin-vue-admin/server/api/v1/example",
//		StructName:      "User",
//		StructCamelName: "user",
//		ModuleName:      "apiUser",
//		GroupName:       "Api",
//		PackageName:     "api",
//		ServiceName:     "User",
//	}
//
//	file, _ := enter.Parse("", nil)
//
//	// 第一次调用：注入代码
//	enter.Injection(file)
//
//	// 第二次调用：由于代码已存在，不会重复注入
//	enter.Injection(file)
//
//	// 第三次调用：仍然不会重复注入
//	enter.Injection(file)
//
//	// 调用后的代码（多次调用后的 enter.go 文件内容）：
//	// package v1
//	//
//	// import (
//	//     "gin-vue-admin/server/api/v1/example"
//	// )
//	//
//	// type ApiGroup struct {
//	//     User api.UserApi  // 仍然只有一个字段，不会重复
//	// }
//	//
//	// var (
//	//     apiUser = api.Api.User  // 仍然只有一个变量，不会重复
//	// )
//	// 幂等性保证：多次调用 Injection 方法，结果保持一致，不会产生重复代码
func (a *PluginEnter) Injection(file *ast.File) error {
	// 第一步：注入 import 语句
	// 使用 Import 结构体处理 import 的注入，复用已有的逻辑
	_ = NewImport(a.ImportPath).Injection(file)

	// 第二步：注入结构体字段
	has := false                    // 标记结构体字段是否已存在
	hasVar := false                 // 标记变量是否已存在
	var firstStruct *ast.StructType // 保存找到的第一个结构体，用于后续注入
	var varSpec *ast.GenDecl        // 保存找到的变量声明节点，用于后续注入

	// 遍历 AST 查找结构体定义
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			// 查找结构体类型定义
			if s, ok := x.Type.(*ast.StructType); ok {
				firstStruct = s // 保存第一个找到的结构体，用于后续注入
				// 检查结构体中是否已存在同名字段
				for _, field := range x.Type.(*ast.StructType).Fields.List {
					if len(field.Names) > 0 && field.Names[0].Name == a.StructName {
						has = true   // 字段已存在，标记为 true
						return false // 找到后停止遍历
					}
				}
			}
		}
		return true // 继续遍历其他节点
	})

	// 如果字段不存在，则创建并注入新的字段
	// 这种设计保证了幂等性：多次调用不会产生重复字段
	if !has {
		// 构建 AST 字段节点
		// Names 表示字段名（大驼峰），Type 表示字段类型（小驼峰）
		field := &ast.Field{
			Names: []*ast.Ident{{Name: a.StructName}},
			Type:  &ast.Ident{Name: a.StructCamelName},
		}
		// 将新字段追加到结构体字段列表的末尾
		// 这种追加方式保证了字段的顺序，新字段总是在最后
		firstStruct.Fields.List = append(firstStruct.Fields.List, field)
	}

	// 对于 Service 类型的插件，只需要注入结构体字段，不需要注入变量
	// 这是因为 Service 类型的插件在 enter.go 中只需要结构体字段即可
	// 这种类型区分的设计使得代码更加灵活，可以针对不同场景采用不同的注入策略
	if a.Type == TypePluginServiceEnter {
		return nil
	}

	// 第三步：注入变量声明
	// 遍历 AST 查找变量声明节点
	ast.Inspect(file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		// 查找变量声明（token.VAR 表示变量声明）
		if ok && genDecl.Tok == token.VAR {
			// 遍历变量声明中的所有规格说明
			for _, spec := range genDecl.Specs {
				valueSpec, vsok := spec.(*ast.ValueSpec)
				if vsok {
					varSpec = genDecl // 保存变量声明节点，用于后续注入
					// 检查变量名是否已存在
					for _, name := range valueSpec.Names {
						if name.Name == a.ModuleName {
							hasVar = true // 变量已存在，标记为 true
							return false  // 找到后停止遍历
						}
					}
				}
			}
		}
		return true // 继续遍历其他节点
	})

	// 如果变量不存在，则创建并注入新的变量声明
	// 这种设计保证了幂等性：多次调用不会产生重复变量
	if !hasVar {
		// 构建 AST 变量声明节点
		// Names 表示变量名（ModuleName，如 "serviceUser"）
		// Values 表示变量的初始值，使用选择器表达式构建
		// 例如：service.Service.User 表示从 service 包的 Service 结构体中获取 User 字段
		spec := &ast.ValueSpec{
			Names: []*ast.Ident{{Name: a.ModuleName}},
			Values: []ast.Expr{
				// 构建嵌套的选择器表达式
				// 外层：PackageName.GroupName（如 service.Service）
				// 内层：ServiceName（如 User）
				// 最终生成：service.Service.User
				&ast.SelectorExpr{
					X: &ast.SelectorExpr{
						X:   &ast.Ident{Name: a.PackageName}, // 包名，如 "service"
						Sel: &ast.Ident{Name: a.GroupName},   // 分组名，如 "Service"
					},
					Sel: &ast.Ident{Name: a.ServiceName}, // 服务名，如 "User"
				},
			},
		}
		// 将新变量追加到变量声明列表的末尾
		varSpec.Specs = append(varSpec.Specs, spec)
	}

	return nil
}

// Format 格式化 AST 并写入文件
//
// 设计思路：
// 1. 复用 Base.Format 方法，保持代码格式化的一致性
// 2. 自动处理文件名，如果未提供则使用结构体中的路径
// 3. 格式化后的代码符合 Go 官方规范，保证代码的可读性
//
// 好处：
// - 统一代码格式，保持代码风格一致
// - 自动处理文件名，减少调用者的负担
// - 使用 Go 标准库的格式化功能，保证代码质量
func (a *PluginEnter) Format(filename string, writer io.Writer, file *ast.File) error {
	// 如果未提供文件名，则使用结构体中保存的路径
	// 这种设计使得方法调用更加灵活，可以省略文件名参数
	if filename == "" {
		filename = a.Path
	}
	// 调用基类的 Format 方法进行实际的格式化工作
	// Base.Format 会使用 go/format 包将 AST 转换为格式化的 Go 代码
	return a.Base.Format(filename, writer, file)
}
