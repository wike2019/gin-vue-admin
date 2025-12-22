package config

// AliyunOSS 阿里云对象存储服务（OSS）配置结构体
// 阿里云OSS是阿里云提供的对象存储服务，具有高可用、高扩展、低成本的特点
// 适用场景：
// 1. 大规模文件存储：图片、视频、文档等静态资源
// 2. CDN加速：结合阿里云CDN实现全球加速
// 3. 数据备份：重要数据的云端备份
// 设计优势：
// 1. 高可用：99.995%的服务可用性，数据自动多副本存储
// 2. 无限扩展：存储容量无上限，按需付费
// 3. 安全可靠：支持加密、访问控制、防盗链等安全功能
type AliyunOSS struct {
	// Endpoint OSS服务端点：OSS服务的访问地址
	// 格式：根据Bucket所在区域不同而不同，例如：
	// - 华东1（杭州）：oss-cn-hangzhou.aliyuncs.com
	// - 华北2（北京）：oss-cn-beijing.aliyuncs.com
	// 获取方式：在阿里云OSS控制台查看Bucket的Endpoint
	Endpoint string `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`
	// AccessKeyId 访问密钥ID：阿里云账号的访问密钥ID
	// 安全要求：
	// 1. 不要使用主账号的AccessKey，应创建子账号的AccessKey
	// 2. 遵循最小权限原则，只授予必要的OSS权限
	// 3. 定期轮换密钥，提高安全性
	// 4. 生产环境使用环境变量或密钥管理服务，不要提交到代码仓库
	AccessKeyId string `mapstructure:"access-key-id" json:"access-key-id" yaml:"access-key-id"`
	// AccessKeySecret 访问密钥Secret：与AccessKeyId配对的密钥
	// 安全要求：与AccessKeyId相同，必须严格保密
	AccessKeySecret string `mapstructure:"access-key-secret" json:"access-key-secret" yaml:"access-key-secret"`
	// BucketName 存储桶名称：OSS中存储文件的容器名称
	// 命名规则：
	// 1. 全局唯一：Bucket名称在整个OSS中必须唯一
	// 2. 命名规范：只能包含小写字母、数字和短横线，长度3-63字符
	// 3. 不能修改：创建后不能修改名称
	BucketName string `mapstructure:"bucket-name" json:"bucket-name" yaml:"bucket-name"`
	// BucketUrl 存储桶访问URL：Bucket的公共访问地址或CDN加速地址
	// 格式：https://bucket-name.oss-region.aliyuncs.com 或 CDN域名
	// 设计目的：用于生成文件的公开访问链接
	// 使用场景：
	// - 公共读文件：直接使用此URL访问文件
	// - CDN加速：配置CDN后使用CDN域名，提高访问速度
	BucketUrl string `mapstructure:"bucket-url" json:"bucket-url" yaml:"bucket-url"`
	// BasePath 基础路径：文件在Bucket中的存储路径前缀
	// 格式：路径字符串，例如 "uploads/" 或 "images/2024/"
	// 设计目的：
	// 1. 路径组织：将文件按模块或日期组织到不同目录
	// 2. 权限控制：可以对不同路径设置不同的访问权限
	// 3. 便于管理：通过路径前缀快速定位和管理文件
	// 示例：如果BasePath="uploads/"，文件"image.jpg"的实际存储路径为"uploads/image.jpg"
	BasePath string `mapstructure:"base-path" json:"base-path" yaml:"base-path"`
}
