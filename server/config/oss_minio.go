package config

// Minio MinIO对象存储配置结构体
// MinIO是高性能、S3兼容的对象存储服务，可以自建或使用MinIO云服务
// 设计优势：
// 1. S3兼容：完全兼容AWS S3 API，可以无缝替换S3
// 2. 高性能：专为高性能设计，适合大规模数据存储
// 3. 开源免费：开源软件，可以自建部署，降低成本
// 4. 分布式：支持分布式部署，实现高可用和扩展性
// 适用场景：
// 1. 自建存储：需要完全控制数据的场景
// 2. 私有云：企业私有云存储方案
// 3. 开发测试：本地搭建S3兼容的存储服务
type Minio struct {
	// Endpoint MinIO服务端点：MinIO服务器的访问地址
	// 格式：host:port，例如 "localhost:9000" 或 "minio.example.com:9000"
	// 设计目的：指定MinIO服务器的地址和端口
	Endpoint string `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`
	// AccessKeyId 访问密钥ID：MinIO账号的访问密钥ID
	// 安全要求：
	// 1. 使用强密码，定期更换
	// 2. 遵循最小权限原则
	// 3. 生产环境使用环境变量管理
	AccessKeyId string `mapstructure:"access-key-id" json:"access-key-id" yaml:"access-key-id"`
	// AccessKeySecret 访问密钥Secret：与AccessKeyId配对的密钥
	// 安全要求：必须严格保密
	AccessKeySecret string `mapstructure:"access-key-secret" json:"access-key-secret" yaml:"access-key-secret"`
	// BucketName 存储桶名称：MinIO中存储文件的容器名称
	// 命名规则：遵循S3规范，全局唯一
	BucketName string `mapstructure:"bucket-name" json:"bucket-name" yaml:"bucket-name"`
	// UseSSL 是否使用SSL：控制是否使用HTTPS连接MinIO
	// true：使用HTTPS，保障传输安全（生产环境推荐）
	// false：使用HTTP，仅用于开发测试或内网环境
	UseSSL bool `mapstructure:"use-ssl" json:"use-ssl" yaml:"use-ssl"`
	// BasePath 基础路径：文件在Bucket中的存储路径前缀
	// 格式：路径字符串，例如 "uploads/" 或 "images/"
	// 设计目的：组织文件结构，便于管理和权限控制
	BasePath string `mapstructure:"base-path" json:"base-path" yaml:"base-path"`
	// BucketUrl 存储桶访问URL：Bucket的公共访问地址
	// 格式：完整的URL，例如 "https://minio.example.com/bucket-name"
	// 设计目的：用于生成文件的公开访问链接
	// 注意：需要配置MinIO的公共访问策略才能通过此URL访问文件
	BucketUrl string `mapstructure:"bucket-url" json:"bucket-url" yaml:"bucket-url"`
}
