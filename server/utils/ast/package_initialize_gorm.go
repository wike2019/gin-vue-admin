package ast

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
)

// PackageInitializeGorm 包初始化gorm
// 该结构体用于在gorm初始化文件中自动注入或回滚结构体的AutoMigrate调用
// 通过AST操作实现代码的自动化管理，避免手动维护数据库迁移代码
// 好处：
// 1. 自动化管理：新增模型时自动添加到AutoMigrate，删除时自动移除
// 2. 减少错误：避免手动编辑时遗漏或重复添加
// 3. 代码一致性：确保所有模型都按照统一的方式注册到数据库迁移中
type PackageInitializeGorm struct {
	Base              // 基础AST操作能力，提供文件解析、格式化等通用功能
	Type       Type   // 类型：标识当前操作的类型（注入/回滚等）
	Path       string // 文件路径：目标初始化文件的绝对路径，用于定位需要修改的文件
	ImportPath string // 导包路径：需要导入的包路径，用于确保模型包被正确导入
	Business   string // 业务库：业务数据库名称，如"gva"表示gva业务库
	// 注意：传入"gva"而不是"gva"，用于区分不同的业务数据库实例
	// 空字符串表示使用默认的"db"变量
	StructName   string // 结构体名称：要注册到AutoMigrate的模型结构体名称
	PackageName  string // 包名：结构体所在的包名，用于生成完整的类型引用（如：package.Model）
	RelativePath string // 相对路径：相对于项目根目录的路径，用于跨平台兼容性
	IsNew        bool   // 是否使用new关键字：true使用new(PackageName.StructName)，false使用&PackageName.StructName{}
	// 虽然定义了但当前代码中未使用，预留用于未来扩展不同的实例化方式
}

