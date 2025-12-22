package config

// System 系统级配置结构体
// 设计意义：
// 1. 核心配置集中：将应用的核心开关和基础配置集中管理，便于统一控制和查看
// 2. 功能开关：通过布尔字段实现功能开关，可以灵活启用/禁用某些功能模块
// 3. 环境适配：不同环境（开发/测试/生产）可以通过修改这些配置快速切换行为
// 4. 安全控制：包含IP限流、多点登录等安全相关配置，保障系统安全
type System struct {
	// DbType 数据库类型：指定应用使用的主数据库类型
	// 支持：mysql(默认)|sqlite|sqlserver|postgresql|oracle
	// 设计好处：通过配置切换数据库，无需修改代码，提高可移植性
	DbType string `mapstructure:"db-type" json:"db-type" yaml:"db-type"`
	// OssType 对象存储类型：指定文件存储服务类型
	// 支持：local|qiniu|aliyun-oss|hua-wei-obs|tencent-cos|aws-s3|cloudflare-r2|minio
	// 设计好处：通过配置切换存储服务，实现存储层的抽象，便于迁移和扩展
	OssType string `mapstructure:"oss-type" json:"oss-type" yaml:"oss-type"`
	// RouterPrefix 路由前缀：所有API路由的统一前缀
	// 例如："/api/v1"，便于版本控制和路由管理
	// 设计好处：统一管理API版本，支持多版本API共存
	RouterPrefix string `mapstructure:"router-prefix" json:"router-prefix" yaml:"router-prefix"`
	// Addr 服务监听端口：HTTP服务器监听的端口号
	// 设计好处：通过配置指定端口，避免硬编码，便于部署时灵活调整
	Addr int `mapstructure:"addr" json:"addr" yaml:"addr"`
	// LimitCountIP IP限流次数：在LimitTimeIP时间窗口内允许的最大请求次数
	// 设计目的：防止单个IP的恶意请求或DDoS攻击，保护服务器资源
	LimitCountIP int `mapstructure:"iplimit-count" json:"iplimit-count" yaml:"iplimit-count"`
	// LimitTimeIP IP限流时间窗口：限流统计的时间窗口（单位：秒）
	// 设计目的：与LimitCountIP配合，实现滑动窗口限流算法
	LimitTimeIP int `mapstructure:"iplimit-time" json:"iplimit-time" yaml:"iplimit-time"`
	// UseMultipoint 多点登录拦截：是否允许同一账号在多个设备/地点同时登录
	// true：允许多点登录（不拦截）
	// false：只允许单点登录，新登录会踢掉旧会话
	// 设计好处：根据业务需求灵活控制登录策略，提高安全性或用户体验
	UseMultipoint bool `mapstructure:"use-multipoint" json:"use-multipoint" yaml:"use-multipoint"`
	// UseRedis 是否使用Redis：控制是否启用Redis缓存和会话存储
	// 设计好处：可以在没有Redis的环境（如开发环境）中禁用Redis，降低部署复杂度
	UseRedis bool `mapstructure:"use-redis" json:"use-redis" yaml:"use-redis"`
	// UseMongo 是否使用MongoDB：控制是否启用MongoDB数据库
	// 设计好处：按需启用MongoDB，避免不必要的连接和资源消耗
	UseMongo bool `mapstructure:"use-mongo" json:"use-mongo" yaml:"use-mongo"`
	// UseStrictAuth 严格权限模式：是否使用树形角色分配模式
	// true：启用严格的权限继承和树形角色体系，权限控制更精细
	// false：使用简单的角色权限模式
	// 设计好处：根据业务复杂度选择合适的权限模型，平衡安全性和易用性
	UseStrictAuth bool `mapstructure:"use-strict-auth" json:"use-strict-auth" yaml:"use-strict-auth"`
	// DisableAutoMigrate 禁用自动迁移：是否禁用GORM的自动数据库表结构迁移
	// true：禁用自动迁移，需要手动执行数据库迁移脚本（生产环境推荐）
	// false：启动时自动检查并迁移表结构（开发环境推荐）
	// 设计意义：
	// 1. 生产安全：生产环境禁用自动迁移，避免意外修改数据库结构
	// 2. 开发便利：开发环境启用自动迁移，提高开发效率
	// 3. 版本控制：手动迁移可以更好地控制数据库变更，便于版本管理和回滚
	DisableAutoMigrate bool `mapstructure:"disable-auto-migrate" json:"disable-auto-migrate" yaml:"disable-auto-migrate"`
}
