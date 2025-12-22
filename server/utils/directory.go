package utils

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"go.uber.org/zap"
)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: PathExists
//@description: 文件目录是否存在
//@param: path string
//@return: bool, error

// PathExists 检查指定路径是否存在，并区分是目录还是文件
// 为什么使用 os.Stat 而不是 os.Lstat：
//   - os.Stat 会跟随符号链接，获取目标文件/目录的信息，更符合"路径是否存在"的语义
//   - 如果路径是符号链接，我们通常关心的是链接指向的目标是否存在
//
// 实现逻辑说明：
//  1. 首先尝试获取路径信息，如果成功(err == nil)，说明路径存在
//  2. 使用 fi.IsDir() 判断是否为目录，如果是目录返回 true
//  3. 如果存在但不是目录（即同名文件），返回错误，避免后续操作误判
//  4. 如果错误是 os.IsNotExist，说明路径不存在，返回 false, nil（不是错误情况）
//  5. 其他错误（如权限不足）直接返回，让调用者处理
//
// 好处：
//   - 明确区分"路径不存在"和"存在同名文件"两种情况，避免误操作
//   - 错误处理清晰，调用者可以根据错误类型做不同处理
func PathExists(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err == nil {
		// 路径存在，检查是否为目录
		if fi.IsDir() {
			return true, nil
		}
		// 存在同名文件，返回错误避免误操作
		return false, errors.New("存在同名文件")
	}
	// 路径不存在是正常情况，不是错误
	if os.IsNotExist(err) {
		return false, nil
	}
	// 其他错误（如权限问题）直接返回
	return false, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: CreateDir
//@description: 批量创建文件夹
//@param: dirs ...string
//@return: err error

// CreateDir 批量创建目录，支持一次创建多个目录
// 为什么使用可变参数 ...string：
//   - 提供灵活性，可以一次创建多个目录，减少函数调用次数
//   - 符合 Go 语言惯用法，如 fmt.Println 也使用可变参数
//
// 实现逻辑说明：
//  1. 遍历所有传入的目录路径
//  2. 先检查目录是否已存在，避免重复创建（性能优化）
//  3. 如果 PathExists 返回错误（如存在同名文件），立即返回错误
//  4. 只有当目录不存在时才创建，使用 os.MkdirAll 可以创建多级目录
//  5. 使用 os.ModePerm (0777) 给予最大权限，确保目录可访问
//
// 为什么使用 os.MkdirAll 而不是 os.Mkdir：
//   - os.MkdirAll 可以递归创建多级目录，如 /a/b/c 即使 a 和 b 不存在也能创建
//   - os.Mkdir 只能创建单级目录，父目录不存在会失败
//   - 这样调用者不需要手动创建父目录，使用更方便
//
// 好处：
//   - 幂等性：已存在的目录不会重复创建，多次调用安全
//   - 批量操作：一次调用可以创建多个目录，提高效率
//   - 自动创建父目录：无需手动处理多级目录的创建
//   - 日志记录：记录创建过程，便于调试和监控
func CreateDir(dirs ...string) (err error) {
	for _, v := range dirs {
		// 先检查目录是否存在，避免不必要的创建操作
		exist, err := PathExists(v)
		if err != nil {
			// 如果存在同名文件等错误，立即返回
			return err
		}
		// 只有不存在时才创建
		if !exist {
			global.GVA_LOG.Debug("create directory" + v)
			// os.MkdirAll 可以递归创建多级目录，os.ModePerm 给予最大权限
			if err := os.MkdirAll(v, os.ModePerm); err != nil {
				global.GVA_LOG.Error("create directory"+v, zap.Any(" error:", err))
				return err
			}
		}
	}
	return err
}

//@author: [songzhibin97](https://github.com/songzhibin97)
//@function: FileMove
//@description: 文件移动供外部调用
//@param: src string, dst string(src: 源位置,绝对路径or相对路径, dst: 目标位置,绝对路径or相对路径,必须为文件夹)
//@return: err error

// FileMove 移动文件或目录到目标位置
// 为什么先转换为绝对路径：
//   - filepath.Abs 将相对路径转换为绝对路径，消除路径歧义
//   - 确保在不同工作目录下调用都能正确工作，提高代码健壮性
//   - 便于后续路径操作和错误定位
//
// 为什么使用 goto 和 revoke 标志：
//   - 这是一个防御性编程技巧，确保目录创建后再次验证
//   - 第一次创建目录后，可能由于文件系统延迟等原因，立即检查可能仍失败
//   - 通过 revoke 标志控制只重试一次，避免无限循环
//   - 虽然现代文件系统很少需要，但在高并发或网络文件系统上可能有用
//
// 为什么使用 os.Rename 而不是复制+删除：
//   - os.Rename 是原子操作，要么成功要么失败，不会出现中间状态
//   - 性能更好，不需要复制文件内容，只是修改文件系统元数据
//   - 在同一文件系统内移动文件，Rename 是最快的方式
//
// 实现逻辑说明：
//  1. 空目标路径直接返回，避免无效操作
//  2. 转换为绝对路径，确保路径准确性
//  3. 获取目标目录（dst 的父目录），因为 dst 可能是文件路径
//  4. 检查并创建目标目录（如果不存在）
//  5. 使用 os.Rename 执行移动操作
//
// 好处：
//   - 自动创建目标目录，调用者无需手动处理
//   - 原子操作保证数据一致性
//   - 支持相对路径和绝对路径，使用灵活
func FileMove(src string, dst string) (err error) {
	// 空目标路径直接返回，避免无效操作
	if dst == "" {
		return nil
	}
	// 转换为绝对路径，消除路径歧义，提高健壮性
	src, err = filepath.Abs(src)
	if err != nil {
		return err
	}
	dst, err = filepath.Abs(dst)
	if err != nil {
		return err
	}
	// revoke 标志用于控制重试逻辑，确保目录创建后再次验证
	revoke := false
	// 获取目标文件的父目录，因为 dst 可能是文件路径
	dir := filepath.Dir(dst)
Redirect:
	// 检查目标目录是否存在
	_, err = os.Stat(dir)
	if err != nil {
		// 目录不存在，创建它（包括所有父目录）
		err = os.MkdirAll(dir, 0o755)
		if err != nil {
			return err
		}
		// 创建后再次检查，确保目录真正存在（防御性编程）
		if !revoke {
			revoke = true
			goto Redirect
		}
	}
	// os.Rename 是原子操作，在同一文件系统内移动文件最快且最安全
	return os.Rename(src, dst)
}

// DeLFile 删除文件或目录
// 为什么使用 os.RemoveAll 而不是 os.Remove：
//   - os.RemoveAll 可以递归删除目录及其所有内容
//   - os.Remove 只能删除空目录或单个文件
//   - 使用 RemoveAll 更通用，调用者不需要判断是文件还是目录
//
// 好处：
//   - 简单直接，一个函数处理文件和目录两种情况
//   - 递归删除，无需手动遍历目录
//   - 幂等性：删除不存在的路径不会报错（返回 nil）
func DeLFile(filePath string) error {
	// os.RemoveAll 可以删除文件或目录（包括非空目录），使用简单且安全
	return os.RemoveAll(filePath)
}

//@author: [songzhibin97](https://github.com/songzhibin97)
//@function: TrimSpace
//@description: 去除结构体空格
//@param: target interface (target: 目标结构体,传入必须是指针类型)
//@return: null

// TrimSpace 去除结构体中所有字符串字段的前后空格
// 为什么使用反射（reflect）：
//   - 需要动态处理任意类型的结构体，无法在编译时确定字段类型
//   - 反射允许在运行时检查和修改结构体字段
//   - 这样写一个函数就能处理所有结构体类型，避免为每个结构体写重复代码
//
// 为什么要求传入指针类型：
//   - 只有指针类型才能修改原结构体的值
//   - 如果传入值类型，修改的是副本，不会影响原结构体
//   - 通过 t.Kind() != reflect.Ptr 检查，提前返回避免无效操作
//
// 实现逻辑说明：
//  1. 获取类型信息，检查是否为指针类型
//  2. 使用 Elem() 获取指针指向的实际类型（结构体类型）
//  3. 获取值的反射对象，同样使用 Elem() 获取实际值
//  4. 遍历结构体的所有字段
//  5. 只处理字符串类型的字段，使用 strings.TrimSpace 去除前后空格
//  6. 使用 SetString 将处理后的值写回原字段
//
// 为什么只处理字符串类型：
//   - 空格问题主要出现在字符串字段（如用户输入、配置文件读取等）
//   - 其他类型（int、bool 等）不存在空格问题
//   - 提高效率，避免不必要的类型检查
//
// 好处：
//   - 通用性强：一个函数处理所有结构体类型
//   - 自动化：无需手动为每个字段调用 TrimSpace
//   - 减少代码重复：避免在每个结构体的每个字符串字段上重复处理
//   - 数据清洗：统一处理用户输入或外部数据的前后空格问题
func TrimSpace(target interface{}) {
	// 获取类型信息
	t := reflect.TypeOf(target)
	// 必须是指针类型才能修改原值，否则直接返回
	if t.Kind() != reflect.Ptr {
		return
	}
	// 获取指针指向的实际类型（结构体类型）
	t = t.Elem()
	// 获取值的反射对象，Elem() 获取指针指向的实际值
	v := reflect.ValueOf(target).Elem()
	// 遍历结构体的所有字段
	for i := 0; i < t.NumField(); i++ {
		// 只处理字符串类型的字段
		switch v.Field(i).Kind() {
		case reflect.String:
			// 获取字段值，去除前后空格，然后写回
			v.Field(i).SetString(strings.TrimSpace(v.Field(i).String()))
		}
	}
}

// FileExist 判断文件是否存在（不包括目录）
// 为什么使用 os.Lstat 而不是 os.Stat：
//   - os.Lstat 不会跟随符号链接，直接获取链接本身的信息
//   - 如果路径是符号链接，Lstat 检查的是链接本身是否存在
//   - 在某些场景下，我们可能只关心链接文件本身，而不是它指向的目标
//   - 性能稍好，因为不需要解析符号链接
//
// 为什么返回 !fi.IsDir()：
//   - 如果路径存在且不是目录，说明是文件，返回 true
//   - 如果路径存在但是目录，返回 false（因为我们要检查的是文件）
//   - 这样明确区分文件和目录，避免误判
//
// 为什么返回 !os.IsNotExist(err)：
//   - 如果错误是 os.IsNotExist，说明文件不存在，返回 false
//   - 如果错误是其他类型（如权限不足），返回 true
//   - 这样设计的好处：当无法确定文件是否存在时（如权限问题），返回 true 更安全
//   - 调用者可以根据需要进一步处理权限错误
//
// 与 PathExists 的区别：
//   - PathExists 检查目录是否存在，FileExist 检查文件是否存在
//   - PathExists 返回详细的错误信息，FileExist 只返回布尔值
//   - FileExist 使用 Lstat，PathExists 使用 Stat
//
// 好处：
//   - 明确区分文件和目录，避免混淆
//   - 处理权限错误时更安全（返回 true 而不是 false）
//   - 使用 Lstat 不跟随符号链接，在某些场景下更符合预期
func FileExist(path string) bool {
	// 使用 Lstat 获取文件信息，不跟随符号链接
	fi, err := os.Lstat(path)
	if err == nil {
		// 路径存在，检查是否为文件（不是目录）
		return !fi.IsDir()
	}
	// 如果错误不是"文件不存在"，返回 true（可能是权限问题等，保守处理）
	// 这样设计更安全：当无法确定时，假设文件存在
	return !os.IsNotExist(err)
}
