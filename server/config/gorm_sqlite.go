package config

import (
	"path/filepath"
)

// Sqlite SQLite数据库配置结构体
// SQLite是轻量级的嵌入式数据库，数据存储在单个文件中
// 适用场景：
// 1. 开发测试：无需安装数据库服务器，快速搭建环境
// 2. 小型应用：数据量小、并发低的场景
// 3. 移动应用：iOS和Android原生支持SQLite
// 4. 嵌入式系统：资源受限的环境
// 设计优势：
// 1. 零配置：无需安装和配置数据库服务器
// 2. 文件存储：数据库就是单个文件，便于备份和迁移
// 3. 跨平台：数据库文件可以在不同平台间移动
type Sqlite struct {
	GeneralDB `yaml:",inline" mapstructure:",squash"`
}

// Dsn 生成SQLite数据库文件路径
// SQLite不需要网络连接，DSN就是数据库文件的路径
// 设计考虑：
// 1. 路径拼接：使用filepath.Join确保跨平台路径正确（Windows使用\，Unix使用/）
// 2. 文件扩展名：自动添加.db扩展名，符合SQLite约定
// 3. 目录创建：Path字段指定数据库文件所在目录，需要确保目录存在
// 示例：如果Path="/data/db"，Dbname="app"，则返回"/data/db/app.db"
func (s *Sqlite) Dsn() string {
	// filepath.Join会自动处理路径分隔符，确保跨平台兼容性
	return filepath.Join(s.Path, s.Dbname+".db")
}
