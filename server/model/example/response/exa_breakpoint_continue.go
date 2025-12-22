package response

import "github.com/flipped-aurora/gin-vue-admin/server/model/example"

// FilePathResponse File path response structure
type FilePathResponse struct {
	FilePath string `json:"filePath"`
}

// FileResponse File response structure
type FileResponse struct {
	File example.ExaFile `json:"file"`
}
