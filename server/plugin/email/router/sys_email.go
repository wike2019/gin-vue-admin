package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email/api"
	"github.com/gin-gonic/gin"
)

// EmailRouter 是邮件相关的路由组
// 设计模式：路由模式（Router Pattern）
// 好处：
// 1. 将路由配置集中管理，便于维护和查看所有 API 端点
// 2. 通过空结构体实现，节省内存
// 3. 方法接收者为指针类型，便于后续扩展
type EmailRouter struct{}

// InitEmailRouter 初始化邮件路由
// 设计模式：依赖注入模式（Dependency Injection Pattern）
// 好处：
// 1. 接收外部的 RouterGroup 参数，而不是自己创建，提高灵活性
// 2. 可以挂载到任意路由组下，支持路由前缀和版本控制
// 3. 通过中间件链式调用，实现横切关注点（如日志、权限、限流等）
// 4. 将 API 方法赋值给变量，便于统一管理和调用
// @param Router *gin.RouterGroup 父路由组，邮件路由将挂载到此路由组下
func (s *EmailRouter) InitEmailRouter(Router *gin.RouterGroup) {
	// 应用操作记录中间件，自动记录所有请求的操作日志
	// 好处：
	// 1. 统一记录操作日志，便于审计和问题追踪
	// 2. 中间件模式实现横切关注点，不影响业务代码
	// 3. 可以灵活添加或移除中间件，如权限验证、限流等
	emailRouter := Router.Use(middleware.OperationRecord())

	// 将 API 方法赋值给变量，便于路由注册
	// 好处：
	// 1. 代码更清晰，路由定义和 API 方法分离
	// 2. 便于统一管理和修改路由配置
	// 3. 支持函数式编程风格，提高代码可读性
	EmailApi := api.ApiGroupApp.EmailApi.EmailTest
	SendEmail := api.ApiGroupApp.EmailApi.SendEmail

	// 使用代码块组织路由，提高代码可读性
	// 好处：
	// 1. 代码块内的变量作用域清晰，避免变量污染
	// 2. 视觉上更清晰，便于识别路由组
	// 3. 便于添加路由级别的中间件或配置
	{
		emailRouter.POST("emailTest", EmailApi)  // 发送测试邮件
		emailRouter.POST("sendEmail", SendEmail) // 发送邮件
	}
}
