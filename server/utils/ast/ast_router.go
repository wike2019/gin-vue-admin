package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

// AppendNodeToList 在语句列表的指定位置插入新的语句节点
// 参数:
//   - stmts: 原有的语句列表
//   - stmt: 要插入的语句节点
//   - index: 插入位置（0-based索引）
//
// 返回: 插入后的新语句列表
//
// 实现原理:
//
//	使用Go切片的append操作实现插入：
//	1. stmts[:index] 获取插入位置之前的所有元素
//	2. append([]ast.Stmt{stmt}, stmts[index:]...) 将新节点和后续元素合并
//	3. 将两部分再次合并得到最终结果
//
// 好处:
//   - 时间复杂度O(n)，但代码简洁易读
//   - 不修改原切片，返回新切片，避免副作用
//   - 适用于AST节点插入场景，保持代码结构清晰
func AppendNodeToList(stmts []ast.Stmt, stmt ast.Stmt, index int) []ast.Stmt {
	return append(stmts[:index], append([]ast.Stmt{stmt}, stmts[index:]...)...)
}

// AddRouterCode 动态向Go源文件添加路由注册代码
// 这是代码生成工具的核心函数，通过AST操作实现自动化代码注入
//
// 参数:
//   - path: 目标Go源文件路径
//   - funcName: 目标函数名（通常是路由初始化函数）
//   - pk: 包名/模块标识（用于生成路由变量名）
//   - model: 模型名（用于生成初始化函数名）
//
// 工作流程:
//  1. 读取源文件并解析为AST（抽象语法树）
//  2. 定位目标函数节点
//  3. 检查并添加路由变量声明（如: xxxRouter := router.RouterGroupApp.Xxx）
//  4. 检查并添加路由初始化调用（如: xxxRouter.InitXxxRouter(privateGroup, publicGroup)）
//  5. 将修改后的AST格式化并写回文件
//
// 设计优势:
//   - 使用AST而非字符串操作，保证代码语法正确性
//   - 自动检测重复，避免重复添加相同代码
//   - 保持原有代码格式和注释
//   - 支持增量更新，不会破坏已有代码结构
//
// 使用示例:
//
//	示例1: 添加用户模块路由
//	AddRouterCode(
//		"server/router/enter.go",  // 目标文件路径
//		"Routers",                  // 目标函数名
//		"user",                     // 包名，将生成 userRouter 变量
//		"User",                     // 模型名，将生成 InitUserRouter 函数调用
//	)
//	// 生成的代码:
//	// userRouter := router.RouterGroupApp.User
//	// userRouter.InitUserRouter(privateGroup, publicGroup)
//
//	示例2: 添加订单模块路由
//	AddRouterCode(
//		"server/router/enter.go",
//		"Routers",
//		"order",                    // 包名，将生成 orderRouter 变量
//		"Order",                    // 模型名，将生成 InitOrderRouter 函数调用
//	)
//	// 生成的代码:
//	// orderRouter := router.RouterGroupApp.Order
//	// orderRouter.InitOrderRouter(privateGroup, publicGroup)
//
//	示例3: 添加商品模块路由
//	AddRouterCode(
//		"server/router/enter.go",
//		"Routers",
//		"product",                  // 包名，将生成 productRouter 变量
//		"Product",                  // 模型名，将生成 InitProductRouter 函数调用
//	)
//	// 生成的代码:
//	// productRouter := router.RouterGroupApp.Product
//	// productRouter.InitProductRouter(privateGroup, publicGroup)
//
//	示例4: 添加系统管理模块路由
//	AddRouterCode(
//		"server/router/system/enter.go",  // 不同的目标文件
//		"InitSystemRouter",                // 不同的目标函数
//		"system",                          // 包名
//		"System",                          // 模型名
//	)
//	// 生成的代码:
//	// systemRouter := router.RouterGroupApp.System
//	// systemRouter.InitSystemRouter(privateGroup, publicGroup)
//
//	注意事项:
//	- 函数会自动检测重复，多次调用不会产生重复代码
//	- 确保目标文件存在且可读
//	- 确保目标函数存在于目标文件中
//	- pk 参数建议使用小写，model 参数建议使用首字母大写
func AddRouterCode(path, funcName, pk, model string) {
	// 读取源文件内容
	// 使用os.ReadFile一次性读取整个文件，适合小到中等大小的文件
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
	}

	// 创建文件集（FileSet）用于跟踪源代码位置信息
	// FileSet在AST操作中用于保持源代码位置信息，便于错误报告和格式化
	fileSet := token.NewFileSet()

	// 解析Go源文件为AST
	// parser.ParseComments参数确保保留注释信息，这对于代码生成很重要
	astFile, err := parser.ParseFile(fileSet, "", src, parser.ParseComments)

	if err != nil {
		fmt.Println(err)
	}

	// 在AST中查找目标函数节点
	// 通过函数名定位，这是后续修改的基础
	FuncNode := FindFunction(astFile, funcName)

	// 生成命名约定：
	// pkName: 首字母大写的包名（如: "user" -> "User"）
	// routerName: 路由变量名（如: "userRouter"）
	// modelName: 初始化函数名（如: "InitUserRouter"）
	// 这种命名约定保证了代码的一致性和可读性
	pkName := strings.ToUpper(pk[:1]) + pk[1:]
	routerName := fmt.Sprintf("%sRouter", pk)
	modelName := fmt.Sprintf("Init%sRouter", model)

	// 查找函数体中最后一个BlockStmt（代码块）
	// 从后往前遍历是为了找到最内层或最后添加的代码块
	// 这个代码块将作为新代码的插入位置
	var bloctPre *ast.BlockStmt
	for i := len(FuncNode.Body.List) - 1; i >= 0; i-- {
		if block, ok := FuncNode.Body.List[i].(*ast.BlockStmt); ok {
			bloctPre = block
		}
	}

	// 调试输出：打印函数节点的AST结构
	// 在开发阶段用于验证AST解析是否正确
	ast.Print(fileSet, FuncNode)

	// 检查是否需要添加路由变量声明
	// 避免重复添加相同的路由变量，保证代码的幂等性
	if ok, b := needAppendRouter(FuncNode, pk); ok {
		// 构建路由变量声明的AST节点
		// 生成的代码示例: xxxRouter := router.RouterGroupApp.Xxx
		//
		// AST结构说明:
		// - BlockStmt: 代码块容器，包含一个赋值语句
		// - AssignStmt: 赋值语句，使用:=进行短变量声明
		// - SelectorExpr: 选择器表达式，表示访问结构体字段或方法
		//   两层嵌套的SelectorExpr表示: router.RouterGroupApp.Xxx
		//   这种结构化的AST构建方式保证了生成的代码语法正确
		routerNode :=
			&ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{
							&ast.Ident{Name: routerName},
						},
						Tok: token.DEFINE, // := 短变量声明操作符
						Rhs: []ast.Expr{
							&ast.SelectorExpr{
								X: &ast.SelectorExpr{
									X:   &ast.Ident{Name: "router"},
									Sel: &ast.Ident{Name: "RouterGroupApp"},
								},
								Sel: &ast.Ident{Name: pkName},
							},
						},
					},
				},
			}

		// 将路由节点插入到函数体的倒数第二个位置
		// 插入位置选择在最后（但不在末尾）是为了：
		// 1. 保持代码逻辑顺序（变量声明在前，初始化调用在后）
		// 2. 为后续的初始化调用预留空间
		// 3. 避免破坏函数末尾可能存在的return语句
		FuncNode.Body.List = AppendNodeToList(FuncNode.Body.List, routerNode, len(FuncNode.Body.List)-1)
		bloctPre = routerNode
	} else {
		// 如果路由变量已存在，使用已存在的代码块作为插入点
		// 这样可以确保初始化调用添加到正确的位置
		bloctPre = b
	}

	// 检查是否需要添加路由初始化调用
	// 生成的代码示例: xxxRouter.InitXxxRouter(privateGroup, publicGroup)
	// 这个检查确保不会重复添加相同的初始化调用
	if needAppendInit(FuncNode, routerName, modelName) {
		// 构建函数调用的AST节点并添加到代码块
		// 生成的代码示例: xxxRouter.InitXxxRouter(privateGroup, publicGroup)
		//
		// AST结构说明:
		// - ExprStmt: 表达式语句，将函数调用作为独立语句
		// - CallExpr: 函数调用表达式
		// - SelectorExpr: 方法选择器，表示调用对象的方法
		// - Args: 函数参数列表，这里传入两个路由组参数
		//
		// 设计考虑:
		// 直接append到代码块列表，简单高效
		// 使用AST节点而非字符串拼接，保证代码格式和语法正确
		bloctPre.List = append(bloctPre.List,
			&ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   &ast.Ident{Name: routerName},
						Sel: &ast.Ident{Name: modelName},
					},
					Args: []ast.Expr{
						&ast.Ident{
							Name: "privateGroup",
						},
						&ast.Ident{
							Name: "publicGroup",
						},
					},
				},
			})
	}

	// 将修改后的AST转换回Go源代码
	// 使用bytes.Buffer作为输出缓冲区，高效处理字节流
	var out []byte
	bf := bytes.NewBuffer(out)

	// 使用go/printer包格式化AST为源代码
	// printer.Fprint会按照Go标准格式输出，包括缩进、换行等
	// 这比手动格式化更可靠，保证生成的代码符合Go代码规范
	printer.Fprint(bf, fileSet, astFile)

	// 将格式化后的代码写回源文件
	// 文件权限0666表示所有用户可读写，适合代码文件
	os.WriteFile(path, bf.Bytes(), 0666)
}

