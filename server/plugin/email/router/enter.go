package router

// RouterGroup 是路由层的分组结构体
// 设计模式：组合模式（Composition Pattern）
// 好处：
// 1. 将相关的路由组织在一起，便于统一管理和注册
// 2. 通过结构体组合，可以轻松扩展新的路由组（如添加 SmsRouter、WechatRouter 等）
// 3. 统一入口，所有路由通过 RouterGroupApp 访问，保持代码结构清晰
// 4. 符合插件化设计，每个插件可以独立管理自己的路由
type RouterGroup struct {
	EmailRouter // 邮件相关的路由组
}

// RouterGroupApp 是路由分组的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：
// 1. 确保整个应用只有一个路由分组实例，避免重复创建
// 2. 通过 new() 创建，保证在包加载时初始化，线程安全
// 3. 方便主应用通过 router.RouterGroupApp 统一注册所有插件的路由
// 4. 支持插件热插拔，主应用只需调用 RouterGroupApp 的方法即可
var RouterGroupApp = new(RouterGroup)
