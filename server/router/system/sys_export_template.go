package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// SysExportTemplateRouter 导出模板路由结构体
//
// 设计说明：
// 1. 为什么使用结构体？
//   - 遵循面向对象设计原则，将路由初始化逻辑封装在结构体方法中
//   - 便于代码组织和维护，导出模板相关的所有路由都有独立的结构体
//   - 符合 Go 语言的最佳实践，提高代码的可读性和可维护性
//   - 便于后续扩展，如果需要添加路由相关的配置或状态，可以直接在结构体中添加字段
//
// 2. 好处：
//   - 代码结构清晰：导出模板相关的所有路由定义独立，便于查找和维护
//   - 易于扩展：未来如果需要添加导出模板相关的其他路由，只需在此方法中扩展
//   - 统一管理：所有导出模板相关的路由都在一个地方，避免路由分散
//   - 符合单一职责原则：每个路由结构体只负责一个功能模块的路由定义
type SysExportTemplateRouter struct {
}

// InitSysExportTemplateRouter 初始化导出模板路由信息
//
// 设计说明：
// 1. 路由分组设计：
//   - 使用 Router.Group("sysExportTemplate") 创建路由组，统一管理导出模板相关的所有接口
//   - 路由组路径为 "sysExportTemplate"，访问路径为 /api/v1/sysExportTemplate/xxx
//   - 好处：所有导出模板相关的接口都有统一的前缀，便于管理和识别
//
// 2. 为什么区分三个不同的路由组？
//   - sysExportTemplateRouter：需要记录操作日志的路由组，用于修改类操作（增删改）
//   - sysExportTemplateRouterWithoutRecord：不需要记录操作日志的路由组，用于查询类操作
//   - sysExportTemplateRouterWithoutAuth：不需要认证的公开路由组，用于通过token导出的接口
//   - 这样设计的原因：
//     a. 修改类操作（增删改）涉及数据变更，需要审计追踪，记录操作历史
//     b. 查询类操作不改变数据状态，属于只读操作，通常不需要审计记录
//     c. 查询操作通常非常频繁，如果都记录会产生大量日志，占用存储空间
//     d. 通过token导出的接口需要支持无认证访问，用于异步导出或分享导出链接的场景
//   - 好处：
//     1. 性能优化：查询操作不记录日志，减少系统负担和数据库写入压力
//     2. 存储优化：避免查询日志占用大量存储空间，节省成本
//     3. 日志清晰：只记录关键操作（增删改），便于后续分析和问题排查
//     4. 审计完整：所有数据变更操作都有完整记录，满足安全审计和合规要求
//     5. 灵活导出：支持通过token无认证导出，适用于异步任务和分享场景
//
// 3. 为什么使用 OperationRecord 中间件？
//   - OperationRecord 中间件用于记录操作日志，包括操作人、操作时间、操作内容等
//   - 对于修改类操作（增删改），记录操作日志有以下好处：
//     a. 安全审计：可以追踪谁在什么时候做了什么操作，满足合规要求
//     b. 问题排查：出现问题时可以根据日志快速定位和复现
//     c. 责任追溯：可以明确操作责任人，便于问题追责
//     d. 历史记录：保留完整的操作历史，支持数据恢复和回滚
//
// 4. 为什么需要 pubRouter（公开路由组）？
//   - pubRouter 是不需要认证的公开路由组，用于通过token导出的接口
//   - 这样设计的原因：
//     a. 异步导出场景：用户触发导出后，系统生成token，用户稍后通过token下载文件
//     b. 分享导出链接：用户可以将导出链接分享给他人，无需对方登录系统
//     c. 降低系统负担：导出操作可能耗时较长，通过token机制可以避免长时间占用认证会话
//   - 安全性保障：
//     a. 虽然不需要认证，但通过token机制保证安全性，token具有时效性和唯一性
//     b. token通常包含加密信息，只有持有有效token的用户才能访问
//     c. 相比完全公开的接口，token机制提供了更好的安全控制
//
// 参数说明：
//   - Router: 需要认证的私有路由组，通常是 "/api/v1" 下的子路由组
//     用于需要登录才能访问的接口，保证数据安全性
//   - pubRouter: 公开路由组，用于无需认证的公开接口
//     用于通过token导出的接口，支持异步导出和分享功能
func (s *SysExportTemplateRouter) InitSysExportTemplateRouter(Router *gin.RouterGroup, pubRouter *gin.RouterGroup) {
	// 创建需要记录操作的路由组
	// 为什么使用 OperationRecord 中间件：
	//   - 这些路由都是修改类操作（增删改），涉及导出模板数据的变更
	//   - 导出模板是系统的重要配置，模板的变更会影响数据导出功能
	//   - 记录操作日志便于问题排查、安全监控、合规审计
	// 好处：
	//   1. 可以追踪谁在什么时候修改了导出模板配置
	//   2. 出现问题时可以根据日志快速定位和复现
	//   3. 满足安全审计和合规要求
	//   4. 支持操作回滚，如果误操作可以快速恢复
	sysExportTemplateRouter := Router.Group("sysExportTemplate").Use(middleware.OperationRecord())

	// 创建不需要记录操作的路由组
	// 为什么单独创建一个路由组：
	//   - 查询类操作通常频繁且不需要审计记录
	//   - 导出模板查询操作（如获取模板列表、预览SQL）在前端页面加载时会被频繁调用
	//   - 如果都记录会产生大量无意义的日志，占用存储空间
	//   - 减少数据库写入操作，提高系统性能
	// 好处：
	//   1. 性能优化：查询操作不记录日志，减少系统负担和数据库写入压力
	//   2. 存储优化：避免查询日志占用大量存储空间，节省成本
	//   3. 日志清晰：只记录关键操作，便于后续分析和排查
	//   4. 响应速度：减少中间件处理时间，提高接口响应速度
	sysExportTemplateRouterWithoutRecord := Router.Group("sysExportTemplate")

	// 创建不需要认证的公开路由组
	// 为什么使用 pubRouter：
	//   - 通过token导出的接口需要支持无认证访问
	//   - 适用于异步导出场景：用户触发导出后，系统生成token，用户稍后通过token下载文件
	//   - 支持分享导出链接：用户可以将导出链接分享给他人，无需对方登录系统
	//   - 降低系统负担：导出操作可能耗时较长，通过token机制可以避免长时间占用认证会话
	// 安全性保障：
	//   1. token机制：虽然不需要认证，但通过token保证安全性，token具有时效性和唯一性
	//   2. 加密保护：token通常包含加密信息，只有持有有效token的用户才能访问
	//   3. 访问控制：相比完全公开的接口，token机制提供了更好的安全控制
	// 好处：
	//   1. 用户体验：支持异步导出，用户无需等待长时间操作完成
	//   2. 灵活性：支持分享导出链接，便于协作和分发
	//   3. 性能优化：避免长时间占用认证会话，提高系统并发处理能力
	sysExportTemplateRouterWithoutAuth := pubRouter.Group("sysExportTemplate")

	{
		// 需要记录操作的修改类接口
		// 为什么这些接口需要记录：
		//   - 都是数据修改操作，涉及导出模板配置的变更
		//   - 导出模板配置的变更会影响数据导出功能，属于关键操作
		//   - 需要追踪操作历史和责任人，便于问题排查和安全审计
		sysExportTemplateRouter.POST("createSysExportTemplate", exportTemplateApi.CreateSysExportTemplate)             // 新建导出模板 - 重要操作，需要审计
		sysExportTemplateRouter.DELETE("deleteSysExportTemplate", exportTemplateApi.DeleteSysExportTemplate)           // 删除导出模板 - 危险操作，必须完整记录
		sysExportTemplateRouter.DELETE("deleteSysExportTemplateByIds", exportTemplateApi.DeleteSysExportTemplateByIds) // 批量删除导出模板 - 批量操作，需要审计
		sysExportTemplateRouter.PUT("updateSysExportTemplate", exportTemplateApi.UpdateSysExportTemplate)              // 更新导出模板 - 数据修改，需要可追溯
		sysExportTemplateRouter.POST("importExcel", exportTemplateApi.ImportExcel)                                     // 导入excel模板数据 - 数据导入操作，需要记录
	}
	{
		// 不需要记录操作的查询类接口
		// 为什么这些接口不记录：
		//   - 查询操作不改变数据状态，属于只读操作
		//   - 查询操作通常非常频繁，记录会产生大量日志
		//   - 查询操作通常不需要审计追踪，只有修改操作才需要
		//   - 导出模板查询在前端页面加载时会被频繁调用，如果都记录会影响性能
		sysExportTemplateRouterWithoutRecord.GET("findSysExportTemplate", exportTemplateApi.FindSysExportTemplate)       // 根据ID获取导出模板 - 查询操作，不记录日志
		sysExportTemplateRouterWithoutRecord.GET("getSysExportTemplateList", exportTemplateApi.GetSysExportTemplateList) // 获取导出模板列表 - 查询操作，不记录日志
		sysExportTemplateRouterWithoutRecord.GET("exportExcel", exportTemplateApi.ExportExcel)                           // 获取导出token - 查询操作，不记录日志（实际是生成token，但属于查询类接口）
		sysExportTemplateRouterWithoutRecord.GET("exportTemplate", exportTemplateApi.ExportTemplate)                     // 导出表格模板 - 查询操作，不记录日志（导出文件属于查询类操作）
		sysExportTemplateRouterWithoutRecord.GET("previewSQL", exportTemplateApi.PreviewSQL)                             // 预览SQL - 查询操作，不记录日志（预览功能属于查询类操作）
	}
	{
		// 不需要认证的公开接口（通过token访问）
		// 为什么这些接口不需要认证：
		//   - 这些接口通过token机制保证安全性，token具有时效性和唯一性
		//   - 适用于异步导出场景：用户触发导出后，系统生成token，用户稍后通过token下载文件
		//   - 支持分享导出链接：用户可以将导出链接分享给他人，无需对方登录系统
		//   - 降低系统负担：导出操作可能耗时较长，通过token机制可以避免长时间占用认证会话
		// 安全性说明：
		//   - 虽然不需要认证，但通过token保证安全性，token包含加密信息
		//   - token通常具有时效性，过期后无法使用
		//   - token具有唯一性，每个导出任务都有独立的token
		sysExportTemplateRouterWithoutAuth.GET("exportExcelByToken", exportTemplateApi.ExportExcelByToken)       // 通过token导出表格 - 公开接口，通过token保证安全性
		sysExportTemplateRouterWithoutAuth.GET("exportTemplateByToken", exportTemplateApi.ExportTemplateByToken) // 通过token导出模板 - 公开接口，通过token保证安全性
	}
}