// needAppendRouter 检查函数中是否已存在指定包的路由变量声明
// 参数:
//   - funcNode: 要检查的函数AST节点
//   - pk: 包名/模块标识
//
// 返回:
//   - bool: true表示需要添加，false表示已存在
//   - *ast.BlockStmt: 如果已存在，返回包含该声明的代码块节点
//
// 实现原理:
//
//	使用ast.Inspect深度优先遍历AST树，查找赋值语句
//	检查左侧标识符是否为"xxxRouter"格式的路由变量名
//
// 设计优势:
//   - 使用AST遍历而非字符串匹配，准确识别代码结构
//   - 返回代码块节点，便于后续在正确位置插入新代码
//   - 提前返回（return false）优化性能，找到后立即停止遍历
//
// 检查逻辑:
//
//	遍历所有BlockStmt中的AssignStmt，查找形如"xxxRouter := ..."的声明
//	如果找到匹配的路由变量名，说明已存在，无需重复添加
func needAppendRouter(funcNode ast.Node, pk string) (bool, *ast.BlockStmt) {
	flag := true
	var block *ast.BlockStmt

	// ast.Inspect是Go标准库提供的AST遍历工具
	// 它会深度优先遍历所有节点，对每个节点调用回调函数
	// 回调函数返回false可以提前终止遍历，提高效率
	ast.Inspect(funcNode, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.BlockStmt:
			// 遍历代码块中的所有语句
			for i := range n.List {
				// 检查是否为赋值语句（包括:=和=）
				if assignNode, ok := n.List[i].(*ast.AssignStmt); ok {
					// 检查赋值语句左侧是否为标识符
					if identNode, ok := assignNode.Lhs[0].(*ast.Ident); ok {
						// 检查标识符名称是否匹配路由变量命名规则
						if identNode.Name == fmt.Sprintf("%sRouter", pk) {
							flag = false
							block = n
							// 找到后立即停止遍历，提高效率
							return false
						}
					}
				}
			}

		}
		return true // 继续遍历
	})
	return flag, block
}

