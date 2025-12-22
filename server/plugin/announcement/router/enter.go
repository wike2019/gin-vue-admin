package router

import "github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/api"

// Router 是路由层的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：
// 1. 确保整个插件只有一个路由实例，避免重复创建
// 2. 通过 new() 创建，保证在包加载时初始化，线程安全
// 3. 方便主应用通过 router.Router 统一注册插件的路由
var Router = new(router)

// apiInfo 是 API 层 Info 控制器的引用
// 设计模式：依赖注入模式（Dependency Injection Pattern）
// 好处：
// 1. 在包级别初始化 API 引用，避免每次调用都查找
// 2. 提高性能，减少方法调用链的查找开销
// 3. 代码更清晰，明确显示路由层依赖的 API 控制器
var apiInfo = api.Api.Info

// router 是路由层的分组结构体
// 设计模式：组合模式（Composition Pattern）
// 好处：
// 1. 将相关的路由组织在一起，便于统一管理和注册
// 2. 通过结构体组合，可以轻松扩展新的路由组
// 3. 统一入口，所有路由通过 router.Router 访问
// 4. 符合插件化设计，每个插件可以独立管理自己的路由
type router struct{ Info info }
