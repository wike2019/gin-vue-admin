package example

import "github.com/flipped-aurora/gin-vue-admin/server/service"

// ApiGroup 是 example 业务模块的 API 组
// 采用组合模式，将所有相关的 API 控制器聚合在一起
// 设计优势：
// 1. 业务内聚：将同一业务领域的 API 组织在一起，提高代码可读性
// 2. 统一管理：通过一个结构体管理所有相关 API，便于路由注册
// 3. 依赖注入：每个 API 控制器可以访问嵌入的字段，实现隐式依赖
// 4. 扩展方便：新增 API 控制器只需在此结构体中添加字段即可
type ApiGroup struct {
	CustomerApi              // 客户管理相关 API
	FileUploadAndDownloadApi // 文件上传下载相关 API
	AttachmentCategoryApi    // 附件分类管理相关 API
}

// 服务层依赖注入
// 设计模式：依赖注入（Dependency Injection）
// 好处：
// 1. 解耦合：API 层不直接创建 service 实例，而是通过全局变量注入，降低耦合
// 2. 单例模式：所有 API 方法共享同一个 service 实例，节省内存，保证数据一致性
// 3. 易于测试：可以轻松替换为 mock service 进行单元测试
// 4. 统一管理：所有 service 依赖集中在此处，便于查看和维护
// 5. 延迟初始化：在应用启动时初始化，避免循环依赖问题
var (
	customerService              = service.ServiceGroupApp.ExampleServiceGroup.CustomerService
	fileUploadAndDownloadService = service.ServiceGroupApp.ExampleServiceGroup.FileUploadAndDownloadService
	attachmentCategoryService    = service.ServiceGroupApp.ExampleServiceGroup.AttachmentCategoryService
)
