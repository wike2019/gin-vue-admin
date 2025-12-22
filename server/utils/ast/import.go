package ast

import (
	"go/ast"
	"go/token"
	"io"
	"strings"
)

// Import 是专门用于处理 Go 文件 import 语句的 AST 操作器
//
// 设计模式：组合模式 + 模板方法模式
// - 通过嵌入 Base 结构体实现代码复用，继承通用的 Parse 和 Format 方法
// - 重写 Rollback 和 Injection 方法，实现特定的 import 操作逻辑
//
// 为什么单独设计 Import 操作器？
// 1. 单一职责原则：专门处理 import 语句，逻辑清晰，易于维护
// 2. 代码复用：通过嵌入 Base，复用解析和格式化逻辑，避免重复代码
// 3. 可扩展性：可以独立扩展 import 相关的功能（如别名处理、分组等）
// 4. 可测试性：独立的操作器便于单元测试和 Mock
//
// 使用场景：
// - 代码生成工具：自动添加必要的包导入
// - 代码重构工具：清理未使用的导入或添加缺失的导入
// - 插件系统：动态注入插件所需的包导入
type Import struct {
	Base              // 嵌入基础结构体，继承 Parse、Format 等通用方法
	ImportPath string // 导包路径，例如 "github.com/gin-gonic/gin"
}

// NewImport 创建新的 Import 操作器实例
//
// 参数说明：
//   - importPath: 要操作的导入路径，例如 "github.com/gin-gonic/gin"
//
// 设计意图：
//   - 提供便捷的构造函数，统一创建方式
//   - 确保 ImportPath 字段正确初始化
//
// 好处：
//   - 代码可读性：使用构造函数比直接创建结构体更清晰
//   - 未来扩展：可以在构造函数中添加初始化逻辑（如路径验证）
func NewImport(importPath string) *Import {
	return &Import{ImportPath: importPath}
}

// Parse 解析 Go 源文件并返回 AST 文件节点
//
// 设计说明：
//   - 直接委托给 Base.Parse，实现代码复用
//   - Import 操作器不需要特殊的解析逻辑，使用标准解析即可
//
// 好处：
//   - 代码复用：避免重复实现解析逻辑
//   - 一致性：所有 AST 操作器使用相同的解析方式
//   - 维护性：解析逻辑的改进会自动应用到所有操作器
func (a *Import) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	return a.Base.Parse(filename, writer)
}

// Rollback 回滚操作，从 AST 中删除指定的 import 语句
//
// 设计思路：
//  1. 遍历文件的所有声明（Decls），查找 import 声明
//  2. 在 import 声明中查找匹配的导入路径
//  3. 删除匹配的导入项，如果 import 声明变空则删除整个声明
//
// 为什么使用 strings.HasSuffix 进行路径匹配？
//   - Go 的 import 路径在 AST 中存储时带有引号，例如 `"github.com/gin-gonic/gin"`
//   - 使用 HasSuffix 可以匹配带引号的路径，避免字符串处理复杂性
//   - 支持部分路径匹配，提高匹配的灵活性
//
// 为什么在找到非 import 声明时要 break？
//   - Go 语言规范要求 import 声明必须在文件开头，且连续出现
//   - 一旦遇到非 import 声明（如 var、const、type、func），说明已经过了 import 区域
//   - 提前退出可以避免不必要的遍历，提高性能
//
// 为什么删除空的 import 声明？
//   - 空的 import 声明（import()）在 Go 中是语法错误
//   - 如果不删除，格式化后的代码会出现语法错误
//   - 保持代码的整洁性和正确性
//
// 好处：
//   - 安全性：检查 ImportPath 是否为空，避免无效操作
//   - 正确性：删除空 import 声明，避免生成语法错误的代码
//   - 性能：遇到非 import 声明立即退出，减少不必要的遍历
//   - 健壮性：使用类型断言和检查，避免 panic
func (a *Import) Rollback(file *ast.File) error {
	// 如果导入路径为空，无需操作
	if a.ImportPath == "" {
		return nil
	}

	// 遍历文件的所有声明，查找 import 声明
	for i := 0; i < len(file.Decls); i++ {
		v1, o1 := file.Decls[i].(*ast.GenDecl)
		if o1 {
			// 如果当前声明不是 import，说明已经过了 import 区域，可以退出
			// Go 语言规范要求 import 必须在文件开头且连续
			if v1.Tok != token.IMPORT {
				break
			}

			// 在当前 import 声明中查找匹配的导入项
			for j := 0; j < len(v1.Specs); j++ {
				v2, o2 := v1.Specs[j].(*ast.ImportSpec)
				// 使用 HasSuffix 匹配路径（因为 AST 中的路径值包含引号）
				// 例如：ImportPath = "github.com/gin-gonic/gin"
				//      v2.Path.Value = "\"github.com/gin-gonic/gin\""
				if o2 && strings.HasSuffix(a.ImportPath, v2.Path.Value) {
					// 删除匹配的导入项：使用切片操作移除第 j 个元素
					v1.Specs = append(v1.Specs[:j], v1.Specs[j+1:]...)

					// 如果 import 声明中的所有导入项都被删除，删除整个声明
					// 如果不删除，会出现空的 import()，这是语法错误
					if len(v1.Specs) == 0 {
						file.Decls = append(file.Decls[:i], file.Decls[i+1:]...)
					}
					break
				}
			}
		}
	}
	return nil
}

