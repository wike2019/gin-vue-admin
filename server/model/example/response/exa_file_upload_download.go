package response

import "github.com/flipped-aurora/gin-vue-admin/server/model/example"

// ExaFileResponse File response structure
type ExaFileResponse struct {
	File example.ExaFileUploadAndDownload `json:"file"`
}
