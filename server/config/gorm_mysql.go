package config

// Mysql MySQL数据库配置结构体
// 设计模式：通过嵌入GeneralDB实现配置复用，避免重复定义通用字段
// 使用 yaml:",inline" 和 mapstructure:",squash" 标签实现字段扁平化
// 好处：
// 1. 代码复用：通用配置只需在GeneralDB中定义一次
// 2. 配置简洁：YAML配置文件中字段扁平，无需嵌套
// 3. 类型安全：通过结构体定义确保配置项类型正确
type Mysql struct {
	GeneralDB `yaml:",inline" mapstructure:",squash"`
}

// Dsn 生成MySQL数据库连接字符串（Data Source Name）
// 格式：username:password@tcp(host:port)/dbname?config
// 设计目的：
// 1. 统一接口：实现DsnProvider接口，提供统一的DSN生成方法
// 2. 动态构建：根据配置动态生成连接字符串，支持灵活配置
// 3. 参数扩展：通过Config字段支持额外的连接参数（如charset、timezone等）
// 示例：root:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local
func (m *Mysql) Dsn() string {
	return m.Username + ":" + m.Password + "@tcp(" + m.Path + ":" + m.Port + ")/" + m.Dbname + "?" + m.Config
}
