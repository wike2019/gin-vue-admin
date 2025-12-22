package config

// Pgsql PostgreSQL数据库配置结构体
// PostgreSQL是功能强大的开源关系型数据库，支持高级特性如JSON、数组、全文搜索等
// 设计模式：与Mysql相同，通过嵌入GeneralDB实现配置复用
type Pgsql struct {
	GeneralDB `yaml:",inline" mapstructure:",squash"`
}

// Dsn 生成PostgreSQL数据库连接字符串
// 格式：host=host user=username password=password dbname=dbname port=port config
// PostgreSQL使用key=value格式的连接字符串，与MySQL的格式不同
// 设计目的：
// 1. 标准格式：遵循PostgreSQL官方连接字符串格式
// 2. 参数支持：通过Config字段支持SSL、时区等高级参数
// 3. 灵活性：支持动态配置，适应不同环境需求
// 示例：host=localhost user=postgres password=secret dbname=mydb port=5432 sslmode=disable
// Author [SliverHorn](https://github.com/SliverHorn)
func (p *Pgsql) Dsn() string {
	return "host=" + p.Path + " user=" + p.Username + " password=" + p.Password + " dbname=" + p.Dbname + " port=" + p.Port + " " + p.Config
}

// LinkDsn 根据指定的数据库名生成连接字符串
// 设计目的：
// 1. 动态连接：在运行时连接到不同的数据库，无需修改配置
// 2. 数据库管理：用于创建、删除数据库等管理操作
// 3. 多租户：支持多租户场景，每个租户使用不同的数据库
// 使用场景：
// - 初始化时连接到postgres系统数据库创建新数据库
// - 数据库迁移时临时连接到目标数据库
// - 多租户系统中根据租户ID动态选择数据库
// Author [SliverHorn](https://github.com/SliverHorn)
func (p *Pgsql) LinkDsn(dbname string) string {
	return "host=" + p.Path + " user=" + p.Username + " password=" + p.Password + " dbname=" + dbname + " port=" + p.Port + " " + p.Config
}
