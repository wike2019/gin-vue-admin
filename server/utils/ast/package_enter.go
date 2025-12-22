package ast

import (
	"go/ast"
	"go/token"
	"io"
)

// PackageEnter 模块化入口，用于自动向 Go 代码中注入包导入和结构体字段
//
// 设计模式：模板方法模式 + 组合模式
//
// 核心功能：
// 1. 自动注入包导入（import）：确保新包被正确导入
// 2. 自动注入结构体字段：在指定的结构体中添加新字段，实现模块化注册
//
// 为什么需要这个功能？
// - 模块化架构：在大型项目中，需要将不同模块的 API、Router、Service 等注册到统一的入口结构体中
// - 自动化代码生成：避免手动维护注册代码，减少人为错误
// - 代码一致性：确保所有模块的注册方式统一，便于维护
//
// 使用场景：
// - 向 ApiGroup 结构体中注入新的 API 模块（如：UserApi、OrderApi）
// - 向 RouterGroup 结构体中注入新的路由模块（如：UserRouter、OrderRouter）
// - 向 ServiceGroup 结构体中注入新的服务模块（如：UserService、OrderService）
//
// 设计优势：
// 1. 通过嵌入 Base 结构体实现代码复用，避免重复实现基础的 AST 解析和格式化功能
// 2. 采用模板方法模式，只需重写特定的 Injection 方法即可实现自定义的代码注入逻辑
// 3. 支持相对路径和绝对路径的自动转换，提高代码的可移植性和灵活性
// 4. 幂等性设计：重复调用不会产生重复的字段，通过检查字段是否存在来避免重复注入
//
// 架构好处：
// - 代码复用：Base 提供通用功能，子类只需关注核心逻辑
// - 类型安全：编译时检查，避免运行时错误
// - 可扩展性：新增注入类型只需实现 Ast 接口，无需修改现有代码
// - 可测试性：接口便于 Mock，提高单元测试覆盖率
type PackageEnter struct {
	Base                     // 嵌入基础结构体，继承 Parse、Format、Rollback 等通用方法
	Type              Type   // 类型：用于确定要注入的目标结构体名称（通过 Type.Group() 获取）
	Path              string // 文件路径：目标文件的绝对路径
	ImportPath        string // 导包路径：需要导入的包路径（如："github.com/example/user/api"）
	StructName        string // 结构体名称：要注入的字段名称（如："UserApi"）
	PackageName       string // 包名：导入包的别名或包名（如："userApi"）
	RelativePath      string // 相对路径：相对于项目根目录的路径，用于跨平台和可移植性
	PackageStructName string // 包结构体名称：要注入的字段类型（如："ApiGroup"，表示 userApi.ApiGroup）
}

