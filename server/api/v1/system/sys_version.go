package system

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SysVersionApi struct{}

// buildMenuTree 构建菜单树结构
// 设计要点：
// 1. 树形结构：将扁平菜单列表转换为树形结构，便于前端展示
// 2. 映射优化：使用 map 存储菜单，O(1) 时间复杂度查找子菜单
// 3. 递归构建：通过递归方式构建完整的菜单树
// 4. 排序支持：按 Sort 字段排序，保证菜单显示顺序
//
// 为什么这么写：
// - 映射优化：使用 map[ID]*Menu 存储菜单，查找子菜单时间复杂度从 O(n) 降到 O(1)
// - 递归构建：通过递归方式构建树形结构，代码简洁清晰
// - 排序支持：按 Sort 字段排序，保证菜单显示顺序符合预期
// - 内存效率：使用指针避免数据复制，节省内存
//
// 算法复杂度：
// - 时间复杂度：O(n) - 遍历一次菜单列表，map 查找为 O(1)
// - 空间复杂度：O(n) - 需要 map 和结果数组存储菜单
//
// 好处：
// - 性能：map 查找比线性查找快，适合大量菜单场景
// - 可维护性：递归构建逻辑清晰，易于理解和维护
// - 灵活性：支持任意深度的菜单树结构
func buildMenuTree(menus []system.SysBaseMenu) []system.SysBaseMenu {
	// 步骤1：创建菜单映射表
	// 使用 map[ID]*Menu 存储菜单，便于快速查找子菜单
	// 好处：查找子菜单时间复杂度从 O(n) 降到 O(1)
	menuMap := make(map[uint]*system.SysBaseMenu)
	for i := range menus {
		menuMap[menus[i].ID] = &menus[i]
	}

	// 步骤2：构建树结构 - 找出所有根菜单（ParentId == 0）
	// 设计原因：从根菜单开始递归构建，形成完整的树形结构
	var rootMenus []system.SysBaseMenu
	for _, menu := range menus {
		if menu.ParentId == 0 {
			// 根菜单：递归构建其子菜单树
			menuData := convertMenuToStruct(menu, menuMap)
			rootMenus = append(rootMenus, menuData)
		}
	}

	// 步骤3：按 Sort 字段排序根菜单
	// 设计原因：保证菜单显示顺序符合预期
	// 好处：前端可以直接使用，无需再次排序
	sort.Slice(rootMenus, func(i, j int) bool {
		return rootMenus[i].Sort < rootMenus[j].Sort
	})

	return rootMenus
}

