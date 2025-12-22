package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// RollBackAst 回滚自动生成的 AST 代码
// 参数:
//   - pk: 包名（package name），用于定位需要回滚的代码
//   - model: 模型名，用于定位具体的模型相关代码
//
// 为什么使用 AST 而不是字符串操作？
// 1. 安全性：AST 操作基于语法树结构，不会因为格式变化（空格、换行）而失败
// 2. 准确性：能精确识别代码结构，避免误删或误改
// 3. 可维护性：代码结构清晰，易于理解和维护
// 4. 格式化：使用 go/printer 自动格式化，保持代码风格一致
func RollBackAst(pk, model string) {
	// 分别回滚 GORM 初始化和路由初始化代码
	// 分离关注点：每个函数负责一个文件的回滚，职责清晰
	RollGormBack(pk, model)
	RollRouterBack(pk, model)
}

// RollGormBack 回滚 GORM 初始化代码
// 功能：从 gorm_biz.go 文件中删除指定包和模型的 GORM 初始化调用
//
// 设计思路：
// 1. 首先统计 pk 在整个文件中的使用次数（pkNum）
// 2. 如果 pkNum > 1：说明该包还在其他地方使用，只需删除当前调用，保留 import
// 3. 如果 pkNum == 1：说明这是最后一次使用，需要同时删除 import，避免留下无用导入
//
// 这样设计的好处：
// - 智能清理：避免留下无用的 import，保持代码整洁
// - 安全性：不会误删其他还在使用的 import
// - 精确性：通过 AST 精确匹配，不会因为变量名相似而误删
//
// 使用示例：
//
// 示例 1：删除单个模型（该包中只有一个模型）
//
//	调用：RollGormBack("system", "User")
//	原文件内容：
//	  import (
//	      "github.com/flipped-aurora/gin-vue-admin/server/model/system"
//	  )
//	  func init() {
//	      db.AutoMigrate(&system.User{})
//	  }
//	执行后：
//	  func init() {
//	      // system.User 的调用和 import 都被删除
//	  }
//
// 示例 2：删除模型（该包中还有其他模型在使用）
//
//	调用：RollGormBack("system", "Role")
//	原文件内容：
//	  import (
//	      "github.com/flipped-aurora/gin-vue-admin/server/model/system"
//	  )
//	  func init() {
//	      db.AutoMigrate(&system.User{}, &system.Role{})
//	  }
//	执行后：
//	  import (
//	      "github.com/flipped-aurora/gin-vue-admin/server/model/system"
//	  )
//	  func init() {
//	      db.AutoMigrate(&system.User{})  // 只删除 Role，保留 User 和 import
//	  }
//
// 示例 3：删除多个参数中的中间参数
//
//	调用：RollGormBack("example", "Customer")
//	原文件内容：
//	  db.AutoMigrate(&example.Customer{}, &example.Order{}, &example.Product{})
//	执行后：
//	  db.AutoMigrate(&example.Order{}, &example.Product{})  // 精确删除 Customer 参数
//
// 示例 4：实际使用场景
//
//	// 当用户通过自动代码生成工具删除某个模型时
//	// 需要回滚相关的初始化代码
//	RollGormBack("business", "Order")  // 删除 business 包下的 Order 模型初始化
func RollGormBack(pk, model string) {
	// 构建目标文件路径：gorm_biz.go 是 GORM 业务初始化文件
	path := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "initialize", "gorm_biz.go")
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 创建文件集和解析 AST
	// 使用 AST 解析而不是字符串操作的好处：
	// - 能理解代码的语法结构，不会因为格式问题失败
	// - 可以精确操作语法节点，避免误操作
	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, "", src, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 用于存储找到的目标调用表达式和参数索引
	var n *ast.CallExpr // 目标函数调用节点
	var k int = -1      // 要删除的参数在 Args 中的索引
	var pkNum = 0       // pk 在整个文件中出现的次数

	// 第一次遍历：查找目标调用并统计 pk 使用次数
	// 使用 ast.Inspect 递归遍历所有 AST 节点
	ast.Inspect(astFile, func(node ast.Node) bool {
		// 查找函数调用表达式，例如：AutoMigrate(pk, model)
		if node, ok := node.(*ast.CallExpr); ok {
			// 遍历调用的所有参数
			for i := range node.Args {
				pkOK := false    // 当前参数是否包含 pk
				modelOK := false // 当前参数是否包含 model

				// 递归检查参数内部是否包含 pk 和 model 标识符
				ast.Inspect(node.Args[i], func(item ast.Node) bool {
					if ii, ok := item.(*ast.Ident); ok {
						if ii.Name == pk {
							pkOK = true
							pkNum++ // 统计 pk 出现次数
						}
						if ii.Name == model {
							modelOK = true
						}
					}
					// 如果同时找到 pk 和 model，说明这是目标参数
					if pkOK && modelOK {
						n = node // 保存调用节点
						k = i    // 保存参数索引
					}
					return true
				})
			}
		}
		return true
	})

	// 如果找到了目标参数（k > -1），从调用表达式中删除该参数
	// 使用 append 技巧安全地删除切片元素，避免索引越界
	if k > -1 {
		n.Args = append(append([]ast.Expr{}, n.Args[:k]...), n.Args[k+1:]...)
	}

	// 如果 pk 只出现了一次（pkNum == 1），说明这是最后一次使用
	// 需要删除对应的 import，避免留下无用的导入
	if pkNum == 1 {
		var imI int = -1    // import 在 Specs 中的索引
		var gp *ast.GenDecl // 包含该 import 的声明节点

		// 第二次遍历：查找对应的 import 声明
		ast.Inspect(astFile, func(node ast.Node) bool {
			if gen, ok := node.(*ast.GenDecl); ok {
				// 遍历所有声明规范（可能是 import、var、const 等）
				for i := range gen.Specs {
					if imspec, ok := gen.Specs[i].(*ast.ImportSpec); ok {
						// 匹配目标包的 import 路径
						expectedPath := "\"github.com/flipped-aurora/gin-vue-admin/server/model/" + pk + "\""
						if imspec.Path.Value == expectedPath {
							gp = gen
							imI = i
							return false // 找到后停止遍历
						}
					}
				}
			}
			return true
		})

		// 如果找到了 import，从声明中删除
		if imI > -1 {
			gp.Specs = append(append([]ast.Spec{}, gp.Specs[:imI]...), gp.Specs[imI+1:]...)
		}
	}

	// 将修改后的 AST 写回文件
	// 使用 go/printer 自动格式化，保持代码风格一致
	var out []byte
	bf := bytes.NewBuffer(out)
	printer.Fprint(bf, fileSet, astFile)

	// 先删除原文件再写入，确保原子性操作
	os.Remove(path)
	os.WriteFile(path, bf.Bytes(), 0666)
}

