package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SystemApiApi struct{}

// CreateApi
// @Tags      SysApi
// @Summary   创建基础api
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysApi                  true  "api路径, api中文描述, api组, 方法"
// @Success   200   {object}  response.Response{msg=string}  "创建基础api"
// @Router    /api/createApi [post]
func (s *SystemApiApi) CreateApi(c *gin.Context) {
	var api system.SysApi
	err := c.ShouldBindJSON(&api)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(api, utils.ApiVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = apiService.CreateApi(api)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败", c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// SyncApi 同步API接口
// 设计要点：
// 1. 自动发现：自动扫描代码中的 API 定义，发现新增的 API
// 2. 差异对比：对比代码中的 API 和数据库中的 API，找出差异
// 3. 分类返回：将差异分为新增、删除、忽略三类，便于用户选择
// 4. 非破坏性：只返回差异信息，不自动修改数据库，由用户确认后操作
//
// 为什么这么写：
// - 自动发现：通过代码扫描自动发现 API，无需手动维护
// - 差异对比：对比代码和数据库，找出需要同步的 API
// - 分类返回：将差异分类，便于用户理解和选择
// - 非破坏性：不自动修改数据库，由用户确认后操作，防止误操作
//
// 工作流程：
// 1. 扫描代码中的 API 定义（通过注解或路由注册）
// 2. 对比数据库中的 API 记录
// 3. 找出新增、删除、忽略的 API
// 4. 返回差异信息，等待用户确认
//
// 好处：
// - 自动化：自动发现 API，减少手动维护工作
// - 安全性：非破坏性操作，由用户确认后执行
// - 可维护性：清晰的差异分类，便于用户理解
//
// @Tags      SysApi
// @Summary   同步API
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200   {object}  response.Response{msg=string}  "同步API"
// @Router    /api/syncApi [get]
func (s *SystemApiApi) SyncApi(c *gin.Context) {
	// 调用服务层同步 API
	// service 层会：
	// 1. 扫描代码中的 API 定义
	// 2. 对比数据库中的 API 记录
	// 3. 返回新增、删除、忽略的 API 列表
	// 设计原因：非破坏性操作，只返回差异信息，由用户确认后操作
	newApis, deleteApis, ignoreApis, err := apiService.SyncApi()
	if err != nil {
		global.GVA_LOG.Error("同步失败!", zap.Error(err))
		response.FailWithMessage("同步失败", c)
		return
	}
	
	// 返回差异信息，包含新增、删除、忽略的 API
	// 设计原因：让用户了解需要同步的 API，由用户确认后操作
	// 好处：防止误操作，提升安全性
	response.OkWithData(gin.H{
		"newApis":    newApis,    // 新增的 API（代码中有，数据库中无）
		"deleteApis": deleteApis, // 删除的 API（数据库中有，代码中无）
		"ignoreApis": ignoreApis, // 忽略的 API（用户手动忽略的 API）
	}, c)
}

// GetApiGroups
// @Tags      SysApi
// @Summary   获取API分组
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200   {object}  response.Response{msg=string}  "获取API分组"
// @Router    /api/getApiGroups [get]
func (s *SystemApiApi) GetApiGroups(c *gin.Context) {
	groups, apiGroupMap, err := apiService.GetApiGroups()
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithData(gin.H{
		"groups":      groups,
		"apiGroupMap": apiGroupMap,
	}, c)
}

// IgnoreApi
// @Tags      IgnoreApi
// @Summary   忽略API
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200   {object}  response.Response{msg=string}  "同步API"
// @Router    /api/ignoreApi [post]
func (s *SystemApiApi) IgnoreApi(c *gin.Context) {
	var ignoreApi system.SysIgnoreApi
	err := c.ShouldBindJSON(&ignoreApi)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = apiService.IgnoreApi(ignoreApi)
	if err != nil {
		global.GVA_LOG.Error("忽略失败!", zap.Error(err))
		response.FailWithMessage("忽略失败", c)
		return
	}
	response.Ok(c)
}

// EnterSyncApi
// @Tags      SysApi
// @Summary   确认同步API
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200   {object}  response.Response{msg=string}  "确认同步API"
// @Router    /api/enterSyncApi [post]
func (s *SystemApiApi) EnterSyncApi(c *gin.Context) {
	var syncApi systemRes.SysSyncApis
	err := c.ShouldBindJSON(&syncApi)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = apiService.EnterSyncApi(syncApi)
	if err != nil {
		global.GVA_LOG.Error("忽略失败!", zap.Error(err))
		response.FailWithMessage("忽略失败", c)
		return
	}
	response.Ok(c)
}

// DeleteApi
// @Tags      SysApi
// @Summary   删除api
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysApi                  true  "ID"
// @Success   200   {object}  response.Response{msg=string}  "删除api"
// @Router    /api/deleteApi [post]
func (s *SystemApiApi) DeleteApi(c *gin.Context) {
	var api system.SysApi
	err := c.ShouldBindJSON(&api)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(api.GVA_MODEL, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = apiService.DeleteApi(api)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// GetApiList
// @Tags      SysApi
// @Summary   分页获取API列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      systemReq.SearchApiParams                               true  "分页获取API列表"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取API列表,返回包括列表,总数,页码,每页数量"
// @Router    /api/getApiList [post]
func (s *SystemApiApi) GetApiList(c *gin.Context) {
	var pageInfo systemReq.SearchApiParams
	err := c.ShouldBindJSON(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(pageInfo.PageInfo, utils.PageInfoVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := apiService.GetAPIInfoList(pageInfo.SysApi, pageInfo.PageInfo, pageInfo.OrderKey, pageInfo.Desc)
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

// GetApiById
// @Tags      SysApi
// @Summary   根据id获取api
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.GetById                                   true  "根据id获取api"
// @Success   200   {object}  response.Response{data=systemRes.SysAPIResponse}  "根据id获取api,返回包括api详情"
// @Router    /api/getApiById [post]
func (s *SystemApiApi) GetApiById(c *gin.Context) {
	var idInfo request.GetById
	err := c.ShouldBindJSON(&idInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(idInfo, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	api, err := apiService.GetApiById(idInfo.ID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(systemRes.SysAPIResponse{Api: api}, "获取成功", c)
}

// UpdateApi
// @Tags      SysApi
// @Summary   修改基础api
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysApi                  true  "api路径, api中文描述, api组, 方法"
// @Success   200   {object}  response.Response{msg=string}  "修改基础api"
// @Router    /api/updateApi [post]
func (s *SystemApiApi) UpdateApi(c *gin.Context) {
	var api system.SysApi
	err := c.ShouldBindJSON(&api)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(api, utils.ApiVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = apiService.UpdateApi(api)
	if err != nil {
		global.GVA_LOG.Error("修改失败!", zap.Error(err))
		response.FailWithMessage("修改失败", c)
		return
	}
	response.OkWithMessage("修改成功", c)
}

// GetAllApis
// @Tags      SysApi
// @Summary   获取所有的Api 不分页
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200  {object}  response.Response{data=systemRes.SysAPIListResponse,msg=string}  "获取所有的Api 不分页,返回包括api列表"
// @Router    /api/getAllApis [post]
func (s *SystemApiApi) GetAllApis(c *gin.Context) {
	authorityID := utils.GetUserAuthorityId(c)
	apis, err := apiService.GetAllApis(authorityID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(systemRes.SysAPIListResponse{Apis: apis}, "获取成功", c)
}

// DeleteApisByIds
// @Tags      SysApi
// @Summary   删除选中Api
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.IdsReq                 true  "ID"
// @Success   200   {object}  response.Response{msg=string}  "删除选中Api"
// @Router    /api/deleteApisByIds [delete]
func (s *SystemApiApi) DeleteApisByIds(c *gin.Context) {
	var ids request.IdsReq
	err := c.ShouldBindJSON(&ids)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = apiService.DeleteApisByIds(ids)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// FreshCasbin 刷新 Casbin 权限缓存接口
// 设计要点：
// 1. 权限同步：刷新 Casbin 权限缓存，确保权限与数据库一致
// 2. 实时生效：权限变更后刷新缓存，无需重启服务
// 3. 手动触发：提供手动刷新接口，便于调试和问题排查
// 4. 无认证要求：此接口不需要认证，便于系统内部调用
//
// 为什么这么写：
// - 权限同步：权限变更后需要刷新缓存，确保权限立即生效
// - 实时生效：无需重启服务，权限变更立即生效
// - 手动触发：提供手动刷新接口，便于调试和问题排查
// - 无认证要求：系统内部调用，无需认证
//
// 使用场景：
// - 权限变更后自动刷新（在创建/删除角色、修改权限时调用）
// - 手动刷新（管理员手动刷新权限缓存）
// - 问题排查（权限异常时手动刷新）
//
// 好处：
// - 实时性：权限变更立即生效，无需重启服务
// - 可维护性：提供手动刷新接口，便于问题排查
// - 性能：缓存机制提升权限检查性能
//
// @Tags      SysApi
// @Summary   刷新casbin缓存
// @accept    application/json
// @Produce   application/json
// @Success   200   {object}  response.Response{msg=string}  "刷新成功"
// @Router    /api/freshCasbin [get]
func (s *SystemApiApi) FreshCasbin(c *gin.Context) {
	// 调用服务层刷新 Casbin 权限缓存
	// service 层会：
	// 1. 从数据库加载最新的权限规则
	// 2. 更新 Casbin 内存缓存
	// 3. 确保权限与数据库一致
	// 设计原因：权限变更后需要刷新缓存，确保权限立即生效
	err := casbinService.FreshCasbin()
	if err != nil {
		global.GVA_LOG.Error("刷新失败!", zap.Error(err))
		response.FailWithMessage("刷新失败", c)
		return
	}
	response.OkWithMessage("刷新成功", c)
}
