package ast

import (
	"fmt"
	"go/ast"
	"io"
)

// PluginInitializeV2 插件初始化V2版本的AST操作器
// 用于在插件系统中自动注入插件初始化代码到 bizPluginV2 函数中
//
// 设计优势：
// 1. 通过嵌入 Base 结构体实现代码复用，避免重复实现基础的 AST 解析和格式化功能
// 2. 采用模板方法模式，只需重写特定的 Injection 方法即可实现自定义的代码注入逻辑
// 3. 支持相对路径和绝对路径的自动转换，提高代码的可移植性和灵活性
type PluginInitializeV2 struct {
	Base                // 嵌入基础结构体，继承 Parse、Format、Rollback 等通用方法
	Type         Type   // 类型
	Path         string // 文件路径
	PluginPath   string // 插件路径（绝对路径）
	RelativePath string // 相对路径（用于跨平台和可移植性）
	ImportPath   string // 导包路径（需要导入的插件包路径）
	StructName   string // 结构体名称
	PackageName  string // 包名（用于生成 PluginInitV2 调用时的包前缀）
}

// Parse 解析 Go 源文件并返回 AST 文件节点
//
// 路径处理逻辑说明：
//  1. 如果 filename 为空，需要根据已有信息推断文件路径
//  2. 优先使用 RelativePath（相对路径），因为相对路径具有更好的可移植性
//     可以在不同环境（开发、测试、生产）中正常工作
//  3. 如果 RelativePath 为空，则使用 PluginPath（绝对路径）并计算相对路径
//     这样可以在首次调用时建立路径映射关系
//  4. 如果已有 RelativePath，则反向计算绝对路径，确保路径一致性
//
// 好处：
// - 支持灵活的路径输入方式，提高 API 的易用性
// - 自动维护相对路径和绝对路径的映射，避免路径不一致问题
// - 通过委托给 Base.Parse 实现代码复用，遵循 DRY 原则
//
// 使用示例：
//
//	示例1：首次调用，使用绝对路径
//		parser := &PluginInitializeV2{
//			PluginPath: "/path/to/plugin/plugin.go",
//		}
//		file, err := parser.Parse("", nil)
//		// 此时会自动计算 RelativePath，后续调用可以使用相对路径
//
//	示例2：已有相对路径，自动转换为绝对路径
//		parser := &PluginInitializeV2{
//			RelativePath: "server/plugin/announcement/plugin.go",
//		}
//		file, err := parser.Parse("", nil)
//		// 此时会自动将 RelativePath 转换为绝对路径并解析
//
//	示例3：直接提供文件名
//		parser := &PluginInitializeV2{}
//		file, err := parser.Parse("/path/to/plugin/plugin.go", nil)
//		// 直接使用提供的文件名解析，不依赖内部路径字段
//
//	示例4：完整使用流程
//		parser := &PluginInitializeV2{
//			PluginPath:   "/absolute/path/to/plugin.go",
//			ImportPath:   "gin-vue-admin/server/plugin/announcement",
//			PackageName:  "announcement",
//			StructName:   "Plugin",
//		}
//		// 第一次解析，使用绝对路径
//		file, err := parser.Parse("", nil)
//		if err != nil {
//			return err
//		}
//		// 注入代码
//		if err := parser.Injection(file); err != nil {
//			return err
//		}
//		// 格式化并写回文件
//		if err := parser.Format("", nil, file); err != nil {
//			return err
//		}
func (a *PluginInitializeV2) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	if filename == "" {
		if a.RelativePath == "" {
			// 场景：首次调用，只有绝对路径
			// 使用绝对路径解析，并计算相对路径供后续使用
			filename = a.PluginPath
			a.RelativePath = a.Base.RelativePath(a.PluginPath)
			return a.Base.Parse(filename, writer)
		}
		// 场景：已有相对路径，需要转换为绝对路径
		// 这样可以在不同工作目录下都能正确找到文件
		a.PluginPath = a.Base.AbsolutePath(a.RelativePath)
		filename = a.PluginPath
	}
	// 场景：直接提供了 filename，直接使用
	return a.Base.Parse(filename, writer)
}

