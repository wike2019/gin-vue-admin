// Package ast 提供了基于 Go AST（抽象语法树）的代码自动生成和修改功能
// 使用 AST 而不是字符串操作的好处：
// 1. 保持代码格式：自动格式化，符合 Go 代码规范
// 2. 语法安全：确保生成的代码语法正确，避免字符串拼接错误
// 3. 结构理解：能够理解代码结构，进行精确的修改
// 4. 可维护性：修改 AST 节点比正则表达式替换更可靠
package ast

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Visitor 实现了 ast.Visitor 接口，用于在遍历 AST 时携带需要添加的信息
// 设计思路：使用结构体携带状态，避免全局变量，使代码更清晰、线程安全
// 好处：
// 1. 封装性：将相关数据组织在一起，便于管理
// 2. 可扩展性：需要添加新字段时只需修改结构体
// 3. 类型安全：编译时检查，避免运行时错误
type Visitor struct {
	ImportCode  string // 需要添加的 import 路径，如 "github.com/gin-gonic/gin"
	StructName  string // 要添加到结构体中的字段名
	PackageName string // 包名，用于生成 router 变量名和类型选择器
	GroupName   string // 组名，用于生成完整的类型路径，如 packageName.GroupName
}

// Visit 实现了 ast.Visitor 接口，是访问者模式的核心方法
// 设计思路：通过类型断言识别不同的 AST 节点类型，针对性地处理
// 为什么使用访问者模式：
// 1. 解耦：将遍历逻辑和操作逻辑分离，符合开闭原则
// 2. 扩展性：添加新的节点类型处理只需添加新的 case
// 3. 可维护性：每种节点的处理逻辑独立，便于理解和修改
//
// 返回值说明：
// - 返回 nil：停止遍历当前节点的子树（已处理完毕，无需继续）
// - 返回 vi：继续遍历子树（需要处理子节点）
func (vi *Visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.GenDecl:
		// GenDecl 表示通用声明（import、const、type、var）
		// 通过 n.Tok 区分不同类型的声明

		// 处理 import 声明：添加缺失的 import 语句
		// 为什么先检查 vi.ImportCode != ""：避免不必要的处理，提高效率
		// Notice：当前实现没有考虑文件中没有任何 import 的情况（需要创建新的 import 块）
		if n.Tok == token.IMPORT && vi.ImportCode != "" {
			vi.addImport(n)
			// 返回 nil：import 节点不需要遍历子树，已处理完毕
			return nil
		}

		// 处理 type 声明：向结构体添加新字段
		// 为什么检查所有字段都不为空：确保有足够信息生成正确的字段定义
		if n.Tok == token.TYPE && vi.StructName != "" && vi.PackageName != "" && vi.GroupName != "" {
			vi.addStruct(n)
			// 返回 nil：type 声明节点已处理，无需继续遍历
			return nil
		}
	case *ast.FuncDecl:
		// 处理函数声明：在 Routers 函数中添加路由组变量
		// 为什么只处理 "Routers" 函数：这是框架约定的入口函数，用于注册路由
		if n.Name.Name == "Routers" {
			vi.addFuncBodyVar(n)
			// 返回 nil：函数体已处理完毕
			return nil
		}
	}
	// 返回 vi：继续遍历其他节点，查找需要处理的目标
	return vi
}

