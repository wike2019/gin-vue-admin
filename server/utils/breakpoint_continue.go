package utils

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// 断点续传功能说明：
// 1. 前端传来文件片与当前片为什么文件的第几片
// 2. 后端拿到以后比较次分片是否上传 或者是否为不完全片
// 3. 前端发送每片多大
// 4. 前端告知是否为最后一片且是否完成

const (
	// breakpointDir: 断点续传临时切片存储目录
	// 设计思路：将文件切片存储在临时目录中，每个文件使用其MD5值作为子目录名
	// 好处：
	//   - 通过MD5值组织切片，同一文件的切片自动归类，便于查找和管理
	//   - 即使多个用户同时上传同名文件，也不会产生冲突（MD5唯一）
	//   - 临时目录与最终文件分离，上传失败时便于清理，不影响已完成的文件
	breakpointDir = "./breakpointDir/"
	// finishDir: 文件上传完成后的最终存储目录
	// 设计思路：所有上传完成的文件统一存放在此目录
	// 好处：
	//   - 临时切片和最终文件分离，逻辑清晰
	//   - 便于后续的文件管理和访问控制
	finishDir = "./fileDir/"
)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: BreakPointContinue
//@description: 断点续传 - 接收并保存文件切片
//@param: content []byte, fileName string, contentNumber int, contentTotal int, fileMd5 string
//@return: error, string

// BreakPointContinue 处理断点续传的文件切片上传
// 设计思路：
//  1. 使用文件的MD5值作为目录名，将同一文件的所有切片存储在同一个目录下
//  2. 每个切片独立保存，通过序号标识其在原文件中的位置
//  3. 只有在所有切片都上传完成后，才会合并成完整文件
//
// 为什么这么写：
//   - 使用MD5值作为目录名：确保同一文件的不同上传会话使用相同目录，支持真正的断点续传
//     如果网络中断，下次上传相同文件时，已经上传的切片仍然存在，可以从中断处继续
//   - 使用 os.MkdirAll：如果目录已存在则不做操作，不存在则递归创建，幂等性好
//     好处：并发上传时多个协程同时调用不会出错，系统会自动处理目录已存在的情况
//   - 每个切片单独保存：即使某个切片上传失败，其他切片不受影响
//     好处：最大程度减少重复上传，提高上传效率
//
// 参数说明：
//   - contentNumber: 当前切片的序号（从0开始），用于标识切片在原文件中的位置
//   - contentTotal: 总切片数，可用于验证上传进度（当前代码未使用，但保留以支持未来扩展）
//   - fileMd5: 整个文件的MD5值，用作目录名和组织切片的标识
func BreakPointContinue(content []byte, fileName string, contentNumber int, contentTotal int, fileMd5 string) (string, error) {
	// 构建基于MD5的切片存储路径
	// 好处：同一文件的所有切片都在同一个目录，便于管理和查找
	path := breakpointDir + fileMd5 + "/"

	// 创建目录（如果不存在）
	// os.ModePerm (0777) 允许所有用户读写执行
	// 好处：确保目录存在，避免后续文件创建失败
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return path, err
	}

	// 将切片内容保存到文件
	pathC, err := makeFileContent(content, fileName, path, contentNumber)
	return pathC, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: CheckMd5
//@description: 检查切片MD5值，验证数据完整性
//@param: content []byte, chunkMd5 string
//@return: CanUpload bool

