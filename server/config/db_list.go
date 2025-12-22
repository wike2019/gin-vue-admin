package config

import (
	"strings"

	"gorm.io/gorm/logger"
)

// DsnProvider 数据源名称提供者接口
// 设计意义：
// 1. 统一接口：不同数据库（MySQL、PostgreSQL、SQLite等）都实现此接口，提供统一的DSN生成方法
// 2. 多态支持：通过接口实现多态，可以在运行时根据配置选择不同的数据库实现
// 3. 易于扩展：新增数据库类型只需实现此接口，无需修改调用方代码
// 4. 测试友好：可以轻松创建Mock实现进行单元测试
type DsnProvider interface {
	Dsn() string
}

// GeneralDB 通用数据库配置结构体
// 设计优势：
//  1. 配置复用：通过嵌入（embed）机制，MySQL、PostgreSQL等数据库配置可以复用这些通用字段
//  2. 扁平化配置：使用 yaml:",inline" 和 mapstructure:",squash" 标签，将嵌入的字段"压平"到父结构体
//     这样在YAML配置文件中可以直接写字段名，而不需要嵌套层级，保持配置文件的简洁性
//     示例：https://go.dev/play/p/KIcuhqEoxmY
//  3. 统一管理：所有数据库的通用配置（如连接池、日志等）集中管理，避免重复定义
//  4. 类型安全：通过结构体字段定义，确保配置项的类型正确性
type GeneralDB struct {
	Prefix       string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`                         // 数据库表前缀：统一表名前缀，便于区分不同模块或环境
	Port         string `mapstructure:"port" json:"port" yaml:"port"`                               // 数据库端口：数据库服务监听端口
	Config       string `mapstructure:"config" json:"config" yaml:"config"`                         // 高级配置：数据库连接字符串的额外参数，如字符集、时区等
	Dbname       string `mapstructure:"db-name" json:"db-name" yaml:"db-name"`                      // 数据库名：要连接的数据库名称
	Username     string `mapstructure:"username" json:"username" yaml:"username"`                   // 数据库账号：用于身份认证的用户名
	Password     string `mapstructure:"password" json:"password" yaml:"password"`                   // 数据库密码：用于身份认证的密码
	Path         string `mapstructure:"path" json:"path" yaml:"path"`                               // 数据库地址：数据库服务器的主机地址或IP
	Engine       string `mapstructure:"engine" json:"engine" yaml:"engine" default:"InnoDB"`        // 数据库引擎：MySQL的表存储引擎，InnoDB支持事务和外键
	LogMode      string `mapstructure:"log-mode" json:"log-mode" yaml:"log-mode"`                   // GORM日志模式：控制SQL日志输出级别（silent/error/warn/info）
	MaxIdleConns int    `mapstructure:"max-idle-conns" json:"max-idle-conns" yaml:"max-idle-conns"` // 最大空闲连接数：连接池中保持的空闲连接数，减少连接建立开销
	MaxOpenConns int    `mapstructure:"max-open-conns" json:"max-open-conns" yaml:"max-open-conns"` // 最大打开连接数：同时打开的最大连接数，防止连接数过多导致数据库压力
	Singular     bool   `mapstructure:"singular" json:"singular" yaml:"singular"`                   // 禁用复数表名：true时表名使用单数形式，符合Go命名规范
	LogZap       bool   `mapstructure:"log-zap" json:"log-zap" yaml:"log-zap"`                      // 使用Zap日志：将GORM日志输出到Zap日志系统，便于统一日志管理
}

// LogLevel 将字符串日志级别转换为GORM的LogLevel类型
// 设计好处：
// 1. 类型转换：将配置中的字符串转换为GORM需要的类型，提供类型安全
// 2. 默认值处理：当配置值无效时，默认返回Info级别，保证系统正常运行
// 3. 大小写不敏感：使用strings.ToLower统一处理，提高配置的容错性
func (c GeneralDB) LogLevel() logger.LogLevel {
	switch strings.ToLower(c.LogMode) {
	case "silent":
		return logger.Silent // 静默模式：不输出任何日志，适用于生产环境
	case "error":
		return logger.Error // 错误模式：只输出错误日志
	case "warn":
		return logger.Warn // 警告模式：输出警告和错误日志
	case "info":
		return logger.Info // 信息模式：输出所有SQL语句和日志（开发调试用）
	default:
		return logger.Info // 默认使用Info级别，便于开发调试
	}
}

// SpecializedDB 专用数据库配置结构体
// 设计目的：
// 1. 多数据库支持：允许应用同时连接多个数据库，每个数据库可以有独立的配置
// 2. 别名管理：通过AliasName为数据库实例命名，便于在代码中区分不同的数据库连接
// 3. 动态启用/禁用：通过Disable字段可以临时禁用某个数据库连接，无需删除配置
// 4. 类型标识：Type字段标识数据库类型，用于运行时选择合适的驱动和DSN生成逻辑
// 5. 配置继承：通过嵌入GeneralDB，继承所有通用数据库配置，减少重复代码
type SpecializedDB struct {
	Type      string                                  `mapstructure:"type" json:"type" yaml:"type"`                   // 数据库类型：mysql/pgsql/sqlite等，用于选择对应的驱动
	AliasName string                                  `mapstructure:"alias-name" json:"alias-name" yaml:"alias-name"` // 数据库别名：用于在代码中标识不同的数据库实例
	GeneralDB `yaml:",inline" mapstructure:",squash"` // 嵌入通用配置：使用inline和squash标签，将字段扁平化到当前结构体
	Disable   bool                                    `mapstructure:"disable" json:"disable" yaml:"disable"` // 是否禁用：true时跳过该数据库的初始化，便于临时关闭某个数据库连接
}
