package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"time"
)

// SysParamsSearch System parameters search request structure
type SysParamsSearch struct {
	StartCreatedAt *time.Time `json:"startCreatedAt" form:"startCreatedAt"`
	EndCreatedAt   *time.Time `json:"endCreatedAt" form:"endCreatedAt"`
	Name           string     `json:"name" form:"name" `
	Key            string     `json:"key" form:"key" `
	request.PageInfo
}
