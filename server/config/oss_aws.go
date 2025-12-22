package config

// AwsS3 AWS S3对象存储配置结构体
// AWS S3是Amazon提供的对象存储服务，是全球最广泛使用的云存储服务
// 设计优势：
// 1. 全球覆盖：AWS在全球多个区域提供服务，可以选择最近的区域
// 2. 高可用性：99.999999999%（11个9）的数据持久性
// 3. 无限扩展：存储容量无上限，按需付费
// 4. 生态丰富：与AWS其他服务深度集成
// 适用场景：
// 1. 全球应用：需要为全球用户提供服务的应用
// 2. 大数据存储：需要存储大量数据的场景
// 3. 企业级应用：对可靠性和合规性要求高的场景
type AwsS3 struct {
	// Bucket 存储桶名称：S3中存储文件的容器名称
	// 命名规则：
	// 1. 全局唯一：Bucket名称在整个S3中必须唯一
	// 2. 命名规范：只能包含小写字母、数字、点和短横线
	// 3. 长度限制：3-63个字符
	Bucket string `mapstructure:"bucket" json:"bucket" yaml:"bucket"`
	// Region AWS区域：S3 Bucket所在的AWS区域
	// 常见区域：
	// - "us-east-1"：美国东部（弗吉尼亚）
	// - "us-west-2"：美国西部（俄勒冈）
	// - "eu-west-1"：欧洲（爱尔兰）
	// - "ap-southeast-1"：亚太（新加坡）
	// 设计目的：选择离用户最近的区域，降低延迟和成本
	Region string `mapstructure:"region" json:"region" yaml:"region"`
	// Endpoint S3服务端点：S3服务的访问地址（可选）
	// 格式：通常为 "s3.region.amazonaws.com" 或自定义端点
	// 使用场景：
	// 1. 自定义端点：使用S3兼容服务（如MinIO）时指定自定义端点
	// 2. 特殊区域：某些特殊区域可能需要指定端点
	// 注意：大多数情况下可以留空，SDK会自动根据Region生成端点
	Endpoint string `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`
	// SecretID AWS访问密钥ID：AWS账号的访问密钥ID
	// 安全要求：
	// 1. 使用IAM用户密钥，不要使用根账号密钥
	// 2. 遵循最小权限原则，只授予必要的S3权限
	// 3. 定期轮换密钥
	// 4. 使用AWS Secrets Manager或环境变量管理密钥
	SecretID string `mapstructure:"secret-id" json:"secret-id" yaml:"secret-id"`
	// SecretKey AWS访问密钥Secret：与SecretID配对的密钥
	// 安全要求：必须严格保密，与SecretID配对使用
	SecretKey string `mapstructure:"secret-key" json:"secret-key" yaml:"secret-key"`
	// BaseURL 基础访问URL：文件访问的基础URL
	// 格式：完整的URL，例如 "https://bucket-name.s3.region.amazonaws.com"
	// 设计目的：用于生成文件的公开访问链接
	// 使用场景：
	// - 公共读文件：直接使用此URL访问
	// - CDN加速：配置CloudFront后使用CDN域名
	BaseURL string `mapstructure:"base-url" json:"base-url" yaml:"base-url"`
	// PathPrefix 路径前缀：文件在Bucket中的存储路径前缀
	// 格式：路径字符串，例如 "uploads/" 或 "images/2024/"
	// 设计目的：组织文件结构，便于管理和权限控制
	PathPrefix string `mapstructure:"path-prefix" json:"path-prefix" yaml:"path-prefix"`
	// S3ForcePathStyle 强制路径样式：控制S3 URL的格式
	// true：使用路径样式，URL格式为 "https://s3.region.amazonaws.com/bucket/key"
	// false：使用虚拟主机样式，URL格式为 "https://bucket.s3.region.amazonaws.com/key"（默认）
	// 使用场景：
	// - S3兼容服务：某些S3兼容服务（如MinIO）可能需要使用路径样式
	// - 自定义域名：使用自定义域名时可能需要路径样式
	S3ForcePathStyle bool `mapstructure:"s3-force-path-style" json:"s3-force-path-style" yaml:"s3-force-path-style"`
	// DisableSSL 禁用SSL：是否禁用HTTPS连接
	// true：使用HTTP连接（不推荐，仅用于开发测试）
	// false：使用HTTPS连接（推荐，保障传输安全）
	// 设计目的：在某些特殊场景（如内网环境）可能需要禁用SSL
	DisableSSL bool `mapstructure:"disable-ssl" json:"disable-ssl" yaml:"disable-ssl"`
}
