// Package ast 提供 Go 语言抽象语法树（AST）操作的实用工具函数
// 这些函数主要用于代码生成、代码分析和代码修改场景
// 通过 AST 操作可以避免字符串拼接生成代码，保证生成代码的语法正确性和可维护性
package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"

	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

// AddImport 向 AST 节点中添加 import 声明
// 参数:
//   - astNode: 要操作的 AST 节点（通常是 *ast.File）
//   - imp: 要添加的 import 路径（不含引号）
//
// 实现原理:
//
//	使用 ast.Inspect 遍历 AST 树，查找类型为 token.IMPORT 的 GenDecl 节点
//	在添加前检查是否已存在相同的 import，避免重复导入
//
// 为什么这样写:
//  1. 使用 ast.Inspect 可以安全地遍历整个 AST 树，无需手动处理节点关系
//  2. 先检查再添加，避免重复导入导致的编译错误
//  3. 直接操作 AST 节点，比字符串操作更可靠，不会破坏代码格式
//
// 好处:
//   - 类型安全：编译时检查，避免字符串拼接错误
//   - 自动格式化：生成的代码符合 Go 语法规范
//   - 避免重复：自动检测已存在的 import
func AddImport(astNode ast.Node, imp string) {
	// 将 import 路径格式化为带引号的字符串，符合 Go 语法要求
	impStr := fmt.Sprintf("\"%s\"", imp)
	// 使用 ast.Inspect 深度优先遍历 AST 树
	// 返回 false 可以提前终止遍历，提高效率
	ast.Inspect(astNode, func(node ast.Node) bool {
		// 类型断言：检查是否为通用声明节点（GenDecl）
		// GenDecl 可以表示 import、const、var、type 等声明
		if genDecl, ok := node.(*ast.GenDecl); ok {
			// 检查是否为 import 声明
			if genDecl.Tok == token.IMPORT {
				// 遍历已有的 import 声明，检查是否已存在
				for i := range genDecl.Specs {
					if impNode, ok := genDecl.Specs[i].(*ast.ImportSpec); ok {
						// 如果已存在相同的 import，直接返回 false 终止遍历
						// 这样可以避免重复添加，同时提高效率
						if impNode.Path.Value == impStr {
							return false
						}
					}
				}
				// 不存在重复，添加新的 import 声明
				// 使用 ast.ImportSpec 构造 import 节点，保证语法正确性
				genDecl.Specs = append(genDecl.Specs, &ast.ImportSpec{
					Path: &ast.BasicLit{
						Kind:  token.STRING, // 标记为字符串字面量
						Value: impStr,       // import 路径值
					},
				})
			}
		}
		// 继续遍历子节点
		return true
	})
}

// FindFunction 在 AST 树中查找指定名称的函数声明
// 参数:
//   - astNode: 要搜索的 AST 节点
//   - FunctionName: 要查找的函数名称
//
// 返回:
//   - *ast.FuncDecl: 找到的函数声明节点，未找到返回 nil
//
// 实现原理:
//
//	使用 ast.Inspect 遍历 AST，查找所有 FuncDecl 节点并匹配函数名
//	找到后立即返回 false 终止遍历，提高查找效率
//
// 为什么这样写:
//  1. 使用 ast.Inspect 可以遍历整个 AST 树，包括嵌套的函数
//  2. 找到目标后立即终止遍历（return false），避免不必要的遍历
//  3. 返回指针类型，方便后续修改函数体等操作
//
// 好处:
//   - 高效：找到即停止，不继续遍历
//   - 通用：可以查找任何层级的函数声明
//   - 类型安全：返回标准 AST 节点，可以直接操作
func FindFunction(astNode ast.Node, FunctionName string) *ast.FuncDecl {
	var funcDeclP *ast.FuncDecl
	// 遍历 AST 树查找函数声明
	ast.Inspect(astNode, func(node ast.Node) bool {
		// 类型断言：检查是否为函数声明节点
		if funcDecl, ok := node.(*ast.FuncDecl); ok {
			// 比较函数名，使用 String() 方法获取标识符名称
			if funcDecl.Name.String() == FunctionName {
				// 找到目标函数，保存引用并终止遍历
				funcDeclP = funcDecl
				return false // 返回 false 停止继续遍历，提高效率
			}
		}
		// 继续遍历子节点
		return true
	})
	return funcDeclP
}

