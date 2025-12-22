package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

// SysOperationRecordSearch System operation record search request structure
type SysOperationRecordSearch struct {
	system.SysOperationRecord
	request.PageInfo
}
