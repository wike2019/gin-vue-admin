package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthorityApi struct{}

// CreateAuthority 创建角色接口
// 设计要点：
// 1. 严格权限模式：在严格权限模式下，新角色的父角色自动设置为当前用户角色
// 2. 权限刷新：创建角色后立即刷新 Casbin 权限缓存，确保权限立即生效
// 3. 参数验证：验证角色信息格式，确保数据完整性
// 4. 错误处理：区分创建失败和权限刷新失败，提供精确的错误信息
//
// 为什么这么写：
// - 严格权限模式：防止用户创建比自己权限更高的角色，提升安全性
// - 权限刷新：创建角色后立即刷新缓存，确保权限立即生效，无需重启服务
// - 错误区分：区分创建失败和权限刷新失败，便于问题定位
// - 数据验证：双重验证确保数据完整性和安全性
//
// 工作流程：
// 1. 验证请求参数
// 2. 严格权限模式检查（如果启用）
// 3. 创建角色
// 4. 刷新 Casbin 权限缓存
//
// 好处：
// - 安全性：严格权限模式防止权限提升攻击
// - 实时性：权限刷新确保新角色立即生效
// - 可维护性：清晰的错误处理便于问题定位
//
// @Tags      Authority
// @Summary   创建角色
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysAuthority                                                true  "权限id, 权限名, 父角色id"
// @Success   200   {object}  response.Response{data=systemRes.SysAuthorityResponse,msg=string}  "创建角色,返回包括系统角色详情"
// @Router    /authority/createAuthority [post]
func (a *AuthorityApi) CreateAuthority(c *gin.Context) {
	var authority, authBack system.SysAuthority
	var err error

	// 步骤1：绑定请求参数
	if err = c.ShouldBindJSON(&authority); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	// 步骤2：验证业务规则（如角色名格式、权限ID格式等）
	if err = utils.Verify(authority, utils.AuthorityVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	// 步骤3：严格权限模式检查
	// 如果启用严格权限模式且父角色为0（根角色），则自动设置为当前用户角色
	// 设计原因：防止用户创建比自己权限更高的角色，提升安全性
	// 好处：防止权限提升攻击，确保权限层级正确
	if *authority.ParentId == 0 && global.GVA_CONFIG.System.UseStrictAuth {
		authority.ParentId = utils.Pointer(utils.GetUserAuthorityId(c))
	}

	// 步骤4：调用服务层创建角色
	if authBack, err = authorityService.CreateAuthority(authority); err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败"+err.Error(), c)
		return
	}
	
	// 步骤5：刷新 Casbin 权限缓存
	// 设计原因：创建角色后需要刷新权限缓存，确保新角色权限立即生效
	// 好处：无需重启服务，权限变更立即生效
	err = casbinService.FreshCasbin()
	if err != nil {
		global.GVA_LOG.Error("创建成功，权限刷新失败。", zap.Error(err))
		// 区分创建失败和权限刷新失败，便于问题定位
		response.FailWithMessage("创建成功，权限刷新失败。"+err.Error(), c)
		return
	}
	response.OkWithDetailed(systemRes.SysAuthorityResponse{Authority: authBack}, "创建成功", c)
}

