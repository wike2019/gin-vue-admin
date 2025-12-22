package example

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example/request"
	exampleRes "github.com/flipped-aurora/gin-vue-admin/server/model/example/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"strconv"
)

// FileUploadAndDownloadApi 文件上传下载 API 控制器
// 设计模式：控制器模式（Controller Pattern）
// 职责：
// 1. 接收 HTTP 请求：处理来自客户端的文件上传、下载、删除等请求
// 2. 参数验证：验证请求参数的有效性
// 3. 调用服务层：将业务逻辑委托给 service 层处理
// 4. 响应格式化：统一格式化响应数据返回给客户端
// 设计优势：
// - 单一职责：只负责 HTTP 请求处理，不包含业务逻辑
// - 分层清晰：API 层 -> Service 层 -> Model 层，职责分明
// - 易于测试：可以独立测试 HTTP 处理逻辑
// - 统一错误处理：通过 response 包统一处理成功/失败响应
type FileUploadAndDownloadApi struct{}

// UploadFile 上传文件接口
// 设计要点：
// 1. 使用 multipart/form-data 接收文件：支持大文件上传
// 2. 支持可选参数：noSave（是否保存到数据库）、classId（分类ID）
// 3. 错误处理：每个步骤都有明确的错误处理和日志记录
// 4. 统一响应：使用 response 包统一返回格式，保证 API 响应一致性
//
// 为什么这么写：
// - c.DefaultQuery/DefaultPostForm：提供默认值，避免空值导致的错误
// - 先验证文件接收，再调用 service：提前失败，避免无效的 service 调用
// - 使用 zap 记录错误：结构化日志，便于问题排查和监控
// - 返回详细文件信息：前端可以直接使用返回的文件信息，无需再次查询
//
// @Tags      ExaFileUploadAndDownload
// @Summary   上传文件示例
// @Security  ApiKeyAuth
// @accept    multipart/form-data
// @Produce   application/json
// @Param     file  formData  file                                                           true  "上传文件示例"
// @Success   200   {object}  response.Response{data=exampleRes.ExaFileResponse,msg=string}  "上传文件示例,返回包括文件详情"
// @Router    /fileUploadAndDownload/upload [post]
func (b *FileUploadAndDownloadApi) UploadFile(c *gin.Context) {
	var file example.ExaFileUploadAndDownload
	// 使用 DefaultQuery 提供默认值，避免空字符串导致的逻辑错误
	// noSave="0" 表示默认保存文件，noSave="1" 表示只上传不保存到数据库
	noSave := c.DefaultQuery("noSave", "0")
	// FormFile 用于接收 multipart/form-data 格式的文件
	// 返回文件头信息，包含文件名、大小、MIME 类型等元数据
	_, header, err := c.Request.FormFile("file")
	// 获取文件分类 ID，用于文件分类管理
	classId, _ := strconv.Atoi(c.DefaultPostForm("classId", "0"))
	if err != nil {
		// 记录详细错误日志，包含错误堆栈信息，便于排查问题
		global.GVA_LOG.Error("接收文件失败!", zap.Error(err))
		// 返回用户友好的错误信息，不暴露内部实现细节
		response.FailWithMessage("接收文件失败", c)
		return
	}
	// 调用 service 层处理文件上传逻辑
	// 好处：业务逻辑与 HTTP 处理分离，service 可以被其他层复用
	file, err = fileUploadAndDownloadService.UploadFile(header, noSave, classId)
	if err != nil {
		global.GVA_LOG.Error("上传文件失败!", zap.Error(err))
		response.FailWithMessage("上传文件失败", c)
		return
	}
	// 返回详细的文件信息，包括文件路径、大小、MD5 等
	// 前端可以直接使用这些信息，无需再次调用查询接口
	response.OkWithDetailed(exampleRes.ExaFileResponse{File: file}, "上传成功", c)
}

