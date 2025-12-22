package service

// ServiceGroup 是服务层的分组结构体
// 设计模式：组合模式（Composition Pattern）
// 好处：
// 1. 将相关的业务服务组织在一起，便于管理和维护
// 2. 通过结构体组合，可以轻松扩展新的服务（如添加 SmsService、WechatService 等）
// 3. 统一入口，所有服务通过 ServiceGroupApp 访问，避免分散的全局变量
// 4. 符合单一职责原则，每个服务只负责自己的业务逻辑
// 5. 便于依赖注入和单元测试，可以轻松替换服务实现
type ServiceGroup struct {
	EmailService // 邮件相关的业务服务
}

// ServiceGroupApp 是服务分组的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：
// 1. 确保整个应用只有一个服务分组实例，避免重复创建
// 2. 通过 new() 创建，保证在包加载时初始化，线程安全
// 3. 方便其他层（如 API 层）通过 service.ServiceGroupApp 访问所有服务
// 4. 统一的服务访问入口，便于后续实现服务治理（如熔断、限流等）
var ServiceGroupApp = new(ServiceGroup)
