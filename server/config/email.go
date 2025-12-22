package config

// Email 邮件服务配置结构体
// 用于配置SMTP邮件服务器，支持发送邮件通知、验证码、报告等
// 设计优势：
// 1. 标准化：基于SMTP协议，支持所有主流邮件服务商
// 2. 安全传输：支持SSL/TLS加密，保障邮件传输安全
// 3. 灵活认证：支持多种认证方式，适配不同邮件服务器
// 4. 配置简单：通过配置文件即可完成邮件服务配置
type Email struct {
	// To 收件人：默认收件人邮箱地址，多个收件人用英文逗号分隔
	// 格式：example1@qq.com,example2@qq.com
	// 注意：正式开发中建议将此作为参数传递，而不是使用配置中的默认值
	// 设计目的：提供默认收件人，简化开发测试，但生产环境应通过参数指定
	To string `mapstructure:"to" json:"to" yaml:"to"`
	// From 发件人邮箱：用于发送邮件的邮箱地址
	// 要求：必须是已启用SMTP服务的邮箱，通常需要在邮箱设置中开启SMTP功能
	From string `mapstructure:"from" json:"from" yaml:"from"`
	// Host SMTP服务器地址：邮件服务商的SMTP服务器域名
	// 常见示例：
	// - QQ邮箱：smtp.qq.com
	// - 163邮箱：smtp.163.com
	// - Gmail：smtp.gmail.com
	// - Outlook：smtp-mail.outlook.com
	// 获取方式：在邮箱设置中查看SMTP服务器地址
	Host string `mapstructure:"host" json:"host" yaml:"host"`
	// Secret SMTP认证密钥：用于登录SMTP服务器的密码或授权码
	// 安全建议：
	// 1. 不要使用邮箱登录密码，应使用专门的SMTP授权码
	// 2. 授权码获取：在邮箱设置中生成SMTP授权码
	// 3. 保密性：生产环境应使用环境变量或密钥管理服务，不要提交到代码仓库
	Secret string `mapstructure:"secret" json:"secret" yaml:"secret"`
	// Nickname 发件人昵称：邮件中显示的发件人名称
	// 示例：如果From是"admin@example.com"，Nickname是"系统管理员"
	//       则收件人看到的发件人是"系统管理员 <admin@example.com>"
	Nickname string `mapstructure:"nickname" json:"nickname" yaml:"nickname"`
	// Port SMTP服务器端口：SMTP服务的端口号
	// 常见端口：
	// - 25：标准SMTP端口（通常被ISP屏蔽）
	// - 465：SMTPS端口（SSL加密），最常用
	// - 587：STARTTLS端口（TLS加密），推荐使用
	// 获取方式：在邮箱设置中查看SMTP端口号
	Port int `mapstructure:"port" json:"port" yaml:"port"`
	// IsSSL 是否使用SSL加密：控制是否使用SSL/TLS加密连接
	// true：使用SSL加密（对应465端口）
	// false：使用普通连接或STARTTLS（对应587端口）
	// 设计目的：保障邮件传输安全，防止密码和邮件内容被窃听
	IsSSL bool `mapstructure:"is-ssl" json:"is-ssl" yaml:"is-ssl"`
	// IsLoginAuth 是否使用LoginAuth认证方式
	// true：使用LOGIN认证方式（适用于IBM、微软等企业邮箱）
	// false：使用PLAIN认证方式（适用于大多数邮件服务商）
	// 设计目的：兼容不同邮件服务器的认证方式，提高适配性
	IsLoginAuth bool `mapstructure:"is-loginauth" json:"is-loginauth" yaml:"is-loginauth"`
}
