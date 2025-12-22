package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// ExtractFuncSourceByPosition 根据文件路径与行号，提取包含该行的整个方法源码
// 返回：方法名、完整源码、起止行号
//
// 设计思路：
// 1. 使用 AST 解析而非字符串匹配，确保准确识别函数边界（避免误匹配注释、字符串中的函数签名等）
// 2. 通过 token.FileSet 精确映射 AST 节点到源码位置（行号+字节偏移）
// 3. 使用字节偏移提取源码，保留原始格式（缩进、换行、注释等），而不是重新格式化
func ExtractFuncSourceByPosition(filePath string, line int) (name string, source string, startLine int, endLine int, err error) {
	// 读取源文件到内存
	// 注意：需要完整文件内容，因为后续要用字节偏移直接切片提取源码
	src, readErr := os.ReadFile(filePath)
	if readErr != nil {
		err = fmt.Errorf("read file failed: %w", readErr)
		return
	}

	// 创建 token.FileSet：用于跟踪所有文件的位置信息
	// 这是 Go AST 系统的核心，每个 AST 节点的 Pos()/End() 都关联到 FileSet 中的位置
	fset := token.NewFileSet()
	// 解析文件为 AST，ParseComments 参数确保保留注释信息
	// 好处：提取的源码包含函数前后的文档注释，便于后续展示或分析
	file, parseErr := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if parseErr != nil {
		err = fmt.Errorf("parse file failed: %w", parseErr)
		return
	}

	// 使用 ast.Inspect 深度优先遍历整个 AST 树
	// 优点：比手动递归更简洁，通过返回 bool 控制遍历深度（找到目标后返回 false 提前终止）
	var target *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		// 类型断言：只处理函数声明节点，其他节点（变量、类型等）跳过
		fd, ok := n.(*ast.FuncDecl)
		if !ok {
			return true // true 表示继续遍历子节点
		}

		// 通过 FileSet 将 AST 节点的位置信息转换为行号
		// fd.Pos() 返回函数开始的 token.Pos，fd.End() 返回函数结束的 token.Pos
		// fset.Position() 将 token.Pos 转换为文件位置（包含行号、列号、字节偏移）
		s := fset.Position(fd.Pos()).Line
		e := fset.Position(fd.End()).Line

		// 判断指定行号是否在当前函数的范围内（闭区间）
		// 这里使用行号判断而不是字节偏移，因为用户输入的是行号，更直观
		if line >= s && line <= e {
			target = fd
			startLine = s
			endLine = e
			return false // false 表示停止遍历（已找到目标函数）
		}
		return true // 继续遍历
	})

	// 边界检查：如果没有找到包含指定行号的函数，返回错误
	// 可能原因：该行在函数外部（全局变量、类型定义等），或文件为空
	if target == nil {
		err = fmt.Errorf("no function encloses line %d in %s", line, filePath)
		return
	}

	// 关键步骤：使用字节偏移而非行号来提取源码
	// 原因：
	// 1. 字节偏移是精确的（直接从原始文件中切片），保留所有原始格式
	// 2. 如果只用行号，需要逐行拼接，容易丢失缩进、空白行等细节
	// 3. AST 节点的 Pos()/End() 提供精确的字节偏移，这是提取源码的最可靠方式
	start := fset.Position(target.Pos()).Offset // 函数开始的字节偏移
	end := fset.Position(target.End()).Offset   // 函数结束的字节偏移

	// 安全性检查：确保偏移量有效，防止数组越界
	// 理论上不应该发生，但如果 AST 解析异常或文件被修改，可能产生无效偏移
	if start < 0 || end > len(src) || start >= end {
		err = fmt.Errorf("invalid offsets for function: start=%d end=%d len=%d", start, end, len(src))
		return
	}

	// 直接从原始字节切片提取源码片段
	// 优势：完全保留原始格式（缩进、注释、换行符等），提取后的源码可直接使用
	source = string(src[start:end])

	// 从 AST 节点获取函数名（比字符串解析更可靠）
	name = target.Name.Name
	return
}
