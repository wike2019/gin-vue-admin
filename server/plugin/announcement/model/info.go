package model

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// Info 公告数据模型
// 设计模式：领域模型模式（Domain Model Pattern） + GORM 模型模式
// 好处：
// 1. 将业务实体抽象为结构体，便于理解和维护
// 2. 使用 GORM 标签自动处理数据库映射，减少手动 SQL
// 3. 使用 JSON 标签支持序列化和反序列化，便于 API 交互
// 4. 使用 form 标签支持表单绑定，便于处理表单提交
// 5. 嵌入 GVA_MODEL 自动获得 ID、创建时间、更新时间等通用字段
type Info struct {
	// 嵌入 GVA_MODEL，自动获得以下字段：
	// - ID: 主键，自动递增
	// - CreatedAt: 创建时间，自动填充
	// - UpdatedAt: 更新时间，自动更新
	// - DeletedAt: 删除时间（软删除），如果使用软删除
	// 设计模式：组合模式（Composition Pattern）
	// 好处：复用通用字段，避免在每个模型中重复定义
	global.GVA_MODEL
	
	// Title 公告标题
	// 设计模式：结构体标签模式（Struct Tag Pattern）
	// 标签说明：
	// - json:"title": JSON 序列化时的字段名，用于 API 响应
	// - form:"title": 表单绑定时的字段名，用于接收表单数据
	// - gorm:"column:title;comment:公告标题;": GORM 数据库映射
	//   - column:title: 数据库列名
	//   - comment:公告标题: 数据库注释，便于数据库文档生成
	Title string `json:"title" form:"title" gorm:"column:title;comment:公告标题;"`
	
	// Content 公告内容
	// 设计模式：大文本字段模式（Large Text Field Pattern）
	// 好处：
	// 1. 使用 type:text 支持大文本内容，不受 VARCHAR 长度限制
	// 2. 适合存储富文本、长文章等内容
	// 3. 数据库会根据类型选择合适的存储方式
	Content string `json:"content" form:"content" gorm:"column:content;comment:公告内容;type:text;"`
	
	// UserID 发布者用户ID
	// 设计模式：外键关联模式（Foreign Key Pattern） + 指针类型模式
	// 好处：
	// 1. 使用指针类型（*int），支持 NULL 值，表示可选字段
	// 2. 如果用户ID为空，可以表示系统公告或匿名公告
	// 3. 便于后续扩展用户关联查询（如通过 Preload 预加载用户信息）
	// 4. 符合数据库外键设计规范
	UserID *int `json:"userID" form:"userID" gorm:"column:user_id;comment:发布者;"`
	
	// Attachments 相关附件
	// 设计模式：JSON 字段模式（JSON Field Pattern）
	// 好处：
	// 1. 使用 datatypes.JSON 类型，支持存储 JSON 格式的复杂数据
	// 2. 适合存储数组、对象等结构化数据（如附件列表）
	// 3. 使用 swaggertype 标签，便于 Swagger 文档生成
	// 4. 灵活存储附件信息，不受固定字段数量限制
	// 示例数据格式：[{"name":"file1.pdf","url":"/uploads/file1.pdf","size":1024}]
	Attachments datatypes.JSON `json:"attachments" form:"attachments" gorm:"column:attachments;comment:相关附件;" swaggertype:"array,object"`
}

// TableName 自定义表名
// 设计模式：表名映射模式（Table Name Mapping Pattern）
// 好处：
// 1. 统一表名规范，使用 gva_announcements_info 格式
// 2. 避免 GORM 自动生成的表名（如 infos）不符合规范
// 3. 便于数据库管理和维护，表名清晰表达业务含义
// 4. 支持多数据库兼容，不同数据库可能有不同的命名规范
// @return string 返回自定义的表名
func (Info) TableName() string {
	return "gva_announcements_info"
}