// CheckMd5 验证文件切片的完整性
// 设计思路：
//
//	前端在上传每个切片时，会计算该切片的MD5值并一起发送
//	后端接收到数据后，重新计算MD5值并与前端发送的MD5值进行比较
//
// 为什么这么写：
//   - MD5校验可以检测数据传输过程中的错误（网络丢包、数据损坏等）
//   - 如果MD5不匹配，说明切片在传输过程中被损坏，应该丢弃
//     好处：避免保存损坏的切片，确保最终合并的文件完整性
//   - 如果MD5匹配，说明切片完整且正确，可以保存
//     好处：在数据层面保证文件传输的可靠性
//
// 这样写的意义：
//   - 提高上传可靠性：在网络不稳定的情况下，能及时发现并拒绝损坏的切片
//   - 保证文件完整性：只有所有切片都完整正确，最终合并的文件才是正确的
//   - 节省存储空间：及时发现损坏切片，避免保存无效数据
func CheckMd5(content []byte, chunkMd5 string) (CanUpload bool) {
	// 计算接收到的切片内容的MD5值
	fileMd5 := MD5V(content)

	// 与前端发送的MD5值进行比较
	if fileMd5 == chunkMd5 {
		return true // 可以继续上传 - MD5匹配，切片完整
	} else {
		return false // 切片不完整，废弃 - MD5不匹配，数据可能损坏
	}
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: makeFileContent
//@description: 创建切片文件，将切片内容保存到磁盘
//@param: content []byte, fileName string, FileDir string, contentNumber int
//@return: string, error

// makeFileContent 将文件切片保存到磁盘
// 设计思路：
//  1. 先进行路径安全检查，防止路径遍历攻击
//  2. 为每个切片创建独立文件，文件名格式：原文件名_切片序号
//  3. 使用defer确保文件句柄正确关闭
//
// 为什么这么写：
//
//   - 路径安全检查（检查".."）：
//     好处：防止恶意文件名（如"../../../etc/passwd"）导致的路径遍历攻击
//     这是安全编程的常见做法，保护系统文件不被意外覆盖或读取
//
//   - 文件名格式：fileName + "_" + contentNumber
//     好处：
//
//   - 文件名包含原文件名，便于识别
//
//   - 包含切片序号，便于按顺序读取和合并
//
//   - 简单明了，避免复杂的编码方案
//
//   - 使用 os.Create：
//     好处：如果文件已存在会覆盖，支持重新上传失败的切片
//     这样断点续传时，如果某个切片之前上传失败，现在重新上传可以覆盖旧数据
//
//   - 使用 defer f.Close()：
//     好处：无论函数如何返回（正常返回或错误返回），文件句柄都会被关闭
//     避免文件句柄泄漏，这是Go语言资源管理的最佳实践
//
// 注意事项：
//   - 虽然使用了defer，但在错误情况下仍然返回了path，调用者可以通过path判断哪个文件创建失败
func makeFileContent(content []byte, fileName string, FileDir string, contentNumber int) (string, error) {
	// 安全检查：防止路径遍历攻击
	// 检查文件名和目录路径中是否包含 ".."（父目录引用）
	// 如果包含，可能是恶意构造的路径，尝试访问系统其他目录
	// 好处：防止安全漏洞，保护系统文件
	if strings.Contains(fileName, "..") || strings.Contains(FileDir, "..") {
		return "", errors.New("文件名或路径不合法")
	}

	// 构建切片文件的完整路径
	// 格式：目录路径 + 原文件名 + "_" + 切片序号
	// 例如：./breakpointDir/abc123/example.txt_0
	// 好处：通过序号可以确定切片的顺序，便于后续按顺序合并
	path := FileDir + fileName + "_" + strconv.Itoa(contentNumber)

	// 创建文件（如果已存在则覆盖）
	// 好处：支持重新上传失败的切片，实现真正的断点续传
	f, err := os.Create(path)
	if err != nil {
		return path, err
	} else {
		// 将切片内容写入文件
		_, err = f.Write(content)
		if err != nil {
			return path, err
		}
	}

	// 延迟关闭文件句柄
	// 好处：确保文件正确关闭，避免资源泄漏
	// 即使前面发生错误返回，defer也会执行，保证资源释放
	defer f.Close()
	return path, nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: MakeFile
//@description: 合并所有切片为完整文件
//@param: fileName string, FileMd5 string
//@return: error, string

// MakeFile 将所有切片按顺序合并成完整的文件
// 设计思路：
//  1. 读取切片目录中的所有文件
//  2. 按照切片序号（文件名中的数字部分）顺序读取
//  3. 使用APPEND模式依次写入最终文件
//  4. 如果合并失败，删除不完整的文件
//
// 为什么这么写：
//
//   - 使用 os.ReadDir 读取目录：
//     好处：获取所有切片文件，用于后续按顺序合并
//     注意：这里依赖切片文件名中的序号，按 k 的顺序（0,1,2...）读取
//
//   - 使用 os.OpenFile 的 APPEND 模式：
//     os.O_RDWR|os.O_CREATE|os.O_APPEND 组合：
//
//   - O_RDWR: 读写模式
//
//   - O_CREATE: 如果文件不存在则创建
//
//   - O_APPEND: 追加模式，每次写入都追加到文件末尾
//     好处：按序号顺序读取切片并追加写入，确保文件内容的正确顺序
//
//   - 文件权限 0644：
//
//   - 6 (rw-): 所有者可读写
//
//   - 4 (r--): 组用户可读
//
//   - 4 (r--): 其他用户可读
//     好处：合理的权限设置，保护文件安全
//
//   - 合并失败时删除不完整文件：
//     好处：避免留下损坏或不完整的文件，确保数据完整性
//     如果某个切片读取或写入失败，最终文件可能不完整，删除后可以重新上传
//
//   - 使用 defer 关闭文件：
//     好处：确保文件句柄正确释放，避免资源泄漏
//
// 这样写的意义：
//   - 实现切片到完整文件的转换，完成断点续传的最后一个环节
//   - 通过顺序合并确保文件内容的正确性
//   - 通过错误处理保证数据的完整性
//
// 潜在改进点：
//   - 可以考虑对切片序号进行排序，确保即使文件名顺序不对也能正确合并
//   - 可以验证切片数量是否完整，避免部分切片丢失
func MakeFile(fileName string, FileMd5 string) (string, error) {
	// 读取切片目录，获取所有切片文件
	// 注意：这里假设切片文件名按照序号命名（fileName_0, fileName_1, ...）
	rd, err := os.ReadDir(breakpointDir + FileMd5)
	if err != nil {
		return finishDir + fileName, err
	}

	// 确保最终文件目录存在
	_ = os.MkdirAll(finishDir, os.ModePerm)

	// 打开或创建最终文件，使用追加模式
	// 好处：按顺序追加写入切片内容，确保文件内容的正确顺序
	fd, err := os.OpenFile(finishDir+fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return finishDir + fileName, err
	}
	defer fd.Close() // 确保文件关闭

	// 按序号顺序读取并合并所有切片
	// 注意：这里依赖 ReadDir 返回的顺序，应该与切片序号一致
	for k := range rd {
		// 读取第 k 个切片文件
		content, _ := os.ReadFile(breakpointDir + FileMd5 + "/" + fileName + "_" + strconv.Itoa(k))

		// 将切片内容追加到最终文件
		_, err = fd.Write(content)
		if err != nil {
			// 如果写入失败，删除不完整的文件
			// 好处：避免留下损坏的文件，可以重新上传
			_ = os.Remove(finishDir + fileName)
			return finishDir + fileName, err
		}
	}
	return finishDir + fileName, nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: RemoveChunk
//@description: 移除指定文件的所有切片，清理临时文件
//@param: FileMd5 string
//@return: error

// RemoveChunk 删除指定文件的所有切片
// 设计思路：
//
//	文件上传并合并完成后，临时切片已经不再需要
//	应该及时清理，释放存储空间
//
// 为什么这么写：
//
//   - 使用 os.RemoveAll：
//     好处：递归删除整个目录及其所有内容
//     由于切片是按 MD5 值存储在独立目录中，删除整个目录即可清除所有切片
//
//   - 基于 FileMd5 删除：
//     好处：通过 MD5 值精确定位到特定文件的切片目录
//     不会影响其他文件的切片（每个文件的 MD5 不同）
//
// 这样写的意义：
//   - 资源管理：及时清理临时文件，避免磁盘空间浪费
//   - 数据安全：上传完成后删除切片，减少数据泄露风险
//   - 系统维护：保持临时目录的整洁，便于后续文件上传
//
// 使用场景：
//   - 文件上传完成并成功合并后调用
//   - 清理失败的上传任务（如果实现了超时机制）
//   - 定期清理旧的、未完成的上传任务
func RemoveChunk(FileMd5 string) error {
	// 删除该文件MD5对应的所有切片目录
	// os.RemoveAll 会递归删除目录及其所有内容
	// 好处：一次性清理所有临时切片，释放存储空间
	err := os.RemoveAll(breakpointDir + FileMd5)
	return err
}