// Parse 解析 Go 源文件并返回 AST 文件节点
//
// 路径处理逻辑说明（为什么这样设计？）：
//  1. 如果 filename 为空，需要根据已有信息推断文件路径
//  2. 优先使用 RelativePath（相对路径），因为相对路径具有更好的可移植性
//     可以在不同环境（开发、测试、生产）中正常工作，不依赖具体部署路径
//  3. 如果 RelativePath 为空，则使用 Path（绝对路径）并计算相对路径
//     这样可以在首次调用时建立路径映射关系，后续可以使用相对路径
//  4. 如果已有 RelativePath，则反向计算绝对路径，确保路径一致性
//
// 设计好处：
// - 支持灵活的路径输入方式，提高 API 的易用性
// - 自动维护相对路径和绝对路径的映射，避免路径不一致问题
// - 通过委托给 Base.Parse 实现代码复用，遵循 DRY（Don't Repeat Yourself）原则
// - 相对路径存储更适合版本控制和配置管理
//
// 使用示例：
//
//	示例1：首次调用，使用绝对路径
//		parser := &PackageEnter{
//			Path: "/project/server/api/v1/enter.go",
//		}
//		file, _ := parser.Parse("", nil)
//		// 此时会自动计算 RelativePath = "api/v1/enter.go"
//
//	示例2：后续调用，使用相对路径
//		parser := &PackageEnter{
//			RelativePath: "api/v1/enter.go",
//		}
//		file, _ := parser.Parse("", nil)
//		// 此时会自动计算 Path = "/project/server/api/v1/enter.go"
func (a *PackageEnter) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	if filename == "" {
		if a.RelativePath == "" {
			// 首次调用：使用绝对路径，并计算相对路径
			filename = a.Path
			a.RelativePath = a.Base.RelativePath(a.Path)
			return a.Base.Parse(filename, writer)
		}
		// 后续调用：使用相对路径，反向计算绝对路径
		a.Path = a.Base.AbsolutePath(a.RelativePath)
		filename = a.Path
	}
	// 委托给 Base.Parse，实现代码复用
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作，用于撤销 Injection 所做的修改
//
// 为什么是空实现？
//  1. 模块化入口的注入操作通常是单向的、不可逆的
//     一旦字段被注入，撤销操作需要复杂的逻辑来识别和删除特定字段
//  2. 实际使用场景中，很少需要回滚模块注册操作
//     如果需要移除模块，通常直接删除代码或注释掉相关字段即可
//  3. 保持接口一致性：实现 Ast 接口的所有结构体都有 Rollback 方法
//     空实现避免了强制实现不必要的逻辑，遵循接口隔离原则
//
// 设计好处：
// - 接口统一：所有 AST 操作器都实现相同的方法签名
// - 简化实现：不需要回滚的子类无需实现复杂逻辑
// - 扩展性：如果将来需要回滚功能，可以重写此方法实现具体逻辑
func (a *PackageEnter) Rollback(file *ast.File) error {
	// 无需回滚：模块化入口的注入操作通常是单向的
	return nil
}

