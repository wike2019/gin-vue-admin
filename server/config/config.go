package config

// Server 是整个应用的配置结构体，采用扁平化设计，将所有子配置作为字段嵌入
// 这样设计的好处：
// 1. 统一管理：所有配置集中在一个结构体中，便于管理和查找
// 2. 类型安全：通过结构体字段定义，编译时就能发现配置错误
// 3. 易于扩展：新增配置只需添加字段，不影响现有代码
// 4. 多格式支持：通过 mapstructure/json/yaml 标签，支持多种配置文件格式（YAML/JSON）
// 5. 配置验证：可以在结构体层面统一进行配置验证和初始化
type Server struct {
	// JWT 认证配置：用于生成和验证JWT token，实现无状态的身份认证
	JWT JWT `mapstructure:"jwt" json:"jwt" yaml:"jwt"`
	// Zap 日志配置：高性能结构化日志库，支持多种日志级别和输出格式
	Zap Zap `mapstructure:"zap" json:"zap" yaml:"zap"`
	// Redis 单实例配置：用于缓存、会话存储等场景
	Redis Redis `mapstructure:"redis" json:"redis" yaml:"redis"`
	// RedisList 多实例配置：支持连接多个Redis实例，适用于分布式缓存场景
	RedisList []Redis `mapstructure:"redis-list" json:"redis-list" yaml:"redis-list"`
	// Mongo MongoDB配置：NoSQL数据库，适用于文档存储场景
	Mongo Mongo `mapstructure:"mongo" json:"mongo" yaml:"mongo"`
	// Email 邮件服务配置：用于发送邮件通知、验证码等
	Email Email `mapstructure:"email" json:"email" yaml:"email"`
	// System 系统级配置：应用的基础配置，如数据库类型、端口等
	System System `mapstructure:"system" json:"system" yaml:"system"`
	// Captcha 验证码配置：防止暴力破解和自动化攻击
	Captcha Captcha `mapstructure:"captcha" json:"captcha" yaml:"captcha"`
	// AutoCode 代码生成配置：自动生成CRUD代码，提高开发效率
	AutoCode Autocode `mapstructure:"autocode" json:"autocode" yaml:"autocode"`
	// 数据库配置：支持多种数据库，通过嵌入GeneralDB实现配置复用
	Mysql  Mysql           `mapstructure:"mysql" json:"mysql" yaml:"mysql"`   // MySQL数据库配置
	Mssql  Mssql           `mapstructure:"mssql" json:"mssql" yaml:"mssql"`   // SQL Server数据库配置
	Pgsql  Pgsql           `mapstructure:"pgsql" json:"pgsql" yaml:"pgsql"`   // PostgreSQL数据库配置
	Oracle Oracle          `mapstructure:"oracle" json:"oracle" yaml:"oracle"` // Oracle数据库配置
	Sqlite Sqlite          `mapstructure:"sqlite" json:"sqlite" yaml:"sqlite"` // SQLite数据库配置（轻量级，适合开发测试）
	DBList []SpecializedDB `mapstructure:"db-list" json:"db-list" yaml:"db-list"` // 多数据库列表配置，支持同时连接多个数据库
	// OSS对象存储配置：支持多种云存储服务，实现文件存储的抽象化
	Local        Local        `mapstructure:"local" json:"local" yaml:"local"`               // 本地文件存储
	Qiniu        Qiniu        `mapstructure:"qiniu" json:"qiniu" yaml:"qiniu"`               // 七牛云存储
	AliyunOSS    AliyunOSS    `mapstructure:"aliyun-oss" json:"aliyun-oss" yaml:"aliyun-oss"` // 阿里云OSS
	HuaWeiObs    HuaWeiObs    `mapstructure:"hua-wei-obs" json:"hua-wei-obs" yaml:"hua-wei-obs"` // 华为云OBS
	TencentCOS   TencentCOS   `mapstructure:"tencent-cos" json:"tencent-cos" yaml:"tencent-cos"` // 腾讯云COS
	AwsS3        AwsS3        `mapstructure:"aws-s3" json:"aws-s3" yaml:"aws-s3"`             // AWS S3
	CloudflareR2 CloudflareR2 `mapstructure:"cloudflare-r2" json:"cloudflare-r2" yaml:"cloudflare-r2"` // Cloudflare R2
	Minio        Minio        `mapstructure:"minio" json:"minio" yaml:"minio"`               // MinIO（S3兼容的对象存储）
	// Excel Excel文件处理配置：用于导入导出Excel文件
	Excel Excel `mapstructure:"excel" json:"excel" yaml:"excel"`
	// DiskList 磁盘监控配置列表：监控多个磁盘的挂载点和使用情况
	DiskList []DiskList `mapstructure:"disk-list" json:"disk-list" yaml:"disk-list"`
	// Cors 跨域资源共享配置：控制浏览器跨域请求，保障前端应用安全访问后端API
	Cors CORS `mapstructure:"cors" json:"cors" yaml:"cors"`
	// MCP Model Context Protocol配置：用于AI模型上下文协议，支持AI功能集成
	MCP MCP `mapstructure:"mcp" json:"mcp" yaml:"mcp"`
}
