package api

// ApiGroup 是 API 层的分组结构体
// 设计模式：组合模式（Composition Pattern）
// 好处：
// 1. 将相关的 API 控制器组织在一起，便于管理和维护
// 2. 通过结构体组合，可以轻松扩展新的 API 控制器（如添加 SmsApi、WechatApi 等）
// 3. 统一入口，所有 API 控制器通过 ApiGroupApp 访问，避免分散的全局变量
// 4. 符合单一职责原则，每个 API 控制器只负责自己的业务逻辑
type ApiGroup struct {
	EmailApi // 邮件相关的 API 控制器
}

// ApiGroupApp 是 API 分组的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：
// 1. 确保整个应用只有一个 API 分组实例，避免重复创建
// 2. 通过 new() 创建，保证在包加载时初始化，线程安全
// 3. 方便其他包通过 api.ApiGroupApp 访问所有 API 控制器
var ApiGroupApp = new(ApiGroup)
