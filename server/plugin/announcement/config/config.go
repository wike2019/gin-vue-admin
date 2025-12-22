package config

// Config 插件配置结构体
// 设计模式：配置模式（Configuration Pattern）
// 好处：
// 1. 将插件配置封装为结构体，便于管理和维护
// 2. 支持从配置文件（如 config.yaml）中读取配置
// 3. 配置与代码分离，便于不同环境使用不同配置
// 4. 支持配置热更新，无需重启服务
// 注意：如果插件不需要配置，可以保持结构体为空
// 如果需要配置，可以在此添加配置字段，如：
// type Config struct {
//     MaxSize int    `mapstructure:"maxSize" json:"maxSize" yaml:"maxSize"`
//     Enabled bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
// }
type Config struct {
}
