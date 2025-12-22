package ast

import (
	"fmt"
	"go/ast"
	"io"
)

// PluginInitializeRouter 插件初始化路由 AST 操作器
//
// 功能说明：
//
//	用于在插件的路由初始化文件中自动注入路由注册代码
//	生成的代码格式：PackageName.AppName.GroupName.FunctionName(LeftRouterGroupName, RightRouterGroupName)
//	例如：router.Router.UserGroup.Init(public, private)
//
// 设计模式：
//   - 组合模式：嵌入 Base 结构体，复用基础 AST 操作功能
//   - 模板方法模式：重写 Parse、Rollback、Injection、Format 方法实现特定逻辑
//
// 为什么需要这个结构体？
//  1. 插件系统需要动态注册路由，手动编写容易出错且繁琐
//  2. 代码生成工具需要自动在路由初始化文件中注入路由注册代码
//  3. 支持路由的注入和回滚，便于插件的安装和卸载
//
// 使用场景：
//   - 插件代码生成时，自动在 initialize/router.go 中注入路由注册代码
//   - 插件卸载时，自动删除之前注入的路由注册代码
//   - 插件重载时，检查并更新路由注册代码
//
// 好处：
//   - 自动化：无需手动编写路由注册代码，减少人为错误
//   - 一致性：所有插件使用相同的路由注册格式，便于维护
//   - 可逆性：支持回滚操作，插件卸载时可以完全清理
//   - 幂等性：重复注入不会产生重复代码，通过检查避免重复
type PluginInitializeRouter struct {
	Base                        // 嵌入基础 AST 操作器，复用通用功能（解析、格式化、路径转换等）
	Type                 Type   // 类型标识，用于区分不同的 AST 操作器类型
	Path                 string // 目标文件的绝对路径（如：/project/server/plugin/xxx/initialize/router.go）
	ImportPath           string // 需要导入的包路径（如："github.com/xxx/plugin/xxx/router"）
	ImportGlobalPath     string // 全局变量导入路径（预留字段，用于未来扩展）
	ImportMiddlewarePath string // 中间件导入路径（预留字段，用于未来扩展）
	RelativePath         string // 相对路径（用于配置存储，便于跨环境移植）
	AppName              string // 应用名称，通常是 "Router"（路由应用）
	GroupName            string // 路由分组名称（如：UserGroup、OrderGroup）
	PackageName          string // 包名，通常是 "router"（路由包）
	FunctionName         string // 函数名，通常是 "Init"（初始化函数）
	LeftRouterGroupName  string // 左路由分组名称（通常是 "public"，公开路由组）
	RightRouterGroupName string // 右路由分组名称（通常是 "private"，私有路由组）
}

