package system

// 系统参数管理API层
// 设计说明：
// 1. 参数管理：提供系统参数的增删改查功能，支持按key查询
// 2. 灵活的参数获取方式：支持ID查询和key查询两种方式，满足不同场景需求
// 3. 批量操作：支持批量删除，提高操作效率
// 4. 好处：功能完整、使用灵活、操作高效

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SysParamsApi 系统参数API结构体
type SysParamsApi struct{}

// CreateSysParams 创建参数
// @Tags SysParams
// @Summary 创建参数
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body system.SysParams true "创建参数"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /sysParams/createSysParams [post]
func (sysParamsApi *SysParamsApi) CreateSysParams(c *gin.Context) {
	var sysParams system.SysParams
	err := c.ShouldBindJSON(&sysParams)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = sysParamsService.CreateSysParams(&sysParams)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteSysParams 删除参数
// 设计说明：
// 1. 使用Query参数：DELETE请求使用Query参数传递ID，符合RESTful规范
// 2. 错误信息包含详情：将错误信息附加到响应中，便于前端展示
// 3. 好处：符合HTTP语义、错误信息完整
// @Tags SysParams
// @Summary 删除参数
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body system.SysParams true "删除参数"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /sysParams/deleteSysParams [delete]
func (sysParamsApi *SysParamsApi) DeleteSysParams(c *gin.Context) {
	// DELETE请求从Query参数获取ID
	ID := c.Query("ID")
	err := sysParamsService.DeleteSysParams(ID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		// 将详细错误信息返回，便于前端展示和调试
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteSysParamsByIds 批量删除参数
// 设计说明：
// 1. 使用QueryArray：批量操作使用QueryArray获取数组参数
// 2. 参数格式：使用"IDs[]"格式，前端可以方便地传递数组
// 3. 好处：支持批量操作、提高效率、减少请求次数
// @Tags SysParams
// @Summary 批量删除参数
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /sysParams/deleteSysParamsByIds [delete]
func (sysParamsApi *SysParamsApi) DeleteSysParamsByIds(c *gin.Context) {
	// 使用QueryArray获取数组参数，格式为IDs[]=1&IDs[]=2
	// 好处：前端可以方便地传递多个ID，减少请求次数
	IDs := c.QueryArray("IDs[]")
	err := sysParamsService.DeleteSysParamsByIds(IDs)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateSysParams 更新参数
// @Tags SysParams
// @Summary 更新参数
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body system.SysParams true "更新参数"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /sysParams/updateSysParams [put]
func (sysParamsApi *SysParamsApi) UpdateSysParams(c *gin.Context) {
	var sysParams system.SysParams
	err := c.ShouldBindJSON(&sysParams)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = sysParamsService.UpdateSysParams(sysParams)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindSysParams 用id查询参数
// @Tags SysParams
// @Summary 用id查询参数
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data query system.SysParams true "用id查询参数"
// @Success 200 {object} response.Response{data=system.SysParams,msg=string} "查询成功"
// @Router /sysParams/findSysParams [get]
func (sysParamsApi *SysParamsApi) FindSysParams(c *gin.Context) {
	ID := c.Query("ID")
	resysParams, err := sysParamsService.GetSysParams(ID)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(resysParams, c)
}

// GetSysParamsList 分页获取参数列表
// @Tags SysParams
// @Summary 分页获取参数列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data query systemReq.SysParamsSearch true "分页获取参数列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /sysParams/getSysParamsList [get]
func (sysParamsApi *SysParamsApi) GetSysParamsList(c *gin.Context) {
	var pageInfo systemReq.SysParamsSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := sysParamsService.GetSysParamsInfoList(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// GetSysParam 根据key获取参数value
// 设计说明：
// 1. 按key查询：提供按key查询的接口，满足按业务key获取配置的需求
// 2. 简化参数名：使用k作为变量名，代码更简洁
// 3. 好处：查询方式灵活、代码简洁、符合业务需求
// @Tags SysParams
// @Summary 根据key获取参数value
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param key query string true "key"
// @Success 200 {object} response.Response{data=system.SysParams,msg=string} "获取成功"
// @Router /sysParams/getSysParam [get]
func (sysParamsApi *SysParamsApi) GetSysParam(c *gin.Context) {
	// 按key查询参数，适用于通过业务key获取配置值的场景
	k := c.Query("key")
	params, err := sysParamsService.GetSysParam(k)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(params, "获取成功", c)
}
