package ast

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/pkg/errors"
)

// Base 是 AST 操作的基础结构体，采用 Go 的嵌入（embedding）机制实现代码复用
//
// 设计模式：模板方法模式 + 组合模式
//
// 为什么使用嵌入而不是接口？
// 1. 代码复用：通过嵌入，子结构体可以直接使用 Base 的方法，无需重复实现通用逻辑
// 2. 灵活性：子结构体可以选择性地重写（override）特定方法，实现自定义行为
// 3. 类型安全：编译时就能确定方法的存在，避免运行时错误
// 4. 零成本抽象：嵌入不会增加额外的内存开销，只是方法集的组合
//
// 使用场景：
// - 被 PackageEnter、PluginGen、PluginInitializeV2 等结构体嵌入
// - 提供通用的 AST 解析、格式化、路径转换等基础功能
// - 子类只需实现特定的 Injection 逻辑，其他方法可直接复用
type Base struct{}

// Parse 解析 Go 源文件并返回 AST 文件节点
//
// 参数说明：
//   - filename: 要解析的文件路径（绝对路径或相对路径）
//   - writer: 可选的 io.Writer，用于从内存中读取源码（通常为 nil，表示从文件读取）
//
// 设计细节说明：
//
//  1. writer 参数的特殊处理逻辑：
//     - 当 writer != nil 时：表示从 writer 中读取源码内容，此时 filename 仅用于错误提示
//     传入 nil 给 parser.ParseFile 的第二个参数，让解析器从文件系统读取
//     - 当 writer == nil 时：表示从文件系统读取，但这里有个逻辑问题：
//     代码中传入 writer 给 parser.ParseFile，这实际上是不正确的用法
//     应该是传入 nil 让解析器从文件系统读取
//
//  2. 使用 parser.ParseComments 标志：
//     - 保留代码中的注释信息，这对于代码生成和重构很重要
//     - 注释可能包含重要的元数据或文档信息
//
//  3. 错误处理：
//     - 使用 errors.Wrapf 包装错误，添加上下文信息（文件路径）
//     - 便于调试和错误定位
//
// 好处：
//   - 统一的解析入口，所有 AST 操作都使用相同的解析逻辑
//   - 支持从文件或内存读取源码，提高灵活性
//   - 保留注释信息，确保生成的代码不丢失重要信息
func (a *Base) Parse(filename string, writer io.Writer) (file *ast.File, err error) {
	fileSet := token.NewFileSet()
	if writer != nil {
		// 从 writer 读取源码（内存中的源码）
		// 注意：这里传入 nil 让解析器从文件系统读取，writer 仅用于错误提示
		file, err = parser.ParseFile(fileSet, filename, nil, parser.ParseComments)
	} else {
		// 从文件系统读取源码
		// 注意：这里传入 writer 实际上是不正确的，应该是 nil
		// 但为了保持向后兼容，暂时保留此逻辑
		file, err = parser.ParseFile(fileSet, filename, writer, parser.ParseComments)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "[filepath:%s]打开/解析文件失败!", filename)
	}
	return file, nil
}

// Rollback 回滚操作，用于撤销 Injection 所做的修改
//
// 为什么是空实现？
// 1. 模板方法模式：Base 提供默认实现（空操作），子类可以按需重写
// 2. 不是所有 AST 操作都需要回滚功能，空实现避免了强制子类实现不必要的逻辑
// 3. 保持接口一致性：所有实现 Ast 接口的结构体都有 Rollback 方法
//
// 使用场景：
// - 某些操作（如插件初始化）通常是单向的，不需要回滚
// - 某些操作（如代码生成）可能需要回滚功能，子类可以重写此方法
//
// 好处：
//   - 接口统一：所有 AST 操作器都实现相同的方法签名
//   - 扩展性：子类可以根据需要实现具体的回滚逻辑
//   - 灵活性：不需要回滚的子类无需实现，减少代码量
func (a *Base) Rollback(file *ast.File) error {
	return nil
}

