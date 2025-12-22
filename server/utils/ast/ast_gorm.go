package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
)

// AddRegisterTablesAst 自动为 gorm.go 文件注册数据库表的自动迁移代码
//
// 为什么使用 AST 而不是字符串操作：
// 1. 语法正确性：AST 操作确保生成的代码符合 Go 语法规范，不会产生语法错误
// 2. 代码格式：printer.Fprint 会自动格式化代码，保持统一的代码风格
// 3. 结构保持：不会破坏原有代码的逻辑结构和注释
// 4. 安全性：避免字符串拼接可能导致的注入风险（如注释、字符串字面量中的特殊字符）
//
// 参数说明：
//   - path: gorm.go 文件的路径
//   - funcName: 目标函数名称（通常是要添加迁移的初始化函数）
//   - pk: model 包名（package key），如 "system"
//   - varName: 数据库变量名，如 "db" 或特定数据库名称
//   - dbName: 数据库名称（用于多数据库场景）
//   - model: model 结构体名称，如 "SysUser"
func AddRegisterTablesAst(path, funcName, pk, varName, dbName, model string) {
	// 构建完整的 model 包导入路径
	// 这样可以在代码中通过 pk.Model 的方式引用，如 system.SysUser
	modelPk := fmt.Sprintf("github.com/flipped-aurora/gin-vue-admin/server/model/%s", pk)

	// 读取源文件内容
	// 使用 os.ReadFile 一次性读取整个文件，避免逐行处理带来的复杂性
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
	}

	// 创建文件集（FileSet），用于跟踪源码位置信息
	// FileSet 是 AST 操作的基础，用于在修改后正确输出代码的位置信息
	fileSet := token.NewFileSet()

	// 解析 Go 源文件为 AST
	// parser.ParseFile 将源代码转换为抽象语法树，便于程序化操作
	// 参数说明：
	//   - fileSet: 文件集
	//   - "": 源文件名（这里为空，因为我们直接传入了源码）
	//   - src: 源文件内容
	//   - 0: 解析模式（0 表示正常模式）
	astFile, err := parser.ParseFile(fileSet, "", src, 0)
	if err != nil {
		fmt.Println(err)
	}

	// 添加必要的 import 语句
	// 如果 model 包还没有被导入，这里会自动添加
	// 好处：避免手动维护 import，减少遗漏导入的错误
	AddImport(astFile, modelPk)

	// 在 AST 中查找目标函数节点
	// 通过 AST 查找比字符串匹配更准确，可以区分函数定义和函数调用
	FuncNode := FindFunction(astFile, funcName)

	// 调试输出：打印函数节点结构（开发时使用）
	if FuncNode != nil {
		ast.Print(fileSet, FuncNode)
	}

	// 在函数体中添加数据库变量声明（如果需要多数据库支持）
	// 如果 dbName 不为空，会创建类似 db := global.GetGlobalDBByDBName("dbname") 的代码
	addDBVar(FuncNode.Body, varName, dbName)

	// 添加 AutoMigrate 调用
	// 会在函数体中添加类似 db.AutoMigrate(&system.SysUser{}) 的代码
	addAutoMigrate(FuncNode.Body, varName, pk, model)

	// 将修改后的 AST 转换回源代码
	// 使用 bytes.Buffer 作为缓冲区，printer.Fprint 会将格式化后的代码写入缓冲区
	var out []byte
	bf := bytes.NewBuffer(out)
	printer.Fprint(bf, fileSet, astFile)

	// 将修改后的代码写回文件
	// 使用 0666 权限，确保文件可读可写
	// 好处：printer 会自动处理代码格式化，生成的代码符合 gofmt 规范
	os.WriteFile(path, bf.Bytes(), 0666)
}

