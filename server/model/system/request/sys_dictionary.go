package request

// SysDictionarySearch System dictionary search request structure
type SysDictionarySearch struct {
	Name string `json:"name" form:"name" gorm:"column:name;comment:字典名（中）"` // 字典名（中）
}

// ImportSysDictionaryRequest Import system dictionary request structure
type ImportSysDictionaryRequest struct {
	Json string `json:"json" binding:"required"` // JSON字符串
}
