package global

import (
	"time"

	"gorm.io/gorm"
)

// GVA_MODEL 是全局基础模型结构体，定义了所有数据表通用的字段
// 采用组合模式设计，其他业务模型可以通过嵌入此结构体来复用这些通用字段
//
// 设计意义：
// 1. 统一字段定义：避免在每个模型中重复定义 ID、创建时间、更新时间等通用字段
// 2. 代码复用：通过 Go 的结构体嵌入特性，实现字段的自动继承，减少代码冗余
// 3. 维护便利：如果需要修改通用字段的定义或行为，只需在此处修改一次即可
// 4. 一致性保证：确保所有模型都遵循相同的字段命名和类型规范
//
// 使用方式：
//
//	type User struct {
//	    GVA_MODEL  // 嵌入基础模型，自动获得 ID、CreatedAt、UpdatedAt、DeletedAt 字段
//	    Username string
//	    Email    string
//	}
type GVA_MODEL struct {
	// ID 主键ID，使用 uint 类型可以支持更大的数值范围（0 到 2^32-1）
	// gorm:"primarykey" 标签指定该字段为主键，GORM 会自动处理主键约束
	// json:"ID" 标签指定 JSON 序列化时的字段名，使用大写 ID 保持与前端约定一致
	ID uint `gorm:"primarykey" json:"ID"` // 主键ID

	// CreatedAt 记录创建时间，GORM 会在创建记录时自动填充当前时间
	// 使用 time.Time 类型便于进行时间相关的操作和格式化
	// 注意：字段名必须为 CreatedAt，GORM 才能自动识别并处理
	CreatedAt time.Time // 创建时间

	// UpdatedAt 记录最后更新时间，GORM 会在更新记录时自动更新为当前时间
	// 自动时间戳功能可以确保数据变更历史的准确性，无需手动维护
	// 注意：字段名必须为 UpdatedAt，GORM 才能自动识别并处理
	UpdatedAt time.Time // 更新时间

	// DeletedAt 软删除时间戳，实现逻辑删除而非物理删除
	// gorm.DeletedAt 是 GORM 提供的特殊类型，支持软删除功能
	// gorm:"index" 标签为该字段创建索引，提高查询性能（查询未删除记录时常用）
	// json:"-" 标签表示该字段在 JSON 序列化时被忽略，不返回给前端
	//
	// 软删除的好处：
	// 1. 数据安全：删除的数据仍然保留在数据库中，可以恢复
	// 2. 审计追踪：保留删除时间，便于追溯数据变更历史
	// 3. 关联数据保护：避免因物理删除导致的外键约束问题
	// 4. 业务连续性：某些业务场景需要查看历史数据
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 删除时间（软删除）
}
