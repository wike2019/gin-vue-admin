package example

import (
	"github.com/gin-gonic/gin"
)

// FileUploadAndDownloadRouter 文件上传下载路由结构体
// 负责处理文件相关的所有操作，包括普通上传、断点续传、文件管理等
//
// 设计意义：
// 1. 功能聚合：将文件相关的所有路由操作集中在一个结构体中，便于统一管理
// 2. 业务隔离：文件操作与其他业务模块（如客户管理）分离，符合单一职责原则
// 3. 易于维护：文件上传下载功能通常涉及复杂的业务逻辑，独立管理便于后续优化和扩展
type FileUploadAndDownloadRouter struct{}

// InitFileUploadAndDownloadRouter 初始化文件上传下载相关的路由
// 该方法定义了文件管理模块的完整功能路由，包括普通上传、断点续传、文件列表管理等
//
// 参数说明：
//   - Router: 父级路由组，通常是从主路由中传入的示例模块路由组
//
// 设计意义：
// 1. 路由分组：使用 Router.Group("fileUploadAndDownload") 创建统一的路由前缀
//    所有文件相关路由的前缀为 /fileUploadAndDownload，保持 URL 结构清晰
//
// 2. HTTP 方法选择：
//    - POST: 用于需要传递复杂数据（如文件、表单数据）的操作
//      大部分操作使用 POST 是因为需要传递文件数据、查询参数、分页信息等
//    - GET: 仅用于简单的查询操作（findFile），符合 RESTful 规范
//
// 3. 功能完整性：
//    - 基础功能：上传、删除、编辑、列表查询
//    - 高级功能：断点续传（支持大文件分片上传）、URL 导入
//    体现了对文件上传场景的全面支持
//
// 4. 断点续传设计：
//    - breakpointContinue: 上传文件切片，支持大文件分片上传
//    - findFile: 查询已上传的切片，用于断点续传时判断哪些切片已存在
//    - breakpointContinueFinish: 所有切片上传完成后，通知服务器合并文件
//    - removeChunk: 删除失败的切片，支持重试机制
//    这种设计可以提升大文件上传的可靠性和用户体验
//
// 5. 代码组织：使用代码块 {} 将路由定义分组，虽然当前所有路由都在一个组中，
//    但保持了代码结构的清晰，便于后续按功能进一步分组
func (e *FileUploadAndDownloadRouter) InitFileUploadAndDownloadRouter(Router *gin.RouterGroup) {
	// 创建文件上传下载路由组，所有路由的前缀为 /fileUploadAndDownload
	fileUploadAndDownloadRouter := Router.Group("fileUploadAndDownload")
	{
		// 基础文件操作
		fileUploadAndDownloadRouter.POST("upload", exaFileUploadAndDownloadApi.UploadFile)       // POST /fileUploadAndDownload/upload - 普通文件上传
		fileUploadAndDownloadRouter.POST("getFileList", exaFileUploadAndDownloadApi.GetFileList) // POST /fileUploadAndDownload/getFileList - 获取文件列表（支持分页、筛选，使用 POST 因为需要传递查询参数）
		fileUploadAndDownloadRouter.POST("deleteFile", exaFileUploadAndDownloadApi.DeleteFile)   // POST /fileUploadAndDownload/deleteFile - 删除指定文件
		fileUploadAndDownloadRouter.POST("editFileName", exaFileUploadAndDownloadApi.EditFileName) // POST /fileUploadAndDownload/editFileName - 编辑文件名或备注信息

		// 断点续传相关操作（支持大文件分片上传）
		fileUploadAndDownloadRouter.POST("breakpointContinue", exaFileUploadAndDownloadApi.BreakpointContinue)             // POST /fileUploadAndDownload/breakpointContinue - 上传文件切片（支持断点续传）
		fileUploadAndDownloadRouter.GET("findFile", exaFileUploadAndDownloadApi.FindFile)                                  // GET /fileUploadAndDownload/findFile - 查询已上传的切片列表（用于断点续传时判断上传进度）
		fileUploadAndDownloadRouter.POST("breakpointContinueFinish", exaFileUploadAndDownloadApi.BreakpointContinueFinish) // POST /fileUploadAndDownload/breakpointContinueFinish - 所有切片上传完成，通知服务器合并文件
		fileUploadAndDownloadRouter.POST("removeChunk", exaFileUploadAndDownloadApi.RemoveChunk)                           // POST /fileUploadAndDownload/removeChunk - 删除失败的切片（支持重试机制）

		// 其他功能
		fileUploadAndDownloadRouter.POST("importURL", exaFileUploadAndDownloadApi.ImportURL) // POST /fileUploadAndDownload/importURL - 通过 URL 导入文件（支持从网络地址下载文件）
	}
}
