package ast

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
)

// PackageInitializeRouter 包初始化路由 AST 操作器
//
// 功能说明：
//
//	用于在路由初始化文件中自动注入和回滚路由注册代码
//	生成的代码格式：ModuleName := PackageName.AppName.GroupName
//	                ModuleName.FunctionName(LeftRouterGroupName, RightRouterGroupName)
//	例如：systemRouter := router.RouterGroupApp.System
//	      systemRouter.InitApiRouter(privateGroup, publicGroup)
//
// 设计模式：
//   - 组合模式：嵌入 Base 结构体，复用基础 AST 操作功能（解析、格式化等）
//   - 策略模式：通过 Injection 和 Rollback 方法实现代码注入和回滚策略
//
// 为什么需要这个结构体？
//  1. 自动代码生成：在生成新模块时，需要自动在路由初始化文件中注册路由
//  2. 避免手动错误：手动编写路由注册代码容易出错，且格式不统一
//  3. 支持回滚：删除模块时，需要能够自动清理之前注入的路由注册代码
//  4. 代码一致性：确保所有模块使用相同的路由注册格式和结构
//
// 使用场景：
//   - 代码生成工具生成新模块时，自动注入路由注册代码
//   - 删除模块时，自动回滚（删除）之前注入的路由注册代码
//   - 模块更新时，检查并更新路由注册代码
//
// 好处：
//   - 自动化：无需手动编写路由注册代码，减少人为错误
//   - 一致性：所有模块使用相同的路由注册格式，便于维护
//   - 可逆性：支持回滚操作，删除模块时可以完全清理
//   - 幂等性：重复注入不会产生重复代码，通过检查避免重复
//   - 类型安全：使用 AST 操作，比字符串替换更可靠
type PackageInitializeRouter struct {
	Base                        // 嵌入基础 AST 操作器，复用解析、格式化等功能
	Type                 Type   // 类型标识，用于区分不同的 AST 操作器
	Path                 string // 目标文件的绝对路径，用于文件操作
	ImportPath           string // 导入路径，用于添加 import 语句（如 "router"）
	RelativePath         string // 相对路径，用于跨平台路径处理
	AppName              string // 应用名称，路由组的第一层（如 "RouterGroupApp"）
	GroupName            string // 分组名称，路由组的第二层（如 "System"）
	ModuleName           string // 模块名称，用于变量命名（如 "systemRouter"）
	PackageName          string // 包名，用于构建选择器表达式（如 "router"）
	FunctionName         string // 函数名，要调用的路由初始化函数（如 "InitApiRouter"）
	RouterGroupName      string // 路由分组名称（已废弃，保留用于兼容）
	LeftRouterGroupName  string // 左路由分组名称，函数调用的第一个参数（如 "privateGroup"）
	RightRouterGroupName string // 右路由分组名称，函数调用的第二个参数（如 "publicGroup"）
}

