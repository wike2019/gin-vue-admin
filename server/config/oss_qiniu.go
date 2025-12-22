package config

// Qiniu 七牛云对象存储配置结构体
// 七牛云是国内领先的云存储服务商，提供对象存储和CDN加速服务
// 适用场景：
// 1. 图片存储：特别适合图片、视频等多媒体文件存储
// 2. CDN加速：内置CDN功能，自动实现全球加速
// 3. 数据处理：支持图片处理、视频转码等增值服务
// 设计优势：
// 1. 国内优化：针对国内网络环境优化，访问速度快
// 2. 价格优势：存储和流量价格相对较低
// 3. 功能丰富：提供图片处理、视频处理等增值功能
type Qiniu struct {
	// Zone 存储区域：七牛云存储空间所在的地理区域
	// 可选值：
	// - "z0"：华东（默认）
	// - "z1"：华北
	// - "z2"：华南
	// - "na0"：北美
	// - "as0"：东南亚
	// 设计目的：选择离用户最近的区域，提高访问速度
	// 注意：区域选择后不能修改，需要重新创建空间
	Zone string `mapstructure:"zone" json:"zone" yaml:"zone"`
	// Bucket 存储空间名称：七牛云中存储文件的容器名称
	// 命名规则：全局唯一，只能包含小写字母、数字和短横线
	Bucket string `mapstructure:"bucket" json:"bucket" yaml:"bucket"`
	// ImgPath CDN加速域名：用于访问文件的CDN域名
	// 格式：完整的CDN域名，例如 "https://cdn.example.com"
	// 设计目的：
	// 1. CDN加速：通过CDN加速文件访问，提高用户体验
	// 2. 域名自定义：可以使用自定义域名，提升品牌形象
	// 3. HTTPS支持：支持HTTPS访问，保障传输安全
	ImgPath string `mapstructure:"img-path" json:"img-path" yaml:"img-path"`
	// AccessKey 访问密钥AK：七牛云账号的访问密钥ID
	// 安全要求：
	// 1. 使用子账号密钥，遵循最小权限原则
	// 2. 定期轮换密钥
	// 3. 不要提交到代码仓库
	AccessKey string `mapstructure:"access-key" json:"access-key" yaml:"access-key"`
	// SecretKey 访问密钥SK：与AccessKey配对的密钥
	// 安全要求：必须严格保密，与AccessKey配对使用
	SecretKey string `mapstructure:"secret-key" json:"secret-key" yaml:"secret-key"`
	// UseHTTPS 是否使用HTTPS：控制文件访问是否使用HTTPS协议
	// true：使用HTTPS，保障传输安全，推荐生产环境使用
	// false：使用HTTP，仅用于开发测试
	// 设计目的：保障文件传输安全，防止内容被窃听或篡改
	UseHTTPS bool `mapstructure:"use-https" json:"use-https" yaml:"use-https"`
	// UseCdnDomains 是否使用CDN上传加速：上传文件时是否通过CDN节点上传
	// true：通过CDN节点上传，上传速度更快（推荐）
	// false：直接上传到源站，上传速度可能较慢
	// 设计目的：
	// 1. 提高上传速度：CDN节点通常离用户更近，上传更快
	// 2. 负载均衡：分散上传流量，减轻源站压力
	// 3. 用户体验：特别是在上传大文件时，体验提升明显
	UseCdnDomains bool `mapstructure:"use-cdn-domains" json:"use-cdn-domains" yaml:"use-cdn-domains"`
}
