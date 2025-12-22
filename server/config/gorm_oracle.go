package config

import (
	"fmt"
	"net"
	"net/url"
)

// Oracle Oracle数据库配置结构体
// Oracle是企业级关系型数据库，功能强大但配置相对复杂
// 设计模式：与其他数据库配置相同，通过嵌入GeneralDB实现配置复用
type Oracle struct {
	GeneralDB `yaml:",inline" mapstructure:",squash"`
}

// Dsn 生成Oracle数据库连接字符串
// 格式：oracle://username:password@host:port/dbname?config
// 设计考虑：
// 1. URL编码：使用url.PathEscape对用户名、密码、数据库名进行编码
//    原因：Oracle连接字符串中可能包含特殊字符（如@、#、$等），需要URL编码避免解析错误
// 2. 主机端口拼接：使用net.JoinHostPort确保主机和端口的正确格式
//    好处：自动处理IPv6地址和端口号的格式，提高兼容性
// 3. 安全性：密码经过URL编码，但建议在生产环境使用更安全的认证方式（如Wallet）
// 4. 参数扩展：通过Config字段支持Oracle特有的连接参数
// 示例：oracle://scott:tiger@localhost:1521/orcl?connect_timeout=10
// 注意：Oracle驱动对特殊字符敏感，必须进行URL编码
func (m *Oracle) Dsn() string {
	// 使用url.PathEscape对敏感字段进行编码，防止特殊字符导致连接失败
	// 例如：如果密码包含@符号，不编码会导致URL解析错误
	dsn := fmt.Sprintf("oracle://%s:%s@%s/%s?%s", 
		url.PathEscape(m.Username), 
		url.PathEscape(m.Password),
		net.JoinHostPort(m.Path, m.Port), // 自动处理IPv4/IPv6和端口格式
		url.PathEscape(m.Dbname), 
		m.Config)
	return dsn
}
