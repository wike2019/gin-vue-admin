package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DictionaryApi struct{}

// CreateSysDictionary
// @Tags      SysDictionary
// @Summary   创建SysDictionary
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysDictionary           true  "SysDictionary模型"
// @Success   200   {object}  response.Response{msg=string}  "创建SysDictionary"
// @Router    /sysDictionary/createSysDictionary [post]
func (s *DictionaryApi) CreateSysDictionary(c *gin.Context) {
	var dictionary system.SysDictionary
	err := c.ShouldBindJSON(&dictionary)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = dictionaryService.CreateSysDictionary(dictionary)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败", c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteSysDictionary
// @Tags      SysDictionary
// @Summary   删除SysDictionary
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysDictionary           true  "SysDictionary模型"
// @Success   200   {object}  response.Response{msg=string}  "删除SysDictionary"
// @Router    /sysDictionary/deleteSysDictionary [delete]
func (s *DictionaryApi) DeleteSysDictionary(c *gin.Context) {
	var dictionary system.SysDictionary
	err := c.ShouldBindJSON(&dictionary)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = dictionaryService.DeleteSysDictionary(dictionary)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// UpdateSysDictionary
// @Tags      SysDictionary
// @Summary   更新SysDictionary
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysDictionary           true  "SysDictionary模型"
// @Success   200   {object}  response.Response{msg=string}  "更新SysDictionary"
// @Router    /sysDictionary/updateSysDictionary [put]
func (s *DictionaryApi) UpdateSysDictionary(c *gin.Context) {
	var dictionary system.SysDictionary
	err := c.ShouldBindJSON(&dictionary)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = dictionaryService.UpdateSysDictionary(&dictionary)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败", c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindSysDictionary 查询字典接口
// 设计要点：
// 1. 多条件查询：支持通过 ID 或字典类型（Type）查询
// 2. 状态过滤：支持根据状态过滤字典，只返回启用的字典
// 3. 灵活查询：支持 ID 和 Type 两种查询方式，满足不同场景
// 4. 错误处理：字典不存在或未启用时返回明确的错误信息
//
// 为什么这么写：
// - 多条件查询：支持 ID 和 Type 两种查询方式，提升灵活性
// - 状态过滤：只返回启用的字典，避免返回无效数据
// - 错误处理：明确的错误信息帮助用户理解问题
// - 统一响应：使用统一的响应格式，便于前端处理
//
// 查询方式：
// - 通过 ID 查询：dictionary.ID != 0
// - 通过 Type 查询：dictionary.Type != ""
// - 状态过滤：dictionary.Status 控制是否只返回启用的字典
//
// 好处：
// - 灵活性：支持多种查询方式，满足不同场景
// - 安全性：状态过滤确保只返回有效数据
// - 可维护性：清晰的错误处理便于问题定位
//
// @Tags      SysDictionary
// @Summary   用id查询SysDictionary
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     system.SysDictionary                                       true  "ID或字典英名"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "用id查询SysDictionary"
// @Router    /sysDictionary/findSysDictionary [get]
func (s *DictionaryApi) FindSysDictionary(c *gin.Context) {
	// 步骤1：绑定查询参数（使用 ShouldBindQuery 绑定 URL 参数）
	var dictionary system.SysDictionary
	err := c.ShouldBindQuery(&dictionary)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：调用服务层查询字典
	// 支持通过 ID 或 Type 查询，支持状态过滤
	// service 层会：
	// 1. 根据 ID 或 Type 查询字典
	// 2. 根据 Status 过滤（如果指定）
	// 3. 返回字典及其详情
	sysDictionary, err := dictionaryService.GetSysDictionary(dictionary.Type, dictionary.ID, dictionary.Status)
	if err != nil {
		global.GVA_LOG.Error("字典未创建或未开启!", zap.Error(err))
		// 明确的错误信息，帮助用户理解问题
		response.FailWithMessage("字典未创建或未开启", c)
		return
	}
	response.OkWithDetailed(gin.H{"resysDictionary": sysDictionary}, "查询成功", c)
}

// GetSysDictionaryList
// @Tags      SysDictionary
// @Summary   分页获取SysDictionary列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     request.SysDictionarySearch                                    true  "字典 name 或者 type"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取SysDictionary列表,返回包括列表,总数,页码,每页数量"
// @Router    /sysDictionary/getSysDictionaryList [get]
func (s *DictionaryApi) GetSysDictionaryList(c *gin.Context) {
	var dictionary request.SysDictionarySearch
	err := c.ShouldBindQuery(&dictionary)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, err := dictionaryService.GetSysDictionaryInfoList(c, dictionary)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(list, "获取成功", c)
}

// ExportSysDictionary 导出字典 JSON 接口（包含字典详情）
// 设计要点：
// 1. 完整导出：导出字典及其所有详情，便于跨系统使用
// 2. 数据清理：导出的数据只包含业务字段，不包含数据库字段
// 3. 参数验证：验证字典ID，确保导出有效字典
// 4. 格式统一：导出格式统一，便于导入和跨系统使用
//
// 为什么这么写：
// - 完整导出：包含字典详情，确保导出数据完整可用
// - 数据清理：只保留业务字段，便于跨系统使用
// - 参数验证：验证字典ID，防止无效导出
// - 格式统一：统一的导出格式便于导入和跨系统使用
//
// 使用场景：
// - 字典迁移：将字典从一个系统迁移到另一个系统
// - 备份恢复：备份字典数据，支持恢复
// - 版本管理：将字典纳入版本管理
//
// 好处：
// - 完整性：包含字典详情，确保数据完整
// - 可移植性：清理后的数据便于跨系统使用
// - 可维护性：统一的格式便于维护
//
// @Tags      SysDictionary
// @Summary   导出字典JSON（包含字典详情）
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     system.SysDictionary                                       true  "字典ID"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "导出字典JSON"
// @Router    /sysDictionary/exportSysDictionary [get]
func (s *DictionaryApi) ExportSysDictionary(c *gin.Context) {
	// 步骤1：绑定查询参数
	var dictionary system.SysDictionary
	err := c.ShouldBindQuery(&dictionary)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：验证字典ID
	// 设计原因：确保导出有效字典，防止无效导出
	if dictionary.ID == 0 {
		response.FailWithMessage("字典ID不能为空", c)
		return
	}
	
	// 步骤3：调用服务层导出字典
	// service 层会：
	// 1. 查询字典及其所有详情
	// 2. 清理数据库字段（ID、时间戳等）
	// 3. 构建导出数据格式
	// 4. 返回清理后的字典数据
	exportData, err := dictionaryService.ExportSysDictionary(dictionary.ID)
	if err != nil {
		global.GVA_LOG.Error("导出失败!", zap.Error(err))
		response.FailWithMessage("导出失败", c)
		return
	}
	response.OkWithDetailed(exportData, "导出成功", c)
}

// ImportSysDictionary
// @Tags      SysDictionary
// @Summary   导入字典JSON（包含字典详情）
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.ImportSysDictionaryRequest     true  "字典JSON数据"
// @Success   200   {object}  response.Response{msg=string}          "导入字典"
// @Router    /sysDictionary/importSysDictionary [post]
func (s *DictionaryApi) ImportSysDictionary(c *gin.Context) {
	var req request.ImportSysDictionaryRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = dictionaryService.ImportSysDictionary(req.Json)
	if err != nil {
		global.GVA_LOG.Error("导入失败!", zap.Error(err))
		response.FailWithMessage("导入失败: "+err.Error(), c)
		return
	}
	response.OkWithMessage("导入成功", c)
}
