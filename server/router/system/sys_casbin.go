package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// CasbinRouter Casbin权限管理路由
// 使用结构体封装路由，便于统一管理和扩展
type CasbinRouter struct{}

// InitCasbinRouter 初始化Casbin权限管理相关路由
// 设计思路：根据接口特性分离需要记录操作日志和不需要记录操作日志的路由
//
// 为什么这么设计：
// 1. 分离关注点：修改操作需要审计追踪，查询操作不需要
//   - 修改操作（如updateCasbin）：涉及权限变更，必须记录操作日志用于审计
//   - 查询操作（如getPolicyPathByAuthorityId）：频繁调用，不需要审计记录
//
// 2. 性能优化：
//   - 查询接口通常调用频率高，如果每次都记录操作日志会产生大量数据
//   - 避免不必要的日志写入，减少数据库I/O压力，提高系统性能
//   - 节省存储空间，降低存储成本
//
// 3. 符合业务逻辑：
//   - 审计需求主要针对数据变更操作，查询操作通常不需要审计追踪
//   - 查询操作属于"只读"操作，不会对系统状态产生影响
//
// 好处：
// - 精细化控制：可以针对不同接口选择是否记录操作日志
// - 性能提升：减少不必要的日志记录，提高系统响应速度
// - 存储优化：避免查询类接口产生大量无效日志，节省存储空间
// - 代码清晰：通过命名明确区分需要记录和不需要记录的接口
//
// @param Router *gin.RouterGroup 路由组，用于注册Casbin相关的路由
func (s *CasbinRouter) InitCasbinRouter(Router *gin.RouterGroup) {
	// casbinRouter: 带操作记录中间件的路由组
	// 作用：该路由组下的所有接口都会记录操作日志到数据库
	// 使用场景：数据修改操作，需要审计追踪的接口
	// 好处：完整的操作日志便于问题定位、安全审计和合规要求
	casbinRouter := Router.Group("casbin").Use(middleware.OperationRecord())

	// casbinRouterWithoutRecord: 不带操作记录中间件的路由组
	// 作用：该路由组下的接口不记录操作日志
	// 使用场景：查询类接口，调用频繁但不需要审计的接口
	// 好处：避免频繁的查询操作产生大量日志，提升性能，节省存储空间
	casbinRouterWithoutRecord := Router.Group("casbin")

	// 修改操作路由组：需要记录操作日志
	// 说明：权限更新操作涉及系统安全，必须完整记录操作信息用于审计
	{
		// UpdateCasbin: 更新角色的Casbin权限策略
		// 为什么需要记录：这是关键的权限变更操作，必须记录谁在什么时候修改了哪些权限
		// 好处：便于安全审计、问题排查和权限变更追溯
		casbinRouter.POST("updateCasbin", casbinApi.UpdateCasbin)
	}

	// 查询操作路由组：不需要记录操作日志
	// 说明：查询操作通常调用频繁，且不涉及数据变更，不需要审计记录
	{
		// GetPolicyPathByAuthorityId: 根据权限ID获取权限策略路径列表
		// 为什么不需要记录：这是查询接口，通常在前端页面加载时频繁调用
		// 好处：避免产生大量无效日志，提升查询性能，减少数据库写入压力
		casbinRouterWithoutRecord.POST("getPolicyPathByAuthorityId", casbinApi.GetPolicyPathByAuthorityId)
	}
}
