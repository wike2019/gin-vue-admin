package system

// 操作记录管理API层
// 设计说明：
// 1. CRUD操作标准化：提供完整的增删改查接口，遵循RESTful设计原则
// 2. 参数验证：使用utils.Verify进行业务参数验证，保证数据有效性
// 3. 分页查询：使用统一的PageResult结构，保证分页响应格式一致
// 4. 好处：接口规范、易于使用、便于维护

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OperationRecordApi 操作记录API结构体
type OperationRecordApi struct{}

// DeleteSysOperationRecord
// @Tags      SysOperationRecord
// @Summary   删除SysOperationRecord
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysOperationRecord      true  "SysOperationRecord模型"
// @Success   200   {object}  response.Response{msg=string}  "删除SysOperationRecord"
// @Router    /sysOperationRecord/deleteSysOperationRecord [delete]
func (s *OperationRecordApi) DeleteSysOperationRecord(c *gin.Context) {
	var sysOperationRecord system.SysOperationRecord
	err := c.ShouldBindJSON(&sysOperationRecord)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = operationRecordService.DeleteSysOperationRecord(sysOperationRecord)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteSysOperationRecordByIds
// @Tags      SysOperationRecord
// @Summary   批量删除SysOperationRecord
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.IdsReq                 true  "批量删除SysOperationRecord"
// @Success   200   {object}  response.Response{msg=string}  "批量删除SysOperationRecord"
// @Router    /sysOperationRecord/deleteSysOperationRecordByIds [delete]
func (s *OperationRecordApi) DeleteSysOperationRecordByIds(c *gin.Context) {
	var IDS request.IdsReq
	err := c.ShouldBindJSON(&IDS)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = operationRecordService.DeleteSysOperationRecordByIds(IDS)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败", c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// FindSysOperationRecord 根据ID查询操作记录
// 设计说明：
// 1. 使用GET方法：查询操作使用GET符合RESTful规范
// 2. ShouldBindQuery：绑定URL查询参数，适用于GET请求
// 3. 双重验证：先进行参数绑定验证，再进行业务规则验证（ID有效性）
// 4. 好处：符合HTTP语义、参数验证完整、安全性高
// @Tags      SysOperationRecord
// @Summary   用id查询SysOperationRecord
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     system.SysOperationRecord                                  true  "Id"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "用id查询SysOperationRecord"
// @Router    /sysOperationRecord/findSysOperationRecord [get]
func (s *OperationRecordApi) FindSysOperationRecord(c *gin.Context) {
	var sysOperationRecord system.SysOperationRecord
	// GET请求使用ShouldBindQuery绑定URL查询参数
	err := c.ShouldBindQuery(&sysOperationRecord)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 使用工具函数进行业务规则验证，确保ID有效
	// 好处：统一验证逻辑，避免重复代码
	err = utils.Verify(sysOperationRecord, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	reSysOperationRecord, err := operationRecordService.GetSysOperationRecord(sysOperationRecord.ID)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"reSysOperationRecord": reSysOperationRecord}, "查询成功", c)
}

// GetSysOperationRecordList 分页获取操作记录列表
// 设计说明：
// 1. 使用专门的Search结构体：封装分页参数和搜索条件，结构清晰
// 2. 统一分页响应：使用PageResult结构，包含列表、总数、页码等信息
// 3. 返回完整分页信息：前端可以根据total和PageSize计算总页数
// 4. 好处：分页逻辑统一、前端处理方便、接口规范
// @Tags      SysOperationRecord
// @Summary   分页获取SysOperationRecord列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     request.SysOperationRecordSearch                        true  "页码, 每页大小, 搜索条件"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取SysOperationRecord列表,返回包括列表,总数,页码,每页数量"
// @Router    /sysOperationRecord/getSysOperationRecordList [get]
func (s *OperationRecordApi) GetSysOperationRecordList(c *gin.Context) {
	var pageInfo systemReq.SysOperationRecordSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// Service层返回列表、总数和错误，API层负责组装响应
	list, total, err := operationRecordService.GetSysOperationRecordInfoList(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	// 使用统一的PageResult结构，保证所有分页接口响应格式一致
	// 好处：前端可以统一处理分页数据，减少重复代码
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}