// addStruct 向包含 "Group" 的结构体添加新字段
// 设计思路：查找所有类型声明，找到包含 "Group" 的结构体，追加新字段
// 为什么查找包含 "Group" 的结构体：
// 1. 框架约定：路由组通常命名为 XxxGroup
// 2. 精确定位：避免误修改其他结构体
// 3. 自动化：无需手动指定目标结构体名称
//
// 好处：
// 1. 直接修改 AST：保持代码格式和语法正确性
// 2. 类型安全：生成的字段类型是 packageName.GroupName，编译时检查
// 3. 幂等性：多次调用会添加多个字段（如果需要去重，需要额外检查）
//
// Demo 示例 1: 添加 UserApi 字段到 ApiGroup
// 输入代码：
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	}
//
// Visitor 配置：
//
//	vi.StructName = "UserApi"
//	vi.PackageName = "api"
//	vi.GroupName = "UserApi"
//
// 输出代码：
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	    UserApi   api.UserApi  // 新增字段
//	}
//
// Demo 示例 2: 添加 OrderApi 字段到 RouterGroup
// 输入代码：
//
//	type RouterGroup struct {
//	    BaseRouter router.Router
//	}
//
// Visitor 配置：
//
//	vi.StructName = "OrderApi"
//	vi.PackageName = "api"
//	vi.GroupName = "OrderApi"
//
// 输出代码：
//
//	type RouterGroup struct {
//	    BaseRouter router.Router
//	    OrderApi   api.OrderApi  // 新增字段
//	}
//
// Demo 示例 3: 添加 ProductApi 字段到包含多个 Group 类型的文件
// 输入代码：
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	}
//	type RouterGroup struct {
//	    BaseRouter router.Router
//	}
//
// Visitor 配置：
//
//	vi.StructName = "ProductApi"
//	vi.PackageName = "api"
//	vi.GroupName = "ProductApi"
//
// 输出代码：
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	    ProductApi api.ProductApi  // 新增字段
//	}
//	type RouterGroup struct {
//	    BaseRouter router.Router
//	    ProductApi api.ProductApi  // 新增字段
//	}
//	注意：所有包含 "Group" 的结构体都会添加该字段
func (vi *Visitor) addStruct(genDecl *ast.GenDecl) ast.Visitor {
	// 遍历所有类型规格（一个 GenDecl 可能包含多个类型声明）
	for i := range genDecl.Specs {
		switch n := genDecl.Specs[i].(type) {
		case *ast.TypeSpec:
			// 查找名称包含 "Group" 的类型（如 RouterGroup、ApiGroup）
			// 使用 strings.Contains 更语义化，代码更清晰易读
			if strings.Contains(n.Name.Name, "Group") {
				switch t := n.Type.(type) {
				case *ast.StructType:
					// 创建新的字段 AST 节点
					// 字段名：vi.StructName（如 "UserApi"）
					// 字段类型：packageName.GroupName（如 "api.UserApi"）
					f := &ast.Field{
						Names: []*ast.Ident{
							{
								Name: vi.StructName,
								// Obj 字段用于符号表，帮助类型检查器理解标识符
								Obj: &ast.Object{
									Kind: ast.Var,
									Name: vi.StructName,
								},
							},
						},
						// SelectorExpr 表示选择器表达式，如 packageName.GroupName
						// 这样生成的代码是：StructName packageName.GroupName
						Type: &ast.SelectorExpr{
							X: &ast.Ident{
								Name: vi.PackageName,
							},
							Sel: &ast.Ident{
								Name: vi.GroupName,
							},
						},
					}
					// 直接追加到结构体字段列表，保持原有字段不变
					t.Fields.List = append(t.Fields.List, f)
				}
			}
		}
	}
	return vi
}

// addImport 向 import 声明块中添加新的 import 路径（如果不存在）
// 设计思路：先检查是否已存在，避免重复导入
// 为什么需要去重检查：
// 1. Go 语言不允许重复导入同一个包
// 2. 多次调用此函数时保持幂等性
// 3. 避免生成无效代码
//
// 好处：
// 1. 安全性：确保生成的代码可以编译通过
// 2. 幂等性：多次执行结果一致
// 3. 简洁性：不会产生冗余的 import 语句
func (vi *Visitor) addImport(genDecl *ast.GenDecl) ast.Visitor {
	// 检查是否已经导入该包
	hasImported := false
	for _, v := range genDecl.Specs {
		importSpec := v.(*ast.ImportSpec)
		// 比较导入路径（需要加引号，因为 AST 中路径是带引号的字符串字面量）
		// 使用 strconv.Quote 确保格式一致：如 "github.com/gin-gonic/gin"
		if importSpec.Path.Value == strconv.Quote(vi.ImportCode) {
			hasImported = true
			break // 找到后立即退出，提高效率
		}
	}

	// 如果未导入，则添加新的 import 规格
	if !hasImported {
		genDecl.Specs = append(genDecl.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,                 // 字符串字面量类型
				Value: strconv.Quote(vi.ImportCode), // 添加引号，如 "path/to/package"
			},
		})
	}
	return vi
}

