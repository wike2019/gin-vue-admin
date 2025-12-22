package example

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	exampleRes "github.com/flipped-aurora/gin-vue-admin/server/model/example/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CustomerApi 客户管理 API 控制器
// 实现了完整的 CRUD 操作（Create、Read、Update、Delete）
// 设计模式：RESTful API 设计
// 特点：
// 1. 标准的 REST 操作：POST（创建）、GET（查询）、PUT（更新）、DELETE（删除）
// 2. 参数验证：使用 utils.Verify 进行业务规则验证
// 3. 权限控制：自动获取当前用户信息，实现数据隔离
// 4. 统一响应：所有操作都通过 response 包统一返回格式
type CustomerApi struct{}

// CreateExaCustomer 创建客户
// 设计要点：
// 1. 双重验证：先验证 JSON 格式，再验证业务规则
// 2. 自动关联用户：从 JWT token 中提取用户信息，自动关联到客户记录
// 3. 数据隔离：通过 SysUserID 和 SysUserAuthorityID 实现多租户数据隔离
//
// 为什么这么写：
// - ShouldBindJSON + Verify：分层验证，JSON 验证在前，业务验证在后
// - 自动设置用户信息：避免前端传递用户信息，防止权限绕过
// - 记录创建者：便于后续审计和权限控制
//
// @Tags      ExaCustomer
// @Summary   创建客户
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      example.ExaCustomer            true  "客户用户名, 客户手机号码"
// @Success   200   {object}  response.Response{msg=string}  "创建客户"
// @Router    /customer/customer [post]
func (e *CustomerApi) CreateExaCustomer(c *gin.Context) {
	var customer example.ExaCustomer
	// 第一步：验证 JSON 格式和基本类型
	err := c.ShouldBindJSON(&customer)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 第二步：验证业务规则（如手机号格式、用户名长度等）
	// 这种分层验证的好处：
	// 1. 提前发现格式错误，避免无效的数据库操作
	// 2. 业务规则集中管理，便于维护
	err = utils.Verify(customer, utils.CustomerVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 从 JWT token 中提取当前用户信息
	// 好处：
	// 1. 安全性：用户信息来自 token，前端无法伪造
	// 2. 自动化：无需前端传递，减少参数
	// 3. 一致性：所有创建操作都自动关联创建者
	customer.SysUserID = utils.GetUserID(c)
	customer.SysUserAuthorityID = utils.GetUserAuthorityId(c)
	// 调用 service 层执行创建操作
	err = customerService.CreateExaCustomer(customer)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败", c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteExaCustomer 删除客户
// 设计要点：
// 1. ID 验证：使用 IdVerify 确保 ID 有效
// 2. 软删除：通常使用 GVA_MODEL 的 DeletedAt 字段实现软删除
// 3. 权限控制：service 层会验证用户是否有权限删除该客户
//
// 为什么这么写：
// - 验证 ID：防止无效 ID 导致的错误
// - 使用 GVA_MODEL：继承通用模型，包含 ID、创建时间等通用字段
// - 软删除：数据不真正删除，便于数据恢复和审计
//
// @Tags      ExaCustomer
// @Summary   删除客户
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      example.ExaCustomer            true  "客户ID"
// @Success   200   {object}  response.Response{msg=string}  "删除客户"
// @Router    /customer/customer [delete]
func (e *CustomerApi) DeleteExaCustomer(c *gin.Context) {
	var customer example.ExaCustomer
	err := c.ShouldBindJSON(&customer)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 验证 ID 是否有效（非零值）
	// GVA_MODEL 是通用模型，包含 ID、CreatedAt、UpdatedAt、DeletedAt 等字段
	err = utils.Verify(customer.GVA_MODEL, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// service 层会处理：
	// 1. 权限验证：检查当前用户是否有权限删除该客户
	// 2. 软删除：设置 DeletedAt 字段，而非物理删除
	// 3. 关联数据：处理客户相关的其他数据（如订单、文件等）
	err = customerService.DeleteExaCustomer(customer)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// UpdateExaCustomer 更新客户信息
// 设计要点：
// 1. 双重验证：先验证 ID，再验证业务规则
// 2. 指针传递：使用 &customer 避免结构体拷贝，提高性能
// 3. 部分更新：只更新传入的字段，未传入的字段保持不变
//
// 为什么这么写：
// - 先验证 ID：确保要更新的记录存在
// - 再验证业务规则：确保更新后的数据符合业务要求
// - 指针传递：Go 中结构体是值类型，传递指针避免拷贝大对象
// - 部分更新：GORM 的 Save 方法只更新非零值字段，实现部分更新
//
// @Tags      ExaCustomer
// @Summary   更新客户信息
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      example.ExaCustomer            true  "客户ID, 客户信息"
// @Success   200   {object}  response.Response{msg=string}  "更新客户信息"
// @Router    /customer/customer [put]
func (e *CustomerApi) UpdateExaCustomer(c *gin.Context) {
	var customer example.ExaCustomer
	err := c.ShouldBindJSON(&customer)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 第一步：验证 ID 是否有效
	err = utils.Verify(customer.GVA_MODEL, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 第二步：验证业务规则
	err = utils.Verify(customer, utils.CustomerVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 使用指针传递，service 层可以直接修改结构体
	// GORM 的 Save 方法会智能地只更新非零值字段
	err = customerService.UpdateExaCustomer(&customer)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败", c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// GetExaCustomer 获取单个客户详情
// 设计要点：
// 1. 使用 GET 方法：符合 RESTful 规范，GET 用于查询操作
// 2. ShouldBindQuery：从 URL 查询参数中获取 ID
// 3. 返回详细数据：包含客户的完整信息，便于前端展示
//
// 为什么这么写：
// - GET + Query：查询操作使用 GET，参数放在 URL 中，符合 HTTP 语义
// - 返回响应对象：使用专门的 Response 结构体，便于扩展字段
// - ID 验证：确保查询参数有效，避免无效查询
//
// @Tags      ExaCustomer
// @Summary   获取单一客户信息
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     example.ExaCustomer                                                true  "客户ID"
// @Success   200   {object}  response.Response{data=exampleRes.ExaCustomerResponse,msg=string}  "获取单一客户信息,返回包括客户详情"
// @Router    /customer/customer [get]
func (e *CustomerApi) GetExaCustomer(c *gin.Context) {
	var customer example.ExaCustomer
	// ShouldBindQuery 从 URL 查询参数中绑定数据
	// 例如：/customer/customer?id=123
	err := c.ShouldBindQuery(&customer)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 验证 ID 是否有效
	err = utils.Verify(customer.GVA_MODEL, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// service 层会处理：
	// 1. 权限验证：检查当前用户是否有权限查看该客户
	// 2. 数据加载：加载客户及其关联数据（如订单、文件等）
	data, err := customerService.GetExaCustomer(customer.ID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	// 返回完整的客户信息，包含所有关联数据
	response.OkWithDetailed(exampleRes.ExaCustomerResponse{Customer: data}, "获取成功", c)
}

// GetExaCustomerList 分页获取客户列表（带权限控制）
// 设计要点：
// 1. 权限隔离：通过 GetUserAuthorityId 实现多租户数据隔离
// 2. 分页查询：支持分页，避免一次性加载大量数据
// 3. 权限过滤：只返回当前用户有权限查看的客户
//
// 为什么这么写：
// - 传递权限 ID：service 层根据权限过滤数据，实现数据隔离
// - 分页验证：确保分页参数合理（如页码 > 0，每页大小在合理范围内）
// - 权限隔离：不同角色的用户看到不同的客户列表，保证数据安全
//
// 安全考虑：
// - 即使前端传递了错误的权限 ID，也会被 JWT token 中的真实权限覆盖
// - service 层会再次验证权限，防止权限绕过
//
// @Tags      ExaCustomer
// @Summary   分页获取权限客户列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  query     request.PageInfo                                        true  "页码, 每页大小"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取权限客户列表,返回包括列表,总数,页码,每页数量"
// @Router    /customer/customerList [get]
func (e *CustomerApi) GetExaCustomerList(c *gin.Context) {
	var pageInfo request.PageInfo
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 验证分页参数（页码、每页大小等）
	err = utils.Verify(pageInfo, utils.PageInfoVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 传递当前用户的权限 ID，service 层会根据权限过滤数据
	// 这种设计的好处：
	// 1. 数据隔离：不同角色的用户看到不同的数据
	// 2. 安全性：权限信息来自 JWT token，前端无法伪造
	// 3. 灵活性：可以根据权限实现复杂的过滤逻辑
	customerList, total, err := customerService.GetCustomerInfoList(utils.GetUserAuthorityId(c), pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败"+err.Error(), c)
		return
	}
	// 返回分页结果，包含列表和分页信息
	response.OkWithDetailed(response.PageResult{
		List:     customerList,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}