// Parse 解析目标文件并返回 AST 文件节点
//
// 参数:
//   - filename: 要解析的文件路径，如果为空则使用结构体中的路径
//   - writer: 用于输出日志信息的写入器
//
// 返回:
//   - file: 解析后的 AST 文件节点
//   - err: 解析过程中的错误
//
// 实现逻辑：
//
//  1. 如果 filename 为空，需要从结构体中获取路径
//  2. 优先使用 RelativePath（相对路径），如果不存在则使用 Path（绝对路径）
//  3. 如果使用绝对路径，同时计算并保存相对路径，便于后续使用
//  4. 如果使用相对路径，转换为绝对路径后再解析
//  5. 最终调用 Base.Parse 进行实际的解析工作
//
// 为什么这样写？
//  1. 灵活性：支持传入文件名或使用结构体中的路径，适应不同调用场景
//  2. 路径统一：自动处理相对路径和绝对路径的转换，确保路径一致性
//  3. 缓存优化：计算并保存相对路径，避免重复计算
//  4. 职责分离：路径处理逻辑在这里完成，解析逻辑委托给 Base.Parse
//
// 好处：
//   - 灵活：支持多种路径输入方式
//   - 统一：自动处理路径格式，避免路径不一致问题
//   - 高效：缓存相对路径，避免重复计算
//   - 可维护：路径处理逻辑集中，便于修改
func (a *PackageInitializeRouter) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	if filename == "" {
		if a.RelativePath == "" {
			// 如果没有相对路径，使用绝对路径，并计算相对路径缓存起来
			filename = a.Path
			a.RelativePath = a.Base.RelativePath(a.Path)
			return a.Base.Parse(filename, writer)
		}
		// 如果有相对路径，转换为绝对路径后再解析
		a.Path = a.Base.AbsolutePath(a.RelativePath)
		filename = a.Path
	}
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作：删除之前注入的路由注册代码
//
// 参数:
//   - file: 要操作的 AST 文件节点
//
// 返回:
//   - error: 操作过程中的错误
//
// 实现逻辑：
//
//  1. 查找 initBizRouter 函数（路由初始化函数）
//  2. 遍历函数体中的所有语句，查找包含目标模块变量的代码块
//  3. 在找到的代码块中，统计所有对目标模块的调用次数（exprNum）
//  4. 查找并删除目标函数调用语句（ModuleName.FunctionName(...)）
//  5. 如果删除后该模块不再有任何调用，删除整个代码块（包括变量声明）
//
// 为什么这样写？
//  1. 精确匹配：通过 AST 节点类型检查，确保只删除目标代码，不会误删其他代码
//  2. 计数机制：先统计所有调用次数，删除目标调用后减1，确保完全清理
//  3. 块级清理：如果模块不再使用，删除整个代码块，保持代码整洁
//  4. 类型安全：使用类型断言逐层检查 AST 节点，比字符串匹配更可靠
//
// AST 节点结构说明：
//   - ExprStmt: 表达式语句，如 "systemRouter.InitApiRouter(...)"
//   - CallExpr: 函数调用表达式，包含函数名和参数
//   - SelectorExpr: 选择器表达式，如 "systemRouter.InitApiRouter"
//   - Ident: 标识符，如 "systemRouter"、"InitApiRouter"
//
// 使用示例 Demo：
//
//	示例 1：删除单个函数调用（模块还有其他调用，保留代码块）
//	  输入参数：
//	    ModuleName   = "systemRouter"
//	    FunctionName = "InitApiRouter"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	            systemRouter.InitMenuRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitMenuRouter(privateGroup, publicGroup)  // 保留，因为不是目标函数
//	        }
//	    }
//	  说明：只删除了 InitApiRouter 调用，InitMenuRouter 调用保留，代码块也保留
//
//	示例 2：删除最后一个函数调用（删除整个代码块）
//	  输入参数：
//	    ModuleName   = "systemRouter"
//	    FunctionName = "InitApiRouter"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	        {
//	            exampleRouter := router.RouterGroupApp.Example
//	            exampleRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            exampleRouter := router.RouterGroupApp.Example
//	            exampleRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  说明：删除了 systemRouter 的最后一个调用，整个代码块（包括变量声明）都被删除
//
//	示例 3：删除多个调用中的第一个（保留其他调用）
//	  输入参数：
//	    ModuleName   = "systemRouter"
//	    FunctionName = "InitApiRouter"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)      // 目标：删除这个
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)      // 保留
//	            systemRouter.InitMenuRouter(privateGroup, publicGroup)     // 保留
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)      // 保留
//	            systemRouter.InitMenuRouter(privateGroup, publicGroup)     // 保留
//	        }
//	    }
//	  说明：只删除第一个匹配的 InitApiRouter 调用，其他调用保留
//
//	示例 4：删除不同模块的调用（不影响其他模块）
//	  输入参数：
//	    ModuleName   = "exampleRouter"
//	    FunctionName = "InitApiRouter"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	        {
//	            exampleRouter := router.RouterGroupApp.Example
//	            exampleRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)  // 保留，不是目标模块
//	        }
//	    }
//	  说明：只删除 exampleRouter 的调用，systemRouter 的调用不受影响
//
//	示例 5：删除不存在的调用（幂等性）
//	  输入参数：
//	    ModuleName   = "userRouter"
//	    FunctionName = "InitApiRouter"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  说明：目标调用不存在，函数体保持不变，不会报错
//
//	示例 6：删除特定函数名（区分不同的初始化函数）
//	  输入参数：
//	    ModuleName   = "systemRouter"
//	    FunctionName = "InitMenuRouter"  // 只删除 InitMenuRouter，不删除其他函数
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	            systemRouter.InitMenuRouter(privateGroup, publicGroup)  // 目标：删除这个
//	            systemRouter.InitCasbinRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)      // 保留
//	            systemRouter.InitCasbinRouter(privateGroup, publicGroup)   // 保留
//	        }
//	    }
//	  说明：只删除 InitMenuRouter 调用，其他函数调用保留
//
// 好处：
//   - 精确：只删除目标代码，不会误删其他代码
//   - 完整：确保完全清理，包括变量声明和函数调用
//   - 安全：使用 AST 操作，不会破坏代码结构
//   - 智能：自动判断是否需要删除整个代码块
func (a *PackageInitializeRouter) Rollback(file *ast.File) error {
	// 查找路由初始化函数
	funcDecl := FindFunction(file, "initBizRouter")
	exprNum := 0 // 计数器：统计目标模块的调用次数

	// 遍历函数体中的所有语句，查找包含目标模块变量的代码块
	for i := range funcDecl.Body.List {
		if IsBlockStmt(funcDecl.Body.List[i]) {
			// 检查代码块中是否存在目标模块变量（如 systemRouter）
			if VariableExistsInBlock(funcDecl.Body.List[i].(*ast.BlockStmt), a.ModuleName) {
				// 遍历代码块中的所有语句
				for ii, stmt := range funcDecl.Body.List[i].(*ast.BlockStmt).List {
					// 检查语句是否为表达式语句（如函数调用）
					exprStmt, ok := stmt.(*ast.ExprStmt)
					if !ok {
						continue
					}
					// 检查表达式是否为函数调用表达式
					callExpr, ok := exprStmt.X.(*ast.CallExpr)
					if !ok {
						continue
					}
					// 检查是否为选择器表达式（如 systemRouter.InitApiRouter）
					selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					// 检查选择器的左侧是否为标识符（变量名）
					ident, ok := selExpr.X.(*ast.Ident)
					// 只要存在对目标模块的调用就计数（用于判断是否需要删除整个代码块）
					if ok && ident.Name == a.ModuleName {
						exprNum++
					}
					// 判断是否为目标函数调用（ModuleName.FunctionName）
					if !ok || ident.Name != a.ModuleName || selExpr.Sel.Name != a.FunctionName {
						continue
					}
					// 找到目标调用，计数器减1
					exprNum--
					// 从语句列表中移除目标函数调用语句
					funcDecl.Body.List[i].(*ast.BlockStmt).List = append(
						funcDecl.Body.List[i].(*ast.BlockStmt).List[:ii],
						funcDecl.Body.List[i].(*ast.BlockStmt).List[ii+1:]...,
					)
					// 如果不再存在任何调用，删除整个代码块（包括变量声明）
					if exprNum == 0 {
						funcDecl.Body.List = append(
							funcDecl.Body.List[:i],
							funcDecl.Body.List[i+1:]...,
						)
					}
					break
				}
				break
			}
		}
	}

	return nil
}