// FindArray 查找特定类型的数组字面量（CompositeLit）
// 参数:
//   - astNode: 要搜索的 AST 节点
//   - identName: 数组元素类型的包名（如 "model"）
//   - selectorExprName: 数组元素类型的类型名（如 "SysBaseMenu"）
//
// 返回:
//   - *ast.CompositeLit: 找到的数组字面量节点，未找到返回 nil
//
// 实现原理:
//
//	查找赋值语句中右侧的数组字面量，匹配数组元素类型为指定包.类型名的数组
//	例如：查找 `var menus = []model.SysBaseMenu{...}` 这样的数组
//
// 为什么这样写:
//  1. 使用 switch 语句进行类型断言，代码更清晰
//  2. 逐层检查 AST 节点类型：AssignStmt -> CompositeLit -> ArrayType -> SelectorExpr
//  3. 同时检查包名和类型名，精确定位目标数组
//  4. 找到后立即终止遍历，提高效率
//
// 好处:
//   - 精确匹配：通过包名和类型名双重匹配，避免误匹配
//   - 类型安全：通过 AST 节点类型检查，比字符串匹配更可靠
//   - 可扩展：可以轻松修改匹配条件，查找其他类型的数组
//
// 使用示例:
//
//	示例 1: 查找 []model.SysBaseMenu 类型的数组
//	```go
//	// 假设有以下代码:
//	// var menus = []model.SysBaseMenu{
//	//     {Path: "/dashboard", Name: "Dashboard"},
//	//     {Path: "/user", Name: "User"},
//	// }
//
//	src := `
//	package main
//	import "model"
//	func init() {
//		var menus = []model.SysBaseMenu{
//			{Path: "/dashboard", Name: "Dashboard"},
//		}
//	}
//	`
//	fset := token.NewFileSet()
//	file, _ := parser.ParseFile(fset, "", src, parser.ParseComments)
//
//	// 查找 []model.SysBaseMenu 类型的数组
//	arrayNode := FindArray(file, "model", "SysBaseMenu")
//	if arrayNode != nil {
//		// 找到了数组，可以对 arrayNode 进行操作
//		// 例如：添加新元素、修改现有元素等
//	}
//	```
//
//	示例 2: 查找 []system.SysApi 类型的数组
//	```go
//	// 查找 []system.SysApi 类型的数组
//	apiArray := FindArray(astNode, "system", "SysApi")
//	if apiArray != nil {
//		// 找到了 API 数组，可以进行后续操作
//		fmt.Printf("找到 %d 个 API 元素\n", len(apiArray.Elts))
//	}
//	```
//
//	示例 3: 在函数体中查找数组
//	```go
//	// 假设要在一个函数中查找数组
//	funcDecl := FindFunction(file, "InitMenus")
//	if funcDecl != nil && funcDecl.Body != nil {
//		// 在函数体中查找数组
//		menuArray := FindArray(funcDecl.Body, "model", "SysBaseMenu")
//		if menuArray != nil {
//			// 找到了数组，可以修改或添加元素
//		}
//	}
//	```
//
//	示例 4: 查找自定义类型的数组
//	```go
//	// 查找 []custom.User 类型的数组
//	userArray := FindArray(astNode, "custom", "User")
//	if userArray != nil {
//		// 找到了用户数组
//		for _, elem := range userArray.Elts {
//			// 处理每个用户元素
//		}
//	}
//	```
func FindArray(astNode ast.Node, identName, selectorExprName string) *ast.CompositeLit {
	var assignStmt *ast.CompositeLit
	ast.Inspect(astNode, func(n ast.Node) bool {
		// 使用 switch 进行类型断言，代码更清晰易读
		switch node := n.(type) {
		case *ast.AssignStmt:
			// 遍历赋值语句右侧的所有表达式（支持多重赋值）
			for _, expr := range node.Rhs {
				// 检查是否为复合字面量（结构体、数组、切片等的字面量）
				if exprType, ok := expr.(*ast.CompositeLit); ok {
					// 检查复合字面量的类型是否为数组类型
					if arrayType, ok := exprType.Type.(*ast.ArrayType); ok {
						// 检查数组元素类型是否为选择器表达式（如 model.SysBaseMenu）
						sel, ok1 := arrayType.Elt.(*ast.SelectorExpr)
						// 检查选择器表达式的 X 部分是否为标识符（包名）
						x, ok2 := sel.X.(*ast.Ident)
						// 同时匹配包名和类型名，确保精确找到目标数组
						// 例如：查找 []model.SysBaseMenu 类型的数组
						if ok1 && ok2 && x.Name == identName && sel.Sel.Name == selectorExprName {
							assignStmt = exprType
							return false // 找到后立即终止遍历
						}
					}
				}
			}
		}
		return true // 继续遍历其他节点
	})
	return assignStmt
}