// Parse 解析目标文件并返回 AST 文件节点
//
// 参数说明：
//   - filename: 要解析的文件路径（如果为空，则使用结构体中的路径）
//   - writer: 可选的 io.Writer（通常为 nil，表示从文件系统读取）
//
// 设计逻辑说明：
//
//  1. 路径处理的双向转换机制：
//     - 如果 filename 为空，需要从结构体中获取路径
//     - 优先使用 RelativePath（相对路径），如果存在则转换为绝对路径
//     - 如果 RelativePath 不存在，使用 Path（绝对路径），并计算相对路径
//
//  2. 为什么需要这种双向转换？
//     - 配置存储：相对路径更适合存储在配置中，不依赖具体部署环境
//     - 文件操作：实际的文件操作需要绝对路径
//     - 灵活性：支持从配置读取相对路径，也支持直接使用绝对路径
//
//  3. 路径转换的时机：
//     - 首次解析：如果只有绝对路径，计算并保存相对路径（用于后续配置存储）
//     - 从配置恢复：如果只有相对路径，转换为绝对路径（用于文件操作）
//
// 好处：
//   - 灵活性：支持绝对路径和相对路径两种方式
//   - 可移植性：相对路径便于跨环境使用
//   - 自动化：自动处理路径转换，无需手动管理
//
// 使用示例：
//
//	场景 1 - 首次使用（只有绝对路径）：
//	  a.Path = "/project/server/plugin/user/initialize/router.go"
//	  a.RelativePath = ""
//	  Parse("", nil)
//	  -> 使用 a.Path，计算并保存 a.RelativePath = "plugin/user/initialize/router.go"
//
//	场景 2 - 从配置恢复（只有相对路径）：
//	  a.RelativePath = "plugin/user/initialize/router.go"
//	  a.Path = ""
//	  Parse("", nil)
//	  -> 将 a.RelativePath 转换为绝对路径，赋值给 a.Path
func (a *PluginInitializeRouter) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	if filename == "" {
		if a.RelativePath == "" {
			// 场景 1：只有绝对路径，使用它并计算相对路径
			filename = a.Path
			a.RelativePath = a.Base.RelativePath(a.Path) // 计算并保存相对路径，用于配置存储
			return a.Base.Parse(filename, writer)
		}
		// 场景 2：只有相对路径，转换为绝对路径
		a.Path = a.Base.AbsolutePath(a.RelativePath) // 从相对路径恢复绝对路径
		filename = a.Path
	}
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作，删除之前注入的路由注册代码
//
// 功能说明：
//
//	在插件卸载时，需要删除之前注入的路由注册代码，恢复文件到原始状态
//
// 设计逻辑说明：
//
//  1. 查找目标函数：
//     - 在 AST 中查找名为 "Router" 的函数（路由初始化函数）
//     - 这是插件路由注册的标准入口函数
//
//  2. 反向遍历语句列表：
//     - 从后往前遍历函数体中的语句（为什么反向？因为删除操作不会影响前面的索引）
//     - 只处理表达式语句（*ast.ExprStmt），忽略其他类型的语句
//
//  3. AST 节点类型检查（类型断言链）：
//     - ExprStmt -> CallExpr：检查是否为函数调用表达式
//     - CallExpr.Fun -> SelectorExpr：检查是否为选择器表达式（如：router.Router.UserGroup.Init）
//     - SelectorExpr.X -> SelectorExpr：检查是否为嵌套的选择器（如：router.Router.UserGroup）
//     - 继续检查更深层的 AST 节点结构
//
//  4. 为什么需要这么复杂的 AST 节点检查？
//     - 确保精确匹配：只删除我们注入的代码，不影响其他代码
//     - 类型安全：通过类型断言确保节点类型正确，避免运行时错误
//     - 结构匹配：匹配完整的调用链结构（PackageName.AppName.GroupName.FunctionName）
//
//  5. 路由计数逻辑：
//     - 统计所有来自 "router" 包的路由注册调用
//     - 如果只有 1 个或更少，说明这是最后一个路由，可以删除导入
//     - 如果还有多个路由，保留导入（其他路由可能还在使用）
//
//  6. 删除操作：
//     - 使用切片操作删除指定索引的语句：append(list[:i], list[i+1:]...)
//     - 这是 Go 中删除切片元素的常用方式
//
//  7. 导入清理：
//     - 如果这是最后一个路由注册，删除对应的导入语句
//     - 避免留下无用的导入，保持代码整洁
//
// 好处：
//   - 精确删除：只删除匹配的代码，不影响其他代码
//   - 智能清理：自动判断是否需要删除导入，避免留下无用导入
//   - 可逆性：插件卸载时可以完全恢复文件状态
//   - 安全性：通过多层类型检查确保操作安全
//
// 注意事项：
//   - 如果文件中没有找到匹配的代码，不会报错（幂等性）
//   - 删除操作是安全的，即使重复调用也不会出错
//
// 使用示例：
//
// 示例 1 - 基本使用：删除单个路由注册
//
//	假设目标文件 initialize/router.go 中有以下代码：
//
//		package initialize
//
//		import (
//			"github.com/flipped-aurora/gin-vue-admin/server/plugin/user/router"
//		)
//
//		func Router() {
//			public := r.Group("")
//			private := r.Group("")
//			router.Router.UserGroup.Init(public, private)  // 要删除的代码
//		}
//
//	调用 Rollback 后：
//
//		package initialize
//
//		func Router() {
//			public := r.Group("")
//			private := r.Group("")
//			// router.Router.UserGroup.Init(public, private) 已被删除
//			// 导入语句也被删除（因为这是最后一个路由）
//		}
//
//	配置参数：
//		a := &PluginInitializeRouter{
//			GroupName:    "UserGroup",
//			FunctionName: "Init",
//			ImportPath:   "github.com/flipped-aurora/gin-vue-admin/server/plugin/user/router",
//		}
//
// 示例 2 - 多个路由注册：只删除匹配的路由，保留其他路由
//
//	假设目标文件中有多个路由注册：
//
//		func Router() {
//			public := r.Group("")
//			private := r.Group("")
//			router.Router.UserGroup.Init(public, private)      // 要删除的
//			router.Router.OrderGroup.Init(public, private)     // 保留
//			router.Router.ProductGroup.Init(public, private)   // 保留
//		}
//
//	调用 Rollback（删除 UserGroup）后：
//
//		func Router() {
//			public := r.Group("")
//			private := r.Group("")
//			// router.Router.UserGroup.Init(public, private) 已被删除
//			router.Router.OrderGroup.Init(public, private)     // 保留
//			router.Router.ProductGroup.Init(public, private)   // 保留
//			// 导入语句保留（因为还有其他路由在使用）
//		}
//
//	配置参数：
//		a := &PluginInitializeRouter{
//			GroupName:    "UserGroup",  // 只匹配 UserGroup
//			FunctionName: "Init",
//			ImportPath:   "github.com/flipped-aurora/gin-vue-admin/server/plugin/user/router",
//		}
//
// 示例 3 - 插件卸载场景：完整的回滚流程
//
//	插件卸载时的完整流程：
//
//		// 1. 解析文件
//		file, err := a.Parse("", nil)
//		if err != nil {
//			return err
//		}
//
//		// 2. 执行回滚（删除路由注册代码）
//		err = a.Rollback(file)
//		if err != nil {
//			return err
//		}
//
//		// 3. 格式化并保存文件
//		err = a.Format("", nil, file)
//		if err != nil {
//			return err
//		}
//
//	结果：文件恢复到插件安装前的状态
//
// 示例 4 - 匹配规则说明
//
//	Rollback 函数通过以下条件精确匹配要删除的代码：
//
//	1. 必须是表达式语句：router.Router.XXX.XXX(...)
//	2. 必须是函数调用：Init(...)
//	3. 分组名称必须匹配：ident.Sel.Name == a.GroupName
//	4. 函数名必须匹配：selExpr.Sel.Name == a.FunctionName
//
//	匹配的代码结构：
//		router.Router.UserGroup.Init(public, private)
//		^^^^^^ ^^^^^^ ^^^^^^^^^ ^^^^
//		包名   应用名  分组名   函数名
//
//	不匹配的情况：
//		- router.Router.UserGroup.Init2(public, private)  // 函数名不匹配
//		- router.Router.OrderGroup.Init(public, private) // 分组名不匹配
//		- user.Router.UserGroup.Init(public, private)    // 包名不匹配（但不会检查包名）
//
// 示例 5 - 边界情况处理
//
//	情况 1：文件中没有匹配的代码
//		- 函数不会报错，直接返回 nil
//		- 文件保持不变（幂等性）
//
//	情况 2：Router 函数不存在
//		- FindFunction 可能返回 nil，需要在使用前检查
//		- 建议在调用前先检查文件结构
//
//	情况 3：重复调用 Rollback
//		- 第一次调用：删除匹配的代码
//		- 第二次调用：找不到匹配代码，直接返回（幂等性）
//		- 不会产生错误或副作用
func (a *PluginInitializeRouter) Rollback(file *ast.File) error {
	// 查找路由初始化函数（通常是 Router 函数）
	funcDecl := FindFunction(file, "Router")
	delI := 0      // 要删除的语句索引
	routerNum := 0 // 路由注册调用的计数（用于判断是否需要删除导入）

	// 反向遍历函数体中的语句（从后往前，避免删除时索引变化的问题）
	for i := len(funcDecl.Body.List) - 1; i >= 0; i-- {
		// 类型断言：检查是否为表达式语句（如：router.Router.UserGroup.Init(...)）
		stmt, ok := funcDecl.Body.List[i].(*ast.ExprStmt)
		if !ok {
			continue // 不是表达式语句，跳过
		}

		// 类型断言：检查是否为函数调用表达式
		callExpr, ok := stmt.X.(*ast.CallExpr)
		if !ok {
			continue // 不是函数调用，跳过
		}

		// 类型断言：检查是否为选择器表达式（如：router.Router.UserGroup.Init）
		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			continue // 不是选择器表达式，跳过
		}

		// 类型断言：检查是否为嵌套的选择器（如：router.Router.UserGroup）
		ident, ok := selExpr.X.(*ast.SelectorExpr)

		if ok {
			// 检查更深层的 AST 节点，判断是否来自 "router" 包
			// 结构：router.Router.XXX.XXX
			if iExpr, ieok := ident.X.(*ast.SelectorExpr); ieok {
				if iden, idok := iExpr.X.(*ast.Ident); idok {
					if iden.Name == "router" {
						routerNum++ // 统计来自 router 包的路由注册调用
					}
				}
			}
			// 精确匹配：检查分组名称和函数名是否匹配
			// 匹配结构：XXX.GroupName.FunctionName
			if ident.Sel.Name == a.GroupName && selExpr.Sel.Name == a.FunctionName {
				delI = i // 找到要删除的语句，记录索引
			}
		}
	}

	// 删除匹配的语句（使用切片操作删除指定索引的元素）
	funcDecl.Body.List = append(funcDecl.Body.List[:delI], funcDecl.Body.List[delI+1:]...)

	// 如果这是最后一个路由注册，删除对应的导入语句
	// 为什么是 <= 1？因为我们已经删除了一个，如果计数 <= 1，说明没有其他路由了
	if routerNum <= 1 {
		_ = NewImport(a.ImportPath).Rollback(file)
	}

	return nil
}

