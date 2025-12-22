package initialize

import (
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/plugin"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Viper 从配置文件中加载插件配置
// 设计模式：配置加载模式（Configuration Loading Pattern） + 配置映射模式（Configuration Mapping Pattern）
// 好处：
// 1. 从配置文件（config.yaml）中读取插件配置，实现配置与代码分离
// 2. 使用 Viper 的 UnmarshalKey 方法，自动将配置映射到结构体
// 3. 支持配置热更新，无需重启服务
// 4. 统一的错误处理和日志记录，便于问题追踪
// 注意：此方法在 plugin.go 的 Register 方法中被调用（如果插件需要配置）
// 配置文件的 key 为 "announcement"，对应 config.yaml 中的：
// announcement:
//   maxSize: 100
//   enabled: true
func Viper() {
	// 使用 Viper 的 UnmarshalKey 方法从配置文件中读取配置
	// 设计模式：配置映射模式（Configuration Mapping Pattern）
	// 好处：
	// 1. UnmarshalKey("announcement", &plugin.Config) 从配置文件中读取 "announcement" 键下的配置
	// 2. 自动将配置映射到 plugin.Config 结构体，支持嵌套结构
	// 3. 支持多种配置格式（YAML、JSON、TOML等）
	// 4. 如果配置不存在或格式错误，会返回错误
	err := global.GVA_VP.UnmarshalKey("announcement", &plugin.Config)
	
	// 错误处理和日志记录
	// 设计模式：错误包装模式（Error Wrapping Pattern） + 日志记录模式（Logging Pattern）
	// 好处：
	// 1. 使用 errors.Wrap 包装错误，保留原始错误信息和堆栈
	// 2. 使用 zap 记录错误日志，便于问题追踪和调试
	// 3. 使用 fmt.Sprintf("%+v", err) 输出完整的错误堆栈信息
	// 4. 即使配置加载失败，也不会影响主应用启动（只记录日志）
	if err != nil {
		err = errors.Wrap(err, "初始化配置文件失败!")
		zap.L().Error(fmt.Sprintf("%+v", err))
	}
}