// Injection 将插件初始化代码注入到目标文件的 bizPluginV2 函数中
//
// 注入逻辑说明：
//  1. 首先检查是否已导入插件包，避免重复导入（幂等性设计）
//  2. 如果未导入，则添加 import 语句
//  3. 查找 bizPluginV2 函数（这是插件系统的统一初始化入口函数）
//  4. 在该函数体中追加 PluginInitV2 调用语句，格式为：
//     PluginInitV2(engine, PackageName.Plugin)
//
// 设计优势：
// - 幂等性：通过 CheckImport 检查避免重复导入，多次调用不会产生副作用
// - 自动化：无需手动修改代码，通过 AST 操作自动完成代码注入
// - 统一入口：所有插件都通过 bizPluginV2 函数统一初始化，便于管理和维护
// - 类型安全：通过 AST 操作而非字符串拼接，保证生成的代码语法正确
//
// Demo 示例：
//
//	示例1：基本使用 - 注入公告插件
//		// 初始化插件注入器
//		initializer := &PluginInitializeV2{
//			ImportPath:  "gin-vue-admin/server/plugin/announcement",
//			PackageName: "announcement",
//			PluginPath:  "/path/to/server/initialize/plugin_biz_v2.go",
//		}
//		// 解析目标文件
//		file, err := initializer.Parse("", nil)
//		if err != nil {
//			return err
//		}
//		// 注入插件初始化代码
//		err = initializer.Injection(file)
//		// 注入前：func bizPluginV2(engine *gin.Engine) { ... }
//		// 注入后：
//		//   import "gin-vue-admin/server/plugin/announcement"
//		//   func bizPluginV2(engine *gin.Engine) {
//		//       ...原有代码...
//		//       PluginInitV2(engine, announcement.Plugin)  // 自动添加
//		//   }
//
//	示例2：注入邮件插件
//		initializer := &PluginInitializeV2{
//			ImportPath:  "gin-vue-admin/server/plugin/email",
//			PackageName: "email",
//			PluginPath:  "/path/to/server/initialize/plugin_biz_v2.go",
//		}
//		file, _ := initializer.Parse("", nil)
//		initializer.Injection(file)
//		// 生成：PluginInitV2(engine, email.Plugin)
//
//	示例3：幂等性演示 - 多次调用不会重复添加
//		initializer := &PluginInitializeV2{
//			ImportPath:  "gin-vue-admin/server/plugin/announcement",
//			PackageName: "announcement",
//			PluginPath:  "/path/to/server/initialize/plugin_biz_v2.go",
//		}
//		file, _ := initializer.Parse("", nil)
//		// 第一次调用
//		initializer.Injection(file)
//		// 第二次调用 - 不会重复添加 import 和调用语句（已存在则跳过）
//		initializer.Injection(file)
//
//	示例4：完整流程 - 解析、注入、格式化、写回
//		initializer := &PluginInitializeV2{
//			ImportPath:  "gin-vue-admin/server/plugin/custom",
//			PackageName: "custom",
//			PluginPath:  "/absolute/path/to/plugin_biz_v2.go",
//		}
//		// 1. 解析文件
//		file, err := initializer.Parse("", nil)
//		if err != nil {
//			return err
//		}
//		// 2. 注入代码
//		if err := initializer.Injection(file); err != nil {
//			return err
//		}
//		// 3. 格式化并写回文件
//		if err := initializer.Format("", nil, file); err != nil {
//			return err
//		}
//
//	示例5：批量注入多个插件
//		plugins := []struct {
//			ImportPath  string
//			PackageName string
//		}{
//			{"gin-vue-admin/server/plugin/announcement", "announcement"},
//			{"gin-vue-admin/server/plugin/email", "email"},
//			{"gin-vue-admin/server/plugin/custom", "custom"},
//		}
//		file, _ := initializer.Parse("", nil)
//		for _, p := range plugins {
//			initializer.ImportPath = p.ImportPath
//			initializer.PackageName = p.PackageName
//			initializer.Injection(file)
//		}
//		// 最终结果：所有插件初始化代码都会被添加到 bizPluginV2 函数中
func (a *PluginInitializeV2) Injection(file *ast.File) error {
	// 检查是否已导入，避免重复导入（幂等性保证）
	if !CheckImport(file, a.ImportPath) {
		// 注入 import 语句
		NewImport(a.ImportPath).Injection(file)
		// 查找目标函数 bizPluginV2（插件系统的统一初始化入口）
		funcDecl := FindFunction(file, "bizPluginV2")
		// 创建插件初始化调用语句，格式：PluginInitV2(engine, PackageName.Plugin)
		// 例如：PluginInitV2(engine, announcement.Plugin)
		stmt := CreateStmt(fmt.Sprintf("PluginInitV2(engine, %s.Plugin)", a.PackageName))
		// 将语句追加到函数体末尾
		funcDecl.Body.List = append(funcDecl.Body.List, stmt)
	}
	return nil
}

// Rollback 回滚操作，用于撤销 Injection 所做的修改
//
// 当前实现为空的原因：
// 1. 插件初始化代码的注入通常是单向操作，一旦注入就表示插件已集成
// 2. 如果需要移除插件，更推荐直接删除相关代码或使用版本控制回退
// 3. 保持接口一致性，为未来可能的回滚需求预留接口
//
// 好处：
// - 接口统一：所有 AST 操作器都实现 Rollback 方法，保持接口一致性
// - 扩展性：未来如果需要实现回滚功能，只需在此方法中添加逻辑即可
func (a *PluginInitializeV2) Rollback(file *ast.File) error {
	return nil
}

// Format 将修改后的 AST 格式化并写回文件
//
// 功能说明：
//  1. 如果 filename 为空，使用 PluginPath 作为默认输出路径
//  2. 委托给 Base.Format 执行实际的格式化操作
//     包括：打开文件、使用 go/format 格式化代码、写入文件
//
// 设计优势：
// - 代码复用：通过委托给 Base.Format 避免重复实现格式化逻辑
// - 自动格式化：使用 Go 标准库的 format 包，确保生成的代码符合 Go 代码规范
// - 容错处理：Base.Format 内部已处理文件打开、写入等错误情况
func (a *PluginInitializeV2) Format(filename string, writer io.Writer, file *ast.File) error {
	if filename == "" {
		// 使用插件路径作为默认输出路径
		filename = a.PluginPath
	}
	// 委托给基础类的格式化方法，实现代码复用
	return a.Base.Format(filename, writer, file)
}