// CreateMenuStructAst 将菜单数据转换为 AST 表达式节点数组
// 参数:
//   - menus: 菜单数据切片
//
// 返回:
//   - *[]ast.Expr: 菜单结构体字面量的 AST 表达式数组
//
// 实现原理:
//
//	将数据库中的菜单数据转换为 Go 代码中的结构体字面量 AST 节点
//	支持嵌套结构（Meta、Parameters、MenuBtn）和可选字段
//
// 为什么这样写:
//  1. 使用 AST 节点构造代码，而不是字符串拼接，保证语法正确性
//  2. 使用 KeyValueExpr 显式指定字段名，代码可读性更好
//  3. 条件添加可选字段（Parameters、MenuBtn），避免生成空数组
//  4. 嵌套结构使用 CompositeLit 构造，保持 AST 树结构完整
//  5. 返回指针类型，方便调用方判断 nil 或直接使用
//
// 好处:
//   - 类型安全：编译时检查，避免字段名拼写错误
//   - 格式规范：生成的代码符合 Go 代码格式规范
//   - 可维护：结构清晰，易于理解和修改
//   - 灵活性：支持可选字段，只生成有值的字段
//
// 使用示例:
//
//	示例 1: 基本菜单转换（无参数和按钮）
//	```go
//	// 准备菜单数据
//	menus := []system.SysBaseMenu{
//		{
//			Path:      "/dashboard",
//			Name:      "Dashboard",
//			Component: "view/dashboard/index.vue",
//			Sort:      1,
//			Title:     "仪表盘",
//			Icon:      "odometer",
//			// Parameters 和 MenuBtn 为空，不会生成这些字段
//		},
//	}
//
//	// 转换为 AST 节点
//	menuAsts := CreateMenuStructAst(menus)
//	// 生成的 AST 节点可以用于代码生成，例如：
//	// {
//	//     ParentId: 0,
//	//     Path: "/dashboard",
//	//     Name: "Dashboard",
//	//     Hidden: false,
//	//     Component: "view/dashboard/index.vue",
//	//     Sort: 1,
//	//     Meta: model.Meta{
//	//         Title: "仪表盘",
//	//         Icon: "odometer",
//	//     },
//	// }
//	```
//
//	示例 2: 带参数的菜单转换
//	```go
//	menus := []system.SysBaseMenu{
//		{
//			Path:      "/user/:id",
//			Name:      "UserDetail",
//			Component: "view/user/detail.vue",
//			Sort:      2,
//			Title:     "用户详情",
//			Icon:      "user",
//			Parameters: []system.SysBaseMenuParameter{
//				{
//					Type:  "path",
//					Key:   "id",
//					Value: "用户ID",
//				},
//			},
//		},
//	}
//
//	menuAsts := CreateMenuStructAst(menus)
//	// 生成的 AST 节点会包含 Parameters 字段：
//	// {
//	//     ...
//	//     Parameters: []model.SysBaseMenuParameter{
//	//         {
//	//             Type: "path",
//	//             Key: "id",
//	//             Value: "用户ID",
//	//         },
//	//     },
//	// }
//	```
//
//	示例 3: 带按钮的菜单转换
//	```go
//	menus := []system.SysBaseMenu{
//		{
//			Path:      "/user",
//			Name:      "User",
//			Component: "view/user/index.vue",
//			Sort:      3,
//			Title:     "用户管理",
//			Icon:      "user",
//			MenuBtn: []system.SysBaseMenuBtn{
//				{
//					Name: "add",
//					Desc: "新增用户",
//				},
//				{
//					Name: "edit",
//					Desc: "编辑用户",
//				},
//			},
//		},
//	}
//
//	menuAsts := CreateMenuStructAst(menus)
//	// 生成的 AST 节点会包含 MenuBtn 字段：
//	// {
//	//     ...
//	//     MenuBtn: []model.SysBaseMenuBtn{
//	//         {
//	//             Name: "add",
//	//             Desc: "新增用户",
//	//         },
//	//         {
//	//             Name: "edit",
//	//             Desc: "编辑用户",
//	//         },
//	//     },
//	// }
//	```
//
//	示例 4: 完整菜单转换（包含所有字段）
//	```go
//	menus := []system.SysBaseMenu{
//		{
//			Path:      "/system/user/:id",
//			Name:      "SystemUser",
//			Component: "view/system/user/index.vue",
//			Sort:      10,
//			Title:     "系统用户",
//			Icon:      "setting",
//			Parameters: []system.SysBaseMenuParameter{
//				{Type: "path", Key: "id", Value: "用户ID"},
//				{Type: "query", Key: "page", Value: "页码"},
//			},
//			MenuBtn: []system.SysBaseMenuBtn{
//				{Name: "add", Desc: "新增"},
//				{Name: "edit", Desc: "编辑"},
//				{Name: "delete", Desc: "删除"},
//			},
//		},
//	}
//
//	menuAsts := CreateMenuStructAst(menus)
//	// 生成的 AST 节点包含所有字段，可以用于代码生成
//	// 例如插入到初始化函数中：
//	// func init() {
//	//     var menus = []model.SysBaseMenu{
//	//         { /* 生成的菜单结构体 */ },
//	//     }
//	// }
//	```
//
//	示例 5: 批量转换多个菜单
//	```go
//	// 从数据库查询多个菜单
//	var menus []system.SysBaseMenu
//	// ... 数据库查询逻辑 ...
//
//	// 批量转换为 AST 节点
//	menuAsts := CreateMenuStructAst(menus)
//	if menuAsts != nil {
//		// 将 AST 节点插入到代码文件中
//		// 例如使用 FindArray 找到目标数组，然后追加元素
//		arrayNode := FindArray(file, "model", "SysBaseMenu")
//		if arrayNode != nil {
//			arrayNode.Elts = append(arrayNode.Elts, *menuAsts...)
//		}
//	}
//	```
func CreateMenuStructAst(menus []system.SysBaseMenu) *[]ast.Expr {
	var menuElts []ast.Expr
	for i := range menus {
		// 构造菜单结构体的字段列表
		// 使用 KeyValueExpr 显式指定字段名和值，生成的代码更清晰
		elts := []ast.Expr{
			// ParentId 字段：使用 BasicLit 表示整数字面量
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "ParentId"},
				Value: &ast.BasicLit{Kind: token.INT, Value: "0"},
			},
			// Path 字段：字符串类型，需要加引号
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Path"},
				Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", menus[i].Path)},
			},
			// Name 字段
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Name"},
				Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", menus[i].Name)},
			},
			// Hidden 字段：布尔类型，使用 Ident 表示 false 标识符
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Hidden"},
				Value: &ast.Ident{Name: "false"},
			},
			// Component 字段
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Component"},
				Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", menus[i].Component)},
			},
			// Sort 字段：整数类型
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Sort"},
				Value: &ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", menus[i].Sort)},
			},
			// Meta 字段：嵌套结构体，使用 CompositeLit 构造
			// SelectorExpr 表示 model.Meta 类型
			&ast.KeyValueExpr{
				Key: &ast.Ident{Name: "Meta"},
				Value: &ast.CompositeLit{
					Type: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "model"}, // 包名
						Sel: &ast.Ident{Name: "Meta"},  // 类型名
					},
					Elts: []ast.Expr{
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Title"},
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", menus[i].Title)},
						},
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Icon"},
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", menus[i].Icon)},
						},
					},
				},
			},
		}

		// 条件添加菜单参数（可选字段）
		// 只有当参数列表不为空时才添加，避免生成空的数组字段
		if len(menus[i].Parameters) > 0 {
			var paramElts []ast.Expr
			// 遍历每个参数，构造参数结构体字面量
			for _, param := range menus[i].Parameters {
				paramElts = append(paramElts, &ast.CompositeLit{
					Type: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "model"},
						Sel: &ast.Ident{Name: "SysBaseMenuParameter"},
					},
					Elts: []ast.Expr{
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Type"},
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", param.Type)},
						},
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Key"},
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", param.Key)},
						},
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Value"},
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", param.Value)},
						},
					},
				})
			}
			// 将参数数组添加到字段列表
			elts = append(elts, &ast.KeyValueExpr{
				Key: &ast.Ident{Name: "Parameters"},
				Value: &ast.CompositeLit{
					Type: &ast.ArrayType{
						Elt: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "model"},
							Sel: &ast.Ident{Name: "SysBaseMenuParameter"},
						},
					},
					Elts: paramElts,
				},
			})
		}

		// 条件添加菜单按钮（可选字段）
		// 同样的逻辑，只在有按钮数据时添加
		if len(menus[i].MenuBtn) > 0 {
			var btnElts []ast.Expr
			for _, btn := range menus[i].MenuBtn {
				btnElts = append(btnElts, &ast.CompositeLit{
					Type: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "model"},
						Sel: &ast.Ident{Name: "SysBaseMenuBtn"},
					},
					Elts: []ast.Expr{
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Name"},
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", btn.Name)},
						},
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Desc"},
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", btn.Desc)},
						},
					},
				})
			}
			elts = append(elts, &ast.KeyValueExpr{
				Key: &ast.Ident{Name: "MenuBtn"},
				Value: &ast.CompositeLit{
					Type: &ast.ArrayType{
						Elt: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "model"},
							Sel: &ast.Ident{Name: "SysBaseMenuBtn"},
						},
					},
					Elts: btnElts,
				},
			})
		}

		// 将构造好的字段列表包装成结构体字面量
		// Type 为 nil 表示使用类型推断，编译器会自动推断类型
		menuElts = append(menuElts, &ast.CompositeLit{
			Type: nil, // nil 表示类型推断
			Elts: elts,
		})
	}
	return &menuElts
}

