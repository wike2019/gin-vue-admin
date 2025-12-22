package ast

import (
	"go/ast"
	"io"
)

// PluginInitializeGorm 插件 GORM 初始化 AST 操作器
//
// 核心功能：
// 用于在插件系统中自动向 GORM 的 AutoMigrate 调用中注入插件模型，实现插件的数据库表自动迁移
//
// 设计模式：模板方法模式 + 组合模式
//
// 为什么需要这个结构体？
// 1. 自动化管理：当创建新插件时，需要将插件的模型添加到 AutoMigrate 调用中，手动维护容易出错
// 2. 代码生成：配合代码生成工具，实现"创建插件 -> 自动注册到数据库迁移"的自动化流程
// 3. 可逆操作：支持回滚（Rollback），可以撤销之前注入的代码，便于插件卸载或重构
// 4. 依赖管理：自动管理 import 语句，确保必要的包被正确导入
//
// 设计优势：
// 1. 代码复用：通过嵌入 Base 结构体，复用基础的 AST 解析和格式化功能
// 2. 类型安全：使用 Go AST 包进行代码修改，保证生成的代码语法正确
// 3. 可移植性：支持相对路径和绝对路径的自动转换，便于跨平台使用
// 4. 原子操作：注入和回滚操作都是原子性的，要么全部成功，要么全部失败
//
// 使用场景：
// - 插件系统：当创建新插件时，自动将插件模型添加到数据库迁移逻辑中
// - 代码生成：代码生成工具可以自动调用此结构体来管理数据库表的注册
// - 插件卸载：卸载插件时，可以回滚之前注入的代码，保持代码整洁
//
// 示例用法：
//
//	注入操作：
//	  gorm := &PluginInitializeGorm{
//	      Path:        "/path/to/initialize/gorm_biz.go",
//	      ImportPath:  "github.com/example/plugin/model",
//	      StructName:  "User",
//	      PackageName: "model",
//	  }
//	  file, _ := gorm.Parse("", nil)
//	  gorm.Injection(file)
//	  gorm.Format("", writer, file)
//
//	生成的代码效果：
//	  // 自动添加 import
//	  import "github.com/example/plugin/model"
//
//	  // 在 AutoMigrate 调用中添加参数
//	  db.AutoMigrate(
//	      // ... 其他模型
//	      &model.User{},  // 新注入的插件模型
//	  )
type PluginInitializeGorm struct {
	Base                // 嵌入基础结构体，继承 Parse、Format 等通用方法（组合优于继承的设计模式）
	Type         Type   // 类型：用于标识插件类型，可能用于分组或分类管理
	Path         string // 文件路径：目标文件的绝对路径，用于定位要修改的 Go 源文件
	ImportPath   string // 导包路径：需要导入的包路径（如 "github.com/example/plugin/model"），用于生成 import 语句
	RelativePath string // 相对路径：相对于项目根目录的路径，用于跨平台兼容性和代码可移植性
	StructName   string // 结构体名称：要注入到 AutoMigrate 中的模型结构体名称（如 "User"）
	PackageName  string // 包名：模型所在的包名（如 "model"），用于生成完整类型引用（如 model.User）
	IsNew        bool   // 是否使用new关键字：控制生成代码的方式
	// true: 使用 new() 语法，生成 new(PackageName.StructName)
	// false: 使用结构体字面量，生成 &PackageName.StructName{}
	// 目前代码中虽然定义了此字段，但实际未使用，可能是为未来功能预留的扩展点
}

