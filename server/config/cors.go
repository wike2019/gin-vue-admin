package config

// CORS 跨域资源共享（Cross-Origin Resource Sharing）配置
// CORS是浏览器的安全机制，控制哪些外部域名可以访问本服务器的资源
// 设计目的：
// 1. 安全控制：防止恶意网站跨域访问，保护用户数据
// 2. 前端分离：支持前后端分离架构，前端应用可以跨域调用后端API
// 3. 灵活配置：通过白名单机制，精确控制允许的跨域请求
type CORS struct {
	// Mode CORS模式：控制跨域策略的严格程度
	// 可选值：
	// - "allow-all"：允许所有来源（开发环境使用，不安全）
	// - "whitelist"：仅允许白名单中的来源（生产环境推荐）
	// - "same-origin"：仅允许同源请求（最严格）
	Mode string `mapstructure:"mode" json:"mode" yaml:"mode"`
	// Whitelist 白名单列表：允许跨域访问的域名列表
	// 设计目的：精确控制哪些域名可以访问API，提高安全性
	// 使用场景：多个前端应用（Web、移动端H5等）需要访问同一个后端API
	Whitelist []CORSWhitelist `mapstructure:"whitelist" json:"whitelist" yaml:"whitelist"`
}

// CORSWhitelist 单个CORS白名单项配置
// 设计目的：为每个允许的域名配置详细的跨域策略
// 好处：可以针对不同域名设置不同的权限，实现细粒度控制
type CORSWhitelist struct {
	// AllowOrigin 允许的来源：允许跨域访问的域名
	// 格式：协议://域名:端口，例如 "https://example.com:8080"
	// 支持通配符：可以使用 "*" 允许所有来源（不推荐生产环境使用）
	// 示例：
	// - "https://app.example.com"：允许特定域名
	// - "https://*.example.com"：允许example.com的所有子域名
	AllowOrigin string `mapstructure:"allow-origin" json:"allow-origin" yaml:"allow-origin"`
	// AllowMethods 允许的HTTP方法：允许跨域请求使用的HTTP方法
	// 格式：用逗号分隔的方法列表，例如 "GET,POST,PUT,DELETE,OPTIONS"
	// 常用方法：GET（查询）、POST（创建）、PUT（更新）、DELETE（删除）、PATCH（部分更新）
	// 设计目的：限制允许的HTTP方法，减少攻击面
	AllowMethods string `mapstructure:"allow-methods" json:"allow-methods" yaml:"allow-methods"`
	// AllowHeaders 允许的请求头：允许跨域请求携带的HTTP头
	// 格式：用逗号分隔的头名称列表，例如 "Content-Type,Authorization,X-Requested-With"
	// 常见头：
	// - Content-Type：请求体类型（application/json等）
	// - Authorization：认证令牌（Bearer Token等）
	// - X-Requested-With：标识Ajax请求
	// 设计目的：控制哪些自定义头可以发送，防止恶意头注入
	AllowHeaders string `mapstructure:"allow-headers" json:"allow-headers" yaml:"allow-headers"`
	// ExposeHeaders 暴露的响应头：允许前端JavaScript访问的响应头
	// 格式：用逗号分隔的头名称列表
	// 默认情况下，浏览器只暴露基本的响应头（如Content-Type），
	// 通过此配置可以暴露自定义响应头（如X-Total-Count用于分页）
	// 设计目的：允许前端访问服务器返回的元数据，实现更丰富的功能
	ExposeHeaders string `mapstructure:"expose-headers" json:"expose-headers" yaml:"expose-headers"`
	// AllowCredentials 允许携带凭证：是否允许跨域请求携带Cookie、Authorization头等凭证信息
	// true：允许携带凭证，适用于需要身份认证的跨域请求
	// false：不允许携带凭证，更安全但功能受限
	// 安全注意：
	// 1. 当AllowCredentials为true时，AllowOrigin不能使用通配符"*"
	// 2. 必须明确指定允许的域名，否则浏览器会拒绝请求
	// 3. 生产环境应谨慎使用，确保AllowOrigin配置正确
	AllowCredentials bool `mapstructure:"allow-credentials" json:"allow-credentials" yaml:"allow-credentials"`
}
