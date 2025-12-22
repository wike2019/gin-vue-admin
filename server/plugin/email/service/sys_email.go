package service

import (
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email/utils"
)

// EmailService 是邮件相关的业务服务
// 设计模式：服务层模式（Service Layer Pattern）
// 好处：
// 1. 将业务逻辑从 API 层和工具层分离，实现关注点分离
// 2. 业务逻辑集中管理，便于维护和测试
// 3. 可以轻松替换底层实现（如从 SMTP 切换到邮件服务商 API）
// 4. 便于实现业务规则验证、事务管理等复杂逻辑
// 5. 通过空结构体实现，节省内存
type EmailService struct{}

// EmailTest 发送邮件测试
// 设计模式：门面模式（Facade Pattern）
// 好处：
// 1. 封装底层工具类的调用，为上层提供简洁的接口
// 2. 隐藏实现细节，如果底层实现改变，只需修改服务层
// 3. 便于添加业务逻辑，如参数验证、日志记录、重试机制等
// 4. 统一错误处理，可以将底层错误转换为业务错误
// @author: [maplepie](https://github.com/maplepie)
// @function: EmailTest
// @description: 发送邮件测试
// @return: err error
func (e *EmailService) EmailTest() (err error) {
	// 使用固定的测试参数，简化测试流程
	// 好处：测试接口不需要传入参数，降低使用复杂度
	subject := "test"
	body := "test"
	// 调用工具层方法，实现具体的邮件发送逻辑
	// 好处：服务层只负责业务编排，具体实现委托给工具层
	err = utils.EmailTest(subject, body)
	return err
}

// SendEmail 发送邮件
// 设计模式：门面模式（Facade Pattern） + 适配器模式（Adapter Pattern）
// 好处：
// 1. 封装底层工具类的调用，为上层提供统一的接口
// 2. 参数验证和业务规则可以在服务层统一处理
// 3. 如果底层实现改变（如切换邮件服务商），只需修改服务层和工具层
// 4. 便于添加业务逻辑，如发送频率限制、邮件模板渲染等
// 5. 统一错误处理，可以将底层错误转换为用户友好的错误信息
// @author: [maplepie](https://github.com/maplepie)
// @function: SendEmail
// @description: 发送邮件
// @return: err error
// @params to string 	 收件人
// @params subject string   标题（主题）
// @params body  string 	 邮件内容
func (e *EmailService) SendEmail(to, subject, body string) (err error) {
	// 直接调用工具层方法，传入业务参数
	// 好处：
	// 1. 服务层只负责参数传递和业务编排，具体实现委托给工具层
	// 2. 如果需要在发送前添加业务逻辑（如参数验证、频率限制等），可以在此处添加
	// 3. 便于单元测试，可以 mock 工具层方法
	err = utils.Email(to, subject, body)
	return err
}