// convertMenuToStruct 将菜单转换为结构体并递归处理子菜单
// 设计要点：
// 1. 数据清理：只复制业务字段，不复制数据库字段（ID、时间戳等）
// 2. 递归构建：递归处理子菜单，构建完整的菜单树
// 3. 排序支持：对子菜单进行排序，保证显示顺序
// 4. 内存优化：使用预分配容量，减少内存重新分配
//
// 为什么这么写：
// - 数据清理：导出数据时不需要数据库字段（ID、时间戳等），只保留业务数据
// - 递归构建：通过递归方式构建任意深度的菜单树，代码简洁
// - 排序支持：保证子菜单显示顺序符合预期
// - 内存优化：使用 make(..., 0, len) 预分配容量，减少内存重新分配
//
// 好处：
// - 数据纯净：导出的数据只包含业务字段，便于跨系统使用
// - 性能优化：预分配容量减少内存重新分配，提升性能
// - 可维护性：递归逻辑清晰，易于理解和维护
// - 灵活性：支持任意深度的菜单树结构
func convertMenuToStruct(menu system.SysBaseMenu, menuMap map[uint]*system.SysBaseMenu) system.SysBaseMenu {
	// 步骤1：复制菜单基本信息
	// 只复制业务字段，不复制数据库字段（ID、时间戳等）
	// 设计原因：导出数据时不需要数据库字段，只保留业务数据
	result := system.SysBaseMenu{
		Path:      menu.Path,
		Name:      menu.Name,
		Hidden:    menu.Hidden,
		Component: menu.Component,
		Sort:      menu.Sort,
		Meta:      menu.Meta,
	}

	// 步骤2：清理并复制菜单参数数据
	// 只复制业务字段，不复制数据库字段
	// 好处：导出的数据更纯净，便于跨系统使用
	if len(menu.Parameters) > 0 {
		// 预分配容量，减少内存重新分配
		cleanParameters := make([]system.SysBaseMenuParameter, 0, len(menu.Parameters))
		for _, param := range menu.Parameters {
			cleanParam := system.SysBaseMenuParameter{
				Type:  param.Type,
				Key:   param.Key,
				Value: param.Value,
				// 不复制 ID, CreatedAt, UpdatedAt, SysBaseMenuID
				// 设计原因：这些是数据库字段，导出时不需要
			}
			cleanParameters = append(cleanParameters, cleanParam)
		}
		result.Parameters = cleanParameters
	}

	// 步骤3：清理并复制菜单按钮数据
	// 只复制业务字段，不复制数据库字段
	if len(menu.MenuBtn) > 0 {
		// 预分配容量，减少内存重新分配
		cleanMenuBtns := make([]system.SysBaseMenuBtn, 0, len(menu.MenuBtn))
		for _, btn := range menu.MenuBtn {
			cleanBtn := system.SysBaseMenuBtn{
				Name: btn.Name,
				Desc: btn.Desc,
				// 不复制 ID, CreatedAt, UpdatedAt, SysBaseMenuID
			}
			cleanMenuBtns = append(cleanMenuBtns, cleanBtn)
		}
		result.MenuBtn = cleanMenuBtns
	}

	// 步骤4：递归查找并处理子菜单
	// 通过 map 查找所有 ParentId == menu.ID 的子菜单
	// 设计原因：递归构建完整的菜单树结构
	var children []system.SysBaseMenu
	for _, childMenu := range menuMap {
		if childMenu.ParentId == menu.ID {
			// 递归处理子菜单，构建子菜单树
			childData := convertMenuToStruct(*childMenu, menuMap)
			children = append(children, childData)
		}
	}

	// 步骤5：按 Sort 字段排序子菜单
	// 设计原因：保证子菜单显示顺序符合预期
	if len(children) > 0 {
		sort.Slice(children, func(i, j int) bool {
			return children[i].Sort < children[j].Sort
		})
		result.Children = children
	}

	return result
}

// DeleteSysVersion 删除版本管理
// @Tags SysVersion
// @Summary 删除版本管理
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body system.SysVersion true "删除版本管理"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /sysVersion/deleteSysVersion [delete]
func (sysVersionApi *SysVersionApi) DeleteSysVersion(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ID := c.Query("ID")
	err := sysVersionService.DeleteSysVersion(ctx, ID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteSysVersionByIds 批量删除版本管理
// @Tags SysVersion
// @Summary 批量删除版本管理
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /sysVersion/deleteSysVersionByIds [delete]
func (sysVersionApi *SysVersionApi) DeleteSysVersionByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	IDs := c.QueryArray("IDs[]")
	err := sysVersionService.DeleteSysVersionByIds(ctx, IDs)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// FindSysVersion 用id查询版本管理
// @Tags SysVersion
// @Summary 用id查询版本管理
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询版本管理"
// @Success 200 {object} response.Response{data=system.SysVersion,msg=string} "查询成功"
// @Router /sysVersion/findSysVersion [get]
func (sysVersionApi *SysVersionApi) FindSysVersion(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ID := c.Query("ID")
	resysVersion, err := sysVersionService.GetSysVersion(ctx, ID)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(resysVersion, c)
}

// GetSysVersionList 分页获取版本管理列表
// @Tags SysVersion
// @Summary 分页获取版本管理列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query systemReq.SysVersionSearch true "分页获取版本管理列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /sysVersion/getSysVersionList [get]
func (sysVersionApi *SysVersionApi) GetSysVersionList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo systemReq.SysVersionSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := sysVersionService.GetSysVersionInfoList(ctx, pageInfo)
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

// GetSysVersionPublic 不需要鉴权的版本管理接口
// @Tags SysVersion
// @Summary 不需要鉴权的版本管理接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /sysVersion/getSysVersionPublic [get]
func (sysVersionApi *SysVersionApi) GetSysVersionPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	sysVersionService.GetSysVersionPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的版本管理接口信息",
	}, "获取成功", c)
}

