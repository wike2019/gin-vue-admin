package config

// Mssql Microsoft SQL Server数据库配置结构体
// SQL Server是微软的企业级关系型数据库，常用于Windows环境
// 设计模式：与其他数据库配置相同，通过嵌入GeneralDB实现配置复用
type Mssql struct {
	GeneralDB `yaml:",inline" mapstructure:",squash"`
}

// Dsn 生成SQL Server数据库连接字符串
// 格式：sqlserver://username:password@host:port?database=dbname&encrypt=disable
// 设计说明：
// 1. URL格式：使用URL格式的连接字符串，符合现代数据库驱动的标准
// 2. 加密控制：默认设置encrypt=disable，适用于本地开发或内网环境
//    生产环境如需加密，可通过Config字段添加encrypt=true
// 3. 参数扩展：通过Config字段可以添加其他连接参数
// 示例：sqlserver://sa:password@localhost:1433?database=mydb&encrypt=disable
// 注意：生产环境建议启用加密（encrypt=true）和证书验证，提高安全性
func (m *Mssql) Dsn() string {
	return "sqlserver://" + m.Username + ":" + m.Password + "@" + m.Path + ":" + m.Port + "?database=" + m.Dbname + "&encrypt=disable"
}