// CopyAuthority
// @Tags      Authority
// @Summary   拷贝角色
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      response.SysAuthorityCopyResponse                                  true  "旧角色id, 新权限id, 新权限名, 新父角色id"
// @Success   200   {object}  response.Response{data=systemRes.SysAuthorityResponse,msg=string}  "拷贝角色,返回包括系统角色详情"
// @Router    /authority/copyAuthority [post]
func (a *AuthorityApi) CopyAuthority(c *gin.Context) {
	var copyInfo systemRes.SysAuthorityCopyResponse
	err := c.ShouldBindJSON(&copyInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(copyInfo, utils.OldAuthorityVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(copyInfo.Authority, utils.AuthorityVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	adminAuthorityID := utils.GetUserAuthorityId(c)
	authBack, err := authorityService.CopyAuthority(adminAuthorityID, copyInfo)
	if err != nil {
		global.GVA_LOG.Error("拷贝失败!", zap.Error(err))
		response.FailWithMessage("拷贝失败"+err.Error(), c)
		return
	}
	response.OkWithDetailed(systemRes.SysAuthorityResponse{Authority: authBack}, "拷贝成功", c)
}

// DeleteAuthority 删除角色接口
// 设计要点：
// 1. 使用检查：删除前检查是否有用户正在使用此角色
// 2. 级联处理：service 层会处理相关的级联删除（如角色菜单关联、角色API关联等）
// 3. 权限刷新：删除后刷新 Casbin 权限缓存，确保权限立即生效
// 4. 错误处理：忽略权限刷新错误（使用 _ 接收），因为删除已成功
//
// 为什么这么写：
// - 使用检查：防止删除正在使用的角色，避免数据不一致
// - 级联处理：在 service 层处理相关数据删除，保持数据一致性
// - 权限刷新：删除后刷新缓存，确保权限立即生效
// - 错误忽略：删除成功后，即使权限刷新失败也不影响删除操作
//
// 安全考虑：
// - 数据一致性：检查角色使用情况，防止删除正在使用的角色
// - 级联删除：处理相关数据，保持数据完整性
// - 权限同步：刷新权限缓存，确保权限与数据库一致
//
// @Tags      Authority
// @Summary   删除角色
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysAuthority            true  "删除角色"
// @Success   200   {object}  response.Response{msg=string}  "删除角色"
// @Router    /authority/deleteAuthority [post]
func (a *AuthorityApi) DeleteAuthority(c *gin.Context) {
	var authority system.SysAuthority
	var err error
	
	// 步骤1：绑定请求参数
	if err = c.ShouldBindJSON(&authority); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：验证角色ID格式
	if err = utils.Verify(authority, utils.AuthorityIdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤3：调用服务层删除角色
	// service 层会：
	// 1. 检查是否有用户正在使用此角色
	// 2. 处理级联删除（如角色菜单关联、角色API关联等）
	// 3. 删除角色记录
	// 设计原因：在 service 层统一处理，保持数据一致性
	if err = authorityService.DeleteAuthority(&authority); err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败"+err.Error(), c)
		return
	}
	
	// 步骤4：刷新 Casbin 权限缓存（忽略错误）
	// 设计原因：删除成功后，即使权限刷新失败也不影响删除操作
	// 好处：删除操作不会因为权限刷新失败而回滚
	_ = casbinService.FreshCasbin()
	response.OkWithMessage("删除成功", c)
}

// UpdateAuthority
// @Tags      Authority
// @Summary   更新角色信息
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysAuthority                                                true  "权限id, 权限名, 父角色id"
// @Success   200   {object}  response.Response{data=systemRes.SysAuthorityResponse,msg=string}  "更新角色信息,返回包括系统角色详情"
// @Router    /authority/updateAuthority [put]
func (a *AuthorityApi) UpdateAuthority(c *gin.Context) {
	var auth system.SysAuthority
	err := c.ShouldBindJSON(&auth)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(auth, utils.AuthorityVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	authority, err := authorityService.UpdateAuthority(auth)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败"+err.Error(), c)
		return
	}
	response.OkWithDetailed(systemRes.SysAuthorityResponse{Authority: authority}, "更新成功", c)
}

// GetAuthorityList
// @Tags      Authority
// @Summary   分页获取角色列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.PageInfo                                        true  "页码, 每页大小"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取角色列表,返回包括列表,总数,页码,每页数量"
// @Router    /authority/getAuthorityList [post]
func (a *AuthorityApi) GetAuthorityList(c *gin.Context) {
	authorityID := utils.GetUserAuthorityId(c)
	list, err := authorityService.GetAuthorityInfoList(authorityID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败"+err.Error(), c)
		return
	}
	response.OkWithDetailed(list, "获取成功", c)
}

// SetDataAuthority
// @Tags      Authority
// @Summary   设置角色资源权限
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysAuthority            true  "设置角色资源权限"
// @Success   200   {object}  response.Response{msg=string}  "设置角色资源权限"
// @Router    /authority/setDataAuthority [post]
func (a *AuthorityApi) SetDataAuthority(c *gin.Context) {
	var auth system.SysAuthority
	err := c.ShouldBindJSON(&auth)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(auth, utils.AuthorityIdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	adminAuthorityID := utils.GetUserAuthorityId(c)
	err = authorityService.SetDataAuthority(adminAuthorityID, auth)
	if err != nil {
		global.GVA_LOG.Error("设置失败!", zap.Error(err))
		response.FailWithMessage("设置失败"+err.Error(), c)
		return
	}
	response.OkWithMessage("设置成功", c)
}
