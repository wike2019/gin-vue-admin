package example

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// CustomerRouter 客户管理路由结构体
// 采用空结构体设计，仅作为方法接收器使用
//
// 设计意义：
// 1. 类型封装：将客户相关的路由逻辑封装在独立的结构体中，实现业务模块的隔离
// 2. 方法绑定：通过结构体方法组织路由初始化逻辑，符合 Go 的面向对象编程习惯
// 3. 可扩展性：未来如需添加客户路由级别的配置，可以轻松扩展结构体字段
type CustomerRouter struct{}

// InitCustomerRouter 初始化客户管理相关的路由
// 该方法实现了客户模块的完整 CRUD 操作路由，并区分了需要操作记录和不需要操作记录的路由
//
// 参数说明：
//   - Router: 父级路由组，通常是从主路由中传入的示例模块路由组
//
// 设计意义：
// 1. 中间件分离：区分需要记录操作日志的路由（增删改）和不需要记录的路由（查询）
//    - customerRouter: 使用 OperationRecord 中间件，记录所有写操作（创建、更新、删除）
//    - customerRouterWithoutRecord: 不使用操作记录中间件，避免查询操作产生大量日志
// 2. 性能优化：查询操作通常频率较高，不记录日志可以减少数据库写入压力，提升系统性能
// 3. 日志精准：只记录重要的业务操作（增删改），便于审计和问题追踪，避免日志冗余
// 4. RESTful 设计：
//    - POST: 创建资源（Create）
//    - PUT: 更新资源（Update）
//    - DELETE: 删除资源（Delete）
//    - GET: 查询资源（Read）
// 5. 路由分组：使用代码块 {} 将不同操作类型的路由分组，提高代码可读性和维护性
// 6. 统一前缀：所有客户相关路由使用 "customer" 作为路由组前缀，保持 URL 结构清晰
func (e *CustomerRouter) InitCustomerRouter(Router *gin.RouterGroup) {
	// 创建需要记录操作日志的客户路由组
	// 使用 middleware.OperationRecord() 中间件，自动记录所有写操作的日志
	// 适用于：创建、更新、删除等需要审计的操作
	customerRouter := Router.Group("customer").Use(middleware.OperationRecord())
	
	// 创建不需要记录操作日志的客户路由组
	// 查询操作通常频率高，不记录日志可以提升性能并减少日志冗余
	// 适用于：查询单个客户、查询客户列表等读操作
	customerRouterWithoutRecord := Router.Group("customer")
	
	{
		// 写操作路由组：需要记录操作日志
		customerRouter.POST("customer", exaCustomerApi.CreateExaCustomer)   // POST /customer/customer - 创建新客户
		customerRouter.PUT("customer", exaCustomerApi.UpdateExaCustomer)     // PUT /customer/customer - 更新客户信息
		customerRouter.DELETE("customer", exaCustomerApi.DeleteExaCustomer)  // DELETE /customer/customer - 删除客户
	}
	{
		// 读操作路由组：不需要记录操作日志
		customerRouterWithoutRecord.GET("customer", exaCustomerApi.GetExaCustomer)         // GET /customer/customer - 根据 ID 获取单个客户信息
		customerRouterWithoutRecord.GET("customerList", exaCustomerApi.GetExaCustomerList) // GET /customer/customerList - 获取客户列表（支持分页、筛选）
	}
}