// CreateApiStructAst 将 API 数据转换为 AST 表达式节点数组
// 参数:
//   - apis: API 数据切片
//
// 返回:
//   - *[]ast.Expr: API 结构体字面量的 AST 表达式数组
//
// 实现原理:
//
//	将数据库中的 API 数据转换为 Go 代码中的结构体字面量 AST 节点
//	相比 CreateMenuStructAst，API 结构更简单，没有嵌套结构
//
// 为什么这样写:
//  1. 与 CreateMenuStructAst 保持一致的实现模式，代码风格统一
//  2. 使用 AST 节点构造，保证生成的代码语法正确
//  3. 所有字段都是必需的，不需要条件判断
//  4. 使用 KeyValueExpr 显式指定字段，代码可读性好
//
// 好处:
//   - 类型安全：编译时检查字段名和类型
//   - 代码规范：生成的代码符合 Go 格式规范
//   - 易于维护：结构简单清晰，易于理解和修改
//   - 一致性：与菜单生成函数使用相同的模式
func CreateApiStructAst(apis []system.SysApi) *[]ast.Expr {
	var apiElts []ast.Expr
	for i := range apis {
		// 构造 API 结构体的字段列表
		// API 结构体相对简单，所有字段都是必需的字符串类型
		elts := []ast.Expr{
			// Path 字段：API 路径
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Path"},
				Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", apis[i].Path)},
			},
			// Description 字段：API 描述
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Description"},
				Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", apis[i].Description)},
			},
			// ApiGroup 字段：API 分组
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "ApiGroup"},
				Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", apis[i].ApiGroup)},
			},
			// Method 字段：HTTP 方法（GET、POST 等）
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "Method"},
				Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"%s\"", apis[i].Method)},
			},
		}
		// 将字段列表包装成结构体字面量
		// Type 为 nil 使用类型推断
		apiElts = append(apiElts, &ast.CompositeLit{
			Type: nil,
			Elts: elts,
		})
	}
	return &apiElts
}

