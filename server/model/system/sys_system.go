package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/config"
)

// System System configuration structure
type System struct {
	Config config.Server `json:"config"`
}