// ExportVersion 创建发版数据接口
// 设计要点：
// 1. 选择性导出：支持选择性地导出菜单、API、字典数据
// 2. 数据清理：清理数据库字段，只保留业务数据
// 3. 树形结构：将菜单数据转换为树形结构，便于前端使用
// 4. 版本管理：保存版本信息，支持版本回滚和对比
//
// 为什么这么写：
// - 选择性导出：允许用户选择需要导出的数据，减少不必要的数据传输
// - 数据清理：只保留业务字段，去除数据库字段，便于跨系统使用
// - 树形结构：菜单数据转换为树形结构，前端可以直接使用
// - 版本管理：保存版本信息，支持版本回滚和对比
//
// 工作流程：
// 1. 获取选中的菜单、API、字典数据
// 2. 清理数据（去除数据库字段）
// 3. 构建树形结构（菜单）
// 4. 序列化为 JSON
// 5. 保存版本记录
//
// 好处：
// - 灵活性：支持选择性导出，满足不同场景需求
// - 数据纯净：只保留业务数据，便于跨系统使用
// - 可维护性：版本管理支持回滚和对比
// - 性能：选择性导出减少数据传输量
//
// @Tags SysVersion
// @Summary 创建发版数据
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body systemReq.ExportVersionRequest true "创建发版数据"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /sysVersion/exportVersion [post]
func (sysVersionApi *SysVersionApi) ExportVersion(c *gin.Context) {
	// 创建业务用 Context，支持超时控制和取消
	// 设计原因：使用 Context 可以控制请求超时和取消，提升系统稳定性
	ctx := c.Request.Context()

	// 步骤1：绑定请求参数
	var req systemReq.ExportVersionRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	// 步骤2：获取选中的菜单数据（如果选择了菜单）
	// 设计原因：选择性导出，只获取用户选择的数据
	// 好处：减少数据传输量，提升性能
	var menuData []system.SysBaseMenu
	if len(req.MenuIds) > 0 {
		menuData, err = sysVersionService.GetMenusByIds(ctx, req.MenuIds)
		if err != nil {
			global.GVA_LOG.Error("获取菜单数据失败!", zap.Error(err))
			response.FailWithMessage("获取菜单数据失败:"+err.Error(), c)
			return
		}
	}

	// 步骤3：获取选中的API数据（如果选择了API）
	var apiData []system.SysApi
	if len(req.ApiIds) > 0 {
		apiData, err = sysVersionService.GetApisByIds(ctx, req.ApiIds)
		if err != nil {
			global.GVA_LOG.Error("获取API数据失败!", zap.Error(err))
			response.FailWithMessage("获取API数据失败:"+err.Error(), c)
			return
		}
	}

	// 步骤4：获取选中的字典数据（如果选择了字典）
	var dictData []system.SysDictionary
	if len(req.DictIds) > 0 {
		dictData, err = sysVersionService.GetDictionariesByIds(ctx, req.DictIds)
		if err != nil {
			global.GVA_LOG.Error("获取字典数据失败!", zap.Error(err))
			response.FailWithMessage("获取字典数据失败:"+err.Error(), c)
			return
		}
	}

	// 步骤5：处理菜单数据，构建递归的树形结构
	// 设计原因：菜单数据需要转换为树形结构，便于前端直接使用
	// 好处：前端无需再次处理，直接渲染树形菜单
	processedMenus := buildMenuTree(menuData)

	// 步骤6：处理API数据，清除数据库字段（ID、时间戳等）
	// 设计原因：导出数据时不需要数据库字段，只保留业务数据
	// 好处：数据更纯净，便于跨系统使用
	processedApis := make([]system.SysApi, 0, len(apiData))
	for _, api := range apiData {
		cleanApi := system.SysApi{
			Path:        api.Path,
			Description: api.Description,
			ApiGroup:    api.ApiGroup,
			Method:      api.Method,
			// 不复制 ID, CreatedAt, UpdatedAt 等数据库字段
		}
		processedApis = append(processedApis, cleanApi)
	}

	// 步骤7：处理字典数据，清除数据库字段，包含字典详情
	// 设计原因：字典数据包含详情，需要递归清理所有数据库字段
	// 好处：导出的字典数据完整且纯净
	processedDicts := make([]system.SysDictionary, 0, len(dictData))
	for _, dict := range dictData {
		cleanDict := system.SysDictionary{
			Name:   dict.Name,
			Type:   dict.Type,
			Status: dict.Status,
			Desc:   dict.Desc,
			// 不复制 ID, CreatedAt, UpdatedAt 等数据库字段
		}
		
		// 处理字典详情数据，清除数据库字段
		// 设计原因：字典详情也需要清理，保持数据一致性
		cleanDetails := make([]system.SysDictionaryDetail, 0, len(dict.SysDictionaryDetails))
		for _, detail := range dict.SysDictionaryDetails {
			cleanDetail := system.SysDictionaryDetail{
				Label:  detail.Label,
				Value:  detail.Value,
				Extend: detail.Extend,
				Status: detail.Status,
				Sort:   detail.Sort,
				// 不复制 ID, CreatedAt, UpdatedAt, SysDictionaryID
			}
			cleanDetails = append(cleanDetails, cleanDetail)
		}
		cleanDict.SysDictionaryDetails = cleanDetails
		
		processedDicts = append(processedDicts, cleanDict)
	}

	// 步骤8：构建导出数据对象
	// 包含版本信息和清理后的业务数据
	exportData := systemRes.ExportVersionResponse{
		Version: systemReq.VersionInfo{
			Name:        req.VersionName,
			Code:        req.VersionCode,
			Description: req.Description,
			ExportTime:  time.Now().Format("2006-01-02 15:04:05"), // 记录导出时间
		},
		Menus:        processedMenus,  // 树形结构的菜单数据
		Apis:         processedApis,   // 清理后的API数据
		Dictionaries: processedDicts,   // 清理后的字典数据（包含详情）
	}

	// 步骤9：序列化为 JSON（格式化输出，便于阅读）
	// 使用 MarshalIndent 格式化输出，便于人工阅读和调试
	// 设计原因：格式化的 JSON 便于人工查看和版本对比
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		global.GVA_LOG.Error("JSON序列化失败!", zap.Error(err))
		response.FailWithMessage("JSON序列化失败:"+err.Error(), c)
		return
	}

	// 步骤10：保存版本记录到数据库
	// 设计原因：保存版本信息，支持版本回滚和对比
	// 好处：可以查看历史版本，支持版本回滚
	version := system.SysVersion{
		VersionName: utils.Pointer(req.VersionName),
		VersionCode: utils.Pointer(req.VersionCode),
		Description: utils.Pointer(req.Description),
		VersionData: utils.Pointer(string(jsonData)), // 保存完整的 JSON 数据
	}

	err = sysVersionService.CreateSysVersion(ctx, &version)
	if err != nil {
		global.GVA_LOG.Error("保存版本记录失败!", zap.Error(err))
		response.FailWithMessage("保存版本记录失败:"+err.Error(), c)
		return
	}

	response.OkWithMessage("创建发版成功", c)
}