// Parse 解析 Go 源文件并返回 AST 文件节点
//
// 参数说明：
//   - filename: 要解析的文件路径（绝对路径或相对路径），如果为空则从结构体字段推断
//   - writer: 可选的 io.Writer，用于从内存中读取源码（通常为 nil，表示从文件系统读取）
//
// 返回值：
//   - file: 解析后的 AST 文件节点，包含完整的语法树结构
//   - err: 解析过程中的错误信息
//
// 路径处理逻辑说明（为什么这样设计？）：
//
//  1. 支持多种调用方式：
//     - 直接传入 filename：最直观的方式，适用于明确知道文件路径的场景
//     - 不传 filename（空字符串）：从结构体字段推断路径，适用于路径信息已存储在结构体中的场景
//
//  2. 相对路径优先策略：
//     - 优先使用 RelativePath（相对路径）而不是绝对路径
//     - 好处：跨平台兼容性好，不同操作系统和开发环境都能正常工作
//     - 好处：代码仓库中记录相对路径，便于版本控制和团队协作
//
//  3. 路径自动转换机制：
//     - 如果只有绝对路径（Path），自动计算相对路径并存储，建立双向映射
//     - 如果只有相对路径（RelativePath），自动计算绝对路径用于文件操作
//     - 好处：自动维护路径的一致性，避免路径不一致导致的错误
//
//  4. 委托给 Base.Parse：
//     - 复用 Base 结构体的解析逻辑，遵循 DRY（Don't Repeat Yourself）原则
//     - 好处：代码复用，减少重复代码，降低维护成本
//
// 设计优势：
//   - 灵活性：支持多种路径输入方式，API 更易用
//   - 可移植性：相对路径确保代码在不同环境（开发、测试、生产）都能正常工作
//   - 自动化：自动维护路径映射关系，减少手动维护成本
//   - 代码复用：通过委托避免重复实现解析逻辑
func (a *PluginInitializeGorm) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	// 如果未提供文件名，需要根据结构体字段推断路径
	if filename == "" {
		// 如果相对路径为空，说明首次调用，使用绝对路径并计算相对路径
		// 这样可以在首次调用时建立路径映射关系，后续调用可以使用相对路径
		if a.RelativePath == "" {
			filename = a.Path
			// 计算并存储相对路径，为后续调用建立路径映射
			a.RelativePath = a.Base.RelativePath(a.Path)
			return a.Base.Parse(filename, writer)
		}
		// 如果已有相对路径，反向计算绝对路径
		// 这样可以确保路径一致性，同时支持跨平台使用
		a.Path = a.Base.AbsolutePath(a.RelativePath)
		filename = a.Path
	}
	// 委托给 Base 结构体的 Parse 方法，复用基础解析逻辑
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作，用于撤销 Injection 所做的修改
//
// 参数说明：
//   - file: 要回滚的 AST 文件节点（通常是之前通过 Parse 解析得到的文件）
//
// 返回值：
//   - err: 回滚过程中的错误信息（目前总是返回 nil，但保留错误返回值便于未来扩展）
//
// 核心功能：
//  1. 从 AutoMigrate 调用中删除之前注入的模型参数
//  2. 如果删除后 AutoMigrate 只剩一个参数或为空，则回滚对应的 import 语句
//
// 设计思路说明（为什么这样实现？）：
//
//  1. 使用 ast.Inspect 深度优先遍历：
//     - ast.Inspect 会递归遍历整个 AST 树，找到所有符合条件的节点
//     - 好处：无需手动编写复杂的遍历逻辑，代码简洁且不容易出错
//     - 好处：支持嵌套结构，即使 AutoMigrate 调用嵌套在其他表达式中也能找到
//
//  2. 类型断言链式检查（Type Assertion Chain）：
//     - 通过多层类型断言逐步缩小范围：CallExpr -> SelectorExpr -> CompositeLit -> SelectorExpr -> Ident
//     - 好处：精确匹配目标节点，避免误删其他代码
//     - 好处：每一步都有 ok 检查，代码健壮性好，即使 AST 结构异常也不会 panic
//
//  3. 匹配策略：
//     - 首先检查是否为 AutoMigrate 调用：通过检查 SelectorExpr.Sel.Name == "AutoMigrate"
//     - 然后检查参数类型：必须是 CompositeLit（结构体字面量，如 &model.User{}）
//     - 最后检查包名和结构体名：确保只删除我们注入的参数，不误删其他模型
//     - 好处：精确匹配，避免误删除其他代码
//
//  4. 参数删除技巧：
//     - 使用切片操作 append(args[:i], args[i+1:]...) 删除元素
//     - 找到后立即 break，只删除第一个匹配的参数（避免重复删除导致索引错乱）
//     - 好处：高效且安全，不会影响其他参数
//
//  5. Import 回滚的智能判断：
//     - 只有当 AutoMigrate 参数数量 <= 1 时才回滚 import
//     - 原因：如果还有其他参数使用该包，import 不应该被删除
//     - 好处：避免破坏其他代码的依赖关系
//
//  6. 委托给 NewImport().Rollback：
//     - 复用 Import 结构体的回滚逻辑，遵循单一职责原则
//     - 好处：代码复用，import 管理逻辑集中在一个地方，便于维护
//
// 设计优势：
//   - 精确性：通过多层级类型检查，确保只删除目标参数，不误删其他代码
//   - 安全性：多层类型断言都有 ok 检查，避免 panic
//   - 智能性：根据实际情况决定是否删除 import，避免破坏其他依赖
//   - 可维护性：代码结构清晰，每一步都有明确的意图
//
// 使用场景：
//   - 插件卸载：卸载插件时，需要从数据库迁移中移除插件模型
//   - 代码重构：重构时可能需要回滚某些自动注入的代码
//   - 错误恢复：如果注入后发现问题，可以回滚到之前的状态
func (a *PluginInitializeGorm) Rollback(file *ast.File) error {
	// 标记是否需要回滚 import 语句
	// 如果删除参数后 AutoMigrate 调用只剩一个参数或为空，说明该 import 可能不再需要
	var needRollBackImport bool

	// 使用 ast.Inspect 深度优先遍历 AST 树，查找所有 AutoMigrate 调用
	ast.Inspect(file, func(n ast.Node) bool {
		// 类型断言：检查是否为函数调用表达式（如 db.AutoMigrate(...)）
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			// 不是函数调用，继续遍历子节点
			return true
		}

		// 类型断言：检查是否为选择器表达式（如 db.AutoMigrate）
		selExpr, seok := callExpr.Fun.(*ast.SelectorExpr)
		// 如果不是选择器表达式，或者方法名不是 "AutoMigrate"，继续遍历
		if !seok || selExpr.Sel.Name != "AutoMigrate" {
			return true
		}

		// 如果删除当前参数后，参数数量 <= 1，则需要考虑回滚 import
		// 因为如果只剩一个参数，可能意味着这是最后一个使用该包的代码
		if len(callExpr.Args) <= 1 {
			needRollBackImport = true
		}

		// 遍历 AutoMigrate 的所有参数，查找要删除的目标参数
		for i, arg := range callExpr.Args {
			// 类型断言：检查参数是否为复合字面量（结构体字面量，如 &model.User{}）
			compLit, cok := arg.(*ast.CompositeLit)
			if !cok {
				// 不是结构体字面量，跳过（可能是变量或其他类型）
				continue
			}

			// 类型断言：检查类型是否为选择器表达式（如 model.User）
			cselExpr, sok := compLit.Type.(*ast.SelectorExpr)
			if !sok {
				// 不是选择器表达式（可能是其他类型，如 *User），跳过
				continue
			}

			// 类型断言：检查包名标识符（选择器表达式的 X 部分，如 model）
			ident, idok := cselExpr.X.(*ast.Ident)
			// 检查包名和结构体名是否匹配（精确匹配我们注入的参数）
			if idok && ident.Name == a.PackageName && cselExpr.Sel.Name == a.StructName {
				// 找到目标参数，使用切片操作删除该参数
				// append(args[:i], args[i+1:]...) 的语义：取 i 之前的元素 + i+1 之后的元素
				callExpr.Args = append(callExpr.Args[:i], callExpr.Args[i+1:]...)
				// 找到后立即跳出循环，避免重复删除（防止索引错乱）
				break
			}
		}

		// 继续遍历其他节点（可能有多个 AutoMigrate 调用）
		return true
	})

	// 如果需要回滚 import，委托给 Import 结构体的 Rollback 方法
	// 使用 NewImport 创建临时实例，复用 Import 的回滚逻辑
	// 忽略返回值是因为 Import.Rollback 目前总是返回 nil
	if needRollBackImport {
		_ = NewImport(a.ImportPath).Rollback(file)
	}

	return nil
}