// CheckImport 检查文件中是否已存在指定的 import 声明
// 参数:
//   - file: AST 文件节点
//   - importPath: 要检查的 import 路径（不含引号）
//
// 返回:
//   - bool: 存在返回 true，不存在返回 false
//
// 实现原理:
//
//	遍历文件的所有 import 声明，去除引号后与目标路径比较
//	直接访问 file.Imports 比使用 ast.Inspect 更高效
//
// 为什么这样写:
//  1. 直接访问 file.Imports 切片，比遍历整个 AST 树更高效
//  2. 使用切片操作去除引号（[1:len-1]），简单直接
//  3. 找到匹配项立即返回 true，提高效率
//  4. 使用 *ast.File 作为参数，类型更精确，避免类型断言
//
// 好处:
//   - 高效：直接访问 import 列表，无需遍历整个 AST
//   - 简单：代码逻辑清晰，易于理解
//   - 精确：专门用于检查 import，职责单一
func CheckImport(file *ast.File, importPath string) bool {
	// 遍历文件中的所有 import 声明
	// file.Imports 是预解析好的 import 列表，直接访问更高效
	for _, imp := range file.Imports {
		// 去除 import 路径两端的引号
		// imp.Path.Value 格式为 "\"path/to/package\""，需要去掉首尾的引号
		// 使用切片操作 [1:len-1] 去除第一个和最后一个字符（引号）
		path := imp.Path.Value[1 : len(imp.Path.Value)-1]

		// 比较去除引号后的路径与目标路径
		if path == importPath {
			return true // 找到匹配的 import，立即返回
		}
	}

	return false // 未找到匹配的 import
}

