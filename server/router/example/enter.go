package example

import (
	api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"
)

// RouterGroup 是示例模块的路由组结构体，采用组合模式设计
// 通过嵌入多个具体的路由结构体，实现路由的统一管理和初始化
//
// 设计意义：
// 1. 模块化组织：将不同业务功能的路由（客户、文件上传下载、附件分类）统一管理在一个结构体中
// 2. 代码解耦：每个业务功能的路由定义在独立的文件中，便于维护和扩展
// 3. 统一入口：通过组合模式，外部只需要初始化 RouterGroup 即可完成所有子路由的注册
// 4. 易于扩展：新增业务路由时，只需创建新的路由结构体并嵌入到 RouterGroup 中即可
type RouterGroup struct {
	CustomerRouter              // 客户管理相关路由
	FileUploadAndDownloadRouter // 文件上传下载相关路由
	AttachmentCategoryRouter    // 附件分类相关路由
}

// 包级别的 API 实例变量，用于在路由初始化时引用对应的 API 处理函数
//
// 设计意义：
// 1. 统一管理：所有 API 实例集中定义，便于查找和维护
// 2. 避免重复：每个 API 实例只创建一次，避免在多个路由文件中重复引用
// 3. 依赖注入：通过 api.ApiGroupApp 统一获取 API 实例，实现依赖注入模式
// 4. 命名规范：使用包级别变量，在同一个包内的所有路由文件中可以直接使用，无需重复导入
var (
	exaCustomerApi              = api.ApiGroupApp.ExampleApiGroup.CustomerApi              // 客户管理 API 实例
	exaFileUploadAndDownloadApi = api.ApiGroupApp.ExampleApiGroup.FileUploadAndDownloadApi // 文件上传下载 API 实例
	attachmentCategoryApi       = api.ApiGroupApp.ExampleApiGroup.AttachmentCategoryApi    // 附件分类 API 实例
)
