package config

// CloudflareR2 Cloudflare R2对象存储配置结构体
// Cloudflare R2是Cloudflare提供的S3兼容对象存储服务
// 设计优势：
// 1. 零出口费用：与Cloudflare CDN集成，无出口流量费用
// 2. S3兼容：完全兼容AWS S3 API，可以无缝迁移
// 3. 全球加速：结合Cloudflare CDN，实现全球加速
// 4. 价格优势：存储价格低，无出口费用，适合高流量场景
// 适用场景：
// 1. 高流量应用：需要大量数据传输的应用
// 2. 全球分发：需要为全球用户提供服务的应用
// 3. 成本优化：希望降低存储和流量成本的应用
type CloudflareR2 struct {
	// Bucket 存储桶名称：R2中存储文件的容器名称
	// 命名规则：遵循S3规范，全局唯一
	Bucket string `mapstructure:"bucket" json:"bucket" yaml:"bucket"`
	// BaseURL 基础访问URL：文件访问的基础URL
	// 格式：完整的URL，通常使用Cloudflare CDN域名
	// 设计目的：用于生成文件的公开访问链接
	// 优势：通过Cloudflare CDN加速，访问速度快
	BaseURL string `mapstructure:"base-url" json:"base-url" yaml:"base-url"`
	// Path 存储路径：文件在Bucket中的存储路径前缀（可选）
	// 格式：路径字符串，例如 "uploads/" 或 "images/"
	// 设计目的：组织文件结构，便于管理
	Path string `mapstructure:"path" json:"path" yaml:"path"`
	// AccountID Cloudflare账号ID：Cloudflare账号的唯一标识
	// 获取方式：在Cloudflare控制台的右侧边栏查看
	// 设计目的：用于API认证和资源标识
	AccountID string `mapstructure:"account-id" json:"account-id" yaml:"account-id"`
	// AccessKeyID 访问密钥ID：R2 API的访问密钥ID
	// 获取方式：在Cloudflare控制台创建R2 API Token
	// 安全要求：
	// 1. 使用最小权限原则，只授予必要的R2权限
	// 2. 定期轮换密钥
	// 3. 不要提交到代码仓库
	AccessKeyID string `mapstructure:"access-key-id" json:"access-key-id" yaml:"access-key-id"`
	// SecretAccessKey 访问密钥Secret：与AccessKeyID配对的密钥
	// 安全要求：必须严格保密，与AccessKeyID配对使用
	SecretAccessKey string `mapstructure:"secret-access-key" json:"secret-access-key" yaml:"secret-access-key"`
}
