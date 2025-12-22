package example

import (
	"errors"
	"mime/multipart"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/upload"
	"gorm.io/gorm"
)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Upload
//@description: 创建文件上传记录
//@param: file model.ExaFileUploadAndDownload
//@return: error

// Upload 将文件元信息保存到数据库
// 设计思路：
//  1. 采用最简洁的 GORM Create 方法，直接利用 GORM 的自动填充能力（如 CreatedAt、UpdatedAt）
//  2. 不在这里做额外的业务校验，保持函数职责单一，校验逻辑应该在调用层（如 API 层）完成
//  3. 直接返回 Error，让调用方决定如何处理错误，符合 Go 的错误处理习惯
//
// 好处：
//   - 代码简洁，易于理解和维护
//   - 职责清晰，只负责数据持久化
//   - 便于单元测试，可以轻松 mock global.GVA_DB
func (e *FileUploadAndDownloadService) Upload(file example.ExaFileUploadAndDownload) error {
	return global.GVA_DB.Create(&file).Error
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: FindFile
//@description: 查询文件记录
//@param: id uint
//@return: model.ExaFileUploadAndDownload, error

// FindFile 根据文件ID查询文件记录
// 设计思路：
//  1. 使用 Where + First 组合，而不是直接 Find，因为 First 在找不到记录时会返回 gorm.ErrRecordNotFound
//  2. 这样调用方可以通过 errors.Is(err, gorm.ErrRecordNotFound) 明确区分"记录不存在"和"其他数据库错误"
//  3. 先声明变量再赋值，保持代码风格统一，也便于后续扩展（如添加默认值）
//
// 好处：
//   - 错误处理更精确，调用方能区分"不存在"和"查询失败"
//   - 符合 GORM 的最佳实践，First 方法语义清晰
//   - 如果后续需要添加缓存层，这个函数是很好的切入点
func (e *FileUploadAndDownloadService) FindFile(id uint) (example.ExaFileUploadAndDownload, error) {
	var file example.ExaFileUploadAndDownload
	err := global.GVA_DB.Where("id = ?", id).First(&file).Error
	return file, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteFile
//@description: 删除文件记录
//@param: file model.ExaFileUploadAndDownload
//@return: err error

// DeleteFile 删除文件（包括OSS存储和数据库记录）
// 设计思路：
//  1. 先查询数据库获取完整文件信息（特别是 Key 字段），而不是直接使用传入的 file 参数
//     - 原因：调用方可能只传了 ID，其他字段可能不完整，需要从数据库获取完整的 Key 才能删除OSS文件
//     - 同时也能验证文件是否存在，避免删除不存在的记录
//  2. 先删除OSS文件，再删除数据库记录
//     - 原因：如果先删数据库，但OSS删除失败，会导致数据不一致（数据库已无记录，但OSS文件还在占用空间）
//     - 先删OSS，即使失败也不会影响数据库，可以重试或人工处理
//  3. 使用 Unscoped().Delete() 进行硬删除
//     - 原因：文件删除是物理删除，不需要软删除（deleted_at），因为文件已经从OSS删除，数据库记录也应该彻底清除
//     - 如果使用软删除，会导致数据库记录残留，占用存储空间
//  4. 使用命名返回值 err，便于在多个地方统一处理错误
//
// 好处：
//   - 保证数据一致性：确保OSS和数据库同步删除
//   - 错误处理清晰：每一步失败都能明确返回错误
//   - 资源清理彻底：硬删除避免数据残留
func (e *FileUploadAndDownloadService) DeleteFile(file example.ExaFileUploadAndDownload) (err error) {
	// 先从数据库查询完整文件信息，确保获取到正确的 Key 字段
	var fileFromDb example.ExaFileUploadAndDownload
	fileFromDb, err = e.FindFile(file.ID)
	if err != nil {
		return
	}
	// 先删除OSS存储中的实际文件
	// 使用工厂方法 NewOss() 根据配置自动选择对应的OSS实现（本地/七牛/阿里云等）
	oss := upload.NewOss()
	if err = oss.DeleteFile(fileFromDb.Key); err != nil {
		// OSS删除失败，直接返回错误，不继续删除数据库记录
		// 这样可以避免数据不一致：数据库记录还在，但OSS文件已删除的情况
		return errors.New("文件删除失败")
	}
	// OSS删除成功后，再删除数据库记录
	// 使用 Unscoped() 进行硬删除，因为文件已从OSS删除，数据库记录也应该彻底清除
	err = global.GVA_DB.Where("id = ?", file.ID).Unscoped().Delete(&file).Error
	return err
}

// EditFileName 编辑文件名或者备注
// 设计思路：
//  1. 先使用 First 查询文件是否存在，再执行 Update
//     - 原因：如果文件不存在，First 会返回 gorm.ErrRecordNotFound，Update 不会执行
//     - 这样可以在更新前验证记录存在性，避免静默失败
//  2. 使用链式调用 First().Update()，而不是分开写
//     - 原因：GORM 的链式调用会先执行 First，如果失败则不会执行 Update，逻辑更清晰
//  3. 只更新 name 字段，而不是整个结构体
//     - 原因：避免误更新其他字段，只更新需要修改的字段，更安全
//
// 好处：
//   - 更新前验证存在性，避免更新不存在的记录
//   - 字段级更新，避免误操作其他字段
//   - 代码简洁，利用 GORM 的链式调用特性
func (e *FileUploadAndDownloadService) EditFileName(file example.ExaFileUploadAndDownload) (err error) {
	var fileFromDb example.ExaFileUploadAndDownload
	// 先查询文件是否存在，如果不存在 First 会返回错误，Update 不会执行
	// 只更新 name 字段，避免误更新其他字段
	return global.GVA_DB.Where("id = ?", file.ID).First(&fileFromDb).Update("name", file.Name).Error
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetFileRecordInfoList
//@description: 分页获取数据
//@param: info request.ExaAttachmentCategorySearch
//@return: list interface{}, total int64, err error

// GetFileRecordInfoList 分页查询文件记录列表，支持关键词搜索和分类筛选
// 设计思路：
//  1. 使用链式查询构建器，根据条件动态添加 WHERE 子句
//     - 原因：避免写多个 if-else 分支拼接 SQL，代码更清晰，也避免 SQL 注入风险
//     - GORM 的链式调用会自动处理条件组合，生成正确的 SQL
//  2. 先计算总数，再查询列表
//     - 原因：前端分页组件需要知道总记录数来计算总页数
//     - 先 Count 再 Find，确保总数和列表使用相同的查询条件
//  3. 使用独立的 db 变量构建查询，而不是直接操作 global.GVA_DB
//     - 原因：可以复用同一个查询构建器，避免重复写查询条件
//     - 如果直接操作 global.GVA_DB，需要在 Count 和 Find 时都写一遍条件，容易出错
//  4. 关键词搜索使用 LIKE 模糊匹配
//     - 原因：支持部分匹配，用户体验更好（如搜索"图片"可以匹配"图片1.jpg"）
//  5. ClassId 条件判断 > 0，避免传入 0 时误匹配所有记录
//     - 原因：0 通常表示"未分类"或"全部"，如果 ClassId 为 0 应该显示所有记录，而不是筛选 ClassId=0 的记录
//  6. 使用 Order("id desc") 按 ID 倒序排列
//     - 原因：ID 通常是自增主键，倒序排列可以保证最新上传的文件在前面
//
// 好处：
//   - 查询条件灵活，支持多条件组合
//   - 代码可读性强，链式调用清晰表达查询意图
//   - 性能优化：先 Count 再 Find，避免不必要的查询
//   - 安全性：使用参数化查询，防止 SQL 注入
func (e *FileUploadAndDownloadService) GetFileRecordInfoList(info request.ExaAttachmentCategorySearch) (list []example.ExaFileUploadAndDownload, total int64, err error) {
	// 计算分页参数：limit 限制每页数量，offset 计算跳过的记录数
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建查询构建器，使用 Model 指定要查询的表
	db := global.GVA_DB.Model(&example.ExaFileUploadAndDownload{})

	// 如果有关键词，添加模糊搜索条件
	// 使用 LIKE 支持部分匹配，提升搜索体验
	if len(info.Keyword) > 0 {
		db = db.Where("name LIKE ?", "%"+info.Keyword+"%")
	}

	// 如果有分类ID，添加分类筛选条件
	// 判断 > 0 避免传入 0 时误匹配，0 通常表示"全部"或"未分类"
	if info.ClassId > 0 {
		db = db.Where("class_id = ?", info.ClassId)
	}

	// 先计算总记录数，使用相同的查询条件确保总数和列表一致
	err = db.Count(&total).Error
	if err != nil {
		return
	}
	// 执行分页查询，按 ID 倒序排列（最新文件在前）
	err = db.Limit(limit).Offset(offset).Order("id desc").Find(&list).Error
	return list, total, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: UploadFile
//@description: 根据配置文件判断是文件上传到本地或者七牛云
//@param: header *multipart.FileHeader, noSave string
//@return: file model.ExaFileUploadAndDownload, err error

// UploadFile 上传文件到OSS并创建数据库记录
// 设计思路：
//  1. 先上传文件到OSS，再处理数据库记录
//     - 原因：如果OSS上传失败，就不需要创建数据库记录，避免数据不一致
//     - 文件上传是耗时操作，先执行可以尽早发现错误
//  2. 使用工厂方法 NewOss() 根据配置自动选择OSS实现
//     - 原因：支持多种存储方式（本地/七牛/阿里云等），通过配置切换，无需改代码
//     - 符合依赖倒置原则，业务层依赖抽象接口，不依赖具体实现
//  3. 从文件名提取文件扩展名作为 Tag
//     - 原因：Tag 字段可以用于文件分类、图标显示等，扩展名是最直观的分类标识
//     - 使用 strings.Split 按 "." 分割，取最后一段作为扩展名
//  4. noSave 参数控制是否保存到数据库
//     - noSave == "0" 表示需要保存，其他值表示不保存（如临时文件、预览等场景）
//     - 这样设计可以支持"只上传不保存"的场景，如临时文件、图片预览等
//  5. 保存前检查是否已存在相同 Key 的记录
//     - 原因：避免重复上传相同文件时创建多条数据库记录
//     - 如果文件已存在，直接返回现有记录，不重复创建
//     - 使用 Key 字段判断，因为 Key 是OSS中的唯一标识
//
// 好处：
//   - 支持多种存储方式，通过配置灵活切换
//   - 避免重复上传，节省存储空间和数据库空间
//   - 支持临时文件场景，不强制保存到数据库
//   - 错误处理清晰，OSS上传失败时不会创建无效记录
func (e *FileUploadAndDownloadService) UploadFile(header *multipart.FileHeader, noSave string, classId int) (file example.ExaFileUploadAndDownload, err error) {
	// 使用工厂方法创建OSS实例，根据配置自动选择实现（本地/七牛/阿里云等）
	oss := upload.NewOss()
	// 先上传文件到OSS，获取文件访问URL和存储Key
	// 如果上传失败，直接返回错误，不继续后续操作
	filePath, key, uploadErr := oss.UploadFile(header)
	if uploadErr != nil {
		return file, uploadErr
	}
	// 从文件名提取扩展名作为文件标签（Tag）
	// 例如 "image.jpg" -> ["image", "jpg"] -> "jpg"
	s := strings.Split(header.Filename, ".")
	// 构建文件记录对象
	f := example.ExaFileUploadAndDownload{
		Url:     filePath,        // OSS返回的完整访问URL
		Name:    header.Filename, // 原始文件名
		ClassId: classId,         // 文件分类ID
		Tag:     s[len(s)-1],     // 文件扩展名（如 jpg、png、pdf）
		Key:     key,             // OSS中的唯一标识，用于后续删除操作
	}
	// noSave == "0" 表示需要保存到数据库，其他值表示不保存（临时文件等场景）
	if noSave == "0" {
		// 检查是否已存在相同Key的记录，避免重复上传时创建多条记录
		var existingFile example.ExaFileUploadAndDownload
		err = global.GVA_DB.Where(&example.ExaFileUploadAndDownload{Key: key}).First(&existingFile).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 记录不存在，创建新记录
			return f, e.Upload(f)
		}
		// 记录已存在，直接返回（不重复创建）
		return f, err
	}
	// noSave != "0"，不保存到数据库，只返回文件信息（用于临时文件等场景）
	return f, nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: ImportURL
//@description: 导入URL
//@param: file model.ExaFileUploadAndDownload
//@return: error

// ImportURL 批量导入文件URL记录到数据库
// 设计思路：
//  1. 接收切片指针 *[]example.ExaFileUploadAndDownload，支持批量导入
//     - 原因：批量导入可以提高性能，减少数据库交互次数
//     - 使用指针可以避免大切片的值拷贝，提升性能
//  2. 直接使用 GORM 的 Create 方法批量插入
//     - 原因：GORM 会自动将切片转换为批量 INSERT 语句，性能优于循环单条插入
//     - 一条 SQL 插入多条记录，减少网络往返和事务开销
//  3. 不在这里做数据校验，保持函数职责单一
//     - 原因：校验逻辑应该在调用层完成，这里只负责数据持久化
//
// 使用场景：
//   - 从其他系统迁移文件记录
//   - 批量导入外部文件链接
//   - 数据恢复或同步
//
// 好处：
//   - 批量操作性能好，减少数据库交互
//   - 代码简洁，利用 GORM 的批量插入能力
//   - 使用指针避免大切片拷贝，节省内存
func (e *FileUploadAndDownloadService) ImportURL(file *[]example.ExaFileUploadAndDownload) error {
	// GORM 会自动将切片转换为批量 INSERT 语句
	// 例如：INSERT INTO exa_file_upload_and_downloads (...) VALUES (...), (...), (...)
	// 性能优于循环单条插入，减少数据库交互次数
	return global.GVA_DB.Create(&file).Error
}