// RollRouterBack 回滚路由初始化代码
// 功能：从 router_biz.go 文件中删除指定包和模型的路由初始化调用
//
// 代码结构示例：
//
//	func initBizRouter() {
//	    {
//	        xxxRouter := ...
//	        InitModelRouter(...)  // 需要删除这行
//	    }
//	}
//
// 设计思路：
// 1. 找到 initBizRouter 函数
// 2. 在函数体内找到包含 "pkRouter" 变量的代码块
// 3. 在该代码块中找到 "Init{model}Router" 调用并删除
// 4. 如果代码块变空，删除整个代码块
// 5. 清理所有空的代码块，保持代码整洁
//
// 这样设计的好处：
// - 层次化清理：先删除调用，再清理空块，最后清理空块列表
// - 代码整洁：自动清理无用的空代码块，避免留下冗余结构
// - 结构保持：保留其他路由的初始化代码，只删除目标部分
//
// 使用示例：
//
// 示例 1：删除单个路由初始化（代码块中只有一个路由初始化）
//
//	调用：RollRouterBack("system", "User")
//	原文件内容：
//	  func initBizRouter() {
//	      {
//	          systemRouter := router.Group("system")
//	          InitUserRouter(systemRouter)
//	      }
//	  }
//	执行后：
//	  func initBizRouter() {
//	      // 整个代码块被删除（因为只剩下变量声明）
//	  }
//
// 示例 2：删除多个路由初始化中的一个（代码块中有多个路由初始化）
//
//	调用：RollRouterBack("system", "User")
//	原文件内容：
//	  func initBizRouter() {
//	      {
//	          systemRouter := router.Group("system")
//	          InitUserRouter(systemRouter)
//	          InitRoleRouter(systemRouter)
//	          InitMenuRouter(systemRouter)
//	      }
//	  }
//	执行后：
//	  func initBizRouter() {
//	      {
//	          systemRouter := router.Group("system")
//	          InitRoleRouter(systemRouter)  // User 路由被删除
//	          InitMenuRouter(systemRouter)
//	      }
//	  }
//
// 示例 3：删除不同包的路由初始化
//
//	调用：RollRouterBack("example", "Customer")
//	原文件内容：
//	  func initBizRouter() {
//	      {
//	          exampleRouter := router.Group("example")
//	          InitCustomerRouter(exampleRouter)
//	          InitOrderRouter(exampleRouter)
//	      }
//	      {
//	          systemRouter := router.Group("system")
//	          InitUserRouter(systemRouter)
//	      }
//	  }
//	执行后：
//	  func initBizRouter() {
//	      {
//	          exampleRouter := router.Group("example")
//	          InitOrderRouter(exampleRouter)  // Customer 路由被删除
//	      }
//	      {
//	          systemRouter := router.Group("system")
//	          InitUserRouter(systemRouter)  // 保持不变
//	      }
//	  }
//
// 示例 4：删除后自动清理空代码块
//
//	调用：RollRouterBack("business", "Order")
//	原文件内容：
//	  func initBizRouter() {
//	      {
//	          businessRouter := router.Group("business")
//	          InitOrderRouter(businessRouter)
//	      }
//	      {
//	          systemRouter := router.Group("system")
//	          InitUserRouter(systemRouter)
//	      }
//	  }
//	执行后：
//	  func initBizRouter() {
//	      {
//	          systemRouter := router.Group("system")
//	          InitUserRouter(systemRouter)  // business 代码块被完全删除
//	      }
//	  }
//
// 示例 5：实际使用场景
//
//	// 当用户通过自动代码生成工具删除某个模型时
//	// 需要回滚相关的路由初始化代码
//	RollRouterBack("system", "Api")  // 删除 system 包下的 Api 路由初始化
func RollRouterBack(pk, model string) {
	// 构建目标文件路径：router_biz.go 是路由业务初始化文件
	path := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "initialize", "router_biz.go")
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 解析 AST
	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, "", src, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 用于存储找到的代码块和路由函数
	var block *ast.BlockStmt     // 包含 pkRouter 的代码块
	var routerStmt *ast.FuncDecl // initBizRouter 函数节点

	// 第一次遍历：找到 initBizRouter 函数和包含 pkRouter 的代码块
	ast.Inspect(astFile, func(node ast.Node) bool {
		// 查找 initBizRouter 函数声明
		if n, ok := node.(*ast.FuncDecl); ok {
			if n.Name.Name == "initBizRouter" {
				routerStmt = n
			}
		}

		// 查找代码块（BlockStmt），例如：{ xxxRouter := ... }
		if n, ok := node.(*ast.BlockStmt); ok {
			// 在代码块内部查找标识符
			ast.Inspect(n, func(bNode ast.Node) bool {
				if in, ok := bNode.(*ast.Ident); ok {
					// 如果找到 "pkRouter" 变量，说明这是目标代码块
					// 例如：xxxRouter := router.Group("xxx")
					if in.Name == pk+"Router" {
						block = n
						return false // 找到后停止遍历
					}
				}
				return true
			})
			return true
		}
		return true
	})

	// 在目标代码块中查找 "Init{model}Router" 调用
	// 例如：InitUserRouter(xxxRouter, ...)
	var k int
	for i := range block.List {
		// 查找表达式语句（函数调用通常是表达式语句）
		if stmtNode, ok := block.List[i].(*ast.ExprStmt); ok {
			ast.Inspect(stmtNode, func(node ast.Node) bool {
				if n, ok := node.(*ast.Ident); ok {
					// 匹配目标路由初始化函数名
					if n.Name == "Init"+model+"Router" {
						k = i // 保存语句索引
						return false
					}
				}
				return true
			})
		}
	}

	// 从代码块中删除目标语句
	block.List = append(append([]ast.Stmt{}, block.List[:k]...), block.List[k+1:]...)

	// 如果代码块只剩下一个语句（通常是变量声明），清空整个代码块
	// 因为空的代码块结构 { } 在 Go 中是没有意义的
	if len(block.List) == 1 {
		block.List = nil
	}

	// 清理所有空的代码块
	// 遍历 initBizRouter 函数体中的所有语句
	for i, n := range routerStmt.Body.List {
		if n, ok := n.(*ast.BlockStmt); ok {
			// 如果代码块为空，从函数体中删除
			if n.List == nil {
				routerStmt.Body.List = append(append([]ast.Stmt{}, routerStmt.Body.List[:i]...), routerStmt.Body.List[i+1:]...)
				i-- // 调整索引，因为删除了一个元素
			}
		}
	}

	// 将修改后的 AST 写回文件
	var out []byte
	bf := bytes.NewBuffer(out)
	printer.Fprint(bf, fileSet, astFile)
	os.Remove(path)
	os.WriteFile(path, bf.Bytes(), 0666)
}
