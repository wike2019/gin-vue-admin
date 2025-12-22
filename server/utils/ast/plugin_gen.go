package ast

import (
	"go/ast"
	"go/token"
	"io"
)

// PluginGen 用于在插件生成过程中操作 AST，主要功能是向 gorm.io/gen 的 ApplyBasic() 方法调用中
// 注入插件相关的模型参数。这是插件系统自动生成代码的核心组件。
//
// 设计目的：
//   - 自动化插件代码生成，避免手动修改 gen.go 文件
//   - 支持插件模型的动态注册到 GORM Gen 生成器
//   - 提供代码注入和回滚能力，支持插件的动态添加和移除
//
// 工作原理：
//
//	通过解析 AST 树，找到 ApplyBasic() 调用，然后向其参数列表中添加插件模型。
//	支持两种注入方式：new(PackageName.StructName) 和 &PackageName.StructName{}
type PluginGen struct {
	Base
	Type         Type   // 类型
	Path         string // 文件路径（绝对路径）
	ImportPath   string // 导包路径，用于自动添加 import 语句
	RelativePath string // 相对路径，用于路径转换
	StructName   string // 结构体名称，要注入的插件模型结构体名
	PackageName  string // 包名，插件所在的包名
	IsNew        bool   // 是否使用 new 关键字创建实例（true: new(Pkg.Struct), false: &Pkg.Struct{}）
}

