package example

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemService "github.com/flipped-aurora/gin-vue-admin/server/service/system"
)

// CustomerService 客户服务结构体
// 设计说明：使用空结构体作为服务层，因为服务层主要是方法集合，不需要存储状态
// 好处：
// 1. 内存占用最小：空结构体不占用任何内存空间
// 2. 语义清晰：明确表示这是一个服务类，只提供方法功能
// 3. 符合 Go 语言习惯：Go 中常用空结构体作为方法接收器
type CustomerService struct{}

// CustomerServiceApp 客户服务单例实例
// 设计说明：使用单例模式，全局只有一个服务实例
// 为什么这样写：
// 1. 避免重复创建：每次调用不需要 new，节省内存和初始化开销
// 2. 统一访问入口：所有地方使用同一个实例，保证行为一致性
// 3. 便于依赖注入：其他模块可以直接引用这个实例
// 好处：
// 1. 性能优化：避免重复创建对象
// 2. 代码简洁：直接使用 CustomerServiceApp 调用方法，无需手动创建
var CustomerServiceApp = new(CustomerService)

// @author: [piexlmax](https://github.com/piexlmax)
// @function: CreateExaCustomer
// @description: 创建客户
// @param: e model.ExaCustomer
// @return: err error
//
// 功能说明：在数据库中创建一条新的客户记录
//
// 为什么使用值传递（e example.ExaCustomer）：
// 1. 创建操作不需要修改原对象：创建成功后，GORM 会自动将生成的 ID 等信息填充到传入的对象中
// 2. 值传递更安全：避免意外修改调用方的数据
// 3. 语义清晰：明确表示这是一个创建操作，输入是待创建的数据
//
// 为什么使用 &e 传递地址：
// 1. GORM 的 Create 方法需要指针：这样才能在创建后回填自动生成的字段（如 ID、创建时间等）
// 2. 回填数据：创建成功后，GORM 会将数据库生成的 ID、CreatedAt 等字段写回对象
//
// 好处：
// 1. 简洁高效：一行代码完成创建操作
// 2. 自动回填：创建后自动获取数据库生成的 ID 等信息
// 3. 错误处理：通过 Error 字段统一处理数据库错误
func (exa *CustomerService) CreateExaCustomer(e example.ExaCustomer) (err error) {
	// 使用 GORM 的 Create 方法创建记录
	// Create 会自动处理：主键生成、时间戳填充、关联关系等
	err = global.GVA_DB.Create(&e).Error
	return err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: DeleteExaCustomer
// @description: 删除客户
// @param: e model.ExaCustomer
// @return: err error
//
// 功能说明：从数据库中删除指定的客户记录
//
// 为什么使用值传递但 Delete 需要指针：
// 1. GORM 的 Delete 方法需要指针：这样才能识别要删除的记录（通常通过主键 ID）
// 2. 值传递参数：函数参数使用值传递，避免修改调用方的原始对象
// 3. 内部转换：在调用 Delete 时使用 &e 获取地址
//
// 删除机制说明：
// 1. 软删除：如果模型包含 DeletedAt 字段（GVA_MODEL 中包含），GORM 会执行软删除
// 2. 硬删除：如果模型没有 DeletedAt 字段，会执行物理删除
// 3. 主键识别：GORM 通过对象的主键字段（通常是 ID）来定位要删除的记录
//
// 好处：
// 1. 安全性：支持软删除，数据不会真正丢失，可以恢复
// 2. 简洁性：一行代码完成删除操作
// 3. 灵活性：可以通过设置不同的字段值来控制删除条件
func (exa *CustomerService) DeleteExaCustomer(e example.ExaCustomer) (err error) {
	// 使用 GORM 的 Delete 方法删除记录
	// 如果模型有 DeletedAt 字段，会执行软删除（更新 DeletedAt 时间戳）
	// 如果模型没有 DeletedAt 字段，会执行硬删除（真正从数据库删除）
	err = global.GVA_DB.Delete(&e).Error
	return err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: UpdateExaCustomer
// @description: 更新客户
// @param: e *model.ExaCustomer
// @return: err error
//
// 功能说明：更新数据库中已存在的客户记录
//
// 为什么使用指针传递（e *example.ExaCustomer）：
// 1. 需要修改对象：更新操作需要修改对象的字段值，指针传递允许直接修改
// 2. 性能考虑：避免大对象的值拷贝，指针传递只传递地址（8字节），效率更高
// 3. 语义明确：指针传递明确表示函数可能会修改传入的对象
// 4. GORM 要求：Save 方法需要指针来识别要更新的记录（通过主键 ID）
//
// 为什么使用 Save 而不是 Update：
// 1. Save 会自动判断：如果主键有值且记录存在，执行更新；如果主键为空或记录不存在，执行插入
// 2. 更新所有字段：Save 会更新所有字段（包括零值字段），适合完整更新场景
// 3. 自动处理：自动处理 UpdatedAt 时间戳字段
//
// 好处：
// 1. 灵活性：一个方法同时支持更新和插入（upsert 语义）
// 2. 完整性：确保所有字段都被更新，不会遗漏
// 3. 自动化：自动处理更新时间戳，无需手动设置
func (exa *CustomerService) UpdateExaCustomer(e *example.ExaCustomer) (err error) {
	// 使用 GORM 的 Save 方法保存记录
	// Save 会根据主键 ID 判断是更新还是插入：
	// - 如果 ID 存在且记录存在：执行 UPDATE
	// - 如果 ID 为空或记录不存在：执行 INSERT
	// 会自动更新 UpdatedAt 字段
	err = global.GVA_DB.Save(e).Error
	return err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: GetExaCustomer
// @description: 获取客户信息
// @param: id uint
// @return: customer model.ExaCustomer, err error
//
// 功能说明：根据客户 ID 查询单个客户的详细信息
//
// 为什么使用 Where + First 而不是直接 Find：
// 1. First 语义明确：明确表示只查询一条记录，如果有多条会报错，避免数据异常
// 2. 性能更好：First 找到第一条就停止，不会继续扫描
// 3. 错误处理：如果记录不存在，First 会返回 gorm.ErrRecordNotFound 错误，便于判断
//
// 为什么使用 Where("id = ?", id) 参数化查询：
// 1. 防止 SQL 注入：使用参数化查询，GORM 会自动转义特殊字符
// 2. 类型安全：GORM 会根据参数类型进行类型转换和验证
// 3. 性能优化：数据库可以缓存执行计划
//
// 为什么使用命名返回值：
// 1. 代码简洁：可以直接 return，无需显式返回变量
// 2. 可读性好：函数签名就明确了返回值名称
// 3. 便于文档：返回值名称可以作为文档的一部分
//
// 好处：
// 1. 安全性：参数化查询防止 SQL 注入攻击
// 2. 明确性：First 确保只返回一条记录，避免数据不一致
// 3. 错误处理：可以区分"记录不存在"和"查询出错"两种情况
func (exa *CustomerService) GetExaCustomer(id uint) (customer example.ExaCustomer, err error) {
	// 使用 Where 条件查询，First 获取第一条记录
	// Where("id = ?", id) 使用参数化查询，防止 SQL 注入
	// First 方法：找到第一条记录后立即返回，如果记录不存在返回 gorm.ErrRecordNotFound
	err = global.GVA_DB.Where("id = ?", id).First(&customer).Error
	return
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: GetCustomerInfoList
// @description: 分页获取客户列表
// @param: sysUserAuthorityID uint 当前用户角色ID, info request.PageInfo 分页信息
// @return: list interface{} 客户列表, total int64 总记录数, err error
//
// 功能说明：根据用户权限分页查询客户列表，实现数据权限控制
//
// 设计思路：
// 1. 权限过滤：根据用户角色的数据权限配置，只返回有权限访问的客户数据
// 2. 分页查询：支持分页，避免一次性加载大量数据
// 3. 关联预加载：预加载关联的 SysUser 信息，避免 N+1 查询问题
//
// 为什么需要权限过滤：
// 1. 数据安全：不同角色的用户只能看到自己权限范围内的客户数据
// 2. 多租户支持：通过数据权限实现多租户数据隔离
// 3. 灵活配置：管理员可以配置哪些角色可以访问哪些其他角色的数据
//
// 分页计算说明：
//   - limit：每页记录数
//   - offset：跳过多少条记录 = 每页记录数 × (当前页码 - 1)
//     例如：第2页，每页10条，offset = 10 × (2-1) = 10，跳过前10条
//
// 为什么先 Count 再 Find：
// 1. 前端需要总数：前端分页组件需要知道总记录数来计算总页数
// 2. 性能考虑：Count 查询比 Find 快，先 Count 可以快速判断是否有数据
// 3. 错误处理：如果 Count 失败，直接返回，避免执行更耗时的 Find 操作
//
// 为什么使用 Preload("SysUser")：
// 1. 避免 N+1 查询：不使用 Preload 时，查询 N 个客户会执行 N+1 次查询（1次查客户，N次查用户）
// 2. 性能优化：Preload 会使用 JOIN 或批量查询，一次性加载所有关联数据
// 3. 数据完整：返回的客户列表包含完整的用户信息，前端无需再次请求
//
// 为什么使用 Model(&example.ExaCustomer{})：
// 1. 链式调用：创建一个查询构建器，可以链式调用多个方法
// 2. 类型安全：明确指定查询的表和模型类型
// 3. 复用性：同一个 db 变量可以用于多个查询操作（Count 和 Find）
//
// 数据权限机制说明：
// 1. DataAuthorityId：角色配置的数据权限，表示该角色可以访问哪些其他角色的数据
// 2. sys_user_authority_id：客户记录关联的角色ID，标识该客户属于哪个角色
// 3. 过滤逻辑：只返回 sys_user_authority_id 在 DataAuthorityId 列表中的客户记录
//
// 好处：
// 1. 数据安全：通过权限过滤确保用户只能访问授权数据
// 2. 性能优化：分页减少数据传输量，Preload 减少数据库查询次数
// 3. 用户体验：返回完整数据（包含关联信息），前端无需额外请求
// 4. 可扩展性：权限机制支持灵活的权限配置，适应不同业务场景
func (exa *CustomerService) GetCustomerInfoList(sysUserAuthorityID uint, info request.PageInfo) (list interface{}, total int64, err error) {
	// 第一步：计算分页参数
	// limit：每页要返回的记录数
	limit := info.PageSize
	// offset：要跳过的记录数 = 每页记录数 × (当前页码 - 1)
	// 例如：第1页 offset=0，第2页 offset=PageSize，第3页 offset=2*PageSize
	offset := info.PageSize * (info.Page - 1)

	// 第二步：创建查询构建器
	// Model 方法指定要查询的模型，返回一个可以链式调用的查询构建器
	db := global.GVA_DB.Model(&example.ExaCustomer{})

	// 第三步：获取当前角色的权限信息
	// 为什么需要获取权限信息：需要知道当前角色可以访问哪些其他角色的数据
	var a system.SysAuthority
	a.AuthorityId = sysUserAuthorityID
	// GetAuthorityInfo 会返回角色的完整信息，包括 DataAuthorityId（数据权限配置）
	auth, err := systemService.AuthorityServiceApp.GetAuthorityInfo(a)
	if err != nil {
		// 如果获取权限信息失败，直接返回错误，不继续执行查询
		return
	}

	// 第四步：提取数据权限ID列表
	// DataAuthorityId 是一个关联数组，包含该角色可以访问的所有角色ID
	var dataId []uint
	for _, v := range auth.DataAuthorityId {
		// 将关联对象中的 AuthorityId 提取到切片中
		dataId = append(dataId, v.AuthorityId)
	}

	// 第五步：查询符合条件的客户总数
	// 为什么先查询总数：前端需要知道总记录数来计算总页数和显示分页信息
	var CustomerList []example.ExaCustomer
	// Where("sys_user_authority_id in ?", dataId) 表示只查询权限范围内的客户
	// Count 统计符合条件的记录总数
	err = db.Where("sys_user_authority_id in ?", dataId).Count(&total).Error
	if err != nil {
		// 如果统计失败，直接返回错误，避免执行更耗时的查询操作
		return CustomerList, total, err
	} else {
		// 第六步：分页查询客户列表
		// Limit(limit)：限制返回的记录数
		// Offset(offset)：跳过前面的记录
		// Preload("SysUser")：预加载关联的 SysUser 信息，避免 N+1 查询问题
		// Where("sys_user_authority_id in ?", dataId)：应用权限过滤条件
		// Find(&CustomerList)：执行查询并将结果填充到 CustomerList
		err = db.Limit(limit).Offset(offset).Preload("SysUser").Where("sys_user_authority_id in ?", dataId).Find(&CustomerList).Error
	}
	return CustomerList, total, err
}