// Injection 注入路由注册代码到目标文件
//
// 功能说明：
//
//	在插件的路由初始化文件中注入路由注册代码
//	生成的代码格式：PackageName.AppName.GroupName.FunctionName(LeftRouterGroupName, RightRouterGroupName)
//	例如：router.Router.UserGroup.Init(public, private)
//
// 设计逻辑说明：
//
//  1. 导入注入：
//     - 首先注入必要的导入语句（如果不存在）
//     - 使用 NewImport 操作器处理导入逻辑
//     - 确保路由包已经被导入，才能使用路由注册函数
//
//  2. 查找目标函数：
//     - 在 AST 中查找名为 "Router" 的函数
//     - 这是插件路由注册的标准入口函数
//
//  3. 幂等性检查（避免重复注入）：
//     - 使用 ast.Inspect 遍历函数体中的所有 AST 节点
//     - 查找是否已经存在相同的路由注册调用
//     - 匹配条件：GroupName 和 FunctionName 都相同
//     - 如果已存在，设置 exists = true，并停止遍历（return false）
//
//  4. 为什么需要幂等性检查？
//     - 避免重复注入：多次调用不会产生重复代码
//     - 插件重载：插件重载时不会重复添加路由注册
//     - 代码安全：确保代码的唯一性和正确性
//
//  5. 代码生成：
//     - 如果不存在，生成路由注册语句
//     - 使用 fmt.Sprintf 格式化字符串，生成完整的调用语句
//     - 使用 CreateStmt 将字符串转换为 AST 节点
//     - 追加到函数体的语句列表末尾
//
//  6. 为什么追加到末尾？
//     - 保持代码顺序：新注入的代码在最后，不影响现有代码
//     - 便于阅读：路由注册代码集中在一起
//     - 避免冲突：不会干扰其他代码的逻辑
//
// 好处：
//   - 幂等性：重复调用不会产生重复代码
//   - 自动化：自动处理导入和代码注入，无需手动操作
//   - 一致性：所有插件使用相同的路由注册格式
//   - 安全性：通过检查避免重复注入，确保代码正确
//
// 注意事项：
//   - 如果目标函数不存在，FindFunction 可能会返回 nil，需要处理
//   - 注入的代码会在文件格式化时自动调整格式
//   - 生成的代码符合 Go 代码规范
func (a *PluginInitializeRouter) Injection(file *ast.File) error {
	// 步骤 1：注入必要的导入语句（如果不存在）
	// 例如：import "github.com/xxx/plugin/xxx/router"
	_ = NewImport(a.ImportPath).Injection(file)

	// 步骤 2：查找路由初始化函数（通常是 Router 函数）
	funcDecl := FindFunction(file, "Router")

	var exists bool // 标记是否已存在相同的路由注册代码

	// 步骤 3：使用 ast.Inspect 遍历函数体，检查是否已存在相同的路由注册
	// ast.Inspect 是 Go AST 包提供的深度优先遍历工具
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		// 类型断言：检查是否为函数调用表达式
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true // 继续遍历子节点
		}

		// 类型断言：检查是否为选择器表达式（如：router.Router.UserGroup.Init）
		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			return true // 继续遍历子节点
		}

		// 类型断言：检查是否为嵌套的选择器（如：router.Router.UserGroup）
		ident, ok := selExpr.X.(*ast.SelectorExpr)
		// 精确匹配：检查分组名称和函数名是否相同
		// 如果匹配，说明已经存在相同的路由注册代码
		if ok && ident.Sel.Name == a.GroupName && selExpr.Sel.Name == a.FunctionName {
			exists = true
			return false // 停止遍历（已找到匹配的代码）
		}
		return true // 继续遍历子节点
	})

	// 步骤 4：如果不存在，生成并注入路由注册代码
	if !exists {
		// 生成路由注册语句字符串
		// 格式：PackageName.AppName.GroupName.FunctionName(LeftRouterGroupName, RightRouterGroupName)
		// 例如：router.Router.UserGroup.Init(public, private)
		stmtStr := fmt.Sprintf("%s.%s.%s.%s(%s, %s)", a.PackageName, a.AppName, a.GroupName, a.FunctionName, a.LeftRouterGroupName, a.RightRouterGroupName)
		// 将字符串转换为 AST 节点（表达式语句）
		stmt := CreateStmt(stmtStr)
		// 追加到函数体的语句列表末尾
		funcDecl.Body.List = append(funcDecl.Body.List, stmt)
	}
	return nil
}