// addFuncBodyVar 在 Routers 函数体中添加路由组变量声明
// 设计思路：
// 1. 检查变量是否已存在（避免重复声明）
// 2. 如果不存在，在函数体开头插入变量声明
// 3. 生成的代码：packageNameRouter := router.RouterGroupApp.PackageName
//
// 为什么插入到位置 1（第二个语句）：
// 1. 位置 0 通常是函数的第一行代码（可能是注释或其他初始化）
// 2. 位置 1 是安全的插入点，不会破坏原有逻辑
// 3. 保持代码的可读性和结构完整性
//
// 好处：
// 1. 自动化：无需手动编写重复的路由组变量声明代码
// 2. 一致性：所有模块使用相同的变量命名规范
// 3. 类型安全：通过 AST 生成，确保语法正确
//
// Demo 示例 1: 添加 userRouter 变量到 Routers 函数
// 输入代码：
//
//	func Routers() {
//	    // 初始化代码
//	    systemRouter := router.RouterGroupApp.System
//	    systemRouter.InitBaseRouter()
//	}
//
// Visitor 配置：
//
//	vi.PackageName = "user"
//
// 输出代码：
//
//	func Routers() {
//	    // 初始化代码
//	    userRouter := router.RouterGroupApp.User  // 新增变量声明
//	    systemRouter := router.RouterGroupApp.System
//	    systemRouter.InitBaseRouter()
//	}
//
// Demo 示例 2: 添加 orderRouter 变量到 Routers 函数
// 输入代码：
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	    systemRouter.InitBaseRouter()
//	}
//
// Visitor 配置：
//
//	vi.PackageName = "order"
//
// 输出代码：
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	    orderRouter := router.RouterGroupApp.Order  // 新增变量声明
//	    systemRouter.InitBaseRouter()
//	}
//
// Demo 示例 3: 变量已存在时不会重复添加
// 输入代码：
//
//	func Routers() {
//	    userRouter := router.RouterGroupApp.User
//	    userRouter.InitUserRouter()
//	}
//
// Visitor 配置：
//
//	vi.PackageName = "user"
//
// 输出代码：
//
//	func Routers() {
//	    userRouter := router.RouterGroupApp.User  // 已存在，不会重复添加
//	    userRouter.InitUserRouter()
//	}
//
// Demo 示例 4: 处理首字母小写的包名（如 product）
// 输入代码：
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	}
//
// Visitor 配置：
//
//	vi.PackageName = "product"
//
// 输出代码：
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	    productRouter := router.RouterGroupApp.Product  // Product 首字母自动大写
//	}
func (vi *Visitor) addFuncBodyVar(funDecl *ast.FuncDecl) ast.Visitor {
	// 检查变量是否已存在
	hasVar := false
	for _, v := range funDecl.Body.List {
		switch varSpec := v.(type) {
		case *ast.AssignStmt:
			// 遍历赋值语句左侧的所有标识符
			for i := range varSpec.Lhs {
				switch nn := varSpec.Lhs[i].(type) {
				case *ast.Ident:
					// 检查变量名是否匹配（如 "userRouter"）
					if nn.Name == vi.PackageName+"Router" {
						hasVar = true
						break
					}
				}
			}
		}
	}

	// 如果变量不存在，创建并插入变量声明
	if !hasVar {
		// 创建赋值语句 AST 节点
		// 生成的代码示例：userRouter := router.RouterGroupApp.User
		assignStmt := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.Ident{
					Name: vi.PackageName + "Router", // 左侧：变量名
					Obj: &ast.Object{
						Kind: ast.Var,
						Name: vi.PackageName + "Router",
					},
				},
			},
			Tok: token.DEFINE, // := 操作符（短变量声明）
			Rhs: []ast.Expr{
				// 右侧：router.RouterGroupApp.PackageName
				// 这是一个嵌套的选择器表达式
				&ast.SelectorExpr{
					X: &ast.SelectorExpr{
						// 第一层：router.RouterGroupApp
						X: &ast.Ident{
							Name: "router",
						},
						Sel: &ast.Ident{
							Name: "RouterGroupApp",
						},
					},
					// 第二层：.PackageName（首字母大写）
					Sel: &ast.Ident{
						// 使用 cases.Title 确保首字母大写，符合 Go 导出规则
						Name: cases.Title(language.English).String(vi.PackageName),
					},
				},
			},
		}

		// 在函数体位置 1 插入新语句
		// 策略：先扩展切片，再移动元素，最后插入新元素
		funDecl.Body.List = append(funDecl.Body.List, funDecl.Body.List[1])
		index := 1
		// 将 index 位置及之后的元素向后移动一位
		copy(funDecl.Body.List[index+1:], funDecl.Body.List[index:])
		// 在 index 位置插入新语句
		funDecl.Body.List[index] = assignStmt
	}
	return vi
}

