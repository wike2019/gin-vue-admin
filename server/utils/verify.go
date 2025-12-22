package utils

// 验证规则集中定义文件
//
// 设计目的：
// 1. 集中管理：将所有业务场景的验证规则统一放在一个文件中，便于查找和维护
// 2. 代码复用：避免在每个API处理函数中重复定义相同的验证规则，提高代码复用性
// 3. 统一标准：确保相同业务场景的验证规则在整个项目中保持一致，避免不同地方定义不同的规则
// 4. 易于扩展：新增验证规则时只需在此处添加，无需修改业务逻辑代码
//
// 设计优势：
// - 类型安全：使用Go的类型系统，编译期即可发现规则定义错误
// - 性能优化：包级变量在程序启动时初始化一次，避免重复创建规则对象
// - 可读性强：规则名称清晰表达用途，代码自解释
// - 易于测试：可以单独测试每个验证规则，无需启动整个应用
//
// 使用方式：
// 在API处理函数中通过 utils.Verify(structInstance, utils.XXXVerify) 调用
// 例如：err := utils.Verify(loginReq, utils.LoginVerify)
var (
	// IdVerify ID验证规则
	// 用途：验证ID字段不能为空，用于需要ID参数的接口（如删除、更新、查询详情等）
	// 好处：统一ID验证标准，避免在多个地方重复编写相同的验证逻辑
	IdVerify = Rules{"ID": []string{NotEmpty()}}

	// ApiVerify API接口验证规则
	// 用途：验证API接口创建/更新时的必填字段
	// 包含：路径(Path)、描述(Description)、API分组(ApiGroup)、请求方法(Method)
	// 好处：确保API接口信息的完整性，防止创建不完整的API配置
	ApiVerify = Rules{"Path": {NotEmpty()}, "Description": {NotEmpty()}, "ApiGroup": {NotEmpty()}, "Method": {NotEmpty()}}

	// MenuVerify 菜单验证规则
	// 用途：验证菜单创建/更新时的必填字段和排序值
	// 包含：路径(Path)、名称(Name)、组件(Component)、排序值(Sort >= 0)
	// 好处：确保菜单数据完整性，排序值使用Ge("0")确保非负数，防止菜单顺序异常
	MenuVerify = Rules{"Path": {NotEmpty()}, "Name": {NotEmpty()}, "Component": {NotEmpty()}, "Sort": {Ge("0")}}

	// MenuMetaVerify 菜单元信息验证规则
	// 用途：验证菜单元信息中的标题字段
	// 好处：确保菜单在前端显示时有标题，提升用户体验
	MenuMetaVerify = Rules{"Title": {NotEmpty()}}

	// LoginVerify 登录验证规则
	// 用途：验证用户登录时的用户名和密码
	// 包含：用户名(Username)、密码(Password)
	// 好处：防止空用户名或空密码提交，在请求进入业务逻辑前就拦截无效请求
	LoginVerify = Rules{"Username": {NotEmpty()}, "Password": {NotEmpty()}}

	// RegisterVerify 注册验证规则
	// 用途：验证用户注册时的必填字段
	// 包含：用户名(Username)、昵称(NickName)、密码(Password)、权限ID(AuthorityId)
	// 好处：确保新用户注册时信息完整，特别是权限ID确保用户有明确的权限归属
	RegisterVerify = Rules{"Username": {NotEmpty()}, "NickName": {NotEmpty()}, "Password": {NotEmpty()}, "AuthorityId": {NotEmpty()}}

	// PageInfoVerify 分页信息验证规则
	// 用途：验证分页查询时的页码和每页数量
	// 包含：页码(Page)、每页数量(PageSize)
	// 好处：防止分页参数为空导致查询异常，确保分页查询的健壮性
	PageInfoVerify = Rules{"Page": {NotEmpty()}, "PageSize": {NotEmpty()}}

	// CustomerVerify 客户信息验证规则
	// 用途：验证客户创建/更新时的必填字段
	// 包含：客户名称(CustomerName)、客户电话数据(CustomerPhoneData)
	// 好处：确保客户基本信息完整，特别是联系方式必须填写
	CustomerVerify = Rules{"CustomerName": {NotEmpty()}, "CustomerPhoneData": {NotEmpty()}}

	// AutoCodeVerify 自动代码生成验证规则
	// 用途：验证自动代码生成功能的必填参数
	// 包含：缩写(Abbreviation)、结构体名称(StructName)、包名(PackageName)
	// 好处：确保代码生成所需的关键信息完整，避免生成不完整的代码文件
	AutoCodeVerify = Rules{"Abbreviation": {NotEmpty()}, "StructName": {NotEmpty()}, "PackageName": {NotEmpty()}}

	// AutoPackageVerify 自动包验证规则
	// 用途：验证自动包创建时的包名
	// 好处：确保包名不为空，防止创建无效的包结构
	AutoPackageVerify = Rules{"PackageName": {NotEmpty()}}

	// AuthorityVerify 权限验证规则
	// 用途：验证权限创建/更新时的必填字段
	// 包含：权限ID(AuthorityId)、权限名称(AuthorityName)
	// 好处：确保权限信息完整，权限ID和名称都是权限管理的核心字段
	AuthorityVerify = Rules{"AuthorityId": {NotEmpty()}, "AuthorityName": {NotEmpty()}}

	// AuthorityIdVerify 权限ID验证规则
	// 用途：验证需要权限ID的操作（如查询权限详情、删除权限等）
	// 好处：统一权限ID验证，避免在多个接口中重复编写验证逻辑
	AuthorityIdVerify = Rules{"AuthorityId": {NotEmpty()}}

	// OldAuthorityVerify 旧权限ID验证规则
	// 用途：验证权限变更操作中的旧权限ID
	// 好处：确保权限变更时明确指定要变更的权限，防止误操作
	OldAuthorityVerify = Rules{"OldAuthorityId": {NotEmpty()}}

	// ChangePasswordVerify 修改密码验证规则
	// 用途：验证修改密码时的原密码和新密码
	// 包含：原密码(Password)、新密码(NewPassword)
	// 好处：确保修改密码时两个密码字段都不为空，防止创建无效的密码修改请求
	ChangePasswordVerify = Rules{"Password": {NotEmpty()}, "NewPassword": {NotEmpty()}}

	// SetUserAuthorityVerify 设置用户权限验证规则
	// 用途：验证设置用户权限时的权限ID
	// 好处：确保权限ID不为空，防止将用户权限设置为空值导致权限异常
	SetUserAuthorityVerify = Rules{"AuthorityId": {NotEmpty()}}
)
