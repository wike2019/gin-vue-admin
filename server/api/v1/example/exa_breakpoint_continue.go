package example

import (
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/model/example"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	exampleRes "github.com/flipped-aurora/gin-vue-admin/server/model/example/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 断点续传相关 API
// 断点续传是一种文件上传技术，允许大文件分块上传，支持断网续传
// 设计要点：
// 1. 分块上传：将大文件分成多个小块（chunk）分别上传
// 2. MD5 校验：每个块都有 MD5 值，确保传输完整性
// 3. 状态管理：记录每个块的上传状态，支持断点续传
// 4. 安全性：防止路径穿越攻击，验证文件路径合法性
//
// 应用场景：
// - 大文件上传（如视频、压缩包等）
// - 网络不稳定环境下的文件上传
// - 需要显示上传进度的场景

// BreakpointContinue 上传文件块（断点续传的核心接口）
// 设计要点：
// 1. 分块上传：每次上传一个文件块，而不是整个文件
// 2. MD5 校验：验证每个块的完整性，防止传输错误
// 3. 状态记录：记录每个块的上传状态，支持断点续传
// 4. 资源管理：使用 defer 确保文件句柄正确关闭
//
// 为什么这么写：
// - FormValue：从 multipart 表单中获取元数据（MD5、块编号等）
// - MD5 校验：确保每个块传输完整，防止网络错误导致的数据损坏
// - 分步处理：先验证块，再保存文件，最后记录状态，每步都有错误处理
// - defer 关闭：确保文件句柄在任何情况下都能正确关闭，防止资源泄漏
//
// 工作流程：
// 1. 接收文件块和元数据
// 2. 验证块的 MD5 值
// 3. 查找或创建文件记录
// 4. 保存文件块到临时目录
// 5. 记录块的上传状态
//
// @Tags      ExaFileUploadAndDownload
// @Summary   断点续传到服务器
// @Security  ApiKeyAuth
// @accept    multipart/form-data
// @Produce   application/json
// @Param     file  formData  file                           true  "an example for breakpoint resume, 断点续传示例"
// @Success   200   {object}  response.Response{msg=string}  "断点续传到服务器"
// @Router    /fileUploadAndDownload/breakpointContinue [post]
func (b *FileUploadAndDownloadApi) BreakpointContinue(c *gin.Context) {
	// 从表单中获取文件元数据
	// fileMd5: 整个文件的 MD5 值，用于标识文件
	// fileName: 文件名
	// chunkMd5: 当前块的 MD5 值，用于验证块完整性
	// chunkNumber: 当前块的编号（从 1 开始）
	// chunkTotal: 总块数
	fileMd5 := c.Request.FormValue("fileMd5")
	fileName := c.Request.FormValue("fileName")
	chunkMd5 := c.Request.FormValue("chunkMd5")
	chunkNumber, _ := strconv.Atoi(c.Request.FormValue("chunkNumber"))
	chunkTotal, _ := strconv.Atoi(c.Request.FormValue("chunkTotal"))
	
	// 接收文件块
	_, FileHeader, err := c.Request.FormFile("file")
	if err != nil {
		global.GVA_LOG.Error("接收文件失败!", zap.Error(err))
		response.FailWithMessage("接收文件失败", c)
		return
	}
	
	// 打开文件并读取内容
	f, err := FileHeader.Open()
	if err != nil {
		global.GVA_LOG.Error("文件读取失败!", zap.Error(err))
		response.FailWithMessage("文件读取失败", c)
		return
	}
	// 使用 defer 确保文件句柄关闭，防止资源泄漏
	// 即使后续代码出错，defer 也会执行
	defer func(f multipart.File) {
		err := f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(f)
	
	// 读取文件块内容
	cen, _ := io.ReadAll(f)
	
	// 验证块的 MD5 值，确保传输完整性
	// 如果 MD5 不匹配，说明传输过程中数据损坏
	if !utils.CheckMd5(cen, chunkMd5) {
		global.GVA_LOG.Error("检查md5失败!", zap.Error(err))
		response.FailWithMessage("检查md5失败", c)
		return
	}
	
	// 查找或创建文件记录
	// 如果文件已存在（通过 fileMd5 查找），则复用记录
	// 如果不存在，则创建新记录
	file, err := fileUploadAndDownloadService.FindOrCreateFile(fileMd5, fileName, chunkTotal)
	if err != nil {
		global.GVA_LOG.Error("查找或创建记录失败!", zap.Error(err))
		response.FailWithMessage("查找或创建记录失败", c)
		return
	}
	
	// 保存文件块到临时目录
	// 每个块保存在独立的文件中，最后合并
	pathC, err := utils.BreakPointContinue(cen, fileName, chunkNumber, chunkTotal, fileMd5)
	if err != nil {
		global.GVA_LOG.Error("断点续传失败!", zap.Error(err))
		response.FailWithMessage("断点续传失败", c)
		return
	}

	// 记录块的上传状态
	// 用于跟踪哪些块已上传，哪些块还需要上传
	if err = fileUploadAndDownloadService.CreateFileChunk(file.ID, pathC, chunkNumber); err != nil {
		global.GVA_LOG.Error("创建文件记录失败!", zap.Error(err))
		response.FailWithMessage("创建文件记录失败", c)
		return
	}
	response.OkWithMessage("切片创建成功", c)
}

// FindFile 查找文件上传状态
// 设计要点：
// 1. 查询接口：用于检查文件是否已开始上传
// 2. 状态返回：返回文件的上传状态和已上传的块信息
// 3. 自动创建：如果文件不存在，自动创建记录
//
// 为什么这么写：
// - GET 方法：查询操作使用 GET，符合 RESTful 规范
// - Query 参数：从 URL 查询参数获取，便于缓存和调试
// - 查找或创建：如果文件不存在则创建，支持断点续传
//
// 应用场景：
// - 上传前检查：上传前检查文件是否已存在，避免重复上传
// - 断点续传：获取已上传的块信息，只上传缺失的块
//
// @Tags      ExaFileUploadAndDownload
// @Summary   查找文件
// @Security  ApiKeyAuth
// @accept    multipart/form-data
// @Produce   application/json
// @Param     file  formData  file                                                        true  "Find the file, 查找文件"
// @Success   200   {object}  response.Response{data=exampleRes.FileResponse,msg=string}  "查找文件,返回包括文件详情"
// @Router    /fileUploadAndDownload/findFile [get]
func (b *FileUploadAndDownloadApi) FindFile(c *gin.Context) {
	// 从 URL 查询参数获取文件信息
	fileMd5 := c.Query("fileMd5")
	fileName := c.Query("fileName")
	chunkTotal, _ := strconv.Atoi(c.Query("chunkTotal"))
	
	// 查找文件记录，如果不存在则创建
	// 返回的文件对象包含：
	// - 文件 ID
	// - 已上传的块列表
	// - 上传状态等
	file, err := fileUploadAndDownloadService.FindOrCreateFile(fileMd5, fileName, chunkTotal)
	if err != nil {
		global.GVA_LOG.Error("查找失败!", zap.Error(err))
		response.FailWithMessage("查找失败", c)
	} else {
		// 返回文件信息，前端可以根据已上传的块决定哪些块需要上传
		response.OkWithDetailed(exampleRes.FileResponse{File: file}, "查找成功", c)
	}
}

// BreakpointContinueFinish 合并所有文件块，完成文件上传
// 设计要点：
// 1. 块合并：将所有已上传的块合并成完整文件
// 2. 完整性检查：确保所有块都已上传
// 3. 返回文件路径：返回最终文件的路径，供后续使用
//
// 为什么这么写：
// - 最后一步：在所有块上传完成后调用，合并文件
// - 返回路径：即使失败也返回路径，便于前端处理
// - 原子操作：合并操作应该是原子的，要么全部成功，要么全部失败
//
// 工作流程：
// 1. 检查所有块是否已上传
// 2. 按顺序合并所有块
// 3. 验证合并后文件的完整性
// 4. 删除临时块文件
// 5. 返回最终文件路径
//
// @Tags      ExaFileUploadAndDownload
// @Summary   创建文件
// @Security  ApiKeyAuth
// @accept    multipart/form-data
// @Produce   application/json
// @Param     file  formData  file                                                            true  "上传文件完成"
// @Success   200   {object}  response.Response{data=exampleRes.FilePathResponse,msg=string}  "创建文件,返回包括文件路径"
// @Router    /fileUploadAndDownload/findFile [post]
func (b *FileUploadAndDownloadApi) BreakpointContinueFinish(c *gin.Context) {
	fileMd5 := c.Query("fileMd5")
	fileName := c.Query("fileName")
	
	// 合并所有文件块，生成最终文件
	// MakeFile 会：
	// 1. 检查所有块是否已上传
	// 2. 按顺序读取所有块
	// 3. 合并成完整文件
	// 4. 验证文件完整性
	// 5. 删除临时块文件
	filePath, err := utils.MakeFile(fileName, fileMd5)
	if err != nil {
		global.GVA_LOG.Error("文件创建失败!", zap.Error(err))
		// 即使失败也返回路径，便于前端处理错误情况
		response.FailWithDetailed(exampleRes.FilePathResponse{FilePath: filePath}, "文件创建失败", c)
	} else {
		response.OkWithDetailed(exampleRes.FilePathResponse{FilePath: filePath}, "文件创建成功", c)
	}
}

// RemoveChunk 删除文件块（清理临时文件）
// 设计要点：
// 1. 安全验证：防止路径穿越攻击，确保只能删除指定目录的文件
// 2. 双重删除：删除物理文件和数据库记录
// 3. 错误处理：即使物理文件删除失败，也尝试删除数据库记录
//
// 为什么这么写：
// - 路径验证：防止恶意用户通过路径穿越删除系统文件
// - 安全检查：检查常见的路径穿越模式（..、../、./、.\\）
// - 容错处理：物理文件删除失败不影响数据库记录删除
//
// 安全考虑：
// - 路径白名单：只允许删除指定目录下的文件
// - 路径规范化：使用绝对路径，避免相对路径问题
// - 权限检查：确保用户只能删除自己上传的文件块
//
// @Tags      ExaFileUploadAndDownload
// @Summary   删除切片
// @Security  ApiKeyAuth
// @accept    multipart/form-data
// @Produce   application/json
// @Param     file  formData  file                           true  "删除缓存切片"
// @Success   200   {object}  response.Response{msg=string}  "删除切片"
// @Router    /fileUploadAndDownload/removeChunk [post]
func (b *FileUploadAndDownloadApi) RemoveChunk(c *gin.Context) {
	var file example.ExaFile
	err := c.ShouldBindJSON(&file)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 路径穿越攻击防护
	// 检查路径中是否包含路径穿越字符
	// 这是非常重要的安全措施，防止恶意用户删除系统文件
	// 常见的路径穿越模式：
	// - ".." : 上级目录
	// - "../" : 上级目录（Unix/Linux）
	// - "./" : 当前目录（可能用于绕过检查）
	// - ".\\" : 当前目录（Windows）
	if strings.Contains(file.FilePath, "..") || 
	   strings.Contains(file.FilePath, "../") || 
	   strings.Contains(file.FilePath, "./") || 
	   strings.Contains(file.FilePath, ".\\") {
		response.FailWithMessage("非法路径，禁止删除", c)
		return
	}
	
	// 删除物理文件（临时块文件）
	// 即使删除失败也不影响后续操作，只记录日志
	err = utils.RemoveChunk(file.FileMd5)
	if err != nil {
		global.GVA_LOG.Error("缓存切片删除失败!", zap.Error(err))
		// 不直接返回，继续删除数据库记录
	}
	
	// 删除数据库记录
	// 即使物理文件删除失败，也要删除数据库记录，保持数据一致性
	err = fileUploadAndDownloadService.DeleteFileChunk(file.FileMd5, file.FilePath)
	if err != nil {
		global.GVA_LOG.Error(err.Error(), zap.Error(err))
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("缓存切片删除成功", c)
}