// clearPosition 清除 AST 节点中的所有位置信息
// 参数:
//   - astNode: 要清除位置信息的 AST 节点
//
// 实现原理:
//
//	遍历 AST 树，将所有节点的位置字段设置为 token.NoPos
//	位置信息用于代码格式化，清除后可以让代码格式化工具重新计算位置
//
// 为什么这样写:
//  1. 使用 ast.Inspect 遍历所有节点，确保不遗漏任何位置信息
//  2. 使用 switch 语句处理不同类型的节点，代码清晰
//  3. 将位置设置为 token.NoPos（无效位置），让格式化工具重新计算
//  4. 只处理包含位置信息的节点类型，提高效率
//
// 好处:
//   - 代码格式化：清除位置信息后，格式化工具可以重新计算最佳位置
//   - 代码生成：生成的代码位置信息正确，避免格式混乱
//   - 可维护性：统一的格式化规则，生成的代码风格一致
//
// 使用示例（清除前后对比）:
//
//	示例 1: 函数调用表达式 - 清除前后对比
//		清除前（原始代码，位置信息保留）:
//			code := `fmt.Println("hello","world")`
//			// 注意：参数之间没有空格
//
//		清除位置信息:
//			expr, _ := parser.ParseExpr(code)
//			clearPosition(expr)
//
//		清除后（格式化输出）:
//			fmt.Println("hello", "world")
//			// 格式化工具自动在参数之间添加空格
//
//	示例 2: 结构体字面量 - 清除前后对比
//		清除前（格式不统一）:
//			code := `model.SysUser{ID:1,Username:"admin",Password:"123456"}`
//			// 字段之间没有空格，格式混乱
//
//		清除位置信息:
//			expr, _ := parser.ParseExpr(code)
//			clearPosition(expr)
//
//		清除后（格式化输出）:
//			model.SysUser{
//				ID:       1,
//				Username: "admin",
//				Password: "123456",
//			}
//			// 格式化工具自动对齐字段，添加换行和缩进
//
//	示例 3: 选择器表达式 - 清除前后对比
//		清除前:
//			code := `model.SysBaseMenu.ID`
//			// 位置信息保留，格式化时可能保持原样
//
//		清除位置信息:
//			expr, _ := parser.ParseExpr(code)
//			clearPosition(expr)
//
//		清除后（格式化输出）:
//			model.SysBaseMenu.ID
//			// 格式化工具重新计算点号位置，确保格式规范
//
//	示例 4: 二元表达式 - 清除前后对比
//		清除前（运算符周围空格不一致）:
//			code := `a+b*c-d/e`
//			// 没有空格，难以阅读
//
//		清除位置信息:
//			expr, _ := parser.ParseExpr(code)
//			clearPosition(expr)
//
//		清除后（格式化输出）:
//			a + b*c - d/e
//			// 格式化工具根据运算符优先级添加空格
//			// * 和 / 优先级高，与操作数紧密；+ 和 - 优先级低，周围有空格
//
//	示例 5: 完整文件 - 清除前后对比
//		清除前（格式混乱的代码）:
//			src := `package main
//			import "fmt"
//			func main(){
//			var user=model.SysUser{ID:1,Username:"admin"}
//			fmt.Println(user.Username)
//			}`
//			// 问题：函数声明格式不规范、变量声明格式错误、结构体格式混乱
//
//		清除位置信息:
//			fset := token.NewFileSet()
//			file, _ := parser.ParseFile(fset, "", src, parser.ParseComments)
//			clearPosition(file)
//
//		清除后（格式化输出）:
//			package main
//
//			import "fmt"
//
//			func main() {
//				var user = model.SysUser{
//					ID:       1,
//					Username: "admin",
//				}
//				fmt.Println(user.Username)
//			}
//			// 格式化工具自动修复：
//			// 1. import 后添加空行
//			// 2. 函数声明格式规范化（main() 后添加空格）
//			// 3. 变量声明格式规范化（var user= 变为 var user =）
//			// 4. 结构体字段对齐和换行
//			// 5. 统一缩进格式
//
//		完整代码实现:
//			import (
//				"bytes"
//				"go/format"
//				"go/parser"
//				"go/printer"
//				"go/token"
//			)
//
//			fset := token.NewFileSet()
//			file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
//			if err != nil {
//				log.Fatal(err)
//			}
//			clearPosition(file) // 关键步骤：清除所有位置信息
//
//			// 方法1: 使用 format.Source 格式化（推荐）
//			var buf bytes.Buffer
//			printer.Fprint(&buf, fset, file)
//			formatted, _ := format.Source(buf.Bytes())
//			// formatted 就是格式化后的代码字节数组
//
//			// 方法2: 直接使用 printer.Fprint 输出
//			printer.Fprint(os.Stdout, fset, file)
//			// 输出到标准输出，格式符合 Go 代码规范
//
//	示例 6: 复杂表达式 - 清除前后对比
//		清除前（格式混乱）:
//			code := `x:=1+2*3-4/2`
//			// 没有空格，运算符优先级不清晰
//
//		清除位置信息:
//			expr, _ := parser.ParseExpr(code)
//			clearPosition(expr)
//
//		清除后（格式化输出）:
//			x := 1 + 2*3 - 4/2
//			// 格式化工具根据优先级添加空格：
//			// - * 和 / 优先级高，与操作数紧密（2*3, 4/2）
//			// - + 和 - 优先级低，周围有空格（1 + ..., ... - ...）
//			// - := 运算符周围有空格
func clearPosition(astNode ast.Node) {
	// 遍历 AST 树，清除所有位置信息
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Ident:
			// 清除标识符的名称位置
			// 标识符位置用于代码格式化时的对齐
			node.NamePos = token.NoPos
		case *ast.CallExpr:
			// 清除函数调用的左右括号位置
			// 括号位置影响代码格式化的换行和缩进
			node.Lparen = token.NoPos
			node.Rparen = token.NoPos
		case *ast.BasicLit:
			// 清除字面量的值位置
			// 字面量位置用于字符串、数字等的格式化
			node.ValuePos = token.NoPos
		case *ast.SelectorExpr:
			// 清除选择器表达式的选择器位置
			// 例如 model.SysBaseMenu 中 SysBaseMenu 的位置
			node.Sel.NamePos = token.NoPos
		case *ast.BinaryExpr:
			// 清除二元表达式的操作符位置
			// 操作符位置影响 +、-、*、/ 等运算符的格式化
			node.OpPos = token.NoPos
		case *ast.UnaryExpr:
			// 清除一元表达式的操作符位置
			// 例如 !、-、*、& 等一元运算符的位置
			node.OpPos = token.NoPos
		case *ast.StarExpr:
			// 清除指针表达式的星号位置
			// 例如 *Type 中 * 的位置
			node.Star = token.NoPos
		}
		// 继续遍历子节点，清除所有位置信息
		return true
	})
}

