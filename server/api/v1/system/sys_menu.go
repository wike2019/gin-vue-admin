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

type AuthorityMenuApi struct{}

// GetMenu 获取用户动态路由接口
// 设计要点：
// 1. 权限过滤：根据用户角色获取对应的菜单，实现权限控制
// 2. 动态路由：返回用户有权限访问的菜单，前端根据此动态生成路由
// 3. 空值处理：如果用户没有菜单权限，返回空数组而非 nil
// 4. 树形结构：返回的菜单是树形结构，便于前端直接渲染
//
// 为什么这么写：
// - 权限过滤：只返回用户有权限的菜单，实现细粒度权限控制
// - 动态路由：前端根据返回的菜单动态生成路由，无需硬编码
// - 空值处理：返回空数组而非 nil，避免前端空指针异常
// - 树形结构：前端可以直接渲染树形菜单，无需再次处理
//
// 工作流程：
// 1. 从 JWT token 获取用户角色ID
// 2. 根据角色ID查询用户有权限的菜单
// 3. 构建树形结构的菜单数据
// 4. 返回菜单数据
//
// 好处：
// - 安全性：只返回用户有权限的菜单，防止越权访问
// - 灵活性：动态路由支持灵活的权限配置
// - 用户体验：前端可以直接使用，无需额外处理
//
// @Tags      AuthorityMenu
// @Summary   获取用户动态路由
// @Security  ApiKeyAuth
// @Produce   application/json
// @Param     data  body      request.Empty                                                  true  "空"
// @Success   200   {object}  response.Response{data=systemRes.SysMenusResponse,msg=string}  "获取用户动态路由,返回包括系统菜单详情列表"
// @Router    /menu/getMenu [post]
func (a *AuthorityMenuApi) GetMenu(c *gin.Context) {
	// 步骤1：从 JWT token 获取用户角色ID
	// 设计原因：根据用户角色获取对应的菜单，实现权限控制
	authorityId := utils.GetUserAuthorityId(c)
	
	// 步骤2：调用服务层获取用户菜单树
	// service 层会：
	// 1. 根据角色ID查询角色关联的菜单
	// 2. 构建树形结构的菜单数据
	// 3. 返回菜单树
	menus, err := menuService.GetMenuTree(authorityId)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	
	// 步骤3：空值处理 - 如果用户没有菜单权限，返回空数组
	// 设计原因：避免前端空指针异常，统一返回格式
	// 好处：前端无需额外判断 nil，直接使用即可
	if menus == nil {
		menus = []system.SysMenu{}
	}
	
	response.OkWithDetailed(systemRes.SysMenusResponse{Menus: menus}, "获取成功", c)
}

// GetBaseMenuTree
// @Tags      AuthorityMenu
// @Summary   获取用户动态路由
// @Security  ApiKeyAuth
// @Produce   application/json
// @Param     data  body      request.Empty                                                      true  "空"
// @Success   200   {object}  response.Response{data=systemRes.SysBaseMenusResponse,msg=string}  "获取用户动态路由,返回包括系统菜单列表"
// @Router    /menu/getBaseMenuTree [post]
func (a *AuthorityMenuApi) GetBaseMenuTree(c *gin.Context) {
	authority := utils.GetUserAuthorityId(c)
	menus, err := menuService.GetBaseMenuTree(authority)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(systemRes.SysBaseMenusResponse{Menus: menus}, "获取成功", c)
}

// AddMenuAuthority
// @Tags      AuthorityMenu
// @Summary   增加menu和角色关联关系
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      systemReq.AddMenuAuthorityInfo  true  "角色ID"
// @Success   200   {object}  response.Response{msg=string}   "增加menu和角色关联关系"
// @Router    /menu/addMenuAuthority [post]
func (a *AuthorityMenuApi) AddMenuAuthority(c *gin.Context) {
	var authorityMenu systemReq.AddMenuAuthorityInfo
	err := c.ShouldBindJSON(&authorityMenu)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := utils.Verify(authorityMenu, utils.AuthorityIdVerify); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	adminAuthorityID := utils.GetUserAuthorityId(c)
	if err := menuService.AddMenuAuthority(authorityMenu.Menus, adminAuthorityID, authorityMenu.AuthorityId); err != nil {
		global.GVA_LOG.Error("添加失败!", zap.Error(err))
		response.FailWithMessage("添加失败", c)
	} else {
		response.OkWithMessage("添加成功", c)
	}
}

