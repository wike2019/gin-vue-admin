package common

// ClearDB Database table cleanup configuration structure
type ClearDB struct {
	TableName    string
	CompareField string
	Interval     string
}
