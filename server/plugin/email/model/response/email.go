package response

// Email 邮件请求参数结构体
// 设计模式：数据传输对象模式（DTO Pattern）
// 好处：
// 1. 将 API 请求参数封装为结构体，便于参数验证和类型检查
// 2. 使用 JSON 标签，自动处理 JSON 序列化和反序列化
// 3. 清晰的字段定义，提高代码可读性
// 4. 便于添加参数验证规则（如必填、格式验证等）
// 5. 与业务模型分离，API 层和业务层可以有不同的数据结构
type Email struct {
	// To 收件人邮箱地址
	// 说明：可以是一个或多个邮箱，多个以逗号分隔
	// 示例：user@example.com 或 user1@example.com,user2@example.com
	// 好处：支持批量发送，提高使用灵活性
	To string `json:"to"`

	// Subject 邮件主题/标题
	// 说明：邮件的主题行，收件人首先看到的内容
	// 建议：使用简洁明了的主题，提高邮件打开率
	Subject string `json:"subject"`

	// Body 邮件内容
	// 说明：邮件正文，支持 HTML 格式
	// 示例：可以使用 HTML 标签实现富文本邮件
	// 好处：支持富文本格式，可以发送格式化的邮件内容
	Body string `json:"body"`
}