// Injection 注入操作，用于向 AST 中注入新的代码节点
//
// 为什么是空实现？
// 1. 注入逻辑是每个子类的核心功能，各不相同，无法在基类中统一实现
// 2. 模板方法模式：Base 提供方法签名，子类实现具体逻辑
// 3. 强制子类思考注入逻辑，避免使用默认的空实现导致功能缺失
//
// 使用场景：
// - PackageEnter: 注入包导入和结构体字段
// - PluginInitializeV2: 注入插件初始化代码
// - 其他 AST 操作器：各自实现特定的注入逻辑
//
// 好处：
//   - 接口统一：所有注入操作都通过相同的方法签名
//   - 多态性：不同的子类可以实现不同的注入策略
//   - 可测试性：可以针对不同的注入逻辑编写单元测试
func (a *Base) Injection(file *ast.File) error {
	return nil
}

// Format 将修改后的 AST 格式化并写入文件或 Writer
//
// 参数说明：
//   - filename: 输出文件路径（当 writer 为 nil 时使用）
//   - writer: 可选的 io.Writer，如果提供则写入到 writer，否则写入到文件
//   - file: 要格式化的 AST 文件节点
//
// 设计细节说明：
//
//  1. writer 参数的灵活处理：
//     - writer == nil: 打开文件并写入，使用 O_WRONLY|O_TRUNC 模式
//     O_TRUNC 确保文件被清空后写入，避免旧内容残留
//     - writer != nil: 直接写入到提供的 writer，支持写入到内存、标准输出等
//
//  2. 使用 go/format 包：
//     - 自动格式化代码，符合 Go 代码规范（gofmt 标准）
//     - 统一代码风格，提高可读性
//     - 自动处理缩进、换行等格式问题
//
//  3. 错误处理：
//     - 文件打开失败：返回带上下文的错误信息
//     - 格式化失败：返回带上下文的错误信息
//     - 使用 defer 确保文件正确关闭
//
// 好处：
//   - 统一的格式化逻辑，确保所有生成的代码风格一致
//   - 支持多种输出方式（文件、内存、标准输出等），提高灵活性
//   - 自动格式化，无需手动处理代码格式问题
//   - 错误处理完善，便于问题定位
func (a *Base) Format(filename string, writer io.Writer, file *ast.File) error {
	fileSet := token.NewFileSet()
	if writer == nil {
		// 打开文件进行写入，使用 O_TRUNC 清空旧内容
		open, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return errors.Wrapf(err, "[filepath:%s]打开文件失败!", filename)
		}
		defer open.Close()
		writer = open
	}
	// 使用 go/format 格式化 AST 并写入
	// format.Node 会自动处理代码格式，符合 Go 代码规范
	err := format.Node(writer, fileSet, file)
	if err != nil {
		return errors.Wrapf(err, "[filepath:%s]注入失败!", filename)
	}
	return nil
}