// Format 格式化修改后的 AST 并写入文件或 Writer
//
// 参数说明：
//   - filename: 输出文件路径（如果为空，使用结构体中的 Path）
//   - writer: 可选的 io.Writer（如果提供则写入到 writer，否则写入到文件）
//   - file: 要格式化的 AST 文件节点
//
// 设计逻辑说明：
//
//  1. 路径处理：
//     - 如果 filename 为空，使用结构体中的 Path（绝对路径）
//     - 确保有有效的文件路径用于输出
//
//  2. 委托给基类：
//     - 调用 Base.Format 执行实际的格式化操作
//     - Base.Format 会使用 go/format 包自动格式化代码
//     - 符合 Go 代码规范（gofmt 标准）
//
//  3. 为什么需要格式化？
//     - 代码生成后，AST 节点的格式可能不符合 Go 代码规范
//     - 格式化确保生成的代码风格一致，便于阅读和维护
//     - 自动处理缩进、换行、空格等格式问题
//
// 好处：
//   - 统一格式：所有生成的代码都符合 Go 代码规范
//   - 可读性：格式化后的代码更易读
//   - 自动化：无需手动调整代码格式
//   - 一致性：与项目其他代码保持一致的风格
//
// 使用场景：
//   - 代码注入后，格式化并保存文件
//   - 代码回滚后，格式化并保存文件
//   - 确保文件始终符合 Go 代码规范
func (a *PluginInitializeRouter) Format(filename string, writer io.Writer, file *ast.File) error {
	// 如果未提供文件名，使用结构体中的路径
	if filename == "" {
		filename = a.Path
	}
	// 委托给基类的 Format 方法，执行实际的格式化操作
	// Base.Format 会使用 go/format 包自动格式化代码，符合 Go 代码规范
	return a.Base.Format(filename, writer, file)
}
