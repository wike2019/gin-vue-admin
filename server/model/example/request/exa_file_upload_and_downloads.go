package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

// ExaAttachmentCategorySearch Attachment category search request structure
type ExaAttachmentCategorySearch struct {
	ClassId int `json:"classId" form:"classId"`
	request.PageInfo
}