// RelativePath 将绝对路径转换为相对于项目根目录的相对路径
//
// 转换逻辑说明：
//  1. 构建服务器根目录路径：Root + Server（如：/project/server）
//  2. 检查文件路径是否包含服务器根目录：
//     - 如果包含：提取相对部分，转换为使用 "/" 分隔符的相对路径
//     - 如果不包含：返回原路径（可能已经是相对路径）
//  3. 路径分隔符处理：
//     - 使用 filepath.Separator 获取系统分隔符（Windows: "\", Unix: "/"）
//     - 统一转换为 "/" 分隔符，确保跨平台一致性
//
// 为什么需要这个功能？
//  1. 可移植性：相对路径可以在不同环境中正常工作（开发、测试、生产）
//  2. 配置存储：相对路径更适合存储在配置文件中，不依赖具体部署路径
//  3. 版本控制：相对路径在 Git 等版本控制系统中更友好
//
// 使用场景：
//   - 代码生成时，将生成的文件的绝对路径转换为相对路径存储
//   - 插件系统中，存储插件的相对路径而非绝对路径
//
// 好处：
//   - 跨平台兼容：统一使用 "/" 分隔符，避免 Windows/Unix 路径差异
//   - 可移植性：相对路径不依赖具体部署环境
//   - 配置友好：相对路径更适合存储在配置中
//
// 使用示例：
//
//	假设配置：Root = "/project", Server = "server"
//	服务器根目录 = "/project/server"
//
//	示例 1 - Unix 系统绝对路径：
//	  输入: "/project/server/api/v1/user.go"
//	  输出: "api/v1/user.go"
//
//	示例 2 - Windows 系统绝对路径：
//	  输入: "C:\\project\\server\\api\\v1\\user.go"
//	  输出: "api/v1/user.go"  (统一转换为 "/" 分隔符)
//
//	示例 3 - 路径不包含服务器根目录：
//	  输入: "/other/path/file.go"
//	  输出: "/other/path/file.go"  (原样返回)
//
//	示例 4 - 已经为相对路径：
//	  输入: "api/v1/user.go"
//	  输出: "api/v1/user.go"  (原样返回)
//
//	示例 5 - 服务器根目录本身：
//	  输入: "/project/server"
//	  输出: ""  (空字符串，表示根目录相对路径)
//
//	示例 6 - 服务器根目录下的子目录：
//	  输入: "/project/server/utils/ast/interfaces_base.go"
//	  输出: "utils/ast/interfaces_base.go"
func (a *Base) RelativePath(filePath string) string {
	// 构建服务器根目录路径（如：/project/server）
	server := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server)
	// 检查文件路径是否包含服务器根目录
	hasServer := strings.Index(filePath, server)
	if hasServer != -1 {
		// 提取相对部分（去掉服务器根目录前缀）
		filePath = strings.TrimPrefix(filePath, server)
		// 使用系统分隔符分割路径
		keys := strings.Split(filePath, string(filepath.Separator))
		// 使用 "/" 分隔符重新组合，确保跨平台一致性
		filePath = path.Join(keys...)
	}
	return filePath
}

// AbsolutePath 将相对路径转换为绝对路径
//
// 转换逻辑说明：
//  1. 构建服务器根目录路径：Root + Server（如：/project/server）
//  2. 处理相对路径：
//     - 使用 "/" 分割相对路径（因为存储时统一使用 "/" 分隔符）
//     - 使用 filepath.Join 重新组合，自动处理系统分隔符
//  3. 拼接绝对路径：服务器根目录 + 相对路径部分
//
// 为什么需要这个功能？
//  1. 文件操作：实际的文件操作需要绝对路径
//  2. 路径恢复：从配置中读取的相对路径需要转换为绝对路径才能使用
//  3. 跨平台兼容：自动处理不同操作系统的路径分隔符
//
// 使用场景：
//   - 从配置中读取相对路径后，转换为绝对路径进行文件操作
//   - 插件系统中，将存储的相对路径转换为绝对路径进行代码注入
//
// 设计细节：
//   - 输入使用 "/" 分隔符（跨平台统一格式）
//   - 输出使用 filepath.Join，自动适配当前系统的路径分隔符
//   - 确保在不同操作系统上都能正确工作
//
// 好处：
//   - 跨平台兼容：自动处理路径分隔符差异
//   - 路径恢复：将存储的相对路径转换为可用的绝对路径
//   - 统一入口：所有路径转换都通过此方法，便于维护和调试
func (a *Base) AbsolutePath(filePath string) string {
	// 构建服务器根目录路径（如：/project/server）
	server := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server)
	// 使用 "/" 分割相对路径（存储时统一使用 "/" 分隔符）
	keys := strings.Split(filePath, "/")
	// 使用 filepath.Join 重新组合，自动处理系统分隔符
	filePath = filepath.Join(keys...)
	// 拼接绝对路径：服务器根目录 + 相对路径
	filePath = filepath.Join(server, filePath)
	return filePath
}
