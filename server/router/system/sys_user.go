package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

// UserRouter 用户路由结构体
// 为什么使用结构体：遵循面向对象设计，将路由初始化逻辑封装在结构体方法中
// 好处：1. 代码组织更清晰 2. 易于扩展和维护 3. 符合 Go 的最佳实践
type UserRouter struct{}

// InitUserRouter 初始化用户相关路由
// 参数 Router：路由组，通常是 "/api/v1" 下的子路由组
// 为什么这么写：
//  1. 将用户相关的所有路由集中管理，便于维护和查找
//  2. 通过路由分组，统一添加中间件，避免重复代码
//  3. 区分需要记录操作和不需要记录操作的路由，优化性能和存储
func (s *UserRouter) InitUserRouter(Router *gin.RouterGroup) {
	// 创建需要记录操作的路由组
	// 为什么使用 OperationRecord 中间件：
	//   - 这些路由都是修改类操作（增删改），需要审计追踪
	//   - 记录操作日志便于问题排查、安全监控、合规审计
	// 好处：
	//   1. 可以追踪谁在什么时候做了什么操作
	//   2. 出现问题时可以根据日志快速定位和复现
	//   3. 满足安全审计和合规要求
	userRouter := Router.Group("user").Use(middleware.OperationRecord())

	// 创建不需要记录操作的路由组
	// 为什么单独创建一个路由组：
	//   - 查询类操作通常频繁且不需要审计记录
	//   - 避免产生大量无意义的日志，节省存储空间
	//   - 减少数据库写入操作，提高系统性能
	// 好处：
	//   1. 性能优化：查询操作不记录日志，减少系统负担
	//   2. 存储优化：避免查询日志占用大量存储空间
	//   3. 日志清晰：只记录关键操作，便于后续分析和排查
	userRouterWithoutRecord := Router.Group("user")

	{
		// 需要记录操作的修改类接口
		// 为什么这些接口需要记录：
		//   - 都是数据修改操作，涉及数据安全性和完整性
		//   - 需要追踪操作历史和责任人
		userRouter.POST("admin_register", baseApi.Register)               // 管理员注册账号 - 重要操作，需要审计
		userRouter.POST("changePassword", baseApi.ChangePassword)         // 用户修改密码 - 安全相关操作，必须记录
		userRouter.POST("setUserAuthority", baseApi.SetUserAuthority)     // 设置用户权限 - 权限变更需要审计追踪
		userRouter.DELETE("deleteUser", baseApi.DeleteUser)               // 删除用户 - 危险操作，必须完整记录
		userRouter.PUT("setUserInfo", baseApi.SetUserInfo)                // 设置用户信息 - 数据修改，需要可追溯
		userRouter.PUT("setSelfInfo", baseApi.SetSelfInfo)                // 设置自身信息 - 虽然是自己修改，但仍需记录
		userRouter.POST("setUserAuthorities", baseApi.SetUserAuthorities) // 设置用户权限组 - 权限批量变更，重要操作
		userRouter.POST("resetPassword", baseApi.ResetPassword)           // 重置用户密码 - 安全敏感操作，必须记录
		userRouter.PUT("setSelfSetting", baseApi.SetSelfSetting)          // 用户界面配置 - 配置变更，记录便于问题排查
	}
	{
		// 不需要记录操作的查询类接口
		// 为什么这些接口不记录：
		//   - 查询操作不改变数据状态，属于只读操作
		//   - 查询操作通常非常频繁，记录会产生大量日志
		//   - 查询操作通常不需要审计追踪
		userRouterWithoutRecord.POST("getUserList", baseApi.GetUserList) // 分页获取用户列表 - 查询操作，不记录日志
		userRouterWithoutRecord.GET("getUserInfo", baseApi.GetUserInfo)  // 获取自身信息 - 查询操作，不记录日志
	}
}