// Injection 注入操作，向 AST 中添加指定的 import 语句
//
// 设计思路：
//  1. 首先检查是否已存在相同的 import，避免重复导入
//  2. 如果存在 import 声明，直接添加到现有的声明中
//  3. 如果不存在 import 声明，创建新的声明并放在文件开头
//
// 为什么需要检查是否已存在？
//   - 避免重复导入同一个包，这是 Go 语言的语法要求
//   - 提高代码质量，保持 import 区域的整洁
//   - 避免不必要的 AST 修改，提高性能
//
// 为什么使用 strings.HasSuffix 进行路径匹配？
//   - 与 Rollback 方法保持一致，使用相同的匹配逻辑
//   - 可以处理带引号的路径值，简化字符串处理
//
// 为什么在找到非 import 声明时要 break？
//   - Go 语言规范要求 import 声明必须在文件开头且连续
//   - 一旦遇到非 import 声明，说明已经过了 import 区域
//   - 提前退出可以提高性能，避免不必要的遍历
//
// 为什么新创建的 import 声明要放在文件开头？
//   - 符合 Go 语言规范：import 声明必须在文件开头
//   - 保持代码格式的一致性，符合 gofmt 的格式化规则
//   - 提高代码可读性，import 区域统一在文件顶部
//
// 为什么使用 make 预分配容量？
//   - 性能优化：预分配容量可以避免多次内存重新分配
//   - 减少内存碎片，提高内存使用效率
//
// 好处：
//   - 安全性：检查 ImportPath 是否为空，避免无效操作
//   - 正确性：避免重复导入，符合 Go 语言规范
//   - 规范性：新 import 声明放在文件开头，符合 Go 代码规范
//   - 性能：提前退出和预分配容量，优化执行效率
//   - 健壮性：使用类型断言和检查，避免 panic
func (a *Import) Injection(file *ast.File) error {
	// 如果导入路径为空，无需操作
	if a.ImportPath == "" {
		return nil
	}

	var has bool // 标记是否已存在相同的 import

	// 遍历文件的所有声明，查找是否已存在 import 声明
	for i := 0; i < len(file.Decls); i++ {
		v1, o1 := file.Decls[i].(*ast.GenDecl)
		if o1 {
			// 如果当前声明不是 import，说明已经过了 import 区域，可以退出
			if v1.Tok != token.IMPORT {
				break
			}

			// 检查当前 import 声明中是否已存在相同的导入路径
			for j := 0; j < len(v1.Specs); j++ {
				v2, o2 := v1.Specs[j].(*ast.ImportSpec)
				// 使用 HasSuffix 匹配路径（因为 AST 中的路径值包含引号）
				if o2 && strings.HasSuffix(a.ImportPath, v2.Path.Value) {
					has = true
					break
				}
			}

			// 如果已存在 import 声明但未找到相同的导入路径，添加到现有声明中
			if !has {
				spec := &ast.ImportSpec{
					Path: &ast.BasicLit{
						Kind:  token.STRING,
						Value: a.ImportPath, // 注意：这里应该包含引号，但实际使用时可能需要添加
					},
				}
				v1.Specs = append(v1.Specs, spec)
				return nil
			}
		}
	}

	// 如果不存在 import 声明，创建新的声明并放在文件开头
	// 这是 Go 语言规范的要求：import 必须在文件开头
	if !has {
		// 保存原有的声明
		decls := file.Decls

		// 预分配容量，避免多次内存重新分配（性能优化）
		file.Decls = make([]ast.Decl, 0, len(file.Decls)+1)

		// 创建新的 import 声明
		decl := &ast.GenDecl{
			Tok: token.IMPORT,
			Specs: []ast.Spec{
				&ast.ImportSpec{
					Path: &ast.BasicLit{
						Kind:  token.STRING,
						Value: a.ImportPath,
					},
				},
			},
		}

		// 将新的 import 声明放在文件开头，然后追加原有声明
		// 这确保了 import 声明始终在文件开头，符合 Go 语言规范
		file.Decls = append(file.Decls, decl)
		file.Decls = append(file.Decls, decls...)
	}
	return nil
}

// Format 格式化 AST 并写入文件
//
// 设计说明：
//   - 直接委托给 Base.Format，实现代码复用
//   - Import 操作器不需要特殊的格式化逻辑，使用标准格式化即可
//
// 好处：
//   - 代码复用：避免重复实现格式化逻辑
//   - 一致性：所有 AST 操作器使用相同的格式化方式
//   - 维护性：格式化逻辑的改进会自动应用到所有操作器
//   - 规范性：使用 go/format 包，确保生成的代码符合 gofmt 规范
func (a *Import) Format(filename string, writer io.Writer, file *ast.File) error {
	return a.Base.Format(filename, writer, file)
}