// needAppendInit 检查函数中是否已存在指定路由的初始化调用
// 参数:
//   - funcNode: 要检查的函数AST节点
//   - routerName: 路由变量名（如: "userRouter"）
//   - modelName: 初始化函数名（如: "InitUserRouter"）
//
// 返回:
//   - bool: true表示需要添加，false表示已存在
//
// 实现原理:
//
//	使用ast.Inspect遍历AST，查找函数调用表达式
//	检查调用是否为"xxxRouter.InitXxxRouter(...)"格式
//
// 设计优势:
//   - 通过AST结构匹配而非字符串匹配，准确识别函数调用
//   - 同时检查调用对象和方法名，避免误判
//   - 提前返回优化性能
//
// 注意:
//
//	此函数用于避免重复添加相同的路由初始化调用
//	保证代码生成的幂等性，多次运行不会产生重复代码
func needAppendInit(funcNode ast.Node, routerName string, modelName string) bool {
	flag := true

	// 遍历AST查找函数调用表达式
	ast.Inspect(funcNode, func(node ast.Node) bool {
		// 使用node而不是funcNode，这样才能正确遍历所有子节点
		// 如果使用funcNode，只会检查函数节点本身，而不会检查函数体内的语句
		switch n := node.(type) {
		case *ast.CallExpr:
			// 检查是否为方法调用（通过SelectorExpr表示）
			if selectNode, ok := n.Fun.(*ast.SelectorExpr); ok {
				// 检查调用对象是否为指定的路由变量
				x, xok := selectNode.X.(*ast.Ident)
				// 同时匹配调用对象名和方法名，确保精确匹配
				if xok && x.Name == routerName && selectNode.Sel.Name == modelName {
					flag = false
					return false // 找到后立即停止遍历
				}
			}
		}
		return true // 继续遍历
	})
	return flag
}
