package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CasbinApi struct{}

// UpdateCasbin 更新角色 API 权限接口
// 设计要点：
// 1. 权限控制：从 token 获取当前用户角色，确保操作合法性
// 2. 批量更新：支持批量更新角色的 API 权限
// 3. 权限模型：使用 Casbin 权限模型，支持灵活的权限配置
// 4. 实时生效：权限更新后立即生效，无需重启服务
//
// 为什么这么写：
// - 权限控制：从 token 获取当前用户角色，防止越权操作
// - 批量更新：支持一次性更新多个 API 权限，提升效率
// - Casbin 模型：使用 Casbin 权限模型，支持复杂的权限规则
// - 实时生效：权限更新后立即生效，提升用户体验
//
// 工作流程：
// 1. 验证请求参数
// 2. 从 token 获取当前用户角色
// 3. 更新角色的 API 权限（批量）
// 4. 权限立即生效
//
// 好处：
// - 安全性：权限控制防止越权操作
// - 效率：批量更新提升操作效率
// - 灵活性：Casbin 模型支持复杂权限规则
// - 实时性：权限更新立即生效
//
// @Tags      Casbin
// @Summary   更新角色api权限
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.CasbinInReceive        true  "权限id, 权限模型列表"
// @Success   200   {object}  response.Response{msg=string}  "更新角色api权限"
// @Router    /casbin/UpdateCasbin [post]
func (cas *CasbinApi) UpdateCasbin(c *gin.Context) {
	// 步骤1：绑定请求参数
	var cmr request.CasbinInReceive
	err := c.ShouldBindJSON(&cmr)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：验证角色ID格式
	err = utils.Verify(cmr, utils.AuthorityIdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤3：从 token 获取当前用户角色ID
	// 设计原因：确保操作合法性，防止越权操作
	adminAuthorityID := utils.GetUserAuthorityId(c)
	
	// 步骤4：调用服务层更新角色的 API 权限
	// service 层会：
	// 1. 验证操作权限（当前用户是否有权限修改目标角色）
	// 2. 批量更新角色的 API 权限规则
	// 3. 更新 Casbin 权限缓存
	// 设计原因：批量更新提升效率，权限立即生效
	err = casbinService.UpdateCasbin(adminAuthorityID, cmr.AuthorityId, cmr.CasbinInfos)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败", c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// GetPolicyPathByAuthorityId
// @Tags      Casbin
// @Summary   获取权限列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.CasbinInReceive                                          true  "权限id, 权限模型列表"
// @Success   200   {object}  response.Response{data=systemRes.PolicyPathResponse,msg=string}  "获取权限列表,返回包括casbin详情列表"
// @Router    /casbin/getPolicyPathByAuthorityId [post]
func (cas *CasbinApi) GetPolicyPathByAuthorityId(c *gin.Context) {
	var casbin request.CasbinInReceive
	err := c.ShouldBindJSON(&casbin)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(casbin, utils.AuthorityIdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	paths := casbinService.GetPolicyPathByAuthorityId(casbin.AuthorityId)
	response.OkWithDetailed(systemRes.PolicyPathResponse{Paths: paths}, "获取成功", c)
}
