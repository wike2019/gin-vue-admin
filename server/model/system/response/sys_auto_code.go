package response

// Db Database response structure
type Db struct {
	Database string `json:"database" gorm:"column:database"`
}

// Table Table response structure
type Table struct {
	TableName string `json:"tableName" gorm:"column:table_name"`
}

// Column Column response structure
type Column struct {
	DataType      string `json:"dataType" gorm:"column:data_type"`
	ColumnName    string `json:"columnName" gorm:"column:column_name"`
	DataTypeLong  string `json:"dataTypeLong" gorm:"column:data_type_long"`
	ColumnComment string `json:"columnComment" gorm:"column:column_comment"`
	PrimaryKey    bool   `json:"primaryKey" gorm:"column:primary_key"`
}
