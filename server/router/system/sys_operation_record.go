package system

import (
	"github.com/gin-gonic/gin"
)

// OperationRecordRouter 操作记录路由结构体
//
// 设计说明：
// 1. 为什么使用结构体？
//   - 遵循面向对象设计原则，将路由初始化逻辑封装在结构体方法中
//   - 便于代码组织和维护，每个功能模块的路由都有独立的结构体
//   - 符合 Go 语言的最佳实践，提高代码的可读性和可维护性
//   - 便于后续扩展，如果需要添加路由相关的配置或状态，可以直接在结构体中添加字段
//
// 2. 好处：
//   - 代码结构清晰：每个模块的路由定义独立，便于查找和维护
//   - 易于扩展：未来如果需要添加操作记录相关的其他路由，只需在此方法中扩展
//   - 统一管理：所有操作记录相关的路由都在一个地方，避免路由分散
type OperationRecordRouter struct{}

// InitSysOperationRecordRouter 初始化操作记录相关的路由
//
// 设计说明：
// 1. 路由分组设计：
//   - 使用 Router.Group("sysOperationRecord") 创建路由组，统一管理操作记录相关的所有接口
//   - 路由组路径为 "sysOperationRecord"，访问路径为 /api/v1/sysOperationRecord/xxx
//   - 好处：所有操作记录相关的接口都有统一的前缀，便于管理和识别
//
// 2. 为什么这些路由不使用 OperationRecord 中间件？
//   - 这些路由是对操作记录本身的 CRUD 操作（查询、删除操作记录）
//   - 如果对操作记录的查询/删除操作也记录操作记录，会造成以下问题：
//     a. 逻辑循环：操作记录的操作又产生操作记录，可能导致无限递归
//     b. 日志冗余：查询操作记录本身不需要再记录一次操作记录
//     c. 性能影响：操作记录的查询通常很频繁，如果都记录会产生大量日志
//   - 操作记录的查询和删除操作通常由系统管理员执行，属于系统维护操作
//   - 好处：避免循环记录，减少不必要的日志，提高系统性能
//
// 3. 为什么使用 Router 而非 PublicRouter？
//   - Router 是需要认证的私有路由组，操作记录包含敏感信息（操作人、操作内容等）
//   - 只有经过认证的用户（通常是管理员）才能查看和删除操作记录
//   - PublicRouter 是公开路由组，不需要认证即可访问，不适合操作记录这种敏感数据
//   - 好处：保证操作记录的安全性，防止未授权访问
//
// 4. 路由设计原则：
//   - RESTful 风格：使用标准的 HTTP 方法（GET、DELETE）来区分操作类型
//   - 语义化命名：路由名称清晰表达功能，如 deleteSysOperationRecord 表示删除操作记录
//   - 统一前缀：所有路由都在 sysOperationRecord 组下，便于管理和识别
//
// 参数说明：
//   - Router: 需要认证的私有路由组，用于需要登录才能访问的接口
//     通常是从 /api/v1 路由组传递下来的子路由组
func (s *OperationRecordRouter) InitSysOperationRecordRouter(Router *gin.RouterGroup) {
	// 创建操作记录路由组
	// 为什么使用 Group：
	//   - 将操作记录相关的所有路由组织在一起，统一管理
	//   - 可以为整个路由组统一添加中间件（如认证、权限验证等）
	//   - 路由路径更清晰：/api/v1/sysOperationRecord/xxx
	// 好处：
	//   1. 代码组织清晰：所有操作记录相关的路由都在一个组内
	//   2. 便于维护：如果需要修改路由前缀或添加统一的中间件，只需修改一处
	//   3. 符合 RESTful 设计：资源化的路由设计，操作记录作为一个资源进行管理
	operationRecordRouter := Router.Group("sysOperationRecord")
	{
		// 删除操作记录接口
		// 为什么使用 DELETE 方法：
		//   - 符合 RESTful 设计规范，DELETE 方法表示删除资源
		//   - 语义清晰，一看就知道是删除操作
		// 为什么不需要操作记录中间件：
		//   - 这是对操作记录本身的删除操作，如果记录会产生循环
		//   - 删除操作记录通常由管理员执行，属于系统维护操作
		//   - 避免产生大量删除操作记录的日志，节省存储空间
		// 好处：
		//   1. 语义明确：使用标准 HTTP 方法，符合 RESTful 规范
		//   2. 避免循环记录：不会因为删除操作记录而产生新的操作记录
		//   3. 性能优化：减少不必要的中间件开销和日志写入
		operationRecordRouter.DELETE("deleteSysOperationRecord", operationRecordApi.DeleteSysOperationRecord) // 删除单个操作记录

		// 批量删除操作记录接口
		// 为什么需要单独的批量删除接口：
		//   - 批量操作比单个操作更高效，减少网络请求次数
		//   - 可以一次性删除多条操作记录，提高操作效率
		//   - 前端可以批量选择需要删除的记录，用户体验更好
		// 为什么使用 DELETE 方法：
		//   - 虽然参数可能通过请求体传递，但操作本质是删除，使用 DELETE 更符合语义
		// 好处：
		//   1. 提高效率：一次请求删除多条记录，减少网络开销
		//   2. 用户体验：支持批量操作，操作更便捷
		//   3. 性能优化：减少数据库交互次数，提高删除效率
		operationRecordRouter.DELETE("deleteSysOperationRecordByIds", operationRecordApi.DeleteSysOperationRecordByIds) // 批量删除操作记录

		// 根据ID查询单个操作记录接口
		// 为什么使用 GET 方法：
		//   - GET 方法表示查询操作，符合 RESTful 规范
		//   - GET 请求是幂等的，多次请求返回相同结果
		//   - GET 请求可以被缓存，提高性能
		// 为什么不需要操作记录中间件：
		//   - 查询操作不改变数据状态，属于只读操作
		//   - 查询操作记录通常很频繁，如果都记录会产生大量日志
		//   - 查询操作记录本身不需要审计追踪（只有修改操作才需要）
		// 好处：
		//   1. 性能优化：查询操作不记录日志，减少系统负担
		//   2. 存储优化：避免查询日志占用大量存储空间
		//   3. 符合 RESTful 规范：使用标准 HTTP 方法，语义清晰
		operationRecordRouter.GET("findSysOperationRecord", operationRecordApi.FindSysOperationRecord) // 根据ID获取单个操作记录

		// 获取操作记录列表接口
		// 为什么需要列表查询接口：
		//   - 操作记录通常数量很多，需要分页查询
		//   - 支持条件筛选，如按时间范围、操作人、操作类型等筛选
		//   - 前端需要展示操作记录列表，供管理员查看和筛选
		// 为什么使用 GET 方法：
		//   - 列表查询是只读操作，使用 GET 方法符合 RESTful 规范
		//   - GET 请求可以带查询参数，便于实现分页和筛选
		// 为什么不需要操作记录中间件：
		//   - 列表查询操作非常频繁，如果都记录会产生大量日志
		//   - 查询操作不改变数据状态，不需要审计追踪
		//   - 避免日志冗余，提高系统性能
		// 好处：
		//   1. 支持分页和筛选：可以高效查询大量操作记录
		//   2. 性能优化：不记录查询日志，减少系统负担
		//   3. 用户体验：支持条件查询，便于管理员快速找到需要的操作记录
		operationRecordRouter.GET("getSysOperationRecordList", operationRecordApi.GetSysOperationRecordList) // 分页获取操作记录列表
	}
}
