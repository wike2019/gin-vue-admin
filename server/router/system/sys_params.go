package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// SysParamsRouter 系统参数路由结构体
// 用于封装系统参数相关的路由初始化逻辑
type SysParamsRouter struct{}

// InitSysParamsRouter 初始化系统参数路由信息
//
// 设计说明：
// 1. 路由分组设计：
//   - sysParamsRouter: 带操作记录中间件的路由组，用于需要审计的操作（增删改）
//   - sysParamsRouterWithoutRecord: 不带操作记录的路由组，用于查询操作
//
// 2. 为什么区分带记录和不带记录的路由组？
//   - 写操作（POST/DELETE/PUT）通常需要记录操作日志，用于安全审计和问题追溯
//   - 读操作（GET）调用频繁，如果都记录会产生大量日志，影响性能且没有审计价值
//   - 这种设计可以减少不必要的中间件开销，提高查询接口的响应速度
//   - 符合"最小权限"原则，只对关键操作进行审计记录
//
// 3. 使用 Router 而非 PublicRouter：
//   - Router 是需要认证的私有路由组，适合系统参数这类敏感配置
//   - PublicRouter 是公开路由组，不需要认证即可访问，不适合参数管理
//
// 参数说明：
//   - Router: 需要认证的私有路由组，用于需要登录才能访问的接口
//   - PublicRouter: 公开路由组，用于无需认证的公开接口（此处未使用）
func (s *SysParamsRouter) InitSysParamsRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	// 创建带操作记录中间件的路由组
	// 使用 middleware.OperationRecord() 中间件记录操作日志，用于审计追踪
	// 好处：所有写操作都会自动记录操作时间、操作人、操作内容等信息
	sysParamsRouter := Router.Group("sysParams").Use(middleware.OperationRecord())

	// 创建不带操作记录的路由组
	// 查询操作不记录日志，避免产生大量无意义的日志记录
	// 好处：减少数据库写入压力，提高查询接口的响应速度
	sysParamsRouterWithoutRecord := Router.Group("sysParams")

	// 写操作路由组 - 需要记录操作日志
	// 这些操作会改变系统状态，需要审计追踪
	{
		sysParamsRouter.POST("createSysParams", sysParamsApi.CreateSysParams)             // 新建参数 - 创建操作需要记录
		sysParamsRouter.DELETE("deleteSysParams", sysParamsApi.DeleteSysParams)           // 删除参数 - 删除操作需要记录
		sysParamsRouter.DELETE("deleteSysParamsByIds", sysParamsApi.DeleteSysParamsByIds) // 批量删除参数 - 批量操作需要记录
		sysParamsRouter.PUT("updateSysParams", sysParamsApi.UpdateSysParams)              // 更新参数 - 修改操作需要记录
	}

	// 读操作路由组 - 不需要记录操作日志
	// 这些操作只是查询数据，不会改变系统状态，频繁调用无需记录
	{
		sysParamsRouterWithoutRecord.GET("findSysParams", sysParamsApi.FindSysParams)       // 根据ID获取参数 - 查询操作无需记录
		sysParamsRouterWithoutRecord.GET("getSysParamsList", sysParamsApi.GetSysParamsList) // 获取参数列表 - 查询操作无需记录
		sysParamsRouterWithoutRecord.GET("getSysParam", sysParamsApi.GetSysParam)           // 根据Key获取参数 - 查询操作无需记录
	}
}
