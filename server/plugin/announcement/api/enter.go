package api

import "github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/service"

// Api 是 API 层的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：
// 1. 确保整个插件只有一个 API 实例，避免重复创建
// 2. 通过 new() 创建，保证在包加载时初始化，线程安全
// 3. 方便路由层通过 api.Api 访问所有 API 控制器
var Api = new(api)

// serviceInfo 是服务层 Info 服务的引用
// 设计模式：依赖注入模式（Dependency Injection Pattern）
// 好处：
// 1. 在包级别初始化服务引用，避免每次调用都查找
// 2. 提高性能，减少方法调用链的查找开销
// 3. 代码更清晰，明确显示 API 层依赖的服务
var serviceInfo = service.Service.Info

// api 是 API 层的分组结构体
// 设计模式：组合模式（Composition Pattern）
// 好处：
// 1. 将相关的 API 控制器组织在一起，便于管理和维护
// 2. 通过结构体组合，可以轻松扩展新的 API 控制器
// 3. 统一入口，所有 API 控制器通过 api.Api 访问
// 4. 符合单一职责原则，每个 API 控制器只负责自己的业务逻辑
type api struct{ Info info }
