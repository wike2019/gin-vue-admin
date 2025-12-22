package api

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/model"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/model/request"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Info 是公告 API 控制器的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：确保整个插件只有一个 API 实例，便于统一管理
var Info = new(info)

// info 是公告相关的 API 控制器
// 设计模式：控制器模式（Controller Pattern）
// 好处：
// 1. 将 HTTP 请求处理逻辑与业务逻辑分离，API 层只负责参数验证和响应格式化
// 2. 通过空结构体实现，节省内存
// 3. 符合 RESTful API 设计规范，每个方法对应一个 HTTP 端点
type info struct{}

// CreateInfo 创建公告
// 设计模式：分层架构（Layered Architecture） + 数据绑定模式（Data Binding Pattern）
// 好处：
// 1. API 层只负责接收请求和返回响应，业务逻辑委托给 Service 层处理
// 2. 使用 ShouldBindJSON 自动将 JSON 请求体解析为结构体，减少手动解析代码
// 3. 自动进行参数验证，如果格式不正确会立即返回错误，提高代码健壮性
// 4. 统一的错误处理和日志记录，便于问题追踪和调试
// 5. 使用 Swagger 注解自动生成 API 文档，提高开发效率
// @Tags Info
// @Summary 创建公告
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body model.Info true "创建公告"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /info/createInfo [post]
func (a *info) CreateInfo(c *gin.Context) {
	// 定义请求参数结构体，用于接收和验证 JSON 数据
	var info model.Info
	// 自动将请求体中的 JSON 数据绑定到结构体
	// 好处：如果 JSON 格式不正确或缺少必填字段，会自动返回错误
	err := c.ShouldBindJSON(&info)
	if err != nil {
		// 参数验证失败，直接返回错误信息，不继续处理
		// 好处：快速失败（Fail Fast），避免无效数据进入后续流程
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 调用 Service 层方法，传入解析后的参数
	// 好处：业务逻辑集中在 Service 层，便于单元测试和代码复用
	err = serviceInfo.CreateInfo(&info)
	if err != nil {
		// 记录详细的错误日志，包含错误堆栈
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		// 返回用户友好的错误信息（不暴露内部错误细节，保证安全性）
		response.FailWithMessage("创建失败", c)
		return
	}
	// 统一返回成功响应格式
	response.OkWithMessage("创建成功", c)
}

// DeleteInfo 删除公告
// 设计模式：RESTful API 模式 + 查询参数模式
// 好处：
// 1. 使用 DELETE 方法符合 RESTful 规范，语义清晰
// 2. 通过 URL 查询参数传递 ID，简化请求体
// 3. 统一的错误处理和日志记录，便于问题追踪
// @Tags Info
// @Summary 删除公告
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body model.Info true "删除公告"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /info/deleteInfo [delete]
func (a *info) DeleteInfo(c *gin.Context) {
	// 从 URL 查询参数中获取要删除的公告 ID
	// 好处：使用查询参数传递简单参数，避免不必要的 JSON 解析
	ID := c.Query("ID")
	// 调用 Service 层方法执行删除操作
	// 好处：业务逻辑集中在 Service 层，便于单元测试和代码复用
	err := serviceInfo.DeleteInfo(ID)
	if err != nil {
		// 记录详细的错误日志，包含错误堆栈
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		// 返回用户友好的错误信息（不暴露内部错误细节，保证安全性）
		response.FailWithMessage("删除失败", c)
		return
	}
	// 统一返回成功响应格式
	response.OkWithMessage("删除成功", c)
}

// DeleteInfoByIds 批量删除公告
// 设计模式：批量操作模式（Batch Operation Pattern）
// 好处：
// 1. 支持一次删除多条记录，减少网络请求次数，提高性能
// 2. 使用 QueryArray 自动解析数组参数，简化前端调用
// 3. 在数据库层面使用 IN 查询，比多次单独删除效率更高
// @Tags Info
// @Summary 批量删除公告
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /info/deleteInfoByIds [delete]
func (a *info) DeleteInfoByIds(c *gin.Context) {
	// 从 URL 查询参数中获取要删除的公告 ID 数组
	// 好处：使用 QueryArray 自动解析数组参数（如 IDs[]=1&IDs[]=2），简化前端调用
	IDs := c.QueryArray("IDs[]")
	// 调用 Service 层方法执行批量删除操作
	// 好处：业务逻辑集中在 Service 层，便于单元测试和代码复用
	if err := serviceInfo.DeleteInfoByIds(IDs); err != nil {
		// 记录详细的错误日志，包含错误堆栈
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		// 返回用户友好的错误信息（不暴露内部错误细节，保证安全性）
		response.FailWithMessage("批量删除失败", c)
		return
	}
	// 统一返回成功响应格式
	response.OkWithMessage("批量删除成功", c)
}

// UpdateInfo 更新公告
// 设计模式：RESTful API 模式 + 数据绑定模式（Data Binding Pattern）
// 好处：
// 1. 使用 PUT 方法符合 RESTful 规范，语义清晰
// 2. 使用 ShouldBindJSON 自动将 JSON 请求体解析为结构体，减少手动解析代码
// 3. 自动进行参数验证，如果格式不正确会立即返回错误，提高代码健壮性
// 4. 统一的错误处理和日志记录，便于问题追踪和调试
// @Tags Info
// @Summary 更新公告
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data body model.Info true "更新公告"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /info/updateInfo [put]
func (a *info) UpdateInfo(c *gin.Context) {
	// 定义请求参数结构体，用于接收和验证 JSON 数据
	var info model.Info
	// 自动将请求体中的 JSON 数据绑定到结构体
	// 好处：如果 JSON 格式不正确或缺少必填字段，会自动返回错误
	err := c.ShouldBindJSON(&info)
	if err != nil {
		// 参数验证失败，直接返回错误信息，不继续处理
		// 好处：快速失败（Fail Fast），避免无效数据进入后续流程
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 调用 Service 层方法，传入解析后的参数
	// 好处：业务逻辑集中在 Service 层，便于单元测试和代码复用
	err = serviceInfo.UpdateInfo(info)
	if err != nil {
		// 记录详细的错误日志，包含错误堆栈
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		// 返回用户友好的错误信息（不暴露内部错误细节，保证安全性）
		response.FailWithMessage("更新失败", c)
		return
	}
	// 统一返回成功响应格式
	response.OkWithMessage("更新成功", c)
}

// FindInfo 根据 ID 查询公告
// 设计模式：RESTful API 模式 + 查询参数模式
// 好处：
// 1. 使用 GET 方法符合 RESTful 规范，语义清晰
// 2. 通过 URL 查询参数传递 ID，简化请求体
// 3. 统一的错误处理和日志记录，便于问题追踪
// 4. 返回完整的公告对象，便于前端直接使用
// @Tags Info
// @Summary 用id查询公告
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data query model.Info true "用id查询公告"
// @Success 200 {object} response.Response{data=model.Info,msg=string} "查询成功"
// @Router /info/findInfo [get]
func (a *info) FindInfo(c *gin.Context) {
	// 从 URL 查询参数中获取要查询的公告 ID
	// 好处：使用查询参数传递简单参数，避免不必要的 JSON 解析
	ID := c.Query("ID")
	// 调用 Service 层方法获取公告详情
	// 好处：业务逻辑集中在 Service 层，便于单元测试和代码复用
	reinfo, err := serviceInfo.GetInfo(ID)
	if err != nil {
		// 记录详细的错误日志，包含错误堆栈
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		// 返回用户友好的错误信息（不暴露内部错误细节，保证安全性）
		response.FailWithMessage("查询失败", c)
		return
	}
	// 返回查询到的公告数据
	// 好处：使用统一的响应格式，包含数据和消息，便于前端统一处理
	response.OkWithData(reinfo, c)
}

// GetInfoList 分页获取公告列表
// 设计模式：分页模式（Pagination Pattern） + 查询参数绑定模式
// 好处：
// 1. 使用 ShouldBindQuery 自动将 URL 查询参数解析为结构体，支持分页和搜索条件
// 2. 统一的分页响应格式，包含列表、总数、页码和每页大小
// 3. 支持复杂的查询条件（如时间范围），通过结构体统一管理
// 4. 减少数据库查询压力，只返回当前页的数据
// @Tags Info
// @Summary 分页获取公告列表
// @Security ApiKeyAuth
// @accept application/json
// @Produce application/json
// @Param data query request.InfoSearch true "分页获取公告列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /info/getInfoList [get]
func (a *info) GetInfoList(c *gin.Context) {
	// 定义分页和搜索参数结构体
	var pageInfo request.InfoSearch
	// 自动将 URL 查询参数绑定到结构体
	// 好处：支持分页参数（page、pageSize）和搜索条件（如时间范围）
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 调用 Service 层方法，获取分页数据
	// 返回：列表数据、总记录数、错误信息
	list, total, err := serviceInfo.GetInfoInfoList(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	// 返回统一的分页响应格式
	// 好处：前端可以统一处理分页数据，包含列表、总数、页码等信息
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// GetInfoDataSource 获取公告的数据源
// 设计模式：数据源模式（DataSource Pattern）
// 好处：
// 1. 为前端提供下拉框、选择器等组件所需的数据源
// 2. 统一管理数据源，避免前端硬编码
// 3. 支持动态数据源，数据变化时前端自动更新
// 4. 此接口为公开接口，不需要鉴权，便于前端直接调用
// @Tags Info
// @Summary 获取Info的数据源
// @accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "查询成功"
// @Router /info/getInfoDataSource [get]
func (a *info) GetInfoDataSource(c *gin.Context) {
	// 调用 Service 层方法获取数据源
	// 好处：业务逻辑集中在 Service 层，便于单元测试和代码复用
	// 数据源通常包含下拉框选项、关联数据等，如用户列表、状态选项等
	dataSource, err := serviceInfo.GetInfoDataSource()
	if err != nil {
		// 记录详细的错误日志，包含错误堆栈
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		// 返回用户友好的错误信息（不暴露内部错误细节，保证安全性）
		response.FailWithMessage("查询失败", c)
		return
	}
	// 返回数据源数据
	// 好处：使用统一的响应格式，便于前端统一处理
	response.OkWithData(dataSource, c)
}

// GetInfoPublic 不需要鉴权的公告接口（公开接口）
// 设计模式：公开 API 模式（Public API Pattern）
// 好处：
// 1. 支持 C 端（客户端）访问，无需登录，提高用户体验
// 2. 适用于公开信息展示，如公告列表、新闻等
// 3. 降低访问门槛，便于信息传播
// 4. 注意：此接口为示例实现，实际使用时需要根据业务需求实现具体逻辑
// @Tags Info
// @Summary 不需要鉴权的公告接口
// @accept application/json
// @Produce application/json
// @Param data query request.InfoSearch true "分页获取公告列表"
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /info/getInfoPublic [get]
func (a *info) GetInfoPublic(c *gin.Context) {
	// 此接口不需要鉴权，示例为返回了一个固定的消息接口
	// 设计模式：示例模式（Example Pattern）
	// 好处：
	// 1. 提供接口模板，便于开发者理解如何实现公开接口
	// 2. 一般本接口用于 C 端服务，需要根据实际业务需求实现具体逻辑
	// 3. 可以调用 Service 层方法获取真实的公告数据，如：serviceInfo.GetInfoInfoList(pageInfo)
	// 注意：实际使用时应该实现真实的业务逻辑，而不是返回固定数据
	response.OkWithDetailed(gin.H{"info": "不需要鉴权的公告接口信息"}, "获取成功", c)
}