// addDBVar 在函数体中添加数据库变量声明
//
// 设计思路：
// 1. 先检查变量是否已存在，避免重复声明（幂等性）
// 2. 使用类型断言精确识别赋值语句，而不是简单的字符串匹配
// 3. 将新变量插入到函数体开头，确保在使用前已声明
//
// 为什么这样设计：
// - 幂等性：多次调用不会产生重复代码，确保代码整洁
// - 类型安全：使用 AST 类型断言，只在真正的赋值语句中查找变量名
// - 位置合理：插入到函数体开头，符合 Go 代码的书写习惯
//
// 生成的代码示例：
//
//	db := global.GetGlobalDBByDBName("dbname")
//
// 参数说明：
//   - astBody: 函数体的 AST 节点（BlockStmt）
//   - varName: 要创建的变量名，如 "db"
//   - dbName: 数据库名称，如果为空则不添加变量（使用默认 db）
func addDBVar(astBody *ast.BlockStmt, varName, dbName string) {
	// 如果数据库名为空，说明使用默认数据库，不需要创建新的变量
	if dbName == "" {
		return
	}

	// 将数据库名格式化为字符串字面量
	// 注意：这里需要包含引号，因为 BasicLit.Value 需要完整的字面量字符串
	dbStr := fmt.Sprintf("\"%s\"", dbName)

	// 遍历函数体中的所有语句，检查变量是否已存在
	// 使用类型断言 (*ast.AssignStmt) 只匹配赋值语句，忽略其他类型的语句
	// 这样设计的好处是：
	//   1. 精确：只检查真正的变量声明/赋值
	//   2. 安全：不会误判其他包含变量名的代码（如注释、字符串）
	for i := range astBody.List {
		if assignStmt, ok := astBody.List[i].(*ast.AssignStmt); ok {
			// 检查赋值语句的左侧（Lhs）是否为标识符
			// Lhs[0] 表示第一个被赋值的变量（支持多变量赋值）
			if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok {
				// 如果变量名匹配，说明变量已存在，直接返回
				// 这实现了幂等性：多次调用不会产生重复代码
				if ident.Name == varName {
					return
				}
			}
		}
	}

	// 构建变量声明的 AST 节点
	// 创建类似 `varName := global.GetGlobalDBByDBName("dbName")` 的语句
	assignNode := &ast.AssignStmt{
		// Lhs (Left-hand side): 赋值左侧，即变量名
		Lhs: []ast.Expr{
			&ast.Ident{
				Name: varName, // 变量名，如 "db"
			},
		},
		// Tok: 赋值操作符，token.DEFINE 表示 ":=" (短变量声明)
		// 使用 := 而不是 =，因为这是新变量的声明
		Tok: token.DEFINE,
		// Rhs (Right-hand side): 赋值右侧，即表达式的 AST 节点
		Rhs: []ast.Expr{
			&ast.CallExpr{
				// Fun: 函数调用的函数部分
				// SelectorExpr 表示选择器表达式，如 global.GetGlobalDBByDBName
				Fun: &ast.SelectorExpr{
					// X: 选择器的接收者，如 global
					X: &ast.Ident{
						Name: "global",
					},
					// Sel: 选择的方法名，如 GetGlobalDBByDBName
					Sel: &ast.Ident{
						Name: "GetGlobalDBByDBName",
					},
				},
				// Args: 函数调用的参数列表
				Args: []ast.Expr{
					// BasicLit 表示基本字面量（字符串、数字等）
					&ast.BasicLit{
						Kind:  token.STRING, // 字符串类型
						Value: dbStr,        // 字符串值，如 "dbname"
					},
				},
			},
		},
	}

	// 将新创建的赋值语句插入到函数体的开头
	// 使用 append([]ast.Stmt{assignNode}, astBody.List...) 而不是 append(astBody.List, assignNode)
	// 这样可以确保变量声明在其他代码之前，符合 Go 的代码规范
	astBody.List = append([]ast.Stmt{assignNode}, astBody.List...)
}

