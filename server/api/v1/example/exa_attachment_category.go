package example

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	common "github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AttachmentCategoryApi 附件分类管理 API 控制器
// 用于管理文件上传时的分类功能
// 设计要点：
// 1. 分类管理：支持分类的增删改查，便于文件组织
// 2. 树形结构：分类通常支持层级结构，便于文件分类管理
// 3. 简洁的 API：提供基本的 CRUD 操作，满足分类管理需求
type AttachmentCategoryApi struct{}

// GetCategoryList 获取所有分类列表
// 设计要点：
// 1. 无参数查询：获取所有分类，通常数据量不大，无需分页
// 2. 树形结构：返回的列表通常是树形结构，便于前端展示
// 3. 简洁响应：使用 OkWithData 直接返回数据，无需额外包装
//
// 为什么这么写：
// - 无分页：分类数据通常较少，一次性返回更简单
// - 树形结构：service 层会构建树形结构，前端可以直接使用
// - 缓存友好：分类数据变化不频繁，适合缓存
//
// @Tags      GetCategoryList
// @Summary   媒体库分类列表
// @Security  AttachmentCategory
// @Produce   application/json
// @Success   200   {object}  response.Response{data=example.ExaAttachmentCategory,msg=string}  "媒体库分类列表"
// @Router    /attachmentCategory/getCategoryList [get]
func (a *AttachmentCategoryApi) GetCategoryList(c *gin.Context) {
	// 获取所有分类，service 层会构建树形结构
	res, err := attachmentCategoryService.GetCategoryList()
	if err != nil {
		global.GVA_LOG.Error("获取分类列表失败!", zap.Error(err))
		response.FailWithMessage("获取分类列表失败", c)
		return
	}
	// 直接返回数据，无需额外包装
	response.OkWithData(res, c)
}

// AddCategory 添加或更新分类
// 设计要点：
// 1. 统一接口：创建和更新使用同一个接口，根据 ID 是否存在判断
// 2. 指针传递：使用 &req 避免结构体拷贝
// 3. 详细错误：返回具体的错误信息，便于前端提示
//
// 为什么这么写：
// - 统一接口：减少 API 数量，简化前端调用
// - ID 判断：如果 ID 存在则更新，不存在则创建（upsert 模式）
// - 错误信息：返回详细错误，帮助用户理解失败原因
//
// @Tags      AddCategory
// @Summary   添加媒体库分类
// @Security  AttachmentCategory
// @accept    application/json
// @Produce   application/json
// @Param     data  body      example.ExaAttachmentCategory  true  "媒体库分类数据"
// @Success   200   {object}  response.Response{msg=string}   "添加媒体库分类"
// @Router    /attachmentCategory/addCategory [post]
func (a *AttachmentCategoryApi) AddCategory(c *gin.Context) {
	var req example.ExaAttachmentCategory
	if err := c.ShouldBindJSON(&req); err != nil {
		global.GVA_LOG.Error("参数错误!", zap.Error(err))
		response.FailWithMessage("参数错误", c)
		return
	}

	// service 层会根据 ID 是否存在判断是创建还是更新
	// 这种设计的好处：
	// 1. 前端无需判断是创建还是更新
	// 2. 减少 API 数量，简化调用
	// 3. 保证数据一致性
	if err := attachmentCategoryService.AddCategory(&req); err != nil {
		global.GVA_LOG.Error("创建/更新失败!", zap.Error(err))
		// 返回详细错误信息，包含 service 层的错误描述
		response.FailWithMessage("创建/更新失败："+err.Error(), c)
		return
	}
	response.OkWithMessage("创建/更新成功", c)
}

// DeleteCategory 删除分类
// 设计要点：
// 1. 双重验证：先验证 JSON 格式，再验证 ID 是否为零值
// 2. 指针传递：传递 ID 的指针，避免值拷贝
// 3. 关联检查：service 层会检查分类下是否有文件，有文件则不允许删除
//
// 为什么这么写：
// - 显式检查 ID：虽然 ShouldBindJSON 会验证，但显式检查更清晰
// - 指针传递：虽然 ID 是整数，但传递指针是 Go 的常见做法
// - 关联检查：防止删除有文件的分类，保证数据完整性
//
// @Tags      DeleteCategory
// @Summary   删除分类
// @Security  AttachmentCategory
// @accept    application/json
// @Produce   application/json
// @Param     data  body      common.GetById                true  "分类id"
// @Success   200   {object}  response.Response{msg=string}  "删除分类"
// @Router    /attachmentCategory/deleteCategory [post]
func (a *AttachmentCategoryApi) DeleteCategory(c *gin.Context) {
	var req common.GetById
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage("参数错误", c)
		return
	}

	// 显式检查 ID 是否为零值
	// 这种检查的好处：
	// 1. 提前发现错误，避免无效的数据库查询
	// 2. 代码更清晰，意图更明确
	if req.ID == 0 {
		response.FailWithMessage("参数错误", c)
		return
	}

	// service 层会处理：
	// 1. 检查分类是否存在
	// 2. 检查分类下是否有文件（如果有文件则不允许删除）
	// 3. 删除分类及其子分类（如果是树形结构）
	if err := attachmentCategoryService.DeleteCategory(&req.ID); err != nil {
		response.FailWithMessage("删除失败", c)
		return
	}

	response.OkWithMessage("删除成功", c)
}
