package config

// Local 本地文件存储配置结构体
// 本地存储是最简单的文件存储方式，文件直接存储在服务器本地磁盘
// 适用场景：
// 1. 开发测试：快速搭建，无需配置云服务
// 2. 内网环境：无法访问外网或需要数据不出内网
// 3. 小规模应用：文件量小，单机存储足够
// 设计优势：
// 1. 简单易用：无需配置云服务账号和密钥
// 2. 成本低：无需支付云存储费用
// 3. 数据可控：数据完全在本地，便于管理和备份
// 注意事项：
// 1. 扩展性差：单机存储容量有限，难以水平扩展
// 2. 备份困难：需要自行实现备份策略
// 3. 高可用性差：服务器故障可能导致文件丢失
type Local struct {
	// Path 文件访问路径：通过HTTP访问文件的URL路径前缀
	// 格式：相对路径或绝对URL，例如 "/uploads" 或 "https://example.com/uploads"
	// 设计目的：定义文件访问的URL前缀，用于生成文件的访问链接
	// 示例：如果Path="/uploads"，文件"image.jpg"的访问URL为"/uploads/image.jpg"
	Path string `mapstructure:"path" json:"path" yaml:"path"`
	// StorePath 文件存储路径：文件在服务器上的实际存储目录
	// 格式：绝对路径，例如 "/var/www/uploads" 或 "D:\\uploads"
	// 设计目的：指定文件的物理存储位置
	// 注意事项：
	// 1. 确保目录存在且有写权限
	// 2. 建议使用绝对路径，避免相对路径带来的问题
	// 3. 生产环境应确保磁盘空间充足
	StorePath string `mapstructure:"store-path" json:"store-path" yaml:"store-path"`
}
