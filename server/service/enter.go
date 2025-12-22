package service

import (
	"github.com/flipped-aurora/gin-vue-admin/server/service/example"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
)

// ServiceGroupApp 是全局服务入口，采用单例模式，在整个应用中只需要一个实例
// 好处：
// 1. 统一访问入口：所有服务都通过 ServiceGroupApp 访问，避免在代码中分散导入各个服务模块
// 2. 资源节约：单例模式确保全局只有一个 ServiceGroup 实例，避免重复创建，节省内存
// 3. 依赖注入友好：为后续可能的依赖注入框架集成提供了良好的基础结构
var ServiceGroupApp = new(ServiceGroup)

// ServiceGroup 采用服务聚合模式（Service Aggregation Pattern），将所有业务模块的服务组合在一起
// 设计意义和好处：
// 1. 模块化组织：每个业务模块（如 system、example）都有自己独立的 ServiceGroup，保持代码组织清晰
//   - system.ServiceGroup：系统核心功能服务（用户、权限、菜单等）
//   - example.ServiceGroup：示例业务服务（客户、文件上传等）
//     2. 统一管理：通过结构体组合的方式，将所有服务模块聚合在一个地方，便于管理和查找
//     3. 松耦合：各个模块之间相互独立，修改某个模块不会影响其他模块
//     4. 易扩展：新增业务模块时，只需在此结构体中添加新字段即可，无需修改其他现有代码
//     例如：添加新的 PaymentServiceGroup，只需添加一行：PaymentServiceGroup payment.ServiceGroup
//     5. 清晰的依赖关系：上层（API层）只需要依赖顶层的 service 包，通过 ServiceGroupApp 访问具体服务
//     避免了上层代码直接依赖底层各个子模块，降低了代码耦合度
//     6. 便于测试：可以轻松地为每个 ServiceGroup 编写独立的测试，也可以进行 mock 替换
type ServiceGroup struct {
	SystemServiceGroup  system.ServiceGroup  // 系统核心服务组：用户、权限、菜单、字典等核心功能
	ExampleServiceGroup example.ServiceGroup // 示例服务组：示例业务功能，供开发者参考
}
