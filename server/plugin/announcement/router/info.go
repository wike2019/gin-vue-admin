package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// Info 是公告路由的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：确保整个插件只有一个路由实例，便于统一管理
var Info = new(info)

// info 是公告相关的路由组
// 设计模式：路由模式（Router Pattern）
// 好处：
// 1. 将路由配置集中管理，便于维护和查看所有 API 端点
// 2. 通过空结构体实现，节省内存
type info struct{}

// Init 初始化公告路由信息
// 设计模式：依赖注入模式（Dependency Injection Pattern） + 路由分组模式
// 好处：
// 1. 接收外部的 public 和 private 路由组，实现路由的灵活挂载
// 2. 区分公开路由和私有路由，实现权限控制
// 3. 通过中间件链式调用，实现横切关注点（如日志、权限等）
// 4. 使用代码块组织路由，提高代码可读性
// @param public *gin.RouterGroup 公开路由组，不需要鉴权的接口挂载在此
// @param private *gin.RouterGroup 私有路由组，需要鉴权的接口挂载在此
func (r *info) Init(public *gin.RouterGroup, private *gin.RouterGroup) {
	// 第一组：需要操作记录中间件的写操作路由
	// 设计模式：中间件模式（Middleware Pattern）
	// 好处：
	// 1. 自动记录所有写操作（创建、删除、更新）的日志
	// 2. 便于审计和问题追踪
	// 3. 中间件模式实现横切关注点，不影响业务代码
	{
		group := private.Group("info").Use(middleware.OperationRecord())
		group.POST("createInfo", apiInfo.CreateInfo)             // 新建公告
		group.DELETE("deleteInfo", apiInfo.DeleteInfo)           // 删除公告
		group.DELETE("deleteInfoByIds", apiInfo.DeleteInfoByIds) // 批量删除公告
		group.PUT("updateInfo", apiInfo.UpdateInfo)              // 更新公告
	}

	// 第二组：不需要操作记录中间件的读操作路由
	// 好处：
	// 1. 读操作不需要记录操作日志，减少日志量
	// 2. 提高性能，减少不必要的中间件开销
	// 3. 仍然需要鉴权，保证数据安全
	{
		group := private.Group("info")
		group.GET("findInfo", apiInfo.FindInfo)       // 根据ID获取公告
		group.GET("getInfoList", apiInfo.GetInfoList) // 获取公告列表
	}

	// 第三组：公开路由，不需要鉴权
	// 好处：
	// 1. 支持 C 端（客户端）访问，无需登录
	// 2. 适用于公开信息展示，如公告列表、数据源等
	// 3. 提高用户体验，降低访问门槛
	{
		group := public.Group("info")
		group.GET("getInfoDataSource", apiInfo.GetInfoDataSource) // 获取公告数据源
		group.GET("getInfoPublic", apiInfo.GetInfoPublic)         // 获取公告列表（公开）
	}
}
