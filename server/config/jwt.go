package config

// JWT JSON Web Token配置结构体
// JWT是一种无状态的认证机制，用于在客户端和服务器之间安全地传输信息
// 设计优势：
// 1. 无状态：服务器不需要存储会话信息，便于水平扩展和负载均衡
// 2. 自包含：Token中包含用户信息，减少数据库查询
// 3. 跨域友好：可以在不同域名间传递，适合微服务架构
// 4. 标准化：基于RFC 7519标准，生态成熟，工具支持完善
type JWT struct {
	// SigningKey 签名密钥：用于签名和验证JWT Token的密钥
	// 安全要求：
	// 1. 长度足够：建议至少32字符，使用随机生成的字符串
	// 2. 保密性：生产环境必须保密，不能泄露到代码仓库
	// 3. 定期更换：定期轮换密钥，提高安全性
	// 设计好处：通过配置管理密钥，便于不同环境使用不同密钥
	SigningKey string `mapstructure:"signing-key" json:"signing-key" yaml:"signing-key"`
	// ExpiresTime Token过期时间：Token的有效期，超过此时间需要重新登录
	// 格式：支持时间单位，如 "24h"（24小时）、"7d"（7天）
	// 设计考虑：
	// 1. 安全性：过期时间越短越安全，但用户体验可能受影响
	// 2. 平衡：通常设置为几小时到几天，根据业务需求调整
	// 3. 刷新机制：配合BufferTime实现Token自动刷新，提升用户体验
	ExpiresTime string `mapstructure:"expires-time" json:"expires-time" yaml:"expires-time"`
	// BufferTime 缓冲时间：在Token即将过期前，允许自动刷新Token的时间窗口
	// 例如：ExpiresTime="24h"，BufferTime="1h"，则在Token过期前1小时内可以自动刷新
	// 设计目的：
	// 1. 用户体验：避免用户在操作过程中突然被要求重新登录
	// 2. 无缝刷新：在后台自动刷新Token，用户无感知
	// 3. 减少请求：只在必要时刷新，避免频繁刷新Token
	BufferTime string `mapstructure:"buffer-time" json:"buffer-time" yaml:"buffer-time"`
	// Issuer 签发者：标识JWT的签发方，通常为应用名称或域名
	// 作用：
	// 1. 身份标识：在Token中标识是谁签发的，便于多系统间区分
	// 2. 安全验证：可以验证Token是否来自预期的签发者
	// 3. 日志追踪：在日志中标识Token来源，便于问题排查
	Issuer string `mapstructure:"issuer" json:"issuer" yaml:"issuer"`
}