// CreateStmt 从字符串表达式创建 AST 语句节点
// 参数:
//   - statement: Go 代码表达式字符串（如 "fmt.Println(\"hello\")"）
//
// 返回:
//   - *ast.ExprStmt: 表达式语句的 AST 节点
//
// 实现原理:
//
//	使用 Go 标准库的 parser.ParseExpr 解析字符串为 AST 表达式
//	然后清除位置信息并包装成表达式语句
//
// 为什么这样写:
//  1. 利用 Go 标准库的解析器，保证解析的正确性
//  2. 解析后清除位置信息，让格式化工具重新计算位置
//  3. 包装成 ExprStmt，可以直接插入到函数体中
//  4. 使用 log.Fatal 处理错误，因为解析失败通常是编程错误
//
// 好处:
//   - 灵活性：可以从字符串动态生成代码
//   - 正确性：使用标准解析器，保证语法正确
//   - 易用性：一行代码字符串即可生成 AST 节点
//   - 格式化：清除位置信息后，生成的代码格式规范
//
// 示例:
//
//	// 示例1: 创建函数调用语句
//	// 输入: `fmt.Println("Hello, World!")`
//	// 输出: &ast.ExprStmt{X: &ast.CallExpr{Fun: &ast.SelectorExpr{...}, Args: [...]}}
//	// 使用: 可以插入到函数体的 Body.List 中
//	stmt1 := CreateStmt(`fmt.Println("Hello, World!")`)
//	// 生成的代码等价于: fmt.Println("Hello, World!")
//
//	// 示例2: 创建方法调用语句
//	// 输入: `db.Create(&user)`
//	// 输出: &ast.ExprStmt{X: &ast.CallExpr{Fun: &ast.SelectorExpr{...}, Args: [...]}}
//	// 使用: 可以用于代码生成场景，动态生成数据库操作代码
//	stmt2 := CreateStmt(`db.Create(&user)`)
//	// 生成的代码等价于: db.Create(&user)
//
//	// 示例3: 创建函数调用并传递参数
//	// 输入: `initialize.Database(db)`
//	// 输出: &ast.ExprStmt{X: &ast.CallExpr{Fun: &ast.SelectorExpr{...}, Args: [...]}}
//	// 使用: 常用于插件系统或代码生成工具
//	stmt3 := CreateStmt(`initialize.Database(db)`)
//	// 生成的代码等价于: initialize.Database(db)
//
//	// 示例4: 创建类型断言语句
//	// 输入: `v.(string)`
//	// 输出: &ast.ExprStmt{X: &ast.TypeAssertExpr{X: ..., Type: ...}}
//	// 使用: 生成的语句可以用于类型转换
//	stmt4 := CreateStmt(`v.(string)`)
//	// 生成的代码等价于: v.(string)
//
//	// 示例5: 创建复合字面量语句
//	// 输入: `config{Port: 8080, Host: "localhost"}`
//	// 输出: &ast.ExprStmt{X: &ast.CompositeLit{Type: ..., Elts: [...]}}
//	// 使用: 可以用于初始化配置对象
//	stmt5 := CreateStmt(`config{Port: 8080, Host: "localhost"}`)
//	// 生成的代码等价于: config{Port: 8080, Host: "localhost"}
//
//	// 示例6: 实际使用场景 - 将生成的语句插入到函数体中
//	// 输入: `log.Info("Plugin initialized")`
//	// 输出: &ast.ExprStmt{...}
//	// 使用:
//	stmt6 := CreateStmt(`log.Info("Plugin initialized")`)
//	// 可以将 stmt6 添加到函数体中:
//	//   func init() {
//	//       // 其他代码...
//	//       stmt6  // 这里插入生成的语句
//	//   }
//	// 最终生成的代码:
//	//   func init() {
//	//       // 其他代码...
//	//       log.Info("Plugin initialized")
//	//   }
func CreateStmt(statement string) *ast.ExprStmt {
	// 使用 Go 标准库解析表达式字符串
	// ParseExpr 可以解析任何 Go 表达式，如函数调用、变量访问等
	expr, err := parser.ParseExpr(statement)
	if err != nil {
		// 解析失败通常是代码生成逻辑错误，使用 Fatal 立即终止
		// 这样可以快速发现代码生成的问题
		log.Fatal(err)
	}
	// 清除位置信息，让代码格式化工具重新计算最佳位置
	// 这对于代码生成很重要，可以保证生成的代码格式规范
	clearPosition(expr)
	// 将表达式包装成表达式语句
	// ExprStmt 是可以在函数体中使用的语句类型
	return &ast.ExprStmt{X: expr}
}

