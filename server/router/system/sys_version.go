package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// SysVersionRouter 版本管理路由结构体
// 为什么使用结构体：
// 1. 封装路由初始化逻辑，符合面向对象的设计原则
// 2. 便于后续扩展（可以添加其他路由相关的方法或属性）
// 3. 与项目其他路由模块保持一致的代码风格
// 好处：代码组织更清晰，易于维护和扩展
type SysVersionRouter struct{}

// InitSysVersionRouter 初始化版本管理路由信息
// 参数说明：
// - Router: gin的路由组，用于在该组下注册版本管理相关的路由
//
// 设计思路：将路由分为两类，分别使用不同的中间件策略
// 1. 需要操作记录的路由（sysVersionRouter）：用于记录重要的数据变更操作
// 2. 不需要操作记录的路由（sysVersionRouterWithoutRecord）：用于查询操作，避免产生过多日志
//
// 为什么这样设计：
// - 写操作（DELETE、POST）通常涉及数据变更，需要完整记录以便审计和问题排查
// - 读操作（GET）访问频率高，完整记录会产生大量日志，占用存储空间，影响性能
// - 区分记录可以平衡审计需求和系统性能
//
// 好处：
// 1. 性能优化：减少不必要的日志记录，降低数据库写入压力
// 2. 存储优化：避免查询操作产生大量冗余日志，节省存储空间
// 3. 审计精确：只记录重要的数据变更操作，提高日志质量和可追溯性
// 4. 便于分析：重要操作的日志更集中，便于审计和问题排查
func (s *SysVersionRouter) InitSysVersionRouter(Router *gin.RouterGroup) {
	// 创建带操作记录中间件的路由组
	// 为什么使用 middleware.OperationRecord()：
	// - 该中间件会记录请求的详细信息（IP、用户、请求参数、响应内容、执行时间等）
	// - 用于审计追踪、问题排查、性能分析和安全监控
	// 适用场景：数据变更操作（增删改、导入导出）
	// 好处：完整的操作日志便于追溯数据变更历史，满足审计合规要求
	sysVersionRouter := Router.Group("sysVersion").Use(middleware.OperationRecord())

	// 创建不带操作记录中间件的路由组
	// 为什么单独创建一个路由组：
	// - 查询操作访问频率高，如果每个GET请求都记录，会产生大量日志
	// - 查询操作通常不涉及数据变更，审计价值相对较低
	// - 减少日志量可以提升系统性能，降低存储成本
	// 适用场景：查询操作（列表查询、详情查询、数据下载等）
	// 好处：提高系统性能，减少存储开销，同时保持必要的数据查询功能
	sysVersionRouterWithoutRecord := Router.Group("sysVersion")

	// 需要操作记录的路由组：数据变更操作
	// 为什么这些操作需要记录：
	// - DELETE 操作：删除数据是重要操作，需要记录谁在什么时候删除了什么数据
	// - POST 操作（导入导出）：数据导入导出涉及数据变更，需要记录操作历史
	{
		// 删除单个版本管理记录
		// 为什么记录：删除操作不可逆，需要完整记录以便审计和问题排查
		sysVersionRouter.DELETE("deleteSysVersion", sysVersionApi.DeleteSysVersion)

		// 批量删除版本管理记录
		// 为什么记录：批量删除影响范围大，需要记录删除的数据范围，便于审计
		sysVersionRouter.DELETE("deleteSysVersionByIds", sysVersionApi.DeleteSysVersionByIds)

		// 导出版本数据
		// 为什么记录：导出操作可能涉及敏感数据，需要记录谁导出了什么数据，用于安全审计
		sysVersionRouter.POST("exportVersion", sysVersionApi.ExportVersion)

		// 导入版本数据
		// 为什么记录：导入操作会修改数据库，需要记录导入的数据内容和来源，便于问题排查
		sysVersionRouter.POST("importVersion", sysVersionApi.ImportVersion)
	}

	// 不需要操作记录的路由组：查询操作
	// 为什么这些操作不记录：
	// - GET 操作访问频率高，完整记录会产生大量日志
	// - 查询操作不涉及数据变更，审计价值相对较低
	// - 如果确实需要查询日志，可以通过其他方式（如访问日志）记录
	{
		// 根据ID获取版本管理详情
		// 为什么不记录：查询操作频繁，记录会产生大量日志，通常不需要详细的查询审计
		sysVersionRouterWithoutRecord.GET("findSysVersion", sysVersionApi.FindSysVersion)

		// 获取版本管理列表（分页查询）
		// 为什么不记录：列表查询访问频率最高，记录会产生大量冗余日志
		// 如果业务需要，可以通过访问日志（如 nginx 日志）来记录查询行为
		sysVersionRouterWithoutRecord.GET("getSysVersionList", sysVersionApi.GetSysVersionList)

		// 下载版本JSON数据
		// 为什么不记录：下载操作虽然可能涉及数据，但通常是响应客户端请求，不是数据变更
		// 注意：如果需要安全审计，可以考虑将这个接口移到带记录的路由组
		sysVersionRouterWithoutRecord.GET("downloadVersionJson", sysVersionApi.DownloadVersionJson)
	}
}
