package example

// ServiceGroup 采用 Go 语言的嵌入结构体模式（Embedded Struct），将示例模块的所有服务聚合在一起
// 这种设计模式的意义和好处：
//
//  1. 【服务聚合统一管理】
//     将所有示例相关的服务（如客户管理、文件上传下载、附件分类等）集中在一个结构体中，
//     形成统一的示例服务入口，便于管理和维护
//
//  2. 【简化访问路径】
//     通过嵌入结构体，可以直接通过 ServiceGroup 访问各个服务的方法，无需通过中间字段
//     例如：service.ServiceGroupApp.ExampleServiceGroup.CustomerService.GetCustomers()
//     而不是：service.ServiceGroupApp.ExampleServiceGroup.Services.CustomerService.GetCustomers()
//     这样访问路径更短、更直观
//
//  3. 【代码组织清晰】
//     将同一业务模块（example）的所有服务集中管理，与 system 等其他业务模块的服务分离
//     每个模块都有独立的 ServiceGroup，保持了代码的模块化和层次化
//
//  4. 【类型安全和编译检查】
//     Go 语言的类型系统会在编译时检查所有嵌入的服务类型是否正确，
//     如果某个服务类型不存在或方法签名不匹配，编译时就会报错，避免了运行时错误
//
//  5. 【易于扩展和维护】
//     新增示例服务时，只需在此结构体中添加一个新的嵌入字段即可
//     例如：添加新的 OrderService，只需添加一行：OrderService
//     无需修改其他现有代码，符合开闭原则（对扩展开放，对修改关闭）
//
//  6. 【依赖注入友好】
//     可以通过统一的方式初始化和管理所有服务，便于后续集成依赖注入框架
//     所有服务的生命周期可以统一管理，降低了耦合度
//
//  7. 【便于测试】
//     可以轻松地为整个 ServiceGroup 创建 mock 对象，也可以单独测试某个服务
//     测试时可以通过替换整个 ServiceGroup 来实现服务层的隔离测试
//
//  8. 【清晰的依赖关系】
//     上层代码（API 层）通过 service.ServiceGroupApp.ExampleServiceGroup 访问服务，
//     明确的访问路径使得代码的依赖关系一目了然，便于理解和维护
type ServiceGroup struct {
	CustomerService              // 客户服务：客户信息的增删改查等业务逻辑
	FileUploadAndDownloadService // 文件上传下载服务：处理文件的上传、下载、断点续传等功能
	AttachmentCategoryService    // 附件分类服务：管理附件分类的 CRUD 操作
}
