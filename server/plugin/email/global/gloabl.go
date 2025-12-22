package global

import "github.com/flipped-aurora/gin-vue-admin/server/plugin/email/config"

// GlobalConfig 是邮件插件的全局配置实例
// 设计模式：单例模式（Singleton Pattern） + 全局变量模式
// 好处：
// 1. 确保整个插件只有一个配置实例，避免配置不一致
// 2. 通过 new() 创建，保证在包加载时初始化，线程安全
// 3. 全局可访问，工具层、服务层等都可以直接使用，无需传递参数
// 4. 便于配置热更新，只需更新这一个实例，所有使用配置的地方都会生效
// 5. 符合插件化设计，每个插件独立管理自己的配置
//
// 使用场景：
// - 在插件初始化时从配置文件加载配置到此实例
// - 在工具层、服务层等需要读取配置时直接使用此实例
//
// 注意：配置应该在插件初始化时加载，确保在使用前已正确初始化
var GlobalConfig = new(config.Email)