// DownloadVersionJson 下载版本JSON数据
// @Tags SysVersion
// @Summary 下载版本JSON数据
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query string true "版本ID"
// @Success 200 {object} response.Response{data=object,msg=string} "下载成功"
// @Router /sysVersion/downloadVersionJson [get]
func (sysVersionApi *SysVersionApi) DownloadVersionJson(c *gin.Context) {
	ctx := c.Request.Context()

	ID := c.Query("ID")
	if ID == "" {
		response.FailWithMessage("版本ID不能为空", c)
		return
	}

	// 获取版本记录
	version, err := sysVersionService.GetSysVersion(ctx, ID)
	if err != nil {
		global.GVA_LOG.Error("获取版本记录失败!", zap.Error(err))
		response.FailWithMessage("获取版本记录失败:"+err.Error(), c)
		return
	}

	// 构建JSON数据
	var jsonData []byte
	if version.VersionData != nil && *version.VersionData != "" {
		jsonData = []byte(*version.VersionData)
	} else {
		// 如果没有存储的JSON数据，构建一个基本的结构
		basicData := systemRes.ExportVersionResponse{
			Version: systemReq.VersionInfo{
				Name:        *version.VersionName,
				Code:        *version.VersionCode,
				Description: *version.Description,
				ExportTime:  version.CreatedAt.Format("2006-01-02 15:04:05"),
			},
			Menus: []system.SysBaseMenu{},
			Apis:  []system.SysApi{},
		}
		jsonData, _ = json.MarshalIndent(basicData, "", "  ")
	}

	// 设置下载响应头
	filename := fmt.Sprintf("version_%s_%s.json", *version.VersionCode, time.Now().Format("20060102150405"))
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Length", strconv.Itoa(len(jsonData)))

	c.Data(http.StatusOK, "application/json", jsonData)
}

