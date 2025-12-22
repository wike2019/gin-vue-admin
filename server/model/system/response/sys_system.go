package response

import "github.com/flipped-aurora/gin-vue-admin/server/config"

// SysConfigResponse System configuration response structure
type SysConfigResponse struct {
	Config config.Server `json:"config"`
}
