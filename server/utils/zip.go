package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unzip 解压 ZIP 文件到指定目录
// 参数:
//   - zipFile: ZIP 文件的路径
//   - destDir: 目标解压目录
//
// 返回:
//   - []string: 解压后的所有文件路径列表，便于后续操作（如批量处理、清理等）
//   - error: 解压过程中的错误信息
//
// 设计说明：
// 1. 返回文件路径列表的好处：
//   - 调用方可以知道解压了哪些文件，便于后续处理
//   - 支持批量操作（如批量删除、权限设置等）
//   - 提供操作的可追溯性
//
// 2. 安全考虑：
//   - 防止路径遍历攻击（Zip Slip 漏洞）
//   - 使用 filepath.Join 确保路径安全拼接
func Unzip(zipFile string, destDir string) ([]string, error) {
	// 打开 ZIP 文件进行读取
	// 使用 zip.OpenReader 而不是 zip.NewReader，因为前者会自动处理文件打开和关闭
	zipReader, err := zip.OpenReader(zipFile)
	var paths []string
	if err != nil {
		// 返回空切片而不是 nil，保持返回类型一致性，避免调用方需要额外判断 nil
		return []string{}, err
	}
	// 使用 defer 确保 ZIP 文件资源一定会被关闭
	// 好处：即使函数中途返回或发生 panic，也能保证资源释放，防止文件句柄泄漏
	defer zipReader.Close()

	// 遍历 ZIP 文件中的所有条目（文件或目录）
	for _, f := range zipReader.File {
		// 安全检查：防止路径遍历攻击（Zip Slip 漏洞）
		// 为什么需要这个检查：
		//   - 恶意 ZIP 文件可能包含 "../" 路径，如 "../../etc/passwd"
		//   - 如果不检查，解压时可能覆盖系统关键文件，造成安全风险
		//   - 这是 OWASP 明确指出的安全漏洞
		// 注意：这里使用简单的 Contains 检查，更严格的实现可以使用 filepath.Clean 和路径验证
		if strings.Contains(f.Name, "..") {
			return []string{}, fmt.Errorf("%s 文件名不合法", f.Name)
		}

		// 使用 filepath.Join 构建目标路径
		// 好处：
		//   - 自动处理不同操作系统的路径分隔符（Windows 的 \ 和 Unix 的 /）
		//   - 自动处理路径中的多余分隔符
		//   - 确保路径规范化，避免路径拼接错误
		fpath := filepath.Join(destDir, f.Name)

		// 记录解压后的文件路径，用于返回值
		// 这样调用方可以知道解压了哪些文件，便于后续处理
		paths = append(paths, fpath)

		// 判断当前条目是目录还是文件
		if f.FileInfo().IsDir() {
			// 如果是目录，创建目录结构
			// 使用 MkdirAll 而不是 Mkdir：
			//   - MkdirAll 会递归创建所有必需的父目录
			//   - 如果目录已存在不会报错，适合幂等操作
			//   - os.ModePerm (0777) 给予最大权限，实际权限会受到 umask 限制
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			// 如果是文件，需要先确保其父目录存在
			// 为什么需要这一步：
			//   - ZIP 文件中可能只有文件路径，没有显式的目录条目
			//   - 例如：ZIP 中有 "subdir/file.txt"，但可能没有 "subdir/" 目录条目
			//   - 如果不创建父目录，后续 OpenFile 会失败
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return []string{}, err
			}

			// 打开 ZIP 文件中的源文件进行读取
			inFile, err := f.Open()
			if err != nil {
				return []string{}, err
			}
			// 使用 defer 确保源文件句柄关闭
			// 注意：在循环中使用 defer 需要注意，defer 会在函数返回时执行
			// 这里在循环中 defer，意味着所有文件句柄会在函数结束时统一关闭
			// 对于大量文件，可能会占用较多资源，但保证了资源安全释放
			defer inFile.Close()

			// 创建目标文件用于写入
			// 标志位说明：
			//   - os.O_WRONLY: 只写模式
			//   - os.O_CREATE: 如果文件不存在则创建
			//   - os.O_TRUNC: 如果文件存在则清空（覆盖）
			// 权限：使用 f.Mode() 保留原始文件的权限信息
			// 好处：保持文件权限一致性，确保解压后的文件权限与压缩前一致
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return []string{}, err
			}
			// 使用 defer 确保目标文件句柄关闭
			// 好处：即使后续 io.Copy 失败，文件句柄也会被正确关闭
			defer outFile.Close()

			// 使用 io.Copy 复制文件内容
			// 为什么使用 io.Copy：
			//   - 自动处理缓冲区，效率高（默认 32KB 缓冲区）
			//   - 适合大文件传输，避免一次性加载到内存
			//   - 返回复制的字节数和错误，便于调试和验证
			// 注意：这里忽略了返回的字节数，如果需要可以用于验证文件完整性
			_, err = io.Copy(outFile, inFile)
			if err != nil {
				return []string{}, err
			}
		}
	}
	// 返回所有解压文件的路径列表和 nil 错误
	// 这样调用方可以知道解压了哪些文件，便于后续操作
	return paths, nil
}