// IsBlockStmt 检查 AST 节点是否为代码块语句（BlockStmt）
// 参数:
//   - node: 要检查的 AST 节点
//
// 返回:
//   - bool: 是 BlockStmt 返回 true，否则返回 false
//
// 实现原理:
//
//	使用类型断言检查节点类型是否为 *ast.BlockStmt
//	BlockStmt 表示用花括号包围的代码块，如函数体、if 语句体等
//
// 为什么这样写:
//  1. 使用类型断言是最直接高效的检查方式
//  2. 只关心类型，不关心值，使用空白标识符忽略值
//  3. 代码简洁，一行完成检查
//
// 好处:
//   - 高效：类型断言是 O(1) 操作
//   - 简洁：代码清晰易读
//   - 通用：可以用于任何需要检查代码块的场景
func IsBlockStmt(node ast.Node) bool {
	// 类型断言：检查节点是否为代码块语句
	// 使用空白标识符 _ 忽略值，只检查类型
	_, ok := node.(*ast.BlockStmt)
	return ok
}

// VariableExistsInBlock 检查代码块中是否存在指定名称的变量声明
// 参数:
//   - block: 要检查的代码块节点
//   - varName: 要查找的变量名
//
// 返回:
//   - bool: 存在返回 true，不存在返回 false
//
// 实现原理:
//
//	遍历代码块中的所有赋值语句，检查左侧是否有指定名称的变量
//	只检查赋值语句，不检查变量声明（var、const 等）
//
// 为什么这样写:
//  1. 使用 ast.Inspect 遍历代码块，可以找到所有层级的赋值语句
//  2. 只检查 AssignStmt 的左侧（Lhs），因为左侧是被赋值的变量
//  3. 找到匹配变量后立即返回 false 终止遍历，提高效率
//  4. 使用标识符名称匹配，精确查找目标变量
//
// 好处:
//   - 精确：通过变量名精确匹配，避免误判
//   - 高效：找到后立即停止遍历
//   - 全面：可以查找嵌套代码块中的变量
//   - 实用：用于代码生成时避免重复声明变量
//
// 使用示例:
//
//	示例 1: 检查简单赋值语句
//	代码块内容:
//	  {
//	    x := 10
//	    y = 20
//	  }
//	调用: VariableExistsInBlock(block, "x") // 返回 true
//	调用: VariableExistsInBlock(block, "y") // 返回 true
//	调用: VariableExistsInBlock(block, "z") // 返回 false
//
//	示例 2: 检查多重赋值
//	代码块内容:
//	  {
//	    a, b := 1, 2
//	    c, d = 3, 4
//	  }
//	调用: VariableExistsInBlock(block, "a") // 返回 true
//	调用: VariableExistsInBlock(block, "b") // 返回 true
//	调用: VariableExistsInBlock(block, "c") // 返回 true
//
//	示例 3: 检查嵌套代码块
//	代码块内容:
//	  {
//	    outer := 100
//	    if true {
//	        inner := 200
//	    }
//	  }
//	调用: VariableExistsInBlock(block, "outer") // 返回 true
//	调用: VariableExistsInBlock(block, "inner") // 返回 true（嵌套块中的变量也能找到）
//
//	示例 4: 实际使用场景 - 代码生成时避免重复声明
//	代码:
//	  src := `
//	    package main
//	    func test() {
//	        result := 0
//	        // ... 其他代码
//	    }
//	  `
//	  fset := token.NewFileSet()
//	  f, _ := parser.ParseFile(fset, "", src, parser.ParseComments)
//	  ast.Inspect(f, func(n ast.Node) bool {
//	      if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "test" {
//	          if fn.Body != nil {
//	              exists := VariableExistsInBlock(fn.Body, "result")
//	              if !exists {
//	                  // 变量不存在，可以安全地添加声明
//	                  // result := 0
//	              }
//	          }
//	          return false
//	      }
//	      return true
//	  })
func VariableExistsInBlock(block *ast.BlockStmt, varName string) bool {
	exists := false
	// 遍历代码块中的所有节点
	ast.Inspect(block, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// 检查赋值语句左侧的所有表达式
			// 左侧是被赋值的变量，右侧是值
			for _, expr := range node.Lhs {
				// 检查是否为标识符（变量名）且名称匹配
				if ident, ok := expr.(*ast.Ident); ok && ident.Name == varName {
					exists = true
					return false // 找到后立即终止遍历
				}
			}
		}
		// 继续遍历其他节点
		return true
	})
	return exists
}
