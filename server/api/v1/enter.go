package v1

import (
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/example"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/system"
)

// ApiGroupApp 是 v1 版本 API 的统一入口点
// 采用单例模式，确保整个应用只有一个 API 组实例
// 好处：
// 1. 统一管理：所有 API 模块通过一个入口访问，便于维护和扩展
// 2. 版本控制：通过 v1 包名明确标识 API 版本，支持未来多版本共存
// 3. 模块化设计：将不同业务模块（system、example）分离，降低耦合度
// 4. 易于路由注册：在 router 层可以统一注册所有 API 组
var ApiGroupApp = new(ApiGroup)

// ApiGroup 是 v1 版本所有 API 模块的聚合结构体
// 设计理念：
// 1. 组合模式：通过嵌入各个子模块的 ApiGroup，实现功能的组合而非继承
// 2. 职责分离：每个子模块（system、example）负责自己的业务领域
// 3. 扩展性强：新增业务模块只需在此结构体中添加新字段，无需修改现有代码
// 4. 类型安全：通过结构体字段访问，编译期就能发现错误，避免运行时错误
type ApiGroup struct {
	SystemApiGroup  system.ApiGroup  // 系统管理相关 API（用户、权限、菜单等）
	ExampleApiGroup example.ApiGroup // 示例业务 API（客户、文件上传等）
}