// EditFileName 编辑文件名或备注
// 设计要点：
// 1. 使用 ShouldBindJSON：自动将 JSON 请求体绑定到结构体
// 2. 直接返回绑定错误：Gin 的 ShouldBindJSON 会返回详细的验证错误信息
// 3. 简洁的响应：编辑操作通常只需要返回成功/失败状态
//
// 为什么这么写：
// - ShouldBindJSON：自动处理 JSON 解析和类型转换，减少样板代码
// - 直接返回 err.Error()：Gin 的验证错误信息对用户友好，可以直接返回
// - 不返回数据：编辑操作成功后通常不需要返回完整对象，节省带宽
func (b *FileUploadAndDownloadApi) EditFileName(c *gin.Context) {
	var file example.ExaFileUploadAndDownload
	// ShouldBindJSON 会自动验证 JSON 格式和字段类型
	// 如果验证失败，返回详细的错误信息（如字段缺失、类型不匹配等）
	err := c.ShouldBindJSON(&file)
	if err != nil {
		// 直接返回 Gin 的验证错误，这些错误信息对前端开发友好
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 调用 service 层执行更新操作
	err = fileUploadAndDownloadService.EditFileName(file)
	if err != nil {
		global.GVA_LOG.Error("编辑失败!", zap.Error(err))
		response.FailWithMessage("编辑失败", c)
		return
	}
	// 编辑操作成功，只返回成功消息，不返回数据（节省带宽）
	response.OkWithMessage("编辑成功", c)
}

// DeleteFile 删除文件接口
// 设计要点：
// 1. 使用 POST 而非 DELETE：虽然语义上 DELETE 更合适，但 POST 更灵活，可以传递复杂参数
// 2. 只需要文件 ID：删除操作通常只需要标识符，不需要完整对象
// 3. 错误处理：删除失败时记录详细日志，便于排查问题
//
// 为什么这么写：
// - 使用结构体接收：虽然只需要 ID，但使用结构体更规范，便于未来扩展字段
// - 统一错误处理：所有错误都通过 response 包统一返回，保证 API 一致性
// - 记录错误日志：删除操作失败可能是重要问题，需要详细记录
//
// @Tags      ExaFileUploadAndDownload
// @Summary   删除文件
// @Security  ApiKeyAuth
// @Produce   application/json
// @Param     data  body      example.ExaFileUploadAndDownload  true  "传入文件里面id即可"
// @Success   200   {object}  response.Response{msg=string}     "删除文件"
// @Router    /fileUploadAndDownload/deleteFile [post]
func (b *FileUploadAndDownloadApi) DeleteFile(c *gin.Context) {
	var file example.ExaFileUploadAndDownload
	err := c.ShouldBindJSON(&file)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 调用 service 层删除文件（包括物理文件和数据库记录）
	// service 层会处理文件系统删除和数据库删除的原子性
	if err := fileUploadAndDownloadService.DeleteFile(file); err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// GetFileList 分页获取文件列表
// 设计要点：
// 1. 分页查询：使用 PageResult 统一分页响应格式
// 2. 返回完整分页信息：包括列表、总数、当前页、每页大小，前端可以直接使用
// 3. 支持分类筛选：通过 ExaAttachmentCategorySearch 支持按分类查询
//
// 为什么这么写：
// - 使用 POST 而非 GET：分页查询参数可能较复杂，POST 更灵活
// - 返回 PageResult：统一的分页响应格式，前端可以复用分页组件
// - 返回完整分页信息：前端无需额外计算，直接使用返回的数据
// - 多返回值：Go 语言习惯，list、total、error 分离，代码清晰
//
// @Tags      ExaFileUploadAndDownload
// @Summary   分页文件列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.ExaAttachmentCategorySearch                                        true  "页码, 每页大小, 分类id"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页文件列表,返回包括列表,总数,页码,每页数量"
// @Router    /fileUploadAndDownload/getFileList [post]
func (b *FileUploadAndDownloadApi) GetFileList(c *gin.Context) {
	var pageInfo request.ExaAttachmentCategorySearch
	err := c.ShouldBindJSON(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// service 层返回列表、总数和错误
	// 这种设计的好处：
	// 1. 列表和总数在一次查询中返回，保证数据一致性
	// 2. 错误单独返回，便于区分业务错误和系统错误
	list, total, err := fileUploadAndDownloadService.GetFileRecordInfoList(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	// 返回完整的分页信息，包括：
	// - List: 当前页的数据列表
	// - Total: 总记录数（用于计算总页数）
	// - Page: 当前页码（用于前端高亮显示）
	// - PageSize: 每页大小（用于前端显示）
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// ImportURL 批量导入 URL 作为文件记录
// 设计要点：
// 1. 支持批量导入：接收文件数组，可以一次导入多个 URL
// 2. 使用指针传递：避免大数组的拷贝，提高性能
// 3. 简洁的响应：批量操作通常只需要返回成功/失败状态
//
// 为什么这么写：
// - 数组类型：支持一次导入多个 URL，提高操作效率
// - 指针传递：Go 中数组是值类型，传递指针避免拷贝，特别是数组较大时
// - 统一错误处理：所有错误都记录日志并返回友好提示
//
// 应用场景：从其他系统迁移文件时，可以批量导入文件 URL，无需重新上传
//
// @Tags      ExaFileUploadAndDownload
// @Summary   导入URL
// @Security  ApiKeyAuth
// @Produce   application/json
// @Param     data  body      example.ExaFileUploadAndDownload  true  "对象"
// @Success   200   {object}  response.Response{msg=string}     "导入URL"
// @Router    /fileUploadAndDownload/importURL [post]
func (b *FileUploadAndDownloadApi) ImportURL(c *gin.Context) {
	// 接收文件数组，支持批量导入
	var file []example.ExaFileUploadAndDownload
	err := c.ShouldBindJSON(&file)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 使用指针传递，避免数组拷贝，提高性能
	// service 层会处理 URL 验证、文件信息提取等逻辑
	if err := fileUploadAndDownloadService.ImportURL(&file); err != nil {
		global.GVA_LOG.Error("导入URL失败!", zap.Error(err))
		response.FailWithMessage("导入URL失败", c)
		return
	}
	response.OkWithMessage("导入URL成功", c)
}
