package config

// Captcha 验证码配置结构体
// 验证码用于防止自动化攻击（如暴力破解、爬虫、刷接口等）
// 设计目的：
// 1. 安全防护：防止暴力破解密码、恶意注册、刷接口等攻击
// 2. 用户体验：通过智能触发机制，在保证安全的同时减少对正常用户的影响
// 3. 灵活配置：可根据业务需求调整验证码的触发条件和样式
type Captcha struct {
	// KeyLong 验证码字符长度：验证码中包含的字符数量
	// 范围：通常为4-6位
	// 平衡考虑：
	// - 长度越长越安全，但用户体验越差
	// - 长度越短越方便，但安全性降低
	// 推荐：4-5位，在安全性和易用性之间取得平衡
	KeyLong int `mapstructure:"key-long" json:"key-long" yaml:"key-long"`
	// ImgWidth 验证码图片宽度：验证码图片的像素宽度
	// 设计目的：控制验证码图片大小，影响显示效果和加载速度
	// 推荐值：根据字符长度调整，通常为字符数*30-40像素
	ImgWidth int `mapstructure:"img-width" json:"img-width" yaml:"img-width"`
	// ImgHeight 验证码图片高度：验证码图片的像素高度
	// 设计目的：控制验证码图片大小，保证字符清晰可辨
	// 推荐值：通常为40-50像素
	ImgHeight int `mapstructure:"img-height" json:"img-height" yaml:"img-height"`
	// OpenCaptcha 验证码触发阈值：触发验证码显示的错误次数
	// 规则：
	// - 0：每次登录都需要验证码（最严格，适合高安全场景）
	// - N（N>0）：错误N次后显示验证码（智能触发，平衡安全性和用户体验）
	// 示例：
	// - 3：密码错误3次后显示验证码
	// - 5：密码错误5次后显示验证码
	// 设计优势：
	// 1. 智能触发：正常用户很少输错密码，不会触发验证码
	// 2. 自动防护：攻击者多次尝试时自动触发验证码，阻止暴力破解
	// 3. 用户体验：减少对正常用户的干扰，提升使用体验
	OpenCaptcha int `mapstructure:"open-captcha" json:"open-captcha" yaml:"open-captcha"`
	// OpenCaptchaTimeOut 验证码超时时间：验证码的有效期（单位：秒）
	// 设计目的：
	// 1. 安全考虑：验证码过期后失效，防止验证码被长期使用
	// 2. 防止重放：限制验证码的使用时间窗口，减少重放攻击风险
	// 3. 用户体验：给用户足够的时间输入验证码，但不会过长
	// 推荐值：60-300秒（1-5分钟），根据业务需求调整
	// 注意：超时后需要重新获取验证码
	OpenCaptchaTimeOut int `mapstructure:"open-captcha-timeout" json:"open-captcha-timeout" yaml:"open-captcha-timeout"`
}
