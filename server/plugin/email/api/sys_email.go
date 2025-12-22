package api

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	email_response "github.com/flipped-aurora/gin-vue-admin/server/plugin/email/model/response"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// EmailApi 是邮件相关的 API 控制器
// 设计模式：控制器模式（Controller Pattern）
// 好处：
// 1. 将 HTTP 请求处理逻辑与业务逻辑分离，API 层只负责参数验证和响应格式化
// 2. 通过空结构体实现，节省内存（Go 中空结构体不占用内存）
// 3. 方法接收者为指针类型，便于后续扩展和性能优化
// 4. 符合 RESTful API 设计规范，每个方法对应一个 HTTP 端点
type EmailApi struct{}

// EmailTest 发送测试邮件接口
// 设计模式：分层架构（Layered Architecture）
// 好处：
// 1. API 层只负责接收请求和返回响应，业务逻辑委托给 Service 层处理
// 2. 统一的错误处理和日志记录，便于问题追踪和调试
// 3. 使用 Swagger 注解自动生成 API 文档，提高开发效率
// 4. 通过中间件进行权限验证（ApiKeyAuth），保证接口安全性
// @Tags      System
// @Summary   发送测试邮件
// @Security  ApiKeyAuth
// @Produce   application/json
// @Success   200  {string}  string  "{"success":true,"data":{},"msg":"发送成功"}"
// @Router    /email/emailTest [post]
func (s *EmailApi) EmailTest(c *gin.Context) {
	// 调用 Service 层的测试方法，实现业务逻辑与 API 层的解耦
	err := service.ServiceGroupApp.EmailTest()
	if err != nil {
		// 使用结构化日志记录错误，包含完整的错误堆栈信息
		// 好处：便于生产环境问题排查和监控告警
		global.GVA_LOG.Error("发送失败!", zap.Error(err))
		// 统一返回错误响应格式，保持 API 响应的一致性
		response.FailWithMessage("发送失败", c)
		return
	}
	// 统一返回成功响应格式
	response.OkWithMessage("发送成功", c)
}

// SendEmail 发送邮件接口
// 设计模式：数据绑定模式（Data Binding Pattern）
// 好处：
// 1. 使用 ShouldBindJSON 自动将 JSON 请求体解析为结构体，减少手动解析代码
// 2. 自动进行参数验证，如果格式不正确会返回错误，提高代码健壮性
// 3. 类型安全，编译时就能发现类型错误，避免运行时错误
// 4. 参数验证失败时立即返回，避免无效请求进入业务逻辑层
// @Tags      System
// @Summary   发送邮件
// @Security  ApiKeyAuth
// @Produce   application/json
// @Param     data  body      email_response.Email  true  "发送邮件必须的参数"
// @Success   200   {string}  string                "{"success":true,"data":{},"msg":"发送成功"}"
// @Router    /email/sendEmail [post]
func (s *EmailApi) SendEmail(c *gin.Context) {
	// 定义请求参数结构体，用于接收和验证 JSON 数据
	var email email_response.Email
	// 自动将请求体中的 JSON 数据绑定到结构体
	// 好处：如果 JSON 格式不正确或缺少必填字段，会自动返回错误
	err := c.ShouldBindJSON(&email)
	if err != nil {
		// 参数验证失败，直接返回错误信息，不继续处理
		// 好处：快速失败（Fail Fast），避免无效数据进入后续流程
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 调用 Service 层方法，传入解析后的参数
	// 好处：业务逻辑集中在 Service 层，便于单元测试和代码复用
	err = service.ServiceGroupApp.SendEmail(email.To, email.Subject, email.Body)
	if err != nil {
		// 记录详细的错误日志，包含错误堆栈
		global.GVA_LOG.Error("发送失败!", zap.Error(err))
		// 返回用户友好的错误信息（不暴露内部错误细节，保证安全性）
		response.FailWithMessage("发送失败", c)
		return
	}
	response.OkWithMessage("发送成功", c)
}