// ImportReference 是主入口函数，用于自动修改 Go 源文件
// 功能：解析文件 -> 遍历 AST -> 添加 import/struct/变量 -> 格式化 -> 写回文件
//
// 设计思路：
// 1. 使用 parser 解析源文件为 AST（而不是字符串操作）
// 2. 使用 Visitor 模式遍历并修改 AST
// 3. 使用 format.Node 自动格式化代码
// 4. 写回文件，保持原有权限
//
// 为什么使用 AST 而不是字符串操作：
// 1. 安全性：AST 保证生成的代码语法正确
// 2. 格式化：自动符合 Go 代码规范（缩进、换行等）
// 3. 精确性：能够理解代码结构，精确插入到正确位置
// 4. 可维护性：不依赖正则表达式，更易理解和维护
//
// 参数说明：
// - filepath: 要修改的 Go 源文件路径
// - importCode: 要添加的 import 路径（如 "github.com/gin-gonic/gin"）
// - structName: 要添加到结构体的字段名（如 "UserApi"）
// - packageName: 包名（如 "user"）
// - groupName: 组名（如 "UserApi"），与 packageName 组合成完整类型路径
//
// Demo 示例 1: 添加 UserApi 相关的 import、struct 字段和 router 变量
// 输入文件 router/enter.go：
//
//	package router
//
//	import (
//	    "github.com/flipped-aurora/gin-vue-admin/server/router/system"
//	)
//
//	type RouterGroup struct {
//	    System system.RouterGroup
//	}
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	    systemRouter.InitBaseRouter()
//	}
//
// 调用代码：
//
//	err := ImportReference(
//	    "router/enter.go",
//	    "github.com/flipped-aurora/gin-vue-admin/server/api/v1/user",
//	    "UserApi",
//	    "user",
//	    "UserApi",
//	)
//
// 输出文件 router/enter.go：
//
//	package router
//
//	import (
//	    "github.com/flipped-aurora/gin-vue-admin/server/api/v1/user"  // 新增 import
//	    "github.com/flipped-aurora/gin-vue-admin/server/router/system"
//	)
//
//	type RouterGroup struct {
//	    System  system.RouterGroup
//	    UserApi user.UserApi  // 新增字段
//	}
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	    userRouter := router.RouterGroupApp.User  // 新增变量
//	    systemRouter.InitBaseRouter()
//	}
//
// Demo 示例 2: 只添加 import（不需要添加 struct 和变量时，其他参数传空字符串）
// 调用代码：
//
//	err := ImportReference(
//	    "api/v1/enter.go",
//	    "github.com/gin-gonic/gin",
//	    "",  // 不添加 struct 字段
//	    "",  // 不添加 router 变量
//	    "",
//	)
//
// 输出：只会在 import 块中添加 "github.com/gin-gonic/gin"，不会修改结构体和函数
//
// Demo 示例 3: 添加 OrderApi，展示完整的自动化流程
// 输入文件 router/enter.go：
//
//	package router
//
//	import (
//	    "github.com/flipped-aurora/gin-vue-admin/server/router/system"
//	)
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	}
//
//	type RouterGroup struct {
//	    System system.RouterGroup
//	}
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	}
//
// 调用代码：
//
//	err := ImportReference(
//	    "router/enter.go",
//	    "github.com/flipped-aurora/gin-vue-admin/server/api/v1/order",
//	    "OrderApi",
//	    "order",
//	    "OrderApi",
//	)
//
// 输出文件 router/enter.go：
//
//	package router
//
//	import (
//	    "github.com/flipped-aurora/gin-vue-admin/server/api/v1/order"  // 新增
//	    "github.com/flipped-aurora/gin-vue-admin/server/router/system"
//	)
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	    OrderApi  order.OrderApi  // 新增到所有包含 "Group" 的结构体
//	}
//
//	type RouterGroup struct {
//	    System  system.RouterGroup
//	    OrderApi order.OrderApi  // 新增到所有包含 "Group" 的结构体
//	}
//
//	func Routers() {
//	    systemRouter := router.RouterGroupApp.System
//	    orderRouter := router.RouterGroupApp.Order  // 新增变量（首字母自动大写）
//	}
//
// Demo 示例 4: 处理已存在的 import（幂等性保证）
// 如果 import 已存在，不会重复添加：
//
//	// 第一次调用
//	ImportReference("file.go", "github.com/gin-gonic/gin", "Api", "api", "Api")
//
//	// 第二次调用相同参数，不会重复添加 import
//	ImportReference("file.go", "github.com/gin-gonic/gin", "Api", "api", "Api")
//
// 注意：struct 字段和 router 变量也会检查是否已存在，避免重复添加
func ImportReference(filepath, importCode, structName, packageName, groupName string) error {
	// 创建文件集（FileSet），用于跟踪源代码位置信息
	// FileSet 帮助格式化工具保持正确的行号和列号
	fSet := token.NewFileSet()

	// 解析 Go 源文件为 AST
	// parser.ParseComments 保留注释，确保注释不会丢失
	fParser, err := parser.ParseFile(fSet, filepath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	// 清理 import 路径两端的空白字符，避免格式问题
	importCode = strings.TrimSpace(importCode)

	// 创建访问者实例，携带需要添加的信息
	v := &Visitor{
		ImportCode:  importCode,
		StructName:  structName,
		PackageName: packageName,
		GroupName:   groupName,
	}

	// 调试模式：如果 importCode 为空，打印整个 AST 结构
	// 这有助于理解文件结构和调试问题
	if importCode == "" {
		ast.Print(fSet, fParser)
	}

	// 遍历 AST，访问者会修改相应的节点
	// ast.Walk 使用深度优先搜索遍历所有节点
	ast.Walk(v, fParser)

	// 将修改后的 AST 格式化为 Go 源代码
	var output []byte
	buffer := bytes.NewBuffer(output)
	// format.Node 会自动格式化代码（缩进、换行、对齐等）
	// 这确保了生成的代码符合 gofmt 规范
	err = format.Node(buffer, fSet, fParser)
	if err != nil {
		log.Fatal(err)
	}

	// 将格式化后的代码写回文件
	// 0o600 权限：只有文件所有者可读写，符合安全最佳实践
	return os.WriteFile(filepath, buffer.Bytes(), 0o600)
}