// Injection 注入操作，向 AST 中注入包导入和结构体字段
//
// 核心逻辑说明（为什么这样写？）：
//
//  1. 先注入包导入（ImportPath）：
//     - 原因：在添加结构体字段之前，必须先确保包被正确导入
//     - 好处：避免编译错误，确保字段类型（如：userApi.ApiGroup）可以被正确识别
//     - 幂等性：NewImport 内部会检查导入是否已存在，避免重复导入
//
//  2. 使用 ast.Inspect 遍历 AST：
//     - 原因：需要在整个语法树中查找目标结构体定义
//     - 好处：ast.Inspect 是 Go 标准库提供的深度优先遍历工具，安全可靠
//     - 灵活性：可以访问所有 AST 节点，支持复杂的查找逻辑
//
//  3. 查找类型声明（GenDecl with token.TYPE）：
//     - 原因：结构体定义是通过类型声明（type X struct）实现的
//     - 好处：精确定位结构体定义，避免误操作其他类型的声明
//
//  4. 匹配目标结构体（Type.Group()）：
//     - 原因：不同的 Type 对应不同的结构体（如：ApiGroup、RouterGroup、ServiceGroup）
//     - 好处：确保注入到正确的结构体中，避免错误注入
//
//  5. 检查字段是否已存在：
//     - 原因：避免重复注入相同字段，实现幂等性
//     - 好处：重复调用不会产生重复字段，保证代码正确性
//     - 检查逻辑：遍历现有字段，如果找到同名字段则直接返回，不进行注入
//
//  6. 创建字段节点（ast.Field）：
//     - 字段名：使用 StructName（如："UserApi"）
//     - 字段类型：使用 SelectorExpr（如："userApi.ApiGroup"）
//     - X: 包名（PackageName，如："userApi"）
//     - Sel: 结构体名（PackageStructName，如："ApiGroup"）
//     - 好处：生成的代码符合 Go 语法规范，可以直接编译
//
//  7. 使用 SelectorExpr 的原因：
//     - 表示包.类型的形式（如：userApi.ApiGroup）
//     - 这是 Go 中引用其他包中类型的标准方式
//     - 确保生成的代码类型正确，可以被编译器识别
//
//  8. 提前返回（return false）：
//     - 原因：找到目标结构体并成功注入后，无需继续遍历
//     - 好处：提高性能，避免不必要的遍历
//
// 设计好处总结：
// - 幂等性：重复调用不会产生副作用，保证代码正确性
// - 安全性：检查字段是否存在，避免重复注入
// - 正确性：先导入包再添加字段，确保类型可以被识别
// - 性能：找到目标后提前返回，避免不必要的遍历
// - 可维护性：代码逻辑清晰，易于理解和修改
//
// 生成的代码示例：
//
//	注入前：
//		package v1
//		import "github.com/example/user/api"
//		type ApiGroup struct {
//			ExampleApi example.ApiGroup
//		}
//
//	注入后（添加 UserApi 字段）：
//		package v1
//		import (
//			"github.com/example/user/api"
//			userApi "github.com/example/user/api"  // 自动添加的导入
//		)
//		type ApiGroup struct {
//			ExampleApi example.ApiGroup
//			UserApi    userApi.ApiGroup  // 自动添加的字段
//		}
func (a *PackageEnter) Injection(file *ast.File) error {
	// 步骤1：先注入包导入，确保后续字段类型可以被正确识别
	// 注意：使用 _ 忽略返回值，因为 NewImport 内部已经处理了重复导入的情况
	_ = NewImport(a.ImportPath).Injection(file)

	// 步骤2：遍历 AST，查找目标结构体并注入字段
	ast.Inspect(file, func(n ast.Node) bool {
		// 步骤3：查找类型声明（type X struct）
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			// 不是类型声明，继续遍历
			return true
		}

		// 步骤4：遍历类型声明中的所有类型规范
		for _, spec := range genDecl.Specs {
			typeSpec, specok := spec.(*ast.TypeSpec)
			// 步骤5：匹配目标结构体名称（如：ApiGroup、RouterGroup、ServiceGroup）
			if !specok || typeSpec.Name.Name != a.Type.Group() {
				continue
			}

			// 步骤6：确认是结构体类型
			structType, structTypeOK := typeSpec.Type.(*ast.StructType)
			if !structTypeOK {
				continue
			}

			// 步骤7：检查字段是否已存在，实现幂等性
			for _, field := range structType.Fields.List {
				// 检查字段名是否匹配（只检查有名称的字段，忽略嵌入字段）
				if len(field.Names) == 1 && field.Names[0].Name == a.StructName {
					// 字段已存在，无需重复注入，直接返回
					return true
				}
			}

			// 步骤8：创建新字段节点
			field := &ast.Field{
				Names: []*ast.Ident{{Name: a.StructName}}, // 字段名（如："UserApi"）
				Type: &ast.SelectorExpr{ // 字段类型：包.类型（如："userApi.ApiGroup"）
					X:   &ast.Ident{Name: a.PackageName},       // 包名（如："userApi"）
					Sel: &ast.Ident{Name: a.PackageStructName}, // 类型名（如："ApiGroup"）
				},
			}
			// 步骤9：将新字段添加到结构体中
			structType.Fields.List = append(structType.Fields.List, field)
			// 步骤10：找到目标并成功注入，提前返回，无需继续遍历
			return false
		}

		// 继续遍历其他节点
		return true
	})
	return nil
}

// Format 将修改后的 AST 格式化并写入文件或 Writer
//
// 为什么委托给 Base.Format？
//  1. 代码复用：Base.Format 已经实现了完善的格式化逻辑
//     包括文件打开、代码格式化（go/format）、错误处理等
//  2. 统一性：所有 AST 操作器都使用相同的格式化逻辑
//     确保生成的代码风格一致，符合 Go 代码规范
//  3. 维护性：格式化逻辑的改进只需要在 Base 中修改一次
//     所有子类都会自动受益
//
// 路径处理说明：
// - 如果 filename 为空，使用结构体的 Path 字段
// - 确保格式化时使用正确的文件路径
//
// 设计好处：
// - 代码复用：避免重复实现格式化逻辑
// - 统一风格：所有生成的代码都符合 gofmt 标准
// - 易于维护：格式化逻辑集中管理，便于改进和调试
func (a *PackageEnter) Format(filename string, writer io.Writer, file *ast.File) error {
	// 如果未提供文件名，使用结构体的 Path 字段
	if filename == "" {
		filename = a.Path
	}
	// 委托给 Base.Format，实现代码复用和统一格式化
	return a.Base.Format(filename, writer, file)
}
