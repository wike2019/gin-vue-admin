package example

import (
	"github.com/gin-gonic/gin"
)

// AttachmentCategoryRouter 附件分类路由结构体
// 采用空结构体设计，仅作为方法接收器使用，不存储任何状态
//
// 设计意义：
// 1. 类型安全：通过结构体方法组织路由，比直接使用函数更符合 Go 的面向对象设计
// 2. 命名空间：将路由初始化方法绑定到特定类型，避免方法名冲突
// 3. 扩展性：未来如需添加路由级别的配置或状态，可以轻松扩展结构体字段
// 4. 一致性：与项目中其他路由结构体的设计保持一致，便于团队协作和维护
type AttachmentCategoryRouter struct{}

// InitAttachmentCategoryRouterRouter 初始化附件分类相关的路由
// 该方法定义了附件分类模块的所有 HTTP 路由端点
//
// 参数说明：
//   - Router: 父级路由组，通常是从主路由中传入的示例模块路由组
//
// 设计意义：
// 1. 路由分组：使用 Router.Group("attachmentCategory") 创建子路由组，所有相关路由统一前缀为 /attachmentCategory
// 2. RESTful 设计：根据 HTTP 方法（GET、POST）区分不同的操作语义
//    - GET: 用于查询操作（获取分类列表）
//    - POST: 用于创建、更新、删除等需要请求体的操作
// 3. 代码块组织：使用 {} 代码块将路由定义分组，提高代码可读性
// 4. 单一职责：每个路由文件只负责一个业务模块的路由定义，符合单一职责原则
// 5. 易于维护：路由集中管理，修改或新增路由时只需在此处操作
func (r *AttachmentCategoryRouter) InitAttachmentCategoryRouterRouter(Router *gin.RouterGroup) {
	// 创建附件分类路由组，所有路由的前缀为 /attachmentCategory
	// 例如：/attachmentCategory/getCategoryList
	router := Router.Group("attachmentCategory")
	{
		router.GET("getCategoryList", attachmentCategoryApi.GetCategoryList) // GET 请求：获取分类列表，查询操作使用 GET 方法
		router.POST("addCategory", attachmentCategoryApi.AddCategory)        // POST 请求：添加/编辑分类，需要传递分类数据
		router.POST("deleteCategory", attachmentCategoryApi.DeleteCategory)  // POST 请求：删除分类，需要传递删除参数（如 ID）
	}
}
