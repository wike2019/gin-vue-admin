package config

// HuaWeiObs 华为云对象存储服务（OBS）配置结构体
// 华为云OBS是华为云提供的对象存储服务，兼容S3协议
// 适用场景：
// 1. 企业应用：适合企业级应用，特别是对数据安全要求高的场景
// 2. 国内部署：针对国内用户，访问速度快
// 3. 混合云：支持混合云部署方案
// 设计优势：
// 1. 企业级：提供企业级的安全和合规保障
// 2. 高可用：99.995%的服务可用性
// 3. 数据安全：支持加密、访问控制等安全功能
type HuaWeiObs struct {
	// Path 存储路径：文件在Bucket中的存储路径前缀（可选）
	// 格式：路径字符串，例如 "uploads/" 或 "images/"
	// 设计目的：组织文件结构，便于管理
	Path string `mapstructure:"path" json:"path" yaml:"path"`
	// Bucket 存储桶名称：OBS中存储文件的容器名称
	// 命名规则：全局唯一，只能包含小写字母、数字和短横线
	Bucket string `mapstructure:"bucket" json:"bucket" yaml:"bucket"`
	// Endpoint OBS服务端点：OBS服务的访问地址
	// 格式：根据Bucket所在区域不同而不同，例如：
	// - 华北-北京一：obs.cn-north-1.myhuaweicloud.com
	// - 华东-上海一：obs.cn-east-3.myhuaweicloud.com
	// 获取方式：在华为云OBS控制台查看Bucket的Endpoint
	Endpoint string `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`
	// AccessKey 访问密钥AK：华为云账号的访问密钥ID
	// 安全要求：
	// 1. 使用IAM用户密钥，遵循最小权限原则
	// 2. 定期轮换密钥
	// 3. 不要提交到代码仓库
	AccessKey string `mapstructure:"access-key" json:"access-key" yaml:"access-key"`
	// SecretKey 访问密钥SK：与AccessKey配对的密钥
	// 安全要求：必须严格保密
	SecretKey string `mapstructure:"secret-key" json:"secret-key" yaml:"secret-key"`
}
