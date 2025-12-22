package config

import (
	"fmt"
	"strings"
)

// Mongo MongoDB数据库配置结构体
// MongoDB是NoSQL文档数据库，适合存储非结构化或半结构化数据
// 设计优势：
// 1. 灵活模式：无需预定义表结构，适合快速迭代和变化频繁的业务
// 2. 水平扩展：原生支持分片，可以轻松扩展处理大数据量
// 3. 高可用：支持副本集，提供自动故障转移
// 4. 丰富查询：支持复杂的查询和聚合操作
type Mongo struct {
	// Coll 默认集合名称：MongoDB中集合（Collection）类似于关系数据库中的表
	// 设计目的：为应用指定默认使用的集合，简化代码中的集合名指定
	Coll string `json:"coll" yaml:"coll" mapstructure:"coll"`
	// Options MongoDB连接选项：连接字符串的额外参数
	// 格式：key=value&key2=value2，例如 "authSource=admin&ssl=true"
	// 常用选项：
	// - authSource: 认证数据库
	// - ssl: 是否使用SSL连接
	// - replicaSet: 副本集名称
	// - readPreference: 读偏好（primary/secondary等）
	Options string `json:"options" yaml:"options" mapstructure:"options"`
	// Database 数据库名称：要连接的MongoDB数据库名
	Database string `json:"database" yaml:"database" mapstructure:"database"`
	// Username 用户名：用于MongoDB身份认证
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	// Password 密码：用于MongoDB身份认证
	Password string `json:"password" yaml:"password" mapstructure:"password"`
	// AuthSource 认证数据库：存储用户凭据的数据库
	// 通常为 "admin"，MongoDB的用户信息存储在admin数据库中
	AuthSource string `json:"auth-source" yaml:"auth-source" mapstructure:"auth-source"`
	// MinPoolSize 最小连接池大小：连接池中保持的最小连接数
	// 设计目的：保持一定数量的连接，减少连接建立的开销，提高响应速度
	MinPoolSize uint64 `json:"min-pool-size" yaml:"min-pool-size" mapstructure:"min-pool-size"`
	// MaxPoolSize 最大连接池大小：连接池允许的最大连接数
	// 设计目的：限制并发连接数，防止连接数过多导致MongoDB服务器压力过大
	MaxPoolSize uint64 `json:"max-pool-size" yaml:"max-pool-size" mapstructure:"max-pool-size"`
	// SocketTimeoutMs Socket超时时间（毫秒）：单个操作的超时时间
	// 设计目的：防止长时间等待，超时后返回错误，避免连接挂起
	SocketTimeoutMs int64 `json:"socket-timeout-ms" yaml:"socket-timeout-ms" mapstructure:"socket-timeout-ms"`
	// ConnectTimeoutMs 连接超时时间（毫秒）：建立连接的超时时间
	// 设计目的：如果MongoDB服务器不可达，快速失败，避免长时间阻塞
	ConnectTimeoutMs int64 `json:"connect-timeout-ms" yaml:"connect-timeout-ms" mapstructure:"connect-timeout-ms"`
	// IsZap 是否使用Zap日志：控制MongoDB驱动是否输出日志到Zap
	// 设计目的：统一日志管理，将MongoDB操作日志纳入应用的日志系统
	IsZap bool `json:"is-zap" yaml:"is-zap" mapstructure:"is-zap"`
	// Hosts MongoDB主机列表：支持配置多个MongoDB节点（副本集或分片集群）
	// 设计目的：
	// 1. 高可用：配置多个节点，自动故障转移
	// 2. 负载均衡：客户端可以连接到任意节点
	// 3. 副本集：支持MongoDB副本集架构
	Hosts []*MongoHost `json:"hosts" yaml:"hosts" mapstructure:"hosts"`
}

// MongoHost MongoDB主机配置
// 设计目的：将主机地址和端口分离，便于管理和配置多个节点
type MongoHost struct {
	Host string `json:"host" yaml:"host" mapstructure:"host"` // 主机地址：IP地址或域名
	Port string `json:"port" yaml:"port" mapstructure:"port"` // 端口：MongoDB服务端口，默认27017
}

// Uri 生成MongoDB连接URI
// 格式：mongodb://host1:port1,host2:port2/database?options
// 设计目的：
// 1. 动态构建：根据配置动态生成连接URI，支持多节点配置
// 2. 格式标准：遵循MongoDB官方URI格式规范
// 3. 容错处理：自动过滤无效的主机配置（空主机或端口）
// 4. 选项支持：支持通过Options字段添加连接选项
// 设计好处：
// - 支持单节点和多节点（副本集）配置
// - 自动处理主机列表的拼接
// - 灵活支持连接选项
func (x *Mongo) Uri() string {
	length := len(x.Hosts)
	// 预分配容量，避免多次扩容，提高性能
	hosts := make([]string, 0, length)
	// 遍历主机列表，过滤无效配置并拼接为host:port格式
	for i := 0; i < length; i++ {
		// 只添加有效的主机配置（主机和端口都不为空）
		if x.Hosts[i].Host != "" && x.Hosts[i].Port != "" {
			hosts = append(hosts, x.Hosts[i].Host+":"+x.Hosts[i].Port)
		}
	}
	// 如果有连接选项，添加到URI中
	if x.Options != "" {
		return fmt.Sprintf("mongodb://%s/%s?%s", strings.Join(hosts, ","), x.Database, x.Options)
	}
	// 无选项时使用简化格式
	return fmt.Sprintf("mongodb://%s/%s", strings.Join(hosts, ","), x.Database)
}
