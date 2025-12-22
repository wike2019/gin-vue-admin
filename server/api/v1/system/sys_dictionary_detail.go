package system

// 字典详情管理API层
// 设计说明：
// 1. 树形结构支持：提供树形结构查询，支持层级字典数据
// 2. 多种查询方式：支持按ID、类型、父级ID等多种查询方式
// 3. 路径查询：提供路径查询功能，便于展示完整层级
// 4. 类型转换：手动进行字符串到uint的转换，保证类型安全
// 5. 好处：功能完整、查询灵活、类型安全

import (
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DictionaryDetailApi 字典详情API结构体
type DictionaryDetailApi struct{}

// CreateSysDictionaryDetail
// @Tags      SysDictionaryDetail
// @Summary   创建SysDictionaryDetail
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysDictionaryDetail     true  "SysDictionaryDetail模型"
// @Success   200   {object}  response.Response{msg=string}  "创建SysDictionaryDetail"
// @Router    /sysDictionaryDetail/createSysDictionaryDetail [post]
func (s *DictionaryDetailApi) CreateSysDictionaryDetail(c *gin.Context) {
	var detail system.SysDictionaryDetail
	err := c.ShouldBindJSON(&detail)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = dictionaryDetailService.CreateSysDictionaryDetail(detail)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败", c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteSysDictionaryDetail
// @Tags      SysDictionaryDetail
// @Summary   删除SysDictionaryDetail
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysDictionaryDetail     true  "SysDictionaryDetail模型"
// @Success   200   {object}  response.Response{msg=string}  "删除SysDictionaryDetail"
// @Router    /sysDictionaryDetail/deleteSysDictionaryDetail [delete]
func (s *DictionaryDetailApi) DeleteSysDictionaryDetail(c *gin.Context) {
	var detail system.SysDictionaryDetail
	err := c.ShouldBindJSON(&detail)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = dictionaryDetailService.DeleteSysDictionaryDetail(detail)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// UpdateSysDictionaryDetail
// @Tags      SysDictionaryDetail
// @Summary   更新SysDictionaryDetail
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysDictionaryDetail     true  "更新SysDictionaryDetail"
// @Success   200   {object}  response.Response{msg=string}  "更新SysDictionaryDetail"
// @Router    /sysDictionaryDetail/updateSysDictionaryDetail [put]
func (s *DictionaryDetailApi) UpdateSysDictionaryDetail(c *gin.Context) {
	var detail system.SysDictionaryDetail
	err := c.ShouldBindJSON(&detail)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = dictionaryDetailService.UpdateSysDictionaryDetail(&detail)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败", c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindSysDictionaryDetail
// @Tags      SysDictionaryDetail
// @Summary   用id查询SysDictionaryDetail
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     system.SysDictionaryDetail                                 true  "用id查询SysDictionaryDetail"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "用id查询SysDictionaryDetail"
// @Router    /sysDictionaryDetail/findSysDictionaryDetail [get]
func (s *DictionaryDetailApi) FindSysDictionaryDetail(c *gin.Context) {
	var detail system.SysDictionaryDetail
	err := c.ShouldBindQuery(&detail)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(detail, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	reSysDictionaryDetail, err := dictionaryDetailService.GetSysDictionaryDetail(detail.ID)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"reSysDictionaryDetail": reSysDictionaryDetail}, "查询成功", c)
}

// GetSysDictionaryDetailList
// @Tags      SysDictionaryDetail
// @Summary   分页获取SysDictionaryDetail列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     request.SysDictionaryDetailSearch                       true  "页码, 每页大小, 搜索条件"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取SysDictionaryDetail列表,返回包括列表,总数,页码,每页数量"
// @Router    /sysDictionaryDetail/getSysDictionaryDetailList [get]
func (s *DictionaryDetailApi) GetSysDictionaryDetailList(c *gin.Context) {
	var pageInfo request.SysDictionaryDetailSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := dictionaryDetailService.GetSysDictionaryDetailInfoList(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// GetDictionaryTreeList 获取字典详情树形结构
// 设计说明：
// 1. 类型转换：手动将字符串转换为uint，进行类型校验
// 2. 参数验证：先检查参数是否为空，再检查格式是否正确
// 3. 树形结构：返回树形结构数据，便于前端展示层级关系
// 4. 好处：类型安全、参数验证完整、数据结构清晰
// @Tags      SysDictionaryDetail
// @Summary   获取字典详情树形结构
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     sysDictionaryID  query     int                                                true  "字典ID"
// @Success   200              {object}  response.Response{data=[]system.SysDictionaryDetail,msg=string}  "获取字典详情树形结构"
// @Router    /sysDictionaryDetail/getDictionaryTreeList [get]
func (s *DictionaryDetailApi) GetDictionaryTreeList(c *gin.Context) {
	sysDictionaryID := c.Query("sysDictionaryID")
	if sysDictionaryID == "" {
		response.FailWithMessage("字典ID不能为空", c)
		return
	}

	// 手动进行类型转换，确保类型安全
	// 使用ParseUint解析为uint64，再转换为uint，保证数值范围正确
	var id uint
	if idUint64, err := strconv.ParseUint(sysDictionaryID, 10, 32); err != nil {
		response.FailWithMessage("字典ID格式错误", c)
		return
	} else {
		id = uint(idUint64)
	}
	
	list, err := dictionaryDetailService.GetDictionaryTreeList(id)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"list": list}, "获取成功", c)
}

// GetDictionaryTreeListByType
// @Tags      SysDictionaryDetail
// @Summary   根据字典类型获取字典详情树形结构
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     type  query     string                                                true  "字典类型"
// @Success   200   {object}  response.Response{data=[]system.SysDictionaryDetail,msg=string}  "获取字典详情树形结构"
// @Router    /sysDictionaryDetail/getDictionaryTreeListByType [get]
func (s *DictionaryDetailApi) GetDictionaryTreeListByType(c *gin.Context) {
	dictType := c.Query("type")
	if dictType == "" {
		response.FailWithMessage("字典类型不能为空", c)
		return
	}
	
	list, err := dictionaryDetailService.GetDictionaryTreeListByType(dictType)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"list": list}, "获取成功", c)
}

// GetDictionaryDetailsByParent
// @Tags      SysDictionaryDetail
// @Summary   根据父级ID获取字典详情
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     request.GetDictionaryDetailsByParentRequest                true  "查询参数"
// @Success   200   {object}  response.Response{data=[]system.SysDictionaryDetail,msg=string}  "获取字典详情列表"
// @Router    /sysDictionaryDetail/getDictionaryDetailsByParent [get]
func (s *DictionaryDetailApi) GetDictionaryDetailsByParent(c *gin.Context) {
	var req request.GetDictionaryDetailsByParentRequest
	err := c.ShouldBindQuery(&req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	list, err := dictionaryDetailService.GetDictionaryDetailsByParent(req)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"list": list}, "获取成功", c)
}

// GetDictionaryPath
// @Tags      SysDictionaryDetail
// @Summary   获取字典详情的完整路径
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     id  query     uint                                                true  "字典详情ID"
// @Success   200 {object}  response.Response{data=[]system.SysDictionaryDetail,msg=string}  "获取字典详情路径"
// @Router    /sysDictionaryDetail/getDictionaryPath [get]
func (s *DictionaryDetailApi) GetDictionaryPath(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		response.FailWithMessage("字典详情ID不能为空", c)
		return
	}
	
	var id uint
	if idUint64, err := strconv.ParseUint(idStr, 10, 32); err != nil {
		response.FailWithMessage("字典详情ID格式错误", c)
		return
	} else {
		id = uint(idUint64)
	}
	
	path, err := dictionaryDetailService.GetDictionaryPath(id)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"path": path}, "获取成功", c)
}