// Injection 注入操作：在路由初始化函数中添加路由注册代码
//
// 参数:
//   - file: 要操作的 AST 文件节点
//
// 返回:
//   - error: 操作过程中的错误
//
// 实现逻辑：
//
//  1. 查找 initBizRouter 函数（路由初始化函数）
//  2. 检查函数体中是否已存在目标模块变量的代码块
//  3. 如果不存在，创建新的代码块并添加变量声明语句
//  4. 在代码块中添加路由注册函数调用语句
//  5. 如果代码块是新创建的，将其添加到函数体中
//
// 为什么这样写？
//  1. 幂等性：先检查变量是否存在，避免重复创建变量声明
//  2. 代码组织：将同一模块的路由注册代码放在同一个代码块中，结构清晰
//  3. 增量添加：如果变量已存在，只需添加函数调用，不重复创建变量
//  4. 格式统一：使用 CreateStmt 生成标准格式的代码，保证代码风格一致
//
// 使用示例 Demo：
//
//	示例 1：首次注入 System 模块路由（变量不存在）
//	  输入参数：
//	    ModuleName           = "systemRouter"
//	    FunctionName         = "InitApiRouter"
//	    LeftRouterGroupName  = "privateGroup"
//	    RightRouterGroupName = "publicGroup"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        // 其他代码...
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        // 其他代码...
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//
//	示例 2：再次注入 System 模块路由（变量已存在，幂等性）
//	  输入参数：同示例 1
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体（添加新的函数调用）：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)  // 新增
//	        }
//	    }
//
//	示例 3：注入 Example 模块路由（新模块）
//	  输入参数：
//	    ModuleName           = "exampleRouter"
//	    FunctionName         = "InitApiRouter"
//	    LeftRouterGroupName  = "privateGroup"
//	    RightRouterGroupName = "publicGroup"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	        {
//	            exampleRouter := router.RouterGroupApp.Example
//	            exampleRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//
//	示例 4：注入自定义函数名的路由（如 InitMenuRouter）
//	  输入参数：
//	    ModuleName           = "systemRouter"
//	    FunctionName         = "InitMenuRouter"  // 不同的函数名
//	    LeftRouterGroupName  = "privateGroup"
//	    RightRouterGroupName = "publicGroup"
//	  操作前 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	        }
//	    }
//	  操作后 initBizRouter 函数体：
//	    func initBizRouter() {
//	        {
//	            systemRouter := router.RouterGroupApp.System
//	            systemRouter.InitApiRouter(privateGroup, publicGroup)
//	            systemRouter.InitMenuRouter(privateGroup, publicGroup)  // 新增
//	        }
//	    }
//
//	示例 5：使用不同的路由组名称
//	  输入参数：
//	    ModuleName           = "userRouter"
//	    FunctionName         = "InitApiRouter"
//	    LeftRouterGroupName  = "adminGroup"     // 不同的路由组
//	    RightRouterGroupName = "guestGroup"     // 不同的路由组
//	  操作后生成的代码：
//	    {
//	        userRouter := router.RouterGroupApp.User
//	        userRouter.InitApiRouter(adminGroup, guestGroup)
//	    }
//
// 生成的代码示例：
//   - 如果变量不存在，生成：
//     {
//     systemRouter := router.RouterGroupApp.System
//     systemRouter.InitApiRouter(privateGroup, publicGroup)
//     }
//   - 如果变量已存在，只添加：
//     systemRouter.InitApiRouter(privateGroup, publicGroup)
//
// 好处：
//   - 幂等：重复调用不会产生重复代码
//   - 智能：自动判断是否需要创建变量声明
//   - 整洁：代码块组织清晰，便于阅读和维护
//   - 安全：使用 AST 操作，不会破坏现有代码结构
func (a *PackageInitializeRouter) Injection(file *ast.File) error {
	// 查找路由初始化函数
	funcDecl := FindFunction(file, "initBizRouter")
	hasRouter := false          // 标记是否已存在目标模块变量
	var varBlock *ast.BlockStmt // 指向包含目标模块变量的代码块

	// 遍历函数体，查找是否已存在目标模块变量的代码块
	for i := range funcDecl.Body.List {
		if IsBlockStmt(funcDecl.Body.List[i]) {
			// 检查代码块中是否存在目标模块变量
			if VariableExistsInBlock(funcDecl.Body.List[i].(*ast.BlockStmt), a.ModuleName) {
				hasRouter = true
				varBlock = funcDecl.Body.List[i].(*ast.BlockStmt)
				break
			}
		}
	}

	// 如果不存在目标模块变量，创建新的代码块并添加变量声明
	if !hasRouter {
		stmt := a.CreateAssignStmt() // 创建变量声明语句（如 systemRouter := router.RouterGroupApp.System）
		varBlock = &ast.BlockStmt{
			List: []ast.Stmt{
				stmt,
			},
		}
	}

	// 创建路由注册函数调用语句（如 systemRouter.InitApiRouter(privateGroup, publicGroup)）
	routerStmt := CreateStmt(fmt.Sprintf("%s.%s(%s,%s)",
		a.ModuleName,
		a.FunctionName,
		a.LeftRouterGroupName,
		a.RightRouterGroupName))

	// 将路由注册语句添加到代码块中
	varBlock.List = append(varBlock.List, routerStmt)

	// 如果代码块是新创建的，将其添加到函数体中
	if !hasRouter {
		funcDecl.Body.List = append(funcDecl.Body.List, varBlock)
	}

	return nil
}