// Parse 解析 Go 源文件并返回 AST 树
//
// 路径处理逻辑：
//   - 如果 filename 为空，根据 RelativePath 和 Path 的优先级自动确定文件路径
//   - 优先使用 RelativePath（如果存在），否则使用 Path
//   - 自动维护 RelativePath 和 Path 的同步，确保路径一致性
//
// 好处：
//   - 灵活性：支持绝对路径和相对路径两种方式
//   - 容错性：自动处理路径转换，减少调用方的路径处理负担
//   - 一致性：确保路径信息在结构体中保持同步
func (a *PluginGen) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	if filename == "" {
		if a.RelativePath == "" {
			// 如果没有相对路径，使用绝对路径，并计算相对路径
			filename = a.Path
			a.RelativePath = a.Base.RelativePath(a.Path)
			return a.Base.Parse(filename, writer)
		}
		// 如果有相对路径，将其转换为绝对路径
		a.Path = a.Base.AbsolutePath(a.RelativePath)
		filename = a.Path
	}
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作，移除之前通过 Injection 注入的代码
//
// 功能说明：
//
//	遍历 AST 树，找到所有 ApplyBasic() 调用，移除其中匹配的插件模型参数。
//	支持两种格式的回滚：
//	1. new(PackageName.StructName) 格式
//	2. &PackageName.StructName{} 格式（CompositeLit）
//
// 为什么需要深度嵌套的类型断言：
//   - Go AST 是树形结构，每个节点类型不同，必须通过类型断言才能访问具体字段
//   - 类型断言失败时返回 false，通过 continue 跳过不匹配的节点
//   - 这种写法虽然嵌套深，但保证了类型安全，避免了运行时 panic
//
// 遍历路径：
//
//	file.Decls (函数声明) -> FuncDecl.Body.List (函数体语句) ->
//	ExprStmt.X (表达式语句) -> CallExpr (函数调用) ->
//	SelectorExpr (选择器表达式，如 g.ApplyBasic) ->
//	CallExpr.Args (参数列表) -> 检查每个参数的类型和内容
//
// 好处：
//   - 类型安全：每一步都进行类型断言，避免访问不存在的字段
//   - 精确匹配：通过多层检查确保只移除正确的代码
//   - 幂等性：多次调用不会出错，已移除的代码不会重复移除
//
// 使用示例：
//
//	示例 1：移除 new() 格式的插件模型
//
//	// 处理前的代码：
//	package initialize
//
//	import (
//		"gorm.io/gen"
//		plugin "github.com/example/plugin"
//	)
//
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(plugin.UserModel),
//			new(plugin.OrderModel),
//		)
//	}
//
//	// 调用 Rollback 移除 plugin.UserModel：
//	pg := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "UserModel",
//		ImportPath:  "github.com/example/plugin",
//	}
//	file, _ := pg.Parse("", nil)
//	pg.Rollback(file)
//
//	// 处理后的代码：
//	package initialize
//
//	import (
//		"gorm.io/gen"
//		plugin "github.com/example/plugin"
//	)
//
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(plugin.OrderModel),
//		)
//	}
//
//	示例 2：移除 &Struct{} 格式的插件模型
//
//	// 处理前的代码：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			&plugin.UserModel{},
//			&plugin.OrderModel{},
//		)
//	}
//
//	// 调用 Rollback 移除 plugin.UserModel：
//	pg := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "UserModel",
//		ImportPath:  "github.com/example/plugin",
//	}
//	file, _ := pg.Parse("", nil)
//	pg.Rollback(file)
//
//	// 处理后的代码：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			&plugin.OrderModel{},
//		)
//	}
//
//	示例 3：移除最后一个参数时，同时移除 import
//
//	// 处理前的代码：
//	package initialize
//
//	import (
//		"gorm.io/gen"
//		plugin "github.com/example/plugin"  // 这个 import 会被移除
//	)
//
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(plugin.UserModel),
//		)
//	}
//
//	// 调用 Rollback 移除 plugin.UserModel：
//	pg := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "UserModel",
//		ImportPath:  "github.com/example/plugin",
//	}
//	file, _ := pg.Parse("", nil)
//	pg.Rollback(file)
//
//	// 处理后的代码：
//	package initialize
//
//	import (
//		"gorm.io/gen"
//		// plugin import 已被移除
//	)
//
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic()  // 参数列表为空
//	}
//
//	示例 4：完整的使用流程
//
//	// 1. 创建 PluginGen 实例
//	pg := &PluginGen{
//		Base:        Base{},
//		Path:        "/path/to/initialize/gen.go",
//		ImportPath:  "github.com/example/myplugin",
//		PackageName: "myplugin",
//		StructName:  "MyModel",
//	}
//
//	// 2. 解析文件
//	file, err := pg.Parse("", nil)
//	if err != nil {
//		return err
//	}
//
//	// 3. 执行回滚操作
//	err = pg.Rollback(file)
//	if err != nil {
//		return err
//	}
//
//	// 4. 格式化并写回文件
//	var buf bytes.Buffer
//	err = pg.Format("", &buf, file)
//	if err != nil {
//		return err
//	}
//
//	// 5. 写入文件
//	return os.WriteFile(pg.Path, buf.Bytes(), 0644)
func (a *PluginGen) Rollback(file *ast.File) error {
	// 遍历文件中的所有声明（函数、变量、类型等）
	for i := 0; i < len(file.Decls); i++ {
		// 类型断言：检查是否为函数声明
		// 使用类型断言而非类型 switch 的原因：只需要处理函数声明，其他类型直接跳过
		v1, o1 := file.Decls[i].(*ast.FuncDecl)
		if o1 {
			// 遍历函数体中的所有语句
			for j := 0; j < len(v1.Body.List); j++ {
				// 类型断言：检查是否为表达式语句（如函数调用）
				v2, o2 := v1.Body.List[j].(*ast.ExprStmt)
				if o2 {
					// 类型断言：检查表达式是否为函数调用
					v3, o3 := v2.X.(*ast.CallExpr)
					if o3 {
						// 类型断言：检查是否为选择器表达式（如 g.ApplyBasic）
						v4, o4 := v3.Fun.(*ast.SelectorExpr)
						if o4 {
							// 只处理 ApplyBasic 方法调用，这是 GORM Gen 的模型注册方法
							if v4.Sel.Name != "ApplyBasic" {
								continue
							}
							// 遍历 ApplyBasic 的所有参数，查找要移除的插件模型
							for k := 0; k < len(v3.Args); k++ {
								// 检查参数是否为 new() 调用格式：new(PackageName.StructName)
								v5, o5 := v3.Args[k].(*ast.CallExpr)
								if o5 {
									// 检查是否为 new 关键字调用
									v6, o6 := v5.Fun.(*ast.Ident)
									if o6 {
										if v6.Name != "new" {
											continue
										}
										// 检查 new() 的参数是否为 PackageName.StructName
										for l := 0; l < len(v5.Args); l++ {
											v7, o7 := v5.Args[l].(*ast.SelectorExpr)
											if o7 {
												v8, o8 := v7.X.(*ast.Ident)
												if o8 {
													// 匹配到目标插件模型，从参数列表中移除
													if v8.Name == a.PackageName && v7.Sel.Name == a.StructName {
														// 使用切片操作移除元素：保留 k 之前的元素和 k+1 之后的元素
														v3.Args = append(v3.Args[:k], v3.Args[k+1:]...)
														continue
													}
												}
											}
										}
									}
								}
								// 检查索引是否越界（因为上面可能已经移除了元素）
								if k >= len(v3.Args) {
									break
								}
								// 检查参数是否为复合字面量格式：&PackageName.StructName{}
								v6, o6 := v3.Args[k].(*ast.CompositeLit)
								if o6 {
									// 检查复合字面量的类型是否为 PackageName.StructName
									v7, o7 := v6.Type.(*ast.SelectorExpr)
									if o7 {
										v8, o8 := v7.X.(*ast.Ident)
										if o8 {
											// 匹配到目标插件模型，从参数列表中移除
											if v8.Name == a.PackageName && v7.Sel.Name == a.StructName {
												v3.Args = append(v3.Args[:k], v3.Args[k+1:]...)
												continue
											}
										}
									}
								}
							}
							// 如果所有参数都被移除了，同时移除对应的 import 语句
							// 这样可以保持代码的整洁，避免无用的 import
							if len(v3.Args) == 0 {
								_ = NewImport(a.ImportPath).Rollback(file)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// Injection 注入代码，向 ApplyBasic() 调用中添加插件模型参数
//
// 功能说明：
//  1. 首先注入 import 语句（如果不存在）
//  2. 查找所有 ApplyBasic() 调用
//  3. 检查是否已存在该插件模型的参数（避免重复注入）
//  4. 如果不存在，根据 IsNew 标志选择注入方式：
//     - true: 注入 new(PackageName.StructName)
//     - false: 注入 &PackageName.StructName{}
//
// 为什么先检查是否存在：
//   - 幂等性保证：多次调用不会重复注入
//   - 避免代码重复：确保每个插件模型只注册一次
//   - 支持增量更新：可以安全地多次调用而不破坏代码结构
//
// 为什么使用深度嵌套的类型断言：
//   - AST 节点类型是接口类型，必须通过类型断言才能访问具体字段
//   - 每一步类型断言都检查是否成功，失败则跳过
//   - 这种方式虽然代码冗长，但保证了类型安全和代码的健壮性
//
// 好处：
//   - 自动化：无需手动修改 gen.go 文件
//   - 安全性：类型断言确保不会访问错误的节点类型
//   - 灵活性：支持两种注入格式，适应不同的代码风格
//
// 使用示例：
//
//	示例 1：使用 new() 格式注入插件模型
//
//	// 处理前的代码：
//	package initialize
//
//	import (
//		"gorm.io/gen"
//	)
//
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(system.SysUser),
//		)
//	}
//
//	// 调用 Injection 注入 plugin.UserModel：
//	pg := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "UserModel",
//		ImportPath:  "github.com/example/plugin",
//		IsNew:       true,  // 使用 new() 格式
//	}
//	file, _ := pg.Parse("", nil)
//	pg.Injection(file)
//
//	// 处理后的代码：
//	package initialize
//
//	import (
//		"gorm.io/gen"
//		plugin "github.com/example/plugin"  // 自动添加的 import
//	)
//
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(system.SysUser),
//			new(plugin.UserModel),  // 新注入的插件模型
//		)
//	}
//
//	示例 2：使用复合字面量格式注入插件模型
//
//	// 处理前的代码：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			&system.SysUser{},
//		)
//	}
//
//	// 调用 Injection 注入 plugin.OrderModel：
//	pg := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "OrderModel",
//		ImportPath:  "github.com/example/plugin",
//		IsNew:       false,  // 使用 &Struct{} 格式
//	}
//	file, _ := pg.Parse("", nil)
//	pg.Injection(file)
//
//	// 处理后的代码：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			&system.SysUser{},
//			&plugin.OrderModel{},  // 新注入的插件模型
//		)
//	}
//
//	示例 3：已存在时不重复注入（幂等性保证）
//
//	// 处理前的代码：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(plugin.UserModel),  // 已存在
//			new(plugin.OrderModel),
//		)
//	}
//
//	// 再次调用 Injection 注入 plugin.UserModel：
//	pg := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "UserModel",
//		ImportPath:  "github.com/example/plugin",
//		IsNew:       true,
//	}
//	file, _ := pg.Parse("", nil)
//	pg.Injection(file)  // 不会重复注入
//
//	// 处理后的代码（保持不变）：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(plugin.UserModel),  // 保持不变，不会重复
//			new(plugin.OrderModel),
//		)
//	}
//
//	示例 4：混合格式的注入（同时支持 new() 和 &Struct{}）
//
//	// 处理前的代码：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(system.SysUser),
//			&system.SysRole{},
//		)
//	}
//
//	// 注入 new() 格式的插件模型：
//	pg1 := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "UserModel",
//		ImportPath:  "github.com/example/plugin",
//		IsNew:       true,
//	}
//	file, _ := pg1.Parse("", nil)
//	pg1.Injection(file)
//
//	// 注入 &Struct{} 格式的插件模型：
//	pg2 := &PluginGen{
//		PackageName: "plugin",
//		StructName:  "OrderModel",
//		ImportPath:  "github.com/example/plugin",
//		IsNew:       false,
//	}
//	pg2.Injection(file)
//
//	// 处理后的代码：
//	func init() {
//		g := gen.NewGenerator(gen.Config{})
//		g.ApplyBasic(
//			new(system.SysUser),
//			&system.SysRole{},
//			new(plugin.UserModel),   // new() 格式
//			&plugin.OrderModel{},    // &Struct{} 格式
//		)
//	}
//
//	示例 5：完整的使用流程
//
//	// 1. 创建 PluginGen 实例
//	pg := &PluginGen{
//		Base:        Base{},
//		Path:        "/path/to/initialize/gen.go",
//		ImportPath:  "github.com/example/myplugin",
//		PackageName: "myplugin",
//		StructName:  "MyModel",
//		IsNew:       true,  // 选择注入格式
//	}
//
//	// 2. 解析文件
//	file, err := pg.Parse("", nil)
//	if err != nil {
//		return err
//	}
//
//	// 3. 执行注入操作
//	err = pg.Injection(file)
//	if err != nil {
//		return err
//	}
//
//	// 4. 格式化并写回文件
//	var buf bytes.Buffer
//	err = pg.Format("", &buf, file)
//	if err != nil {
//		return err
//	}
//
//	// 5. 写入文件
//	return os.WriteFile(pg.Path, buf.Bytes(), 0644)
func (a *PluginGen) Injection(file *ast.File) error {
	// 首先注入 import 语句，确保插件包可以被正确导入
	// 忽略返回值是因为 Import.Injection 内部已经处理了重复导入的情况
	_ = NewImport(a.ImportPath).Injection(file)

	// 遍历文件中的所有声明
	for i := 0; i < len(file.Decls); i++ {
		// 类型断言：只处理函数声明
		v1, o1 := file.Decls[i].(*ast.FuncDecl)
		if o1 {
			// 遍历函数体中的所有语句
			for j := 0; j < len(v1.Body.List); j++ {
				// 类型断言：检查是否为表达式语句
				v2, o2 := v1.Body.List[j].(*ast.ExprStmt)
				if o2 {
					// 类型断言：检查是否为函数调用
					v3, o3 := v2.X.(*ast.CallExpr)
					if o3 {
						// 类型断言：检查是否为选择器表达式（方法调用）
						v4, o4 := v3.Fun.(*ast.SelectorExpr)
						if o4 {
							// 只处理 ApplyBasic 方法调用
							if v4.Sel.Name != "ApplyBasic" {
								continue
							}
							// 标记是否已存在该插件模型的参数
							var has bool
							// 遍历所有参数，检查是否已存在
							for k := 0; k < len(v3.Args); k++ {
								// 检查参数是否为 new() 调用格式
								v5, o5 := v3.Args[k].(*ast.CallExpr)
								if o5 {
									v6, o6 := v5.Fun.(*ast.Ident)
									if o6 {
										if v6.Name != "new" {
											continue
										}
										// 检查 new() 的参数是否匹配目标插件模型
										for l := 0; l < len(v5.Args); l++ {
											v7, o7 := v5.Args[l].(*ast.SelectorExpr)
											if o7 {
												v8, o8 := v7.X.(*ast.Ident)
												if o8 {
													// 找到匹配的插件模型，标记为已存在
													if v8.Name == a.PackageName && v7.Sel.Name == a.StructName {
														has = true
														break
													}
												}
											}
										}
									}
								}
								// 检查参数是否为复合字面量格式
								v6, o6 := v3.Args[k].(*ast.CompositeLit)
								if o6 {
									v7, o7 := v6.Type.(*ast.SelectorExpr)
									if o7 {
										v8, o8 := v7.X.(*ast.Ident)
										if o8 {
											// 找到匹配的插件模型，标记为已存在
											if v8.Name == a.PackageName && v7.Sel.Name == a.StructName {
												has = true
												break
											}
										}
									}
								}
							}
							// 如果不存在，则注入新的参数
							if !has {
								if a.IsNew {
									// 使用 new 关键字创建实例
									// Name 中包含换行和制表符是为了格式化输出时的美观
									arg := &ast.CallExpr{
										Fun: &ast.Ident{Name: "\n\t\tnew"},
										Args: []ast.Expr{
											&ast.SelectorExpr{
												X:   &ast.Ident{Name: a.PackageName},
												Sel: &ast.Ident{Name: a.StructName},
											},
										},
									}
									v3.Args = append(v3.Args, arg)
									// 添加换行符，保持代码格式美观
									v3.Args = append(v3.Args, &ast.BasicLit{
										Kind:  token.STRING,
										Value: "\n",
									})
									break
								}
								// 使用复合字面量创建实例：&PackageName.StructName{}
								arg := &ast.CompositeLit{
									Type: &ast.SelectorExpr{
										X:   &ast.Ident{Name: a.PackageName},
										Sel: &ast.Ident{Name: a.StructName},
									},
								}
								v3.Args = append(v3.Args, arg)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// Format 将修改后的 AST 格式化并写回文件
//
// 功能说明：
//   - 如果 filename 为空，使用 Path 作为默认输出路径
//   - 委托给 Base.Format 执行实际的格式化操作
//   - Base.Format 会使用 go/format 包格式化代码，确保符合 Go 代码规范
//
// 好处：
//   - 代码复用：避免重复实现格式化逻辑
//   - 自动格式化：生成的代码自动符合 Go 代码规范
//   - 一致性：所有 AST 操作都使用统一的格式化方法
func (a *PluginGen) Format(filename string, writer io.Writer, file *ast.File) error {
	if filename == "" {
		filename = a.Path
	}
	return a.Base.Format(filename, writer, file)
}