// GetMenuAuthority
// @Tags      AuthorityMenu
// @Summary   获取指定角色menu
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.GetAuthorityId                                     true  "角色ID"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "获取指定角色menu"
// @Router    /menu/getMenuAuthority [post]
func (a *AuthorityMenuApi) GetMenuAuthority(c *gin.Context) {
	var param request.GetAuthorityId
	err := c.ShouldBindJSON(&param)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(param, utils.AuthorityIdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	menus, err := menuService.GetMenuAuthority(&param)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithDetailed(systemRes.SysMenusResponse{Menus: menus}, "获取失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"menus": menus}, "获取成功", c)
}

// AddBaseMenu
// @Tags      Menu
// @Summary   新增菜单
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysBaseMenu             true  "路由path, 父菜单ID, 路由name, 对应前端文件路径, 排序标记"
// @Success   200   {object}  response.Response{msg=string}  "新增菜单"
// @Router    /menu/addBaseMenu [post]
func (a *AuthorityMenuApi) AddBaseMenu(c *gin.Context) {
	var menu system.SysBaseMenu
	err := c.ShouldBindJSON(&menu)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(menu, utils.MenuVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(menu.Meta, utils.MenuMetaVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = menuService.AddBaseMenu(menu)
	if err != nil {
		global.GVA_LOG.Error("添加失败!", zap.Error(err))
		response.FailWithMessage("添加失败："+err.Error(), c)
		return
	}
	response.OkWithMessage("添加成功", c)
}

// DeleteBaseMenu
// @Tags      Menu
// @Summary   删除菜单
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.GetById                true  "菜单id"
// @Success   200   {object}  response.Response{msg=string}  "删除菜单"
// @Router    /menu/deleteBaseMenu [post]
func (a *AuthorityMenuApi) DeleteBaseMenu(c *gin.Context) {
	var menu request.GetById
	err := c.ShouldBindJSON(&menu)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(menu, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = baseMenuService.DeleteBaseMenu(menu.ID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// UpdateBaseMenu
// @Tags      Menu
// @Summary   更新菜单
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysBaseMenu             true  "路由path, 父菜单ID, 路由name, 对应前端文件路径, 排序标记"
// @Success   200   {object}  response.Response{msg=string}  "更新菜单"
// @Router    /menu/updateBaseMenu [post]
func (a *AuthorityMenuApi) UpdateBaseMenu(c *gin.Context) {
	var menu system.SysBaseMenu
	err := c.ShouldBindJSON(&menu)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(menu, utils.MenuVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(menu.Meta, utils.MenuMetaVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = baseMenuService.UpdateBaseMenu(menu)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败", c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// GetBaseMenuById
// @Tags      Menu
// @Summary   根据id获取菜单
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.GetById                                                   true  "菜单id"
// @Success   200   {object}  response.Response{data=systemRes.SysBaseMenuResponse,msg=string}  "根据id获取菜单,返回包括系统菜单列表"
// @Router    /menu/getBaseMenuById [post]
func (a *AuthorityMenuApi) GetBaseMenuById(c *gin.Context) {
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
	menu, err := baseMenuService.GetBaseMenuById(idInfo.ID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(systemRes.SysBaseMenuResponse{Menu: menu}, "获取成功", c)
}

// GetMenuList
// @Tags      Menu
// @Summary   分页获取基础menu列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.PageInfo                                        true  "页码, 每页大小"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取基础menu列表,返回包括列表,总数,页码,每页数量"
// @Router    /menu/getMenuList [post]
func (a *AuthorityMenuApi) GetMenuList(c *gin.Context) {
	authorityID := utils.GetUserAuthorityId(c)
	menuList, err := menuService.GetInfoList(authorityID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(menuList, "获取成功", c)
}
