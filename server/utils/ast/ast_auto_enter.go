package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
)

// ImportForAutoEnter 自动向指定结构体类型添加字段（如果字段不存在）
//
// 功能说明：
// 1. 解析 Go 源文件为 AST（抽象语法树）
// 2. 查找指定名称的类型声明（TypeSpec）
// 3. 检查该类型是否为结构体类型（StructType）
// 4. 遍历结构体字段，检查是否已存在指定类型的字段
// 5. 如果不存在，则添加新字段
// 6. 将修改后的 AST 转换回源代码并写回文件
//
// 设计思路：为什么使用 AST 而不是字符串操作或正则表达式？
// 1. 语法安全性：AST 保证生成的代码语法正确，不会破坏原有代码结构
// 2. 精确性：能够准确理解代码语义，只修改目标结构体，不影响其他代码
// 3. 格式化：使用 printer 包自动格式化代码，符合 Go 代码规范（缩进、对齐等）
// 4. 可维护性：不依赖正则表达式，代码更清晰易懂，减少匹配错误
// 5. 类型安全：生成的字段类型正确，编译时能够检查类型错误
//
// 参数说明：
//   - path: 目标 Go 源文件的路径（绝对路径或相对路径）
//   - funcName: 要查找的结构体类型名称（如 "ApiGroup"、"RouterGroup"）
//   - code: 要添加的字段类型名称（如 "UserApi"、"OrderService"）
//
// 使用场景示例：
// 输入文件 api/enter.go：
//
//	package api
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	}
//
// 调用代码：
//
//	ImportForAutoEnter("api/enter.go", "ApiGroup", "UserApi")
//
// 输出文件 api/enter.go：
//
//	package api
//
//	type ApiGroup struct {
//	    SystemApi system.SystemApi
//	    UserApi   // 新增字段（字段类型为 UserApi，字段名也为 UserApi）
//	}
//
// 注意事项：
// 1. 错误处理：当前实现使用 fmt.Println 输出错误，不会中断执行，建议改进错误处理
// 2. 字段去重：通过遍历现有字段检查类型名称，避免重复添加相同类型的字段
// 3. 字段命名：当前实现只设置了字段类型，字段名默认为类型名，如需自定义字段名需要额外处理
// 4. 文件权限：写回文件时使用 0666 权限，保持文件可读写
// 5. 代码格式：使用 printer.Fprint 自动格式化，保持代码风格一致
func ImportForAutoEnter(path string, funcName string, code string) {
	// 步骤1: 读取源文件内容
	// 为什么先读取文件：AST 操作需要完整的源代码作为输入
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
	}

	// 步骤2: 创建文件集合（FileSet）
	// 为什么需要 FileSet：FileSet 管理源代码的位置信息，用于错误报告和调试
	// 即使不显示位置信息，printer 也需要 FileSet 来正确格式化代码
	fileSet := token.NewFileSet()

	// 步骤3: 解析源代码为 AST
	// 为什么使用 parser.ParseFile：
	// 1. 将文本代码转换为结构化的 AST 节点树
	// 2. 第一个参数 "" 表示从字符串解析（不是从文件路径解析）
	// 3. 第四个参数 0 表示使用默认解析模式（可以组合 parser.ParseComments 等标志）
	astFile, err := parser.ParseFile(fileSet, "", src, 0)

	// 步骤4: 遍历 AST 查找目标类型并添加字段
	// 为什么使用 ast.Inspect：递归遍历所有 AST 节点，查找匹配的类型声明
	// 返回 true 继续遍历子节点，返回 false 停止遍历（但已找到并处理完目标节点）
	ast.Inspect(astFile, func(node ast.Node) bool {
		// 类型断言：检查节点是否为类型声明（type Xxx struct { ... }）
		// 为什么使用类型断言：AST 中的节点类型很多（函数、变量、类型等），需要精确匹配
		if typeSpec, ok := node.(*ast.TypeSpec); ok {
			// 检查类型名称是否匹配目标结构体名称
			// 为什么精确匹配名称：确保只修改指定的结构体，避免误修改其他类型
			if typeSpec.Name.Name == funcName {
				// 类型断言：检查类型是否为结构体类型（struct { ... }）
				// 为什么检查结构体类型：只有结构体才有字段列表，其他类型（如接口）不能添加字段
				if st, ok := typeSpec.Type.(*ast.StructType); ok {
					// 步骤5: 遍历现有字段，检查是否已存在相同类型的字段
					// 为什么需要检查：避免重复添加相同字段，保持代码整洁
					// 为什么使用 range 而不是 for-each：需要通过索引访问字段列表
					for i := range st.Fields.List {
						// 类型断言：检查字段类型是否为标识符类型（如 UserApi，而不是 *UserApi 或 []UserApi）
						// 为什么只检查 Ident 类型：当前实现只支持简单类型名，不支持指针、切片等复杂类型
						if t, ok := st.Fields.List[i].Type.(*ast.Ident); ok {
							// 如果找到相同类型的字段，停止遍历（返回 false）
							// 为什么返回 false：已经确认字段存在，无需继续查找和添加
							if t.Name == code {
								return false
							}
						}
					}

					// 步骤6: 创建新字段节点
					// 为什么创建 ast.Field：AST 操作必须创建对应的节点对象，不能直接修改字符串
					// 字段类型使用 ast.Ident：标识符类型，表示简单的类型名称（如 UserApi）
					sn := &ast.Field{
						Type: &ast.Ident{Name: code},
					}

					// 步骤7: 将新字段追加到结构体的字段列表
					// 为什么使用 append：向切片追加元素，保持原有字段不变
					// 修改 AST 节点：直接修改节点结构，后续会被序列化为源代码
					st.Fields.List = append(st.Fields.List, sn)
				}
			}
		}
		// 返回 true：继续遍历其他节点，查找其他可能的匹配项
		return true
	})

	// 步骤8: 准备缓冲区，用于存储格式化后的源代码
	// 为什么使用 bytes.Buffer：提供高效的字节缓冲区操作
	// 为什么先声明 out：printer.Fprint 需要一个 io.Writer，bytes.Buffer 实现了该接口
	var out []byte
	bf := bytes.NewBuffer(out)

	// 步骤9: 将修改后的 AST 转换回源代码格式
	// 为什么使用 printer.Fprint：
	// 1. 将 AST 节点序列化为 Go 源代码文本
	// 2. 自动应用 Go 代码格式化规则（缩进、换行、对齐等）
	// 3. 保证输出的代码符合 gofmt 标准
	err = printer.Fprint(bf, fileSet, astFile)
	if err != nil {
		return
	}

	// 步骤10: 将格式化后的源代码写回文件
	// 为什么使用 bf.Bytes()：获取缓冲区中的字节数据
	// 为什么使用 0666 权限：允许所有用户读写，保持文件的可访问性
	// 为什么忽略返回值：当前实现不关心写入是否成功（建议改进错误处理）
	_ = os.WriteFile(path, bf.Bytes(), 0666)
}