// Injection 注入操作，向 AutoMigrate 调用中添加插件模型参数
//
// 参数说明：
//   - file: 要注入的 AST 文件节点（通常是之前通过 Parse 解析得到的文件）
//
// 返回值：
//   - err: 注入过程中的错误信息（目前总是返回 nil，但保留错误返回值便于未来扩展）
//
// 核心功能：
//  1. 确保必要的 import 语句存在
//  2. 找到 AutoMigrate 调用并添加新的模型参数
//
// 设计思路说明（为什么这样实现？）：
//
//  1. 先注入 import，再注入参数：
//     - 依赖关系：参数使用包名（如 model.User），必须先确保 import 存在
//     - 好处：即使后续步骤失败，import 也已正确添加，代码更完整
//     - 好处：符合 Go 语言的依赖关系：使用前必须先导入
//
//  2. 使用 ast.Inspect 查找 AutoMigrate 调用：
//     - 遍历整个 AST 树，找到第一个 AutoMigrate 调用
//     - 找到后立即返回 false 终止遍历（提高效率）
//     - 好处：支持 AutoMigrate 调用位于任何位置（函数体、初始化块等）
//
//  3. 为什么使用 return false 提前终止？
//     - 找到第一个 AutoMigrate 调用后就不需要继续遍历了
//     - 好处：提高性能，避免不必要的遍历
//     - 注意：如果有多个 AutoMigrate 调用，只会修改第一个（这是设计决策，符合业务场景）
//
//  4. 构造 CompositeLit（复合字面量）：
//     - 生成类似 &model.User{} 的结构体字面量语法
//     - 使用 SelectorExpr 表示包名.结构体名的形式
//     - 好处：生成的代码符合 Go 语法规范，类型安全
//
//  5. 为什么使用空的结构体字面量（{}）？
//     - AutoMigrate 只需要类型信息，不需要具体的值
//     - &model.User{} 是获取类型信息的标准方式
//     - 好处：简洁且符合 GORM 的使用习惯
//
//  6. 为什么直接 append 到 Args？
//     - AutoMigrate 支持可变参数，参数顺序通常不重要
//     - 直接追加到末尾，保持代码简洁
//     - 好处：简单高效，无需复杂的插入逻辑
//
// 设计优势：
//   - 自动化：自动管理 import 和参数注入，减少手动维护
//   - 类型安全：使用 AST 构造代码，保证生成的代码语法正确
//   - 高效性：找到目标后立即终止遍历，避免不必要的计算
//   - 可维护性：代码逻辑清晰，易于理解和修改
//
// 使用场景：
//   - 插件创建：创建新插件时，自动将插件模型添加到数据库迁移
//   - 代码生成：代码生成工具可以自动调用此方法注入模型
//
// 生成的代码示例：
//
//	注入前：
//	   import "github.com/example/plugin/model"
//	   db.AutoMigrate(&other.Model{})
//
//	注入后：
//	   import "github.com/example/plugin/model"
//	   db.AutoMigrate(&other.Model{}, &model.User{})  // 新增的模型参数
func (a *PluginInitializeGorm) Injection(file *ast.File) error {
	// 步骤1：先注入 import 语句，确保包可以被使用
	// 使用 NewImport 创建临时实例，复用 Import 的注入逻辑
	// 忽略返回值是因为 Import.Injection 目前总是返回 nil
	// 为什么先注入 import？
	// - 依赖关系：后续代码会使用 model.User 这样的包名引用，必须先确保 import 存在
	// - 代码完整性：即使后续步骤失败，import 也已正确添加
	_ = NewImport(a.ImportPath).Injection(file)

	// 步骤2：查找 AutoMigrate 调用
	var call *ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		// 类型断言：检查是否为函数调用表达式
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			// 不是函数调用，继续遍历
			return true
		}

		// 类型断言：检查是否为选择器表达式（如 db.AutoMigrate）
		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		// 检查方法名是否为 "AutoMigrate"
		if ok && selExpr.Sel.Name == "AutoMigrate" {
			// 找到目标调用，保存引用并终止遍历
			call = callExpr
			// 返回 false 终止遍历，因为已经找到目标
			// 好处：提高效率，避免不必要的遍历
			return false
		}

		// 继续遍历其他节点
		return true
	})

	// 步骤3：构造要注入的参数（结构体字面量）
	// 生成类似 &model.User{} 的 AST 节点
	arg := &ast.CompositeLit{
		// Type 指定类型，使用选择器表达式表示包名.结构体名
		Type: &ast.SelectorExpr{
			// X 是包名标识符（如 model）
			X: &ast.Ident{Name: a.PackageName},
			// Sel 是结构体名标识符（如 User）
			Sel: &ast.Ident{Name: a.StructName},
		},
		// 不指定 Value，使用空的结构体字面量 {}
		// 因为 AutoMigrate 只需要类型信息，不需要具体的值
	}

	// 步骤4：将新参数追加到 AutoMigrate 调用的参数列表
	// AutoMigrate 支持可变参数，参数顺序通常不重要，所以直接追加到末尾
	call.Args = append(call.Args, arg)
	return nil
}

