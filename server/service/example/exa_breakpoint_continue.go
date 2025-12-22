package example

import (
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"gorm.io/gorm"
)

// FileUploadAndDownloadService 文件上传下载服务
// 实现断点续传功能，支持大文件分片上传
// 设计优势：
// 1. 使用 MD5 作为文件唯一标识，可以快速判断文件是否已存在，避免重复上传
// 2. 分片上传可以支持大文件，降低单次请求的内存占用和网络超时风险
// 3. 断点续传机制可以在网络中断后从上次位置继续上传，提升用户体验
type FileUploadAndDownloadService struct{}

var FileUploadAndDownloadServiceApp = new(FileUploadAndDownloadService)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: FindOrCreateFile
//@description: 上传文件时检测当前文件属性，如果没有文件则创建，有则返回文件的当前切片
//@param: fileMd5 string, fileName string, chunkTotal int
//@return: file model.ExaFile, err error

// FindOrCreateFile 查找或创建文件记录
// 功能说明：
//  1. 首先检查是否存在已完成上传的文件（is_finish = true）
//     使用 MD5 值查询，因为 MD5 是文件的唯一指纹，相同内容的文件 MD5 相同
//     好处：可以快速判断文件是否已完整上传，避免重复上传相同文件
//
// 2. 如果不存在已完成的文件：
//   - 查找是否存在未完成的文件记录（可能是之前上传中断的）
//   - 如果不存在则创建新记录
//   - 使用 Preload("ExaFileChunk") 预加载已上传的切片信息
//     好处：返回已上传的切片列表，前端可以跳过这些切片，实现断点续传
//
// 3. 如果存在已完成的文件：
//   - 创建新记录并标记为已完成，复用已存在文件的路径
//     好处：相同文件可以快速完成上传，无需重新上传
//
// 设计优势：
// - 使用 MD5 作为主键查询，查询效率高
// - 支持断点续传，提升大文件上传体验
// - 避免重复上传相同文件，节省带宽和存储
func (e *FileUploadAndDownloadService) FindOrCreateFile(fileMd5 string, fileName string, chunkTotal int) (file example.ExaFile, err error) {
	var cfile example.ExaFile
	// 构建文件记录的基础信息
	cfile.FileMd5 = fileMd5       // MD5 值作为文件唯一标识，相同文件内容 MD5 相同
	cfile.FileName = fileName     // 文件名，用于区分同名但内容不同的文件
	cfile.ChunkTotal = chunkTotal // 总切片数，用于判断上传进度

	// 检查是否存在已完成上传的文件（is_finish = true）
	// 使用 errors.Is 判断是否为记录不存在的错误，这是 GORM 的标准做法
	// 好处：可以区分"记录不存在"和"其他数据库错误"，逻辑更清晰
	if errors.Is(global.GVA_DB.Where("file_md5 = ? AND is_finish = ?", fileMd5, true).First(&file).Error, gorm.ErrRecordNotFound) {
		// 不存在已完成的文件，查找或创建未完成的文件记录
		// Preload("ExaFileChunk") 预加载关联的切片记录
		// 好处：一次查询获取文件信息和已上传切片列表，减少数据库查询次数
		// FirstOrCreate 如果找到则返回，找不到则创建，原子操作保证并发安全
		err = global.GVA_DB.Where("file_md5 = ? AND file_name = ?", fileMd5, fileName).Preload("ExaFileChunk").FirstOrCreate(&file, cfile).Error
		return file, err
	}
	// 存在已完成的文件，创建新记录并复用文件路径
	// 好处：相同文件可以快速完成，无需重新上传和存储
	cfile.IsFinish = true
	cfile.FilePath = file.FilePath // 复用已存在文件的路径，节省存储空间
	err = global.GVA_DB.Create(&cfile).Error
	return cfile, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: CreateFileChunk
//@description: 创建文件切片记录
//@param: id uint, fileChunkPath string, fileChunkNumber int
//@return: error

// CreateFileChunk 创建文件切片记录
// 功能说明：
// 每次上传一个文件切片时，都会调用此函数记录切片信息
// 记录内容包括：切片存储路径、所属文件ID、切片序号
//
// 设计优势：
//
//  1. 每个切片独立记录，可以精确追踪上传进度
//     好处：前端可以根据已上传切片列表，只上传缺失的切片，实现断点续传
//
//  2. 使用切片序号（FileChunkNumber）标识切片顺序
//     好处：合并文件时可以按序号排序，确保文件完整性
//
//  3. 切片路径独立存储
//     好处：可以灵活管理切片存储位置，支持分布式存储
//
//  4. 通过外键关联文件记录（ExaFileID）
//     好处：维护数据一致性，删除文件时可以级联删除所有切片
func (e *FileUploadAndDownloadService) CreateFileChunk(id uint, fileChunkPath string, fileChunkNumber int) error {
	var chunk example.ExaFileChunk
	chunk.FileChunkPath = fileChunkPath     // 切片在服务器上的存储路径
	chunk.ExaFileID = id                    // 关联的文件ID，建立外键关系
	chunk.FileChunkNumber = fileChunkNumber // 切片序号，用于合并时排序
	err := global.GVA_DB.Create(&chunk).Error
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteFileChunk
//@description: 删除文件切片记录
//@param: fileMd5 string, fileName string, filePath string
//@return: error

// DeleteFileChunk 删除文件切片记录（文件上传完成后调用）
// 功能说明：
// 当所有切片上传完成并合并成完整文件后，调用此函数清理切片记录
// 1. 更新文件状态为已完成，并保存合并后的文件路径
// 2. 删除所有临时切片记录
//
// 设计优势：
//
//  1. 先更新文件状态再删除切片
//     好处：如果删除失败，文件状态仍然是未完成，可以重新尝试合并
//
//  2. 使用 Updates 方法批量更新
//     好处：原子操作，保证数据一致性，避免并发问题
//
//  3. 使用 Unscoped().Delete() 物理删除切片记录
//     好处：切片已合并成完整文件，不再需要切片记录，物理删除可以节省存储空间
//     注意：如果使用软删除，切片记录会一直保留，占用数据库空间
//
//  4. 通过 fileMd5 查找文件
//     好处：使用唯一标识快速定位，查询效率高
//
// 工作流程：
// 上传完成 -> 合并切片 -> 调用此函数 -> 更新文件状态 -> 删除切片记录
// 这样设计的好处是：切片记录只在合并过程中需要，合并完成后立即清理，保持数据库整洁
func (e *FileUploadAndDownloadService) DeleteFileChunk(fileMd5 string, filePath string) error {
	var chunks []example.ExaFileChunk
	var file example.ExaFile
	// 根据 MD5 查找文件记录，并更新为已完成状态
	// 使用 Updates 方法批量更新多个字段，原子操作保证一致性
	err := global.GVA_DB.Where("file_md5 = ?", fileMd5).First(&file).
		Updates(map[string]interface{}{
			"IsFinish":  true,     // 标记文件上传已完成
			"file_path": filePath, // 保存合并后的完整文件路径
		}).Error
	if err != nil {
		return err
	}
	// 删除该文件的所有切片记录
	// Unscoped().Delete() 表示物理删除，而不是软删除
	// 好处：切片已合并，不再需要切片记录，物理删除可以释放数据库空间
	// 如果使用软删除，这些记录会一直保留，造成数据冗余
	err = global.GVA_DB.Where("exa_file_id = ?", file.ID).Delete(&chunks).Unscoped().Error
	return err
}