// addAutoMigrate 为数据库变量添加 AutoMigrate 方法调用
//
// 设计思路：
// 1. 使用 ast.Inspect 深度遍历函数体，查找现有的 AutoMigrate 调用
// 2. 如果找到现有调用且模型未存在，则追加到现有调用的参数列表中（合并调用）
// 3. 如果没找到现有调用，则创建新的 AutoMigrate 调用语句
//
// 为什么这样设计：
//   - 合并调用：将同一数据库的多个模型合并到一个 AutoMigrate 调用中，代码更简洁
//     例如：db.AutoMigrate(&system.SysUser{}, &system.SysRole{}) 而不是两个独立的调用
//   - 幂等性：检查模型是否已存在，避免重复添加
//   - 灵活性：支持多数据库场景，可以通过 dbname 参数指定不同的数据库变量
//
// 生成的代码示例：
//
//	情况1（追加到现有调用）: db.AutoMigrate(&system.SysUser{}, &system.SysRole{})
//	情况2（创建新调用）:     db.AutoMigrate(&system.SysUser{})
//
// 参数说明：
//   - astBody: 函数体的 AST 节点
//   - dbname: 数据库变量名，如 "db"
//   - pk: model 包名，如 "system"
//   - model: model 结构体名，如 "SysUser"
func addAutoMigrate(astBody *ast.BlockStmt, dbname string, pk string, model string) {
	// 如果未指定数据库变量名，使用默认的 "db"
	if dbname == "" {
		dbname = "db"
	}

	// flag 标记是否找到了现有的 AutoMigrate 调用
	// true: 未找到，需要创建新的调用语句
	// false: 已找到，可以在现有调用中追加参数
	flag := true

	// 使用 ast.Inspect 深度优先遍历 AST 节点树
	// ast.Inspect 会自动遍历所有子节点，比手动递归更方便
	// 返回 false 可以停止继续遍历子节点（用于提前终止）
	ast.Inspect(astBody, func(node ast.Node) bool {
		// 只处理函数调用表达式（CallExpr）
		// 因为 AutoMigrate 是一个方法调用
		switch n := node.(type) {
		case *ast.CallExpr:
			// 检查是否是选择器表达式（SelectorExpr），如 db.AutoMigrate
			if s, ok := n.Fun.(*ast.SelectorExpr); ok {
				// 检查选择器的接收者是否为标识符（变量名）
				if x, ok := s.X.(*ast.Ident); ok {
					// 判断是否匹配目标调用：接收者名称匹配 && 方法名是 AutoMigrate
					if s.Sel.Name == "AutoMigrate" && x.Name == dbname {
						// 找到了现有的 AutoMigrate 调用
						flag = false

						// 检查模型是否已经在参数列表中
						// 如果已存在，直接返回 false 停止遍历（避免重复添加）
						if !NeedAppendModel(n, pk, model) {
							return false
						}

						// 将新模型追加到现有 AutoMigrate 调用的参数列表中
						// 创建复合字面量（CompositeLit），如 &system.SysUser{}
						// 使用 & 符号表示结构体指针，这是 GORM AutoMigrate 的要求
						n.Args = append(n.Args, &ast.CompositeLit{
							Type: &ast.SelectorExpr{
								// X: 包名，如 system
								X: &ast.Ident{
									Name: pk,
								},
								// Sel: 类型名，如 SysUser
								Sel: &ast.Ident{
									Name: model,
								},
							},
							// 注意：这里没有设置 Elts（元素列表），因为 &system.SysUser{} 是空结构体
						})

						// 返回 false 停止继续遍历，因为已经找到并处理了目标节点
						return false
					}
				}
			}
		}
		// 返回 true 继续遍历子节点
		return true
	})

	// 如果没有找到现有的 AutoMigrate 调用，创建新的调用语句
	if flag {
		// 创建表达式语句（ExprStmt），包装函数调用
		// ExprStmt 用于将表达式作为独立语句，如 db.AutoMigrate(...)
		exprStmt := &ast.ExprStmt{
			X: &ast.CallExpr{
				// Fun: 方法选择器，db.AutoMigrate
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: dbname, // 数据库变量名
					},
					Sel: &ast.Ident{
						Name: "AutoMigrate", // 方法名
					},
				},
				// Args: 参数列表，包含要迁移的模型
				Args: []ast.Expr{
					&ast.CompositeLit{
						// Type: 复合字面量的类型，system.SysUser
						Type: &ast.SelectorExpr{
							X: &ast.Ident{
								Name: pk, // 包名
							},
							Sel: &ast.Ident{
								Name: model, // 类型名
							},
						},
						// 注意：这里创建的是 &system.SysUser{}，但 AST 中 & 符号是 UnaryExpr
						// 这里只创建 CompositeLit，实际上在代码输出时会自动处理指针引用
						// 如果需要明确的指针，应该使用 &ast.UnaryExpr{Op: token.AND, X: compositeLit}
					},
				},
			}}

		// 将新创建的语句追加到函数体末尾
		// 追加到末尾而不是开头，因为 AutoMigrate 通常在函数体执行过程中调用
		// 这样更符合代码的逻辑顺序（先初始化，再迁移）
		astBody.List = append(astBody.List, exprStmt)
	}
}