// Format 格式化 AST 并将其写回到文件或 writer
//
// 参数说明：
//   - filename: 目标文件路径（用于确定格式化选项，如文件扩展名），如果为空则使用结构体的 Path 字段
//   - writer: 输出的 io.Writer，如果为 nil 则写回到源文件
//   - file: 要格式化的 AST 文件节点（通常是经过 Injection 或 Rollback 修改后的节点）
//
// 返回值：
//   - err: 格式化或写入过程中的错误信息
//
// 核心功能：
//
//	将修改后的 AST 节点格式化为标准的 Go 代码并写回文件
//
// 设计思路说明（为什么这样实现？）：
//
//  1. 路径处理逻辑：
//     - 如果未提供 filename，使用结构体存储的 Path 字段
//     - 好处：提供默认值，简化 API 调用
//     - 好处：与 Parse 方法形成对称的设计，都支持默认路径
//
//  2. 委托给 Base.Format：
//     - Base.Format 实现了标准的格式化逻辑（使用 go/format 包）
//     - 好处：代码复用，所有 AST 操作器共享相同的格式化逻辑
//     - 好处：统一的格式化风格，确保生成的代码符合 Go 标准格式
//
//  3. 为什么需要格式化？
//     - AST 修改后，直接输出可能格式不标准（缩进、空格等）
//     - go/format 包可以自动格式化代码，符合 gofmt 标准
//     - 好处：生成的代码可读性好，符合团队代码规范
//     - 好处：避免手动处理格式化细节，减少出错可能性
//
// 设计优势：
//   - 简单性：方法实现简单，主要逻辑委托给 Base，遵循单一职责原则
//   - 一致性：与其他 AST 操作器的 Format 方法保持一致，便于理解和维护
//   - 可扩展性：如果需要自定义格式化逻辑，可以重写此方法
//
// 典型使用流程：
//  1. Parse: 解析文件获取 AST
//  2. Injection/Rollback: 修改 AST
//  3. Format: 格式化并写回文件（当前方法）
//
// 注意事项：
//   - 格式化会覆盖原文件，建议在格式化前做好备份
//   - 格式化后的代码可能与原代码格式略有不同（符合 gofmt 标准）
func (a *PluginInitializeGorm) Format(filename string, writer io.Writer, file *ast.File) error {
	// 如果未提供文件名，使用结构体存储的路径作为默认值
	// 这样设计的好处：提供合理的默认值，简化 API 调用
	if filename == "" {
		filename = a.Path
	}
	// 委托给 Base 结构体的 Format 方法，复用基础格式化逻辑
	// Base.Format 使用 go/format 包进行代码格式化，确保生成的代码符合 Go 标准格式
	return a.Base.Format(filename, writer, file)
}