// ImportVersion 导入版本数据
// @Tags SysVersion
// @Summary 导入版本数据
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body systemReq.ImportVersionRequest true "版本JSON数据"
// @Success 200 {object} response.Response{msg=string} "导入成功"
// @Router /sysVersion/importVersion [post]
func (sysVersionApi *SysVersionApi) ImportVersion(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取JSON数据
	var importData systemReq.ImportVersionRequest
	err := c.ShouldBindJSON(&importData)
	if err != nil {
		response.FailWithMessage("解析JSON数据失败:"+err.Error(), c)
		return
	}

	// 验证数据格式
	if importData.VersionInfo.Name == "" || importData.VersionInfo.Code == "" {
		response.FailWithMessage("版本信息格式错误", c)
		return
	}

	// 导入菜单数据
	if len(importData.ExportMenu) > 0 {
		if err := sysVersionService.ImportMenus(ctx, importData.ExportMenu); err != nil {
			global.GVA_LOG.Error("导入菜单失败!", zap.Error(err))
			response.FailWithMessage("导入菜单失败: "+err.Error(), c)
			return
		}
	}

	// 导入API数据
	if len(importData.ExportApi) > 0 {
		if err := sysVersionService.ImportApis(importData.ExportApi); err != nil {
			global.GVA_LOG.Error("导入API失败!", zap.Error(err))
			response.FailWithMessage("导入API失败: "+err.Error(), c)
			return
		}
	}

	// 导入字典数据
	if len(importData.ExportDictionary) > 0 {
		if err := sysVersionService.ImportDictionaries(importData.ExportDictionary); err != nil {
			global.GVA_LOG.Error("导入字典失败!", zap.Error(err))
			response.FailWithMessage("导入字典失败: "+err.Error(), c)
			return
		}
	}

	// 创建导入记录
	jsonData, _ := json.Marshal(importData)
	version := system.SysVersion{
		VersionName: utils.Pointer(importData.VersionInfo.Name),
		VersionCode: utils.Pointer(fmt.Sprintf("%s_imported_%s", importData.VersionInfo.Code, time.Now().Format("20060102150405"))),
		Description: utils.Pointer(fmt.Sprintf("导入版本: %s", importData.VersionInfo.Description)),
		VersionData: utils.Pointer(string(jsonData)),
	}

	err = sysVersionService.CreateSysVersion(ctx, &version)
	if err != nil {
		global.GVA_LOG.Error("保存导入记录失败!", zap.Error(err))
		// 这里不返回错误，因为数据已经导入成功
	}

	response.OkWithMessage("导入成功", c)
}