// Parse 解析Go源文件为AST
// 为什么这么写：
// 1. 支持两种路径输入方式：绝对路径(Path)和相对路径(RelativePath)，提高灵活性
// 2. 自动转换：当只有一种路径时，自动计算并保存另一种路径，便于后续操作
// 3. 委托给Base：复用基础解析逻辑，避免重复代码
// 好处：
// - 调用方无需关心路径格式，系统自动处理
// - 路径信息自动补全，后续操作可直接使用
func (a *PackageInitializeGorm) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	if filename == "" {
		// 如果没有提供文件名，使用结构体中保存的路径
		if a.RelativePath == "" {
			// 优先使用绝对路径，并计算相对路径保存
			filename = a.Path
			a.RelativePath = a.Base.RelativePath(a.Path)
			return a.Base.Parse(filename, writer)
		}
		// 如果有相对路径，转换为绝对路径
		// 这样设计的好处：支持跨平台，相对路径在不同环境下都能正确解析
		a.Path = a.Base.AbsolutePath(a.RelativePath)
		filename = a.Path
	}
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作：从AutoMigrate调用中删除指定的结构体参数
// 为什么这么写：
// 1. 使用ast.Inspect遍历整个AST树，确保找到所有相关的AutoMigrate调用
// 2. 多层类型断言：通过类型断言逐步缩小范围，精确定位目标代码
// 3. 计数机制：统计包名出现次数，判断是否需要删除导入（只有最后一个时才删除）
// 好处：
// - 精确匹配：通过包名+结构体名双重匹配，避免误删其他结构体
// - 智能清理：只有当包中最后一个结构体被移除时才删除导入，避免破坏其他引用
// - 安全删除：使用切片操作安全删除元素，避免索引越界
//
// 示例1：删除默认db的AutoMigrate调用
// 处理前的代码：
//
//	import "gin-vue-admin/server/model/system"
//	func init() {
//	    db.AutoMigrate(
//	        &system.User{},
//	        &system.Role{},
//	        &system.Menu{},
//	    )
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "",           // 空字符串表示使用默认db
//	    PackageName: "system",
//	    StructName:  "User",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Rollback(file)
//
// 处理后的代码：
//
//	import "gin-vue-admin/server/model/system"
//	func init() {
//	    db.AutoMigrate(
//	        &system.Role{},
//	        &system.Menu{},
//	    )
//	}
//
// 示例2：删除业务数据库的AutoMigrate调用
// 处理前的代码：
//
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate(
//	        &system.User{},
//	        &example.Customer{},
//	    )
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "gva",        // 业务数据库名称
//	    PackageName: "system",
//	    StructName:  "User",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Rollback(file)
//
// 处理后的代码：
//
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate(
//	        &example.Customer{},
//	    )
//	}
//
// 示例3：删除包中最后一个结构体时自动删除导入
// 处理前的代码：
//
//	import "gin-vue-admin/server/model/system"
//	func init() {
//	    db.AutoMigrate(
//	        &system.User{},  // 这是system包中最后一个结构体
//	    )
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "",
//	    PackageName: "system",
//	    StructName:  "User",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Rollback(file)
//
// 处理后的代码（导入也被删除）：
//
//	func init() {
//	    db.AutoMigrate()
//	}
//
// 示例4：多个AutoMigrate调用场景
// 处理前的代码：
//
//	func init() {
//	    db.AutoMigrate(&system.User{})
//	    db.AutoMigrate(&system.Role{})
//	    gvaDb.AutoMigrate(&system.Menu{})
//	}
//
// 调用示例（只删除db.AutoMigrate中的User）：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "",
//	    PackageName: "system",
//	    StructName:  "User",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Rollback(file)
//
// 处理后的代码：
//
//	func init() {
//	    db.AutoMigrate(&system.Role{})
//	    gvaDb.AutoMigrate(&system.Menu{})
//	}
func (a *PackageInitializeGorm) Rollback(file *ast.File) error {
	packageNameNum := 0 // 统计当前包名在AutoMigrate中出现的次数
	// 使用ast.Inspect遍历AST树，查找所有节点
	// 为什么用Inspect：它是Go AST包提供的标准遍历方法，能递归访问所有节点
	ast.Inspect(file, func(n ast.Node) bool {
		// 根据business字段动态确定db变量名
		// 设计原因：支持多数据库场景，不同业务使用不同的db变量
		varDB := a.Business + "Db"

		if a.Business == "" {
			// 空字符串表示使用默认的"db"变量
			varDB = "db"
		}

		// 第一步：检查是否是函数调用表达式
		// 为什么先检查CallExpr：AutoMigrate是方法调用，必须先匹配调用表达式
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true // 继续遍历其他节点
		}

		// 第二步：检查调用的方法名是否是"AutoMigrate"
		// SelectorExpr表示选择器表达式，如 db.AutoMigrate
		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok || selExpr.Sel.Name != "AutoMigrate" {
			return true // 不是AutoMigrate调用，继续查找
		}

		// 第三步：检查调用方是否是目标db变量
		// 为什么检查调用方：确保只处理正确的数据库实例的AutoMigrate
		ident, ok := selExpr.X.(*ast.Ident)
		if !ok || ident.Name != varDB {
			return true // 不是目标db变量，跳过
		}

		// 第四步：遍历AutoMigrate的参数，查找并删除目标结构体
		// 为什么用for循环而不是range：删除元素时需要修改索引
		for i := 0; i < len(callExpr.Args); i++ {
			// CompositeLit表示复合字面量，如 &package.Model{}
			if com, comok := callExpr.Args[i].(*ast.CompositeLit); comok {
				// SelectorExpr表示选择器类型，如 package.Model
				if selector, exprok := com.Type.(*ast.SelectorExpr); exprok {
					// Ident表示标识符，如 package
					if x, identok := selector.X.(*ast.Ident); identok {
						// 匹配包名
						if x.Name == a.PackageName {
							packageNameNum++ // 统计该包出现的次数
							// 匹配结构体名
							if selector.Sel.Name == a.StructName {
								// 删除匹配的参数：使用切片操作删除第i个元素
								// 为什么用append：这是Go中删除切片元素的惯用法
								callExpr.Args = append(callExpr.Args[:i], callExpr.Args[i+1:]...)
								i-- // 调整索引，因为删除了一个元素
							}
						}
					}
				}
			}
		}
		return true // 继续遍历其他节点
	})

	// 只有当包中最后一个结构体被移除时，才删除导入
	// 为什么这样设计：避免删除导入后影响其他仍在使用该包的结构体
	if packageNameNum == 1 {
		_ = NewImport(a.ImportPath).Rollback(file)
	}
	return nil
}

// Injection 注入操作：向AutoMigrate调用中添加结构体参数
// 为什么这么写：
// 1. 先确保导入存在：避免添加引用时出现未导入的错误
// 2. 处理bizModel函数：如果是业务模型初始化函数，需要创建对应的db变量
// 3. 遍历查找AutoMigrate：找到所有匹配的AutoMigrate调用并添加参数
// 好处：
// - 自动化注入：新增模型时自动添加到迁移列表，无需手动编辑
// - 智能处理：自动处理导入和db变量，确保代码完整性
// - 幂等性：多次调用不会重复添加（需要配合检查逻辑）
//
// 示例1：向默认db的AutoMigrate调用中注入结构体
// 处理前的代码：
//
//	func init() {
//	    db.AutoMigrate(
//	        &system.User{},
//	        &system.Role{},
//	    )
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "",                              // 空字符串表示使用默认db
//	    PackageName: "system",
//	    StructName:  "Menu",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Injection(file)
//
// 处理后的代码：
//
//	import "gin-vue-admin/server/model/system"
//	func init() {
//	    db.AutoMigrate(
//	        &system.User{},
//	        &system.Role{},
//	        &system.Menu{},  // 新添加的结构体
//	    )
//	}
//
// 示例2：向业务数据库的AutoMigrate调用中注入结构体
// 处理前的代码：
//
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate(
//	        &system.User{},
//	    )
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "gva",                           // 业务数据库名称
//	    PackageName: "example",
//	    StructName:  "Customer",
//	    ImportPath:  "gin-vue-admin/server/model/example",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Injection(file)
//
// 处理后的代码：
//
//	import "gin-vue-admin/server/model/example"
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate(
//	        &system.User{},
//	        &example.Customer{},  // 新添加的结构体
//	    )
//	}
//
// 示例3：在空的bizModel函数中自动创建db变量和AutoMigrate调用
// 处理前的代码：
//
//	func bizModel() {
//	    return
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "gva",
//	    PackageName: "system",
//	    StructName:  "User",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Injection(file)
//
// 处理后的代码：
//
//	import "gin-vue-admin/server/model/system"
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate(
//	        &system.User{},
//	    )
//	    return
//	}
//
// 示例4：多个AutoMigrate调用场景，只向匹配的调用中注入
// 处理前的代码：
//
//	func init() {
//	    db.AutoMigrate(&system.User{})
//	    db.AutoMigrate(&system.Role{})
//	    gvaDb.AutoMigrate(&system.Menu{})
//	}
//
// 调用示例（只向db.AutoMigrate中注入）：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "",                              // 只匹配db变量
//	    PackageName: "system",
//	    StructName:  "Api",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Injection(file)
//
// 处理后的代码：
//
//	import "gin-vue-admin/server/model/system"
//	func init() {
//	    db.AutoMigrate(&system.User{}, &system.Api{})  // 新添加
//	    db.AutoMigrate(&system.Role{}, &system.Api{})  // 新添加
//	    gvaDb.AutoMigrate(&system.Menu{})              // 未修改（不匹配gvaDb）
//	}
//
// 示例5：自动添加导入语句
// 处理前的代码（没有导入system包）：
//
//	func init() {
//	    db.AutoMigrate(&example.Customer{})
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business:    "",
//	    PackageName: "system",
//	    StructName:  "User",
//	    ImportPath:  "gin-vue-admin/server/model/system",
//	}
//	file, _ := pkg.Parse("", nil)
//	pkg.Injection(file)
//
// 处理后的代码（自动添加导入）：
//
//	import "gin-vue-admin/server/model/system"
//	func init() {
//	    db.AutoMigrate(
//	        &example.Customer{},
//	        &system.User{},  // 新添加，导入也被自动添加
//	    )
//	}
func (a *PackageInitializeGorm) Injection(file *ast.File) error {
	// 第一步：确保导入语句存在
	// 为什么先处理导入：添加结构体引用前必须先导入对应的包
	_ = NewImport(a.ImportPath).Injection(file)

	// 第二步：查找bizModel函数，如果是业务模型初始化函数，需要特殊处理
	// 为什么查找bizModel：业务模型可能需要单独的数据库连接
	bizModelDecl := FindFunction(file, "bizModel")
	if bizModelDecl != nil {
		// 在bizModel函数中添加业务数据库变量和AutoMigrate调用
		a.addDbVar(bizModelDecl.Body)
	}

	// 第三步：遍历AST树，查找所有AutoMigrate调用并添加结构体参数
	ast.Inspect(file, func(n ast.Node) bool {
		// 根据business字段动态确定db变量名
		varDB := a.Business + "Db"

		if a.Business == "" {
			varDB = "db"
		}

		// 检查是否是函数调用表达式
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// 检查调用的方法名是否是"AutoMigrate"
		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok || selExpr.Sel.Name != "AutoMigrate" {
			return true
		}

		// 检查调用方是否是目标db变量
		ident, ok := selExpr.X.(*ast.Ident)
		if !ok || ident.Name != varDB {
			return true
		}

		// 添加结构体参数到AutoMigrate调用中
		// 为什么使用CompositeLit：创建复合字面量，如 &package.Model{}
		// 使用SelectorExpr：生成完整的类型引用，如 package.Model
		callExpr.Args = append(callExpr.Args, &ast.CompositeLit{
			Type: &ast.SelectorExpr{
				X:   ast.NewIdent(a.PackageName), // 包名标识符
				Sel: ast.NewIdent(a.StructName),  // 结构体名标识符
			},
		})
		return true
	})
	return nil
}

// Format 格式化AST并写入文件
// 为什么这么写：
// 1. 参数默认值处理：如果没有提供filename，使用结构体中保存的路径
// 2. 委托给Base：复用基础格式化逻辑，保持代码一致性
// 好处：
// - 统一格式化：确保生成的代码符合Go标准格式
// - 简化调用：调用方可以不传filename，使用默认路径
func (a *PackageInitializeGorm) Format(filename string, writer io.Writer, file *ast.File) error {
	if filename == "" {
		// 使用结构体中保存的绝对路径作为默认值
		filename = a.Path
	}
	return a.Base.Format(filename, writer, file)
}

// addDbVar 在bizModel函数中创建业务数据库变量和AutoMigrate调用
// 为什么这么写：
// 1. 先检查是否已存在：避免重复添加db变量，保持代码整洁
// 2. 构建AST节点：手动构建赋值语句和函数调用，精确控制生成的代码结构
// 3. 插入到return之前：确保db变量在使用前被初始化，且return语句在最后
// 好处：
// - 幂等性：多次调用不会重复添加变量
// - 代码位置正确：变量定义在return之前，符合Go代码规范
// - 自动化初始化：自动创建db变量和AutoMigrate调用，无需手动编写
//
// 示例1：在空的bizModel函数中添加db变量和AutoMigrate调用
// 处理前的代码：
//
//	func bizModel() {
//	    return
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business: "gva",  // 业务数据库名称
//	}
//	bizModelDecl := FindFunction(file, "bizModel")
//	pkg.addDbVar(bizModelDecl.Body)
//
// 处理后的代码：
//
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate()
//	    return
//	}
//
// 示例2：在已有其他代码的bizModel函数中添加db变量
// 处理前的代码：
//
//	func bizModel() {
//	    // 一些其他初始化代码
//	    fmt.Println("初始化业务模型")
//	    return
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business: "gva",
//	}
//	bizModelDecl := FindFunction(file, "bizModel")
//	pkg.addDbVar(bizModelDecl.Body)
//
// 处理后的代码：
//
//	func bizModel() {
//	    // 一些其他初始化代码
//	    fmt.Println("初始化业务模型")
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate()
//	    return
//	}
//
// 示例3：避免重复添加（已存在db变量时直接返回）
// 处理前的代码：
//
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate(&system.User{})
//	    return
//	}
//
// 调用示例（再次调用addDbVar）：
//
//	pkg := &PackageInitializeGorm{
//	    Business: "gva",
//	}
//	bizModelDecl := FindFunction(file, "bizModel")
//	pkg.addDbVar(bizModelDecl.Body)  // 检测到已存在gvaDb，直接返回
//
// 处理后的代码（保持不变）：
//
//	func bizModel() {
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate(&system.User{})
//	    return
//	}
//
// 示例4：不同业务数据库名称的示例
// 处理前的代码：
//
//	func bizModel() {
//	    return
//	}
//
// 调用示例（使用不同的业务数据库名称）：
//
//	pkg := &PackageInitializeGorm{
//	    Business: "order",  // 订单业务数据库
//	}
//	bizModelDecl := FindFunction(file, "bizModel")
//	pkg.addDbVar(bizModelDecl.Body)
//
// 处理后的代码：
//
//	func bizModel() {
//	    orderDb := global.GetGlobalDBByDBName("order")
//	    orderDb.AutoMigrate()
//	    return
//	}
//
// 示例5：在复杂的bizModel函数中正确插入（保持return在最后）
// 处理前的代码：
//
//	func bizModel() {
//	    if err := initConfig(); err != nil {
//	        return
//	    }
//	    log.Println("开始初始化")
//	    return
//	}
//
// 调用示例：
//
//	pkg := &PackageInitializeGorm{
//	    Business: "gva",
//	}
//	bizModelDecl := FindFunction(file, "bizModel")
//	pkg.addDbVar(bizModelDecl.Body)
//
// 处理后的代码（db变量和AutoMigrate插入到最后一个return之前）：
//
//	func bizModel() {
//	    if err := initConfig(); err != nil {
//	        return
//	    }
//	    log.Println("开始初始化")
//	    gvaDb := global.GetGlobalDBByDBName("gva")
//	    gvaDb.AutoMigrate()
//	    return
//	}
func (a *PackageInitializeGorm) addDbVar(astBody *ast.BlockStmt) {
	// 第一步：检查是否已经存在db变量
	// 为什么先检查：避免重复添加，保持代码简洁
	for i := range astBody.List {
		if assignStmt, ok := astBody.List[i].(*ast.AssignStmt); ok {
			if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok {
				// 检查是否已存在目标db变量（默认db或业务db）
				if (a.Business == "" && ident.Name == "db") || ident.Name == a.Business+"Db" {
					return // 已存在，直接返回
				}
			}
		}
	}

	// 第二步：构建db变量赋值语句的AST节点
	// 生成的代码：businessDb := global.GetGlobalDBByDBName("business")
	// 为什么手动构建AST：精确控制代码结构，确保生成的代码符合Go语法
	assignNode := &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.Ident{
				Name: a.Business + "Db", // 左侧：变量名
			},
		},
		Tok: token.DEFINE, // 使用 := 定义变量
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "global", // global包
					},
					Sel: &ast.Ident{
						Name: "GetGlobalDBByDBName", // 方法名
					},
				},
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.STRING,                      // 字符串字面量
						Value: fmt.Sprintf("\"%s\"", a.Business), // 业务名称作为参数
					},
				},
			},
		},
	}

	// 第三步：构建AutoMigrate调用的AST节点
	// 生成的代码：businessDb.AutoMigrate()
	// 为什么单独构建：AutoMigrate调用是独立的语句，需要单独创建
	autoMigrateCall := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: a.Business + "Db", // 调用方：db变量
				},
				Sel: &ast.Ident{
					Name: "AutoMigrate", // 方法名
				},
			},
		},
	}

	// 第四步：将新节点插入到return语句之前
	// 为什么插入到return之前：确保变量在使用前初始化，且return在最后
	// 为什么保存return节点：保持函数结构完整，return语句必须在最后
	returnNode := astBody.List[len(astBody.List)-1]
	// 将新节点插入：原列表（除了最后一个return）+ 新节点 + return
	astBody.List = append(astBody.List[:len(astBody.List)-1], assignNode, autoMigrateCall, returnNode)
}
