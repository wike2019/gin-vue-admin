package plugin

import "github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/config"

// Config 插件的全局配置实例
// 设计模式：单例模式（Singleton Pattern） + 配置模式（Configuration Pattern）
// 好处：
// 1. 确保整个插件只有一个配置实例，避免重复创建
// 2. 通过包级别变量导出，便于其他包访问配置
// 3. 配置在插件初始化时从配置文件加载，统一管理
// 4. 支持配置的全局访问，便于业务代码使用
// 注意：配置通过 initialize.Viper() 方法从 config.yaml 中加载
// 配置文件的 key 为 "announcement"，对应 config.yaml 中的：
// announcement:
//   maxSize: 100
//   enabled: true
var Config config.Config