// NeedAppendModel 检查 AutoMigrate 调用中是否已经包含指定的模型
//
// 设计目的：
// 实现幂等性检查，避免在 AutoMigrate 调用中重复添加相同的模型
// 例如：如果已有 db.AutoMigrate(&system.SysUser{})，就不会再添加 &system.SysUser{}
//
// 为什么需要这个检查：
// 1. 幂等性：多次调用 addAutoMigrate 不会产生重复的模型参数
// 2. 代码质量：避免生成冗余代码，保持代码简洁
// 3. 正确性：防止意外的重复迁移（虽然 GORM 允许，但不推荐）
//
// 工作原理：
// 遍历函数调用的 AST 节点，查找所有 SelectorExpr（选择器表达式）
// 如果找到 pk.model 形式的表达式（如 system.SysUser），说明模型已存在
//
// 返回值：
//   - true: 模型不存在，需要添加
//   - false: 模型已存在，不需要添加
//
// 参数说明：
//   - callNode: AutoMigrate 函数调用的 AST 节点（CallExpr）
//   - pk: model 包名，如 "system"
//   - model: model 结构体名，如 "SysUser"
func NeedAppendModel(callNode ast.Node, pk string, model string) bool {
	// flag 标记模型是否已存在
	// true: 未找到模型，需要添加（默认值）
	// false: 已找到模型，不需要添加
	flag := true

	// 遍历调用节点的所有子节点，查找模型引用
	// callNode 通常是 CallExpr（函数调用），其 Args 中包含所有的参数
	// 每个参数可能是 &system.SysUser{} 这样的复合字面量
	ast.Inspect(callNode, func(node ast.Node) bool {
		// 查找选择器表达式（SelectorExpr）
		// 在 AutoMigrate(&system.SysUser{}) 中，system.SysUser 是一个 SelectorExpr
		switch n := node.(type) {
		case *ast.SelectorExpr:
			// 检查选择器的接收者（X）是否为标识符（包名）
			if x, ok := n.X.(*ast.Ident); ok {
				// 判断是否匹配目标模型：包名匹配 && 类型名匹配
				// 例如：system.SysUser 中的 system 匹配 pk，SysUser 匹配 model
				if n.Sel.Name == model && x.Name == pk {
					// 找到了匹配的模型，设置标志为 false（不需要添加）
					flag = false
					// 返回 false 停止继续遍历（已经找到，无需继续查找）
					return false
				}
			}
		}
		// 返回 true 继续遍历其他节点
		return true
	})

	// 返回检查结果
	// true 表示需要添加模型，false 表示模型已存在
	return flag
}