// Format 格式化 AST 节点并写入文件
//
// 参数:
//   - filename: 输出文件的路径，如果为空则使用结构体中的路径
//   - writer: 用于输出日志信息的写入器
//   - file: 要格式化的 AST 文件节点
//
// 返回:
//   - error: 格式化或写入过程中的错误
//
// 实现逻辑：
//
//  1. 如果文件名为空，使用结构体中保存的路径
//  2. 调用 Base.Format 进行格式化并写入文件
//
// 为什么这样写？
//  1. 默认值：提供默认路径，简化调用
//  2. 职责委托：格式化逻辑由 Base.Format 实现，这里只处理路径
//  3. 一致性：与 Parse 方法保持相同的路径处理逻辑
//
// 好处：
//   - 简单：提供默认路径，调用更方便
//   - 统一：格式化逻辑统一在 Base 中，便于维护
//   - 灵活：支持指定输出路径或使用默认路径
func (a *PackageInitializeRouter) Format(filename string, writer io.Writer, file *ast.File) error {
	if filename == "" {
		filename = a.Path
	}
	return a.Base.Format(filename, writer, file)
}

// CreateAssignStmt 创建变量声明赋值语句的 AST 节点
//
// 返回:
//   - *ast.AssignStmt: 赋值语句的 AST 节点
//
// 生成的代码格式：
//
//	ModuleName := PackageName.AppName.GroupName
//	例如：systemRouter := router.RouterGroupApp.System
//
// 示例 Demo：
//
//	示例 1：System 模块路由初始化
//	  输入参数：
//	    ModuleName  = "systemRouter"
//	    PackageName = "router"
//	    AppName     = "RouterGroupApp"
//	    GroupName   = "System"
//	  生成代码：
//	    systemRouter := router.RouterGroupApp.System
//
//	示例 2：Example 模块路由初始化
//	  输入参数：
//	    ModuleName  = "exampleRouter"
//	    PackageName = "router"
//	    AppName     = "RouterGroupApp"
//	    GroupName   = "Example"
//	  生成代码：
//	    exampleRouter := router.RouterGroupApp.Example
//
//	示例 3：User 模块路由初始化
//	  输入参数：
//	    ModuleName  = "userRouter"
//	    PackageName = "router"
//	    AppName     = "RouterGroupApp"
//	    GroupName   = "User"
//	  生成代码：
//	    userRouter := router.RouterGroupApp.User
//
//	示例 4：自定义包名和路由组
//	  输入参数：
//	    ModuleName  = "apiRouter"
//	    PackageName = "api"
//	    AppName     = "ApiGroup"
//	    GroupName   = "V1"
//	  生成代码：
//	    apiRouter := api.ApiGroup.V1
//
// 实现逻辑：
//
//  1. 创建左侧标识符（变量名），如 "systemRouter"
//  2. 创建右侧选择器表达式，构建嵌套的选择器链：
//     - 最外层：PackageName（如 "router"）
//     - 中间层：AppName（如 "RouterGroupApp"）
//     - 最内层：GroupName（如 "System"）
//  3. 组合成赋值语句，使用 := 操作符（短变量声明）
//
// AST 节点结构说明：
//   - Ident: 标识符节点，表示变量名或包名
//   - SelectorExpr: 选择器表达式，表示 "A.B" 这样的访问
//   - AssignStmt: 赋值语句，包含左侧（Lhs）、操作符（Tok）、右侧（Rhs）
//   - token.DEFINE: 表示 ":=" 操作符（短变量声明）
//
// 为什么这样写？
//  1. 结构清晰：逐层构建 AST 节点，代码可读性强
//  2. 类型安全：使用 AST 节点类型，比字符串拼接更可靠
//  3. 格式规范：生成的代码符合 Go 语言规范
//  4. 可扩展：如果需要修改生成格式，只需修改节点结构
//
// 好处：
//   - 类型安全：使用 AST 节点，编译器可以检查类型
//   - 格式正确：生成的代码格式规范，符合 Go 语言规范
//   - 可维护：结构清晰，便于理解和修改
//   - 灵活：可以轻松修改生成的代码格式
func (a *PackageInitializeRouter) CreateAssignStmt() *ast.AssignStmt {
	// 创建左侧变量标识符（如 systemRouter）
	ident := &ast.Ident{
		Name: a.ModuleName,
	}

	// 创建右侧的嵌套选择器表达式
	// 构建结构：PackageName.AppName.GroupName（如 router.RouterGroupApp.System）
	selector := &ast.SelectorExpr{
		// 中间层选择器：PackageName.AppName（如 router.RouterGroupApp）
		X: &ast.SelectorExpr{
			X:   &ast.Ident{Name: a.PackageName}, // 包名（如 router）
			Sel: &ast.Ident{Name: a.AppName},     // 应用名（如 RouterGroupApp）
		},
		Sel: &ast.Ident{Name: a.GroupName}, // 分组名（如 System）
	}

	// 创建赋值语句：ModuleName := PackageName.AppName.GroupName
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ident},    // 左侧：变量名
		Tok: token.DEFINE,         // 操作符：:=（短变量声明）
		Rhs: []ast.Expr{selector}, // 右侧：选择器表达式
	}

	return stmt
}
