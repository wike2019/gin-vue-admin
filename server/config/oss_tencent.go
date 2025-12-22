package config

// TencentCOS 腾讯云对象存储（COS）配置结构体
// 腾讯云COS是腾讯云提供的对象存储服务，与AWS S3兼容
// 适用场景：
// 1. 国内应用：针对国内用户，访问速度快
// 2. 微信生态：与微信小程序、公众号等深度集成
// 3. 游戏行业：腾讯云在游戏行业有丰富经验
// 设计优势：
// 1. 国内优化：针对国内网络环境优化，延迟低
// 2. 价格优势：存储和流量价格相对较低
// 3. CDN集成：与腾讯云CDN深度集成，加速效果好
type TencentCOS struct {
	// Bucket 存储桶名称：COS中存储文件的容器名称
	// 命名规则：全局唯一，只能包含小写字母、数字和短横线
	Bucket string `mapstructure:"bucket" json:"bucket" yaml:"bucket"`
	// Region 地域：COS存储桶所在的地域
	// 常见地域：
	// - "ap-beijing"：北京
	// - "ap-shanghai"：上海
	// - "ap-guangzhou"：广州
	// - "ap-chengdu"：成都
	// 设计目的：选择离用户最近的地域，降低延迟和成本
	Region string `mapstructure:"region" json:"region" yaml:"region"`
	// SecretID 访问密钥ID：腾讯云账号的访问密钥ID
	// 安全要求：
	// 1. 使用子账号密钥，遵循最小权限原则
	// 2. 定期轮换密钥
	// 3. 不要提交到代码仓库
	SecretID string `mapstructure:"secret-id" json:"secret-id" yaml:"secret-id"`
	// SecretKey 访问密钥Secret：与SecretID配对的密钥
	// 安全要求：必须严格保密
	SecretKey string `mapstructure:"secret-key" json:"secret-key" yaml:"secret-key"`
	// BaseURL 基础访问URL：文件访问的基础URL
	// 格式：完整的URL，例如 "https://bucket-name.cos.region.myqcloud.com"
	// 设计目的：用于生成文件的公开访问链接
	BaseURL string `mapstructure:"base-url" json:"base-url" yaml:"base-url"`
	// PathPrefix 路径前缀：文件在Bucket中的存储路径前缀
	// 格式：路径字符串，例如 "uploads/" 或 "images/2024/"
	// 设计目的：组织文件结构，便于管理和权限控制
	PathPrefix string `mapstructure:"path-prefix" json:"path-prefix" yaml:"path-prefix"`
}
