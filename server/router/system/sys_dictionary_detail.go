package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// DictionaryDetailRouter 字典详情管理路由结构体
//
// 设计说明：
// 1. 为什么使用结构体？
//   - 遵循面向对象设计原则，将路由初始化逻辑封装在结构体方法中
//   - 便于代码组织和维护，字典详情管理相关的所有路由都有独立的结构体
//   - 符合 Go 语言的最佳实践，提高代码的可读性和可维护性
//   - 便于后续扩展，如果需要添加路由相关的配置或状态，可以直接在结构体中添加字段
//
// 2. 好处：
//   - 代码结构清晰：字典详情管理相关的所有路由定义独立，便于查找和维护
//   - 易于扩展：未来如果需要添加字典详情管理相关的其他路由，只需在此方法中扩展
//   - 统一管理：所有字典详情管理相关的路由都在一个地方，避免路由分散
//   - 符合单一职责原则：每个路由结构体只负责一个功能模块的路由定义
type DictionaryDetailRouter struct{}

// InitSysDictionaryDetailRouter 初始化字典详情管理路由信息
//
// 设计说明：
// 1. 路由分组设计：
//   - 使用 Router.Group("sysDictionaryDetail") 创建路由组，统一管理字典详情相关的所有接口
//   - 路由组路径为 "sysDictionaryDetail"，访问路径为 /api/v1/sysDictionaryDetail/xxx
//   - 好处：所有字典详情相关的接口都有统一的前缀，便于管理和识别
//
// 2. 为什么区分两个不同的路由组？
//   - dictionaryDetailRouter：需要记录操作日志的路由组，用于修改类操作（创建、删除、更新）
//   - dictionaryDetailRouterWithoutRecord：不需要记录操作日志的路由组，用于查询类操作
//   - 这样设计的原因：
//     a. 修改类操作（创建、删除、更新）涉及数据变更，需要审计追踪，记录操作历史
//     b. 查询类操作不改变数据状态，属于只读操作，通常不需要审计记录
//     c. 查询操作通常非常频繁，如果都记录会产生大量日志，占用存储空间
//     d. 字典详情查询在前端页面加载时会被频繁调用（如获取字典树、根据类型查询等）
//   - 好处：
//     1. 性能优化：查询操作不记录日志，减少系统负担和数据库写入压力
//     2. 存储优化：避免查询日志占用大量存储空间，节省成本
//     3. 日志清晰：只记录关键操作（增删改），便于后续分析和问题排查
//     4. 审计完整：所有数据变更操作都有完整记录，满足安全审计和合规要求
//     5. 响应速度：减少中间件处理时间，提高接口响应速度
//
// 3. 为什么使用 OperationRecord 中间件？
//   - OperationRecord 中间件用于记录操作日志，包括操作人、操作时间、操作内容等
//   - 对于修改类操作（创建、删除、更新），记录操作日志有以下好处：
//     a. 安全审计：可以追踪谁在什么时候做了什么操作，满足合规要求
//     b. 问题排查：出现问题时可以根据日志快速定位和复现
//     c. 责任追溯：可以明确操作责任人，便于问题追责
//     d. 历史记录：保留完整的操作历史，支持数据恢复和回滚
//     e. 数据安全：字典详情是系统的基础配置数据，变更会影响业务逻辑，需要严格管控
//
// 参数说明：
//   - Router: 需要认证的私有路由组，通常是 "/api/v1" 下的子路由组
//     用于需要登录才能访问的接口，保证数据安全性
func (s *DictionaryDetailRouter) InitSysDictionaryDetailRouter(Router *gin.RouterGroup) {
	// 创建需要记录操作的路由组
	// 为什么使用 OperationRecord 中间件：
	//   - 这些路由都是修改类操作（创建、删除、更新），涉及字典详情数据的变更
	//   - 字典详情是系统的基础配置数据，字典详情的变更会影响系统的业务逻辑和功能展示
	//   - 记录操作日志便于问题排查、安全监控、合规审计
	// 好处：
	//   1. 可以追踪谁在什么时候修改了字典详情配置
	//   2. 出现问题时可以根据日志快速定位和复现
	//   3. 满足安全审计和合规要求
	//   4. 支持操作回滚，如果误操作可以快速恢复
	//   5. 字典详情变更可能影响业务逻辑，需要严格管控和追溯
	dictionaryDetailRouter := Router.Group("sysDictionaryDetail").Use(middleware.OperationRecord())

	// 创建不需要记录操作的路由组
	// 为什么单独创建一个路由组：
	//   - 查询类操作通常频繁且不需要审计记录
	//   - 字典详情查询操作（如获取字典树、根据类型查询、根据父级查询等）在前端页面加载时会被频繁调用
	//   - 如果都记录会产生大量无意义的日志，占用存储空间
	//   - 减少数据库写入操作，提高系统性能
	// 好处：
	//   1. 性能优化：查询操作不记录日志，减少系统负担和数据库写入压力
	//   2. 存储优化：避免查询日志占用大量存储空间，节省成本
	//   3. 日志清晰：只记录关键操作，便于后续分析和排查
	//   4. 响应速度：减少中间件处理时间，提高接口响应速度
	dictionaryDetailRouterWithoutRecord := Router.Group("sysDictionaryDetail")

	{
		// 需要记录操作的修改类接口
		// 为什么这些接口需要记录：
		//   - 都是数据修改或管理操作，涉及字典详情数据的变更
		//   - 字典详情的创建、删除、更新属于关键操作，需要追踪操作历史和责任人
		//   - 字典详情是系统的基础配置数据，变更会影响业务逻辑，需要严格管控
		//   - 便于问题排查和安全审计
		dictionaryDetailRouter.POST("createSysDictionaryDetail", dictionaryDetailApi.CreateSysDictionaryDetail)   // 新建字典详情 - 数据创建，需要可追溯
		dictionaryDetailRouter.DELETE("deleteSysDictionaryDetail", dictionaryDetailApi.DeleteSysDictionaryDetail) // 删除字典详情 - 危险操作，必须完整记录
		dictionaryDetailRouter.PUT("updateSysDictionaryDetail", dictionaryDetailApi.UpdateSysDictionaryDetail)    // 更新字典详情 - 数据修改，需要可追溯
	}
	{
		// 不需要记录操作的查询类接口
		// 为什么这些接口不记录：
		//   - 查询操作不改变数据状态，属于只读操作
		//   - 查询操作通常非常频繁，记录会产生大量日志
		//   - 查询操作通常不需要审计追踪，只有修改操作才需要
		//   - 字典详情查询在前端页面加载时会被频繁调用，如果都记录会影响性能
		//   - 这些查询接口主要用于前端展示和数据获取，不涉及数据变更
		dictionaryDetailRouterWithoutRecord.GET("findSysDictionaryDetail", dictionaryDetailApi.FindSysDictionaryDetail)           // 根据ID获取字典详情 - 查询操作，不记录日志
		dictionaryDetailRouterWithoutRecord.GET("getSysDictionaryDetailList", dictionaryDetailApi.GetSysDictionaryDetailList)     // 获取字典详情列表 - 查询操作，不记录日志
		dictionaryDetailRouterWithoutRecord.GET("getDictionaryTreeList", dictionaryDetailApi.GetDictionaryTreeList)               // 获取字典详情树形结构 - 查询操作，不记录日志
		dictionaryDetailRouterWithoutRecord.GET("getDictionaryTreeListByType", dictionaryDetailApi.GetDictionaryTreeListByType)   // 根据字典类型获取字典详情树形结构 - 查询操作，不记录日志
		dictionaryDetailRouterWithoutRecord.GET("getDictionaryDetailsByParent", dictionaryDetailApi.GetDictionaryDetailsByParent) // 根据父级ID获取字典详情 - 查询操作，不记录日志
		dictionaryDetailRouterWithoutRecord.GET("getDictionaryPath", dictionaryDetailApi.GetDictionaryPath)                       // 获取字典详情的完整路径 - 查询操作，不记录日志
	}
}
