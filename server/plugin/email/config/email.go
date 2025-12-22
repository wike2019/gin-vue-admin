package config

// Email 邮件配置结构体
// 设计模式：配置对象模式（Configuration Object Pattern）
// 好处：
// 1. 将相关配置集中管理，便于维护和修改
// 2. 使用结构体标签支持多种配置格式（mapstructure、json、yaml），提高灵活性
// 3. 类型安全，编译时就能发现配置错误
// 4. 便于配置验证和默认值设置
// 5. 支持配置热更新，无需重启服务
type Email struct {
	// To 收件人邮箱地址，多个以英文逗号分隔
	// 示例：a@qq.com,b@qq.com
	// 注意：正式开发中建议将此作为参数传入，而不是从配置读取
	// 好处：提高灵活性，不同场景可以发送给不同收件人
	To string `mapstructure:"to" json:"to" yaml:"to"`

	// From 发件人邮箱地址
	// 说明：必须与 SMTP 服务器配置的邮箱一致
	// 好处：统一管理发件人，便于邮件识别和过滤
	From string `mapstructure:"from" json:"from" yaml:"from"`

	// Host SMTP 服务器地址
	// 示例：smtp.qq.com（QQ邮箱）、smtp.163.com（网易邮箱）
	// 说明：不同邮箱服务商的 SMTP 地址不同，需要查看对应邮箱的帮助文档
	// 好处：支持多种邮箱服务商，提高兼容性
	Host string `mapstructure:"host" json:"host" yaml:"host"`

	// Secret SMTP 认证密钥
	// 安全建议：
	// 1. 不要使用邮箱登录密码，应该使用专门的 SMTP 授权码
	// 2. 在 QQ 邮箱、163 邮箱等可以申请 SMTP 授权码
	// 3. 定期更换密钥，提高安全性
	// 好处：使用授权码而非密码，即使泄露也不会影响邮箱安全
	Secret string `mapstructure:"secret" json:"secret" yaml:"secret"`

	// Nickname 发件人昵称（可选）
	// 说明：收件人看到的发件人名称，如果不设置则只显示邮箱地址
	// 示例：设置后显示为 "系统管理员 <admin@example.com>"
	// 好处：提高邮件可读性和专业性
	Nickname string `mapstructure:"nickname" json:"nickname" yaml:"nickname"`

	// Port SMTP 服务器端口
	// 常见端口：
	// - 465：SSL/TLS 加密端口（推荐）
	// - 587：STARTTLS 端口
	// - 25：普通端口（不推荐，可能被防火墙拦截）
	// 说明：不同邮箱服务商的端口可能不同，需要查看对应邮箱的帮助文档
	// 好处：支持不同端口配置，适应不同网络环境
	Port int `mapstructure:"port" json:"port" yaml:"port"`

	// IsSSL 是否使用 SSL/TLS 加密连接
	// 说明：
	// - true：使用加密连接（推荐），端口通常是 465
	// - false：使用普通连接（不推荐），端口通常是 25
	// 好处：
	// 1. 加密传输，防止邮件内容被窃听
	// 2. 提高安全性，符合安全最佳实践
	IsSSL bool `mapstructure:"is-ssl" json:"isSSL" yaml:"is-ssl"`

	// IsLoginAuth 是否使用 LOGIN 认证方式
	// 说明：
	// - true：使用 LOGIN 认证（适用于 IBM、微软等邮箱服务器）
	// - false：使用标准的 PLAIN 认证（适用于大多数邮箱服务器，如 QQ、163、Gmail）
	// 好处：
	// 1. 支持非标准认证方式，提高兼容性
	// 2. 兼容更多邮箱服务商
	IsLoginAuth bool `mapstructure:"is-loginauth" json:"is-loginauth" yaml:"is-loginauth"`
}
