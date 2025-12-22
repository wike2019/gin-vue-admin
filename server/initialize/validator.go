package initialize

import "github.com/flipped-aurora/gin-vue-admin/server/utils"

// init 包初始化函数，在包被导入时自动执行
// 设计原因：
// 1. 使用 init 函数在程序启动时自动注册验证规则，无需手动调用
// 2. 将常用的验证规则集中定义，提高代码复用性和一致性
// 3. 使用命名规则（如 PageVerify、IdVerify），便于在业务代码中引用
//
// 好处：
// 1. 零配置：导入包即自动注册，开发者无需关心注册时机
// 2. 统一管理：所有验证规则定义在一处，便于维护和修改
// 3. 类型安全：编译期检查验证规则的语法，避免运行时错误
// 4. 可复用：定义一次，多处使用，避免重复定义相同的验证规则
// 5. 语义清晰：使用有意义的名称（如 PageVerify），提高代码可读性
// 6. 易于扩展：新增验证规则只需在此处添加 RegisterRule 调用即可
//
// 验证规则说明：
// - PageVerify: 用于分页请求，验证 Page 和 PageSize 字段必填
// - IdVerify: 用于 ID 查询请求，验证 Id 字段必填
// - AuthorityIdVerify: 用于权限相关请求，验证 AuthorityId 字段必填
func init() {
	// 注册分页验证规则
	// 使用场景：列表查询接口，需要分页参数
	// 好处：统一分页参数验证逻辑，避免在每个接口中重复验证
	_ = utils.RegisterRule("PageVerify",
		utils.Rules{
			"Page":     {utils.NotEmpty()}, // 页码必须非空
			"PageSize": {utils.NotEmpty()}, // 每页大小必须非空
		},
	)

	// 注册 ID 验证规则
	// 使用场景：根据 ID 查询、删除、更新等操作
	// 好处：统一 ID 验证逻辑，确保所有 ID 操作都经过验证
	_ = utils.RegisterRule("IdVerify",
		utils.Rules{
			"Id": {utils.NotEmpty()}, // ID 必须非空
		},
	)

	// 注册权限 ID 验证规则
	// 使用场景：权限相关的操作（如分配权限、查询权限等）
	// 好处：统一权限 ID 验证，确保权限操作的参数合法性
	_ = utils.RegisterRule("AuthorityIdVerify",
		utils.Rules{
			"AuthorityId": {utils.NotEmpty()}, // 权限 ID 必须非空
		},
	)
}
