package system

import (
	"errors"
	"strconv"

	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"gorm.io/gorm"
)

// ErrRoleExistence 角色已存在的错误
// 设计说明：使用包级错误变量而不是每次创建新的error
// 好处：
// 1. 错误一致性：所有地方返回相同的错误对象，便于错误判断和日志记录
// 2. 性能优化：避免每次创建新的error对象，减少内存分配
// 3. 可测试性：可以通过 errors.Is() 判断特定错误类型
var ErrRoleExistence = errors.New("存在相同角色id")

//@author: [piexlmax](https://github.com/piexlmax)
//@function: CreateAuthority
//@description: 创建一个角色
//@param: auth model.SysAuthority
//@return: authority system.SysAuthority, err error

// AuthorityService 角色服务结构体
// 设计说明：使用空结构体作为服务类，不存储任何状态
// 好处：
// 1. 内存效率：空结构体不占用内存空间，符合Go语言最佳实践
// 2. 语义清晰：明确表示这是一个服务类，包含业务逻辑方法
// 3. 易于扩展：未来如需添加状态，只需在结构体中添加字段即可
type AuthorityService struct{}

// AuthorityServiceApp 角色服务单例
// 设计说明：使用单例模式，全局唯一实例
// 好处：
// 1. 避免重复创建：整个应用生命周期只创建一个实例，节省资源
// 2. 全局访问：任何地方都可以通过 AuthorityServiceApp 访问服务方法
// 3. 方法复用：所有服务方法都可以被多个调用方共享使用
var AuthorityServiceApp = new(AuthorityService)

// CreateAuthority 创建一个角色
// 功能说明：创建新角色时，需要同时创建角色记录、分配默认菜单权限、初始化Casbin权限规则
// 设计思路：使用数据库事务确保所有操作要么全部成功，要么全部回滚，保证数据一致性
//
// 为什么这样写：
// 1. 先检查角色ID是否存在：避免创建重复角色，提前返回错误，节省数据库操作
// 2. 使用事务：角色、菜单关联、权限规则三者必须同时成功，使用事务保证原子性
// 3. 分配默认菜单：新角色自动拥有基础菜单权限（如仪表盘），提供开箱即用的体验
// 4. 初始化Casbin规则：为新角色分配默认API访问权限（如登录、获取用户信息等基础功能）
//
// 好处：
// 1. 数据一致性：事务保证角色、菜单、权限三者数据完全同步，不会出现部分成功的情况
// 2. 错误处理：任何步骤失败都会回滚，数据库状态保持干净，不会留下脏数据
// 3. 用户体验：新角色创建后即可使用，无需手动配置基础权限
// 4. 安全性：默认权限只包含必要的基础功能，符合最小权限原则
func (authorityService *AuthorityService) CreateAuthority(auth system.SysAuthority) (authority system.SysAuthority, err error) {
	// 第一步：检查角色ID是否已存在
	// 使用 errors.Is() 而不是直接比较 err，因为 GORM 可能返回包装后的错误
	// 好处：错误判断更加健壮，即使 GORM 未来改变错误返回方式也能正确判断
	if err = global.GVA_DB.Where("authority_id = ?", auth.AuthorityId).First(&system.SysAuthority{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		return auth, ErrRoleExistence
	}

	// 第二步：在事务中执行所有创建操作
	// 设计说明：使用 Transaction 回调函数，自动管理事务的提交和回滚
	// 好处：代码简洁，无需手动调用 Commit/Rollback，避免忘记处理事务
	e := global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 2.1 创建角色记录
		// 使用事务中的 tx 而不是全局 DB，确保操作在同一个事务中
		if err = tx.Create(&auth).Error; err != nil {
			return err
		}

		// 2.2 分配默认菜单权限
		// 使用 Replace 而不是 Append，确保新角色从干净状态开始
		// 好处：即使 auth 对象中已有菜单数据，也会被替换为默认菜单，保证一致性
		auth.SysBaseMenus = systemReq.DefaultMenu()
		if err = tx.Model(&auth).Association("SysBaseMenus").Replace(&auth.SysBaseMenus); err != nil {
			return err
		}

		// 2.3 初始化 Casbin 权限规则
		// 设计说明：将默认权限规则转换为 Casbin 规则格式 [角色ID, 路径, 方法]
		// 为什么转换：Casbin 使用字符串数组存储规则，需要将 uint 类型的 AuthorityId 转换为字符串
		casbinInfos := systemReq.DefaultCasbin()
		authorityId := strconv.Itoa(int(auth.AuthorityId))
		rules := [][]string{}
		// 批量构建规则数组，一次性添加到 Casbin
		// 好处：减少与 Casbin 的交互次数，提高性能
		for _, v := range casbinInfos {
			rules = append(rules, []string{authorityId, v.Path, v.Method})
		}
		// 将事务对象传递给 CasbinService，确保权限规则也在同一个事务中创建
		// 好处：如果权限规则创建失败，整个事务回滚，数据保持一致
		return CasbinServiceApp.AddPolicies(tx, rules)
	})

	return auth, e
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: CopyAuthority
// @description: 复制一个角色（包括菜单权限、按钮权限、API权限）
// @param: adminAuthorityID 管理员角色ID，用于权限验证；copyInfo 复制信息，包含源角色ID和目标角色信息
// @return: authority system.SysAuthority, err error
//
// 功能说明：快速创建一个新角色，复制已有角色的所有权限配置
// 设计思路：分步骤复制角色的各个维度权限，最后统一处理 Casbin 规则
//
// 为什么这样写：
// 1. 先检查目标角色ID是否已存在：避免覆盖已有角色
// 2. 清空子角色列表：新角色不应该继承源角色的子角色关系
// 3. 分步骤复制：菜单权限、按钮权限、API权限分别处理，逻辑清晰
// 4. 错误回滚：如果 Casbin 权限复制失败，删除已创建的角色，保证数据一致性
//
// 好处：
// 1. 提高效率：管理员可以基于现有角色快速创建相似角色，无需手动配置每个权限
// 2. 权限继承：新角色完全复制源角色的权限，确保权限配置的准确性
// 3. 灵活性：可以基于不同角色的权限组合，创建新的角色配置
func (authorityService *AuthorityService) CopyAuthority(adminAuthorityID uint, copyInfo response.SysAuthorityCopyResponse) (authority system.SysAuthority, err error) {
	// 第一步：检查目标角色ID是否已存在
	var authorityBox system.SysAuthority
	if !errors.Is(global.GVA_DB.Where("authority_id = ?", copyInfo.Authority.AuthorityId).First(&authorityBox).Error, gorm.ErrRecordNotFound) {
		return authority, ErrRoleExistence
	}

	// 第二步：清空子角色列表
	// 设计说明：新角色不应该继承源角色的子角色关系，子角色关系需要单独建立
	// 好处：避免角色层次结构混乱，每个角色的子角色关系都是明确管理的
	copyInfo.Authority.Children = []system.SysAuthority{}

	// 第三步：复制菜单权限
	// 获取源角色的所有菜单权限
	menus, err := MenuServiceApp.GetMenuAuthority(&request.GetAuthorityId{AuthorityId: copyInfo.OldAuthorityId})
	if err != nil {
		return
	}
	// 将菜单权限转换为基础菜单格式
	// 设计说明：MenuService 返回的是 SysMenu（包含角色相关信息），需要转换为 SysBaseMenu
	var baseMenu []system.SysBaseMenu
	for _, v := range menus {
		intNum := v.MenuId
		v.SysBaseMenu.ID = uint(intNum) // 确保菜单ID正确设置
		baseMenu = append(baseMenu, v.SysBaseMenu)
	}
	copyInfo.Authority.SysBaseMenus = baseMenu

	// 第四步：创建新角色（会自动关联菜单权限）
	err = global.GVA_DB.Create(&copyInfo.Authority).Error
	if err != nil {
		return
	}

	// 第五步：复制按钮权限
	// 设计说明：按钮权限是独立的数据表，需要单独查询和创建
	var btns []system.SysAuthorityBtn
	err = global.GVA_DB.Find(&btns, "authority_id = ?", copyInfo.OldAuthorityId).Error
	if err != nil {
		return
	}
	if len(btns) > 0 {
		// 修改按钮权限的角色ID为新角色的ID
		// 使用 range 遍历并修改切片元素，因为 btns 是值类型切片，需要修改索引位置的值
		for i := range btns {
			btns[i].AuthorityId = copyInfo.Authority.AuthorityId
		}
		// 批量创建按钮权限记录
		err = global.GVA_DB.Create(&btns).Error
		if err != nil {
			return
		}
	}

	// 第六步：复制 Casbin API 权限规则
	// 获取源角色的所有 API 权限路径
	paths := CasbinServiceApp.GetPolicyPathByAuthorityId(copyInfo.OldAuthorityId)
	// 将源角色的权限路径应用到新角色
	err = CasbinServiceApp.UpdateCasbin(adminAuthorityID, copyInfo.Authority.AuthorityId, paths)
	// 如果权限复制失败，删除已创建的角色，保证数据一致性
	// 设计说明：使用 _ 忽略删除操作的错误，因为这里主要是清理脏数据
	// 好处：即使删除失败，也不会影响当前错误的返回，调用方可以知道复制操作失败
	if err != nil {
		_ = authorityService.DeleteAuthority(&copyInfo.Authority)
	}
	return copyInfo.Authority, err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: UpdateAuthority
// @description: 更新角色基本信息（不包含权限配置）
// @param: auth model.SysAuthority 包含更新后的角色信息
// @return: authority system.SysAuthority, err error
//
// 功能说明：更新角色的基本信息，如角色名称、父角色ID等
// 设计思路：先查询现有角色，再使用 Updates 方法进行部分更新
//
// 为什么这样写：
// 1. 先查询角色是否存在：提前验证角色是否存在，返回明确的错误信息
// 2. 使用 Model(&oldAuthority).Updates()：基于查询到的记录进行更新，更安全
// 3. Updates 方法只更新非零值字段：避免意外覆盖未提供的字段
//
// 好处：
// 1. 安全性：只更新提供的字段，零值字段不会被执行更新操作
// 2. 可维护性：代码逻辑清晰，先验证后更新，错误处理明确
// 3. 日志记录：更新失败时记录调试日志，便于问题排查
func (authorityService *AuthorityService) UpdateAuthority(auth system.SysAuthority) (authority system.SysAuthority, err error) {
	// 第一步：查询现有角色
	// 设计说明：必须先查询角色是否存在，避免更新不存在的记录
	var oldAuthority system.SysAuthority
	err = global.GVA_DB.Where("authority_id = ?", auth.AuthorityId).First(&oldAuthority).Error
	if err != nil {
		// 记录调试日志，便于开发阶段排查问题
		global.GVA_LOG.Debug(err.Error())
		// 返回友好的错误信息，而不是直接返回数据库错误
		// 好处：错误信息更加用户友好，不暴露数据库实现细节
		return system.SysAuthority{}, errors.New("查询角色数据失败")
	}

	// 第二步：更新角色信息
	// 使用 Updates 方法而不是 Save 方法
	// 好处：Updates 只更新非零值字段，零值字段会被忽略，更安全
	err = global.GVA_DB.Model(&oldAuthority).Updates(&auth).Error
	return auth, err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: DeleteAuthority
// @description: 删除角色（包括关联的菜单权限、数据权限、用户关联、按钮权限、Casbin规则）
// @param: auth *model.SysAuthority 要删除的角色
// @return: err error
//
// 功能说明：安全删除角色，需要先检查各种约束条件，然后删除所有相关数据
// 设计思路：先进行多重安全检查，通过后使用事务删除所有关联数据
//
// 为什么这样写：
// 1. 多重安全检查：确保角色没有被使用，避免删除后导致数据不一致
// 2. 使用事务：删除操作涉及多个表，使用事务保证原子性
// 3. 软删除后硬删除：先软删除角色，再删除关联关系，最后清理 Casbin 规则
// 4. 清理所有关联：菜单关联、数据权限关联、用户关联、按钮权限、Casbin规则都要清理
//
// 好处：
// 1. 数据安全：防止误删除正在使用的角色，保护数据完整性
// 2. 数据一致性：事务保证所有关联数据都被正确清理，不会留下孤立数据
// 3. 权限安全：同时清理 Casbin 规则，确保权限体系的一致性
func (authorityService *AuthorityService) DeleteAuthority(auth *system.SysAuthority) error {
	// ========== 第一步：多重安全检查 ==========

	// 检查1：角色是否存在，并预加载关联的用户信息
	// 使用 Preload("Users") 一次性加载关联用户，避免后续单独查询
	// 好处：减少数据库查询次数，提高性能
	if errors.Is(global.GVA_DB.Debug().Preload("Users").First(&auth).Error, gorm.ErrRecordNotFound) {
		return errors.New("该角色不存在")
	}

	// 检查2：通过关联关系检查是否有用户使用该角色
	// 设计说明：通过 Preload 加载的 Users 字段检查
	if len(auth.Users) != 0 {
		return errors.New("此角色有用户正在使用禁止删除")
	}

	// 检查3：再次通过直接查询检查用户关联表
	// 设计说明：双重检查，确保不会遗漏任何用户关联
	// 为什么需要两次检查：Preload 可能因为关联表结构不同而遗漏某些关联关系
	// 好处：更全面的检查，确保数据安全
	if !errors.Is(global.GVA_DB.Where("authority_id = ?", auth.AuthorityId).First(&system.SysUser{}).Error, gorm.ErrRecordNotFound) {
		return errors.New("此角色有用户正在使用禁止删除")
	}

	// 检查4：检查是否存在子角色
	// 设计说明：如果角色有子角色，不允许删除，保持角色层次结构的完整性
	// 好处：避免删除父角色后导致子角色成为孤儿节点，维护角色树的完整性
	if !errors.Is(global.GVA_DB.Where("parent_id = ?", auth.AuthorityId).First(&system.SysAuthority{}).Error, gorm.ErrRecordNotFound) {
		return errors.New("此角色存在子角色不允许删除")
	}

	// ========== 第二步：在事务中删除所有关联数据 ==========
	// 使用事务确保所有删除操作要么全部成功，要么全部回滚
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		var err error

		// 2.1 软删除角色（使用 Unscoped().Delete 进行硬删除）
		// 设计说明：先 Preload 加载关联数据到 auth 对象中，以便后续删除关联关系
		// Unscoped().Delete 执行硬删除而不是软删除，完全从数据库中移除记录
		// 好处：彻底清理数据，避免软删除后的记录占用存储空间
		if err = tx.Preload("SysBaseMenus").Preload("DataAuthorityId").Where("authority_id = ?", auth.AuthorityId).First(auth).Unscoped().Delete(auth).Error; err != nil {
			return err
		}

		// 2.2 删除菜单关联关系
		// 设计说明：虽然主记录已删除，但关联表的记录需要显式删除
		// 好处：确保中间表（sys_authority_menus）的数据被正确清理
		if len(auth.SysBaseMenus) > 0 {
			if err = tx.Model(auth).Association("SysBaseMenus").Delete(auth.SysBaseMenus); err != nil {
				return err
			}
			// 注释掉的代码是另一种删除方式，当前使用的方式更明确
			// err = db.Association("SysBaseMenus").Delete(&auth)
		}

		// 2.3 删除数据权限关联关系
		// 设计说明：数据权限是角色之间的关联关系，需要单独清理
		if len(auth.DataAuthorityId) > 0 {
			if err = tx.Model(auth).Association("DataAuthorityId").Delete(auth.DataAuthorityId); err != nil {
				return err
			}
		}

		// 2.4 删除用户角色关联表中的记录
		// 设计说明：虽然前面已经检查过没有用户关联，但这里再次清理确保数据完整性
		// 使用条件删除，删除所有与该角色关联的用户角色关系
		if err = tx.Delete(&system.SysUserAuthority{}, "sys_authority_authority_id = ?", auth.AuthorityId).Error; err != nil {
			return err
		}

		// 2.5 删除按钮权限记录
		// 设计说明：按钮权限是独立的权限配置表，需要单独清理
		// 使用 Where 条件删除，删除该角色的所有按钮权限配置
		if err = tx.Where("authority_id = ?", auth.AuthorityId).Delete(&[]system.SysAuthorityBtn{}).Error; err != nil {
			return err
		}

		// 2.6 删除 Casbin 权限规则
		// 设计说明：Casbin 权限规则存储在独立的权限管理系统中，需要同步删除
		// 将角色ID转换为字符串，因为 Casbin 使用字符串作为主体标识
		authorityId := strconv.Itoa(int(auth.AuthorityId))
		// 删除该角色的所有权限规则
		// 好处：确保权限体系的一致性，删除角色后不会遗留无效的权限规则
		if err = CasbinServiceApp.RemoveFilteredPolicy(tx, authorityId); err != nil {
			return err
		}

		return nil
	})
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: GetAuthorityInfoList
// @description: 获取角色列表（树形结构，包含子角色）
// @param: authorityID uint 当前操作的角色ID
// @return: list []system.SysAuthority, err error
//
// 功能说明：根据配置的权限模式，返回不同范围的角色列表，并构建完整的树形结构
// 设计思路：根据是否开启严格权限模式，决定返回哪些角色；然后递归加载子角色
//
// 为什么这样写：
// 1. 支持两种权限模式：严格模式限制角色只能管理子角色，非严格模式显示所有顶级角色
// 2. 先查询当前角色：根据当前角色的父角色ID判断其层级，决定返回范围
// 3. 递归加载子角色：构建完整的角色树，方便前端展示和管理
//
// 好处：
// 1. 权限隔离：严格模式下，非顶级角色只能看到和管理自己的子角色，提高安全性
// 2. 灵活配置：通过配置开关控制权限模式，适应不同的业务场景
// 3. 完整的树形结构：返回完整的角色树，前端可以直接渲染，无需额外处理
func (authorityService *AuthorityService) GetAuthorityInfoList(authorityID uint) (list []system.SysAuthority, err error) {
	// 第一步：查询当前角色信息
	// 设计说明：需要知道当前角色的父角色ID，以判断其层级
	var authority system.SysAuthority
	err = global.GVA_DB.Where("authority_id = ?", authorityID).First(&authority).Error
	if err != nil {
		return nil, err
	}

	var authorities []system.SysAuthority
	db := global.GVA_DB.Model(&system.SysAuthority{})

	// 第二步：根据权限模式决定查询范围
	// 设计说明：严格权限模式提供更强的权限隔离，非严格模式更加灵活
	if global.GVA_CONFIG.System.UseStrictAuth {
		// 严格权限模式：限制角色的管理范围
		if *authority.ParentId == 0 {
			// 顶级角色：可以查看和管理自己及其所有子角色
			// 设计说明：顶级角色拥有完整的权限，可以管理整个角色树
			err = db.Preload("DataAuthorityId").Where("authority_id = ?", authorityID).Find(&authorities).Error
		} else {
			// 非顶级角色：只能查看和管理自己的子角色
			// 设计说明：限制非顶级角色的权限范围，防止越权操作
			// 好处：提高安全性，角色只能在自己的权限范围内操作
			err = db.Debug().Preload("DataAuthorityId").Where("parent_id = ?", authorityID).Find(&authorities).Error
		}
	} else {
		// 非严格模式：所有角色都能看到所有顶级角色
		// 设计说明：查询所有顶级角色（parent_id = 0），提供完整的角色管理视图
		err = db.Preload("DataAuthorityId").Where("parent_id = ?", "0").Find(&authorities).Error
	}

	// 第三步：递归加载每个角色的子角色，构建完整的树形结构
	// 设计说明：使用递归方式加载子角色，直到没有子角色为止
	// 好处：一次调用返回完整的角色树，前端无需多次请求
	for k := range authorities {
		err = authorityService.findChildrenAuthority(&authorities[k])
	}
	return authorities, err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: GetStructAuthorityList
// @description: 递归获取角色及其所有子角色的ID列表（扁平化）
// @param: authorityID uint 起始角色ID
// @return: list []uint 角色ID列表, err error
//
// 功能说明：递归获取指定角色及其所有子角色的ID列表，返回扁平化的ID数组
// 设计思路：使用递归算法遍历角色树，收集所有角色ID
//
// 为什么这样写：
// 1. 递归遍历：角色是树形结构，需要递归方式遍历所有子节点
// 2. 扁平化返回：返回简单的ID数组，便于进行权限检查和批量操作
// 3. 特殊处理顶级角色：如果是顶级角色，将自己也加入列表
//
// 好处：
// 1. 权限范围计算：快速获取角色可以管理的所有角色ID，用于权限验证
// 2. 批量操作：返回ID列表便于进行批量查询或批量权限检查
// 3. 算法高效：递归算法清晰易懂，适合处理树形结构数据
func (authorityService *AuthorityService) GetStructAuthorityList(authorityID uint) (list []uint, err error) {
	// 第一步：查询当前角色信息
	var auth system.SysAuthority
	// 使用 _ 忽略错误，因为即使查询失败也不影响后续逻辑
	_ = global.GVA_DB.First(&auth, "authority_id = ?", authorityID).Error

	// 第二步：查询直接子角色
	var authorities []system.SysAuthority
	err = global.GVA_DB.Preload("DataAuthorityId").Where("parent_id = ?", authorityID).Find(&authorities).Error

	// 第三步：递归处理每个子角色
	if len(authorities) > 0 {
		for k := range authorities {
			// 将子角色ID加入列表
			list = append(list, authorities[k].AuthorityId)
			// 递归获取子角色的所有子角色ID
			childrenList, err := authorityService.GetStructAuthorityList(authorities[k].AuthorityId)
			// 如果递归调用成功，将子角色的ID列表合并到当前列表
			// 设计说明：即使子角色递归失败，也不影响当前层级的处理
			if err == nil {
				list = append(list, childrenList...)
			}
		}
	}

	// 第四步：如果是顶级角色，将自己也加入列表
	// 设计说明：顶级角色可以管理自己，所以需要包含自己的ID
	// 好处：确保权限检查时，角色可以对自己进行操作
	if *auth.ParentId == 0 {
		list = append(list, authorityID)
	}
	return list, err
}

// CheckAuthorityIDAuth 检查角色是否有权限操作目标角色
// 功能说明：在严格权限模式下，验证当前角色是否有权限操作目标角色
// 设计思路：通过获取当前角色管理的所有角色ID列表，判断目标角色是否在其中
//
// 为什么这样写：
// 1. 配置开关检查：非严格模式下直接通过，提高性能
// 2. 获取权限范围：使用 GetStructAuthorityList 获取可管理的角色ID列表
// 3. 线性搜索：在ID列表中查找目标角色ID，找到即说明有权限
//
// 好处：
// 1. 权限隔离：防止角色越权操作其他角色的权限，提高系统安全性
// 2. 性能优化：非严格模式下跳过检查，避免不必要的计算
// 3. 清晰的错误提示：权限不足时返回明确的错误信息，便于调试和用户理解
func (authorityService *AuthorityService) CheckAuthorityIDAuth(authorityID, targetID uint) (err error) {
	// 如果未开启严格权限模式，直接通过检查
	// 设计说明：非严格模式下不限制角色的权限范围，所有角色都可以操作任何角色
	// 好处：性能优化，避免在非严格模式下执行不必要的权限检查
	if !global.GVA_CONFIG.System.UseStrictAuth {
		return nil
	}

	// 获取当前角色可以管理的所有角色ID列表（包括子角色）
	authIDS, err := authorityService.GetStructAuthorityList(authorityID)
	if err != nil {
		return err
	}

	// 在权限范围内查找目标角色ID
	hasAuth := false
	for _, v := range authIDS {
		if v == targetID {
			hasAuth = true
			break // 找到即退出循环，提高性能
		}
	}

	// 如果目标角色不在权限范围内，返回错误
	if !hasAuth {
		return errors.New("您提交的角色ID不合法")
	}
	return nil
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: GetAuthorityInfo
// @description: 获取单个角色的详细信息（包含数据权限配置）
// @param: auth model.SysAuthority 包含角色ID的角色对象
// @return: sa system.SysAuthority 完整的角色信息, err error
//
// 功能说明：根据角色ID查询角色的完整信息，包括数据权限配置
// 设计思路：使用 Preload 预加载关联的数据权限，避免 N+1 查询问题
//
// 为什么这样写：
// 1. 使用 Preload：一次性加载关联的数据权限配置，减少数据库查询次数
// 2. 简洁的实现：逻辑简单直接，只负责查询和返回数据
//
// 好处：
// 1. 性能优化：预加载关联数据，避免后续单独查询导致多次数据库访问
// 2. 数据完整：返回的角色对象包含完整的数据权限信息，便于前端使用
func (authorityService *AuthorityService) GetAuthorityInfo(auth system.SysAuthority) (sa system.SysAuthority, err error) {
	// 查询角色信息并预加载数据权限关联
	// 设计说明：Preload("DataAuthorityId") 会在查询角色的同时加载关联的数据权限配置
	// 好处：避免 N+1 查询问题，一次性获取所有需要的数据
	err = global.GVA_DB.Preload("DataAuthorityId").Where("authority_id = ?", auth.AuthorityId).First(&sa).Error
	return sa, err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: SetDataAuthority
// @description: 设置角色的数据权限（控制角色可以访问哪些其他角色的数据）
// @param: adminAuthorityID uint 执行操作的管理员角色ID；auth system.SysAuthority 包含角色ID和数据权限配置
// @return: error
//
// 功能说明：为角色配置数据权限，决定该角色可以访问哪些其他角色的数据
// 设计思路：先进行权限验证，确保管理员有权限操作所有涉及的角色，然后更新关联关系
//
// 为什么这样写：
// 1. 权限验证：在严格权限模式下，验证管理员是否有权限操作所有涉及的角色
// 2. 批量检查：收集所有需要检查的角色ID，统一进行权限验证
// 3. 使用 Replace：替换现有的数据权限关联，而不是追加
//
// 好处：
// 1. 安全性：防止越权操作，确保管理员只能在自己权限范围内配置数据权限
// 2. 数据一致性：使用 Replace 替换关联，确保数据权限配置准确，不会遗留旧配置
// 3. 权限隔离：在严格模式下，限制角色的数据访问范围，提高系统安全性
func (authorityService *AuthorityService) SetDataAuthority(adminAuthorityID uint, auth system.SysAuthority) error {
	// 第一步：收集所有需要检查权限的角色ID
	var checkIDs []uint
	// 将被操作的角色ID加入检查列表
	checkIDs = append(checkIDs, auth.AuthorityId)
	// 将数据权限配置中的所有角色ID加入检查列表
	for i := range auth.DataAuthorityId {
		checkIDs = append(checkIDs, auth.DataAuthorityId[i].AuthorityId)
	}

	// 第二步：逐个检查管理员是否有权限操作这些角色
	// 设计说明：在严格权限模式下，必须确保管理员有权限操作所有涉及的角色
	// 好处：防止越权操作，确保安全性
	for i := range checkIDs {
		err := authorityService.CheckAuthorityIDAuth(adminAuthorityID, checkIDs[i])
		if err != nil {
			return err
		}
	}

	// 第三步：更新角色的数据权限关联
	var s system.SysAuthority
	// 先查询现有角色及其数据权限配置
	global.GVA_DB.Preload("DataAuthorityId").First(&s, "authority_id = ?", auth.AuthorityId)
	// 使用 Replace 替换现有的数据权限关联
	// 设计说明：Replace 会先删除旧的关联，再建立新的关联，确保数据一致性
	// 好处：完全替换，不会遗留旧的关联关系，避免数据混乱
	err := global.GVA_DB.Model(&s).Association("DataAuthorityId").Replace(&auth.DataAuthorityId)
	return err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: SetMenuAuthority
// @description: 设置角色的菜单权限（控制角色可以访问哪些菜单）
// @param: auth *model.SysAuthority 包含角色ID和菜单列表的角色对象
// @return: error
//
// 功能说明：为角色配置菜单权限，决定该角色可以访问哪些菜单项
// 设计思路：先查询现有角色和菜单关联，然后替换为新的菜单关联
//
// 为什么这样写：
// 1. 先查询现有角色：确保角色存在，并加载现有的菜单关联
// 2. 使用 Replace：完全替换菜单关联，而不是追加，确保配置准确
//
// 好处：
// 1. 数据一致性：使用 Replace 替换关联，确保菜单权限配置准确，不会遗留旧配置
// 2. 操作简单：接口清晰，一次调用即可完成菜单权限的完整更新
// 3. 性能优化：使用 Preload 预加载现有菜单，减少数据库查询
func (authorityService *AuthorityService) SetMenuAuthority(auth *system.SysAuthority) error {
	// 第一步：查询现有角色及其菜单关联
	var s system.SysAuthority
	// 使用 Preload 预加载现有的菜单关联，以便后续替换
	// 设计说明：即使要替换所有菜单，也需要先加载现有关联，让 GORM 知道要替换什么
	global.GVA_DB.Preload("SysBaseMenus").First(&s, "authority_id = ?", auth.AuthorityId)

	// 第二步：替换菜单关联
	// 使用 Replace 方法替换现有的菜单关联
	// 设计说明：Replace 会先删除旧的菜单关联，再建立新的菜单关联
	// 好处：完全替换，确保菜单权限配置准确，不会出现新旧配置混杂的情况
	err := global.GVA_DB.Model(&s).Association("SysBaseMenus").Replace(&auth.SysBaseMenus)
	return err
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: findChildrenAuthority
// @description: 递归查询角色的所有子角色，构建完整的角色树
// @param: authority *model.SysAuthority 要查询子角色的角色对象（会被修改，填充Children字段）
// @return: err error
//
// 功能说明：递归查询指定角色的所有子角色，构建完整的树形结构
// 设计思路：使用递归算法，先查询直接子角色，然后对每个子角色递归查询其子角色
//
// 为什么这样写：
// 1. 递归算法：角色是树形结构，递归是处理树形数据最自然的方式
// 2. 预加载数据权限：在查询子角色时同时加载数据权限配置，避免后续单独查询
// 3. 修改传入参数：直接修改 authority 对象的 Children 字段，避免返回值传递
//
// 好处：
// 1. 代码简洁：递归算法代码清晰易懂，符合树形数据结构的处理模式
// 2. 性能优化：使用 Preload 预加载关联数据，减少数据库查询次数
// 3. 完整的树结构：一次性构建完整的角色树，前端可以直接使用，无需额外处理
func (authorityService *AuthorityService) findChildrenAuthority(authority *system.SysAuthority) (err error) {
	// 第一步：查询当前角色的直接子角色
	// 使用 Preload("DataAuthorityId") 预加载子角色的数据权限配置
	// 设计说明：在查询子角色的同时加载其数据权限，避免后续单独查询导致的 N+1 问题
	// 好处：减少数据库查询次数，提高性能
	err = global.GVA_DB.Preload("DataAuthorityId").Where("parent_id = ?", authority.AuthorityId).Find(&authority.Children).Error

	// 第二步：递归查询每个子角色的子角色
	// 设计说明：如果当前角色有子角色，需要对每个子角色递归调用此方法
	// 好处：构建完整的角色树，直到没有子角色为止
	if len(authority.Children) > 0 {
		for k := range authority.Children {
			// 递归查询子角色的子角色
			// 使用指针传递，直接修改子角色的 Children 字段
			err = authorityService.findChildrenAuthority(&authority.Children[k])
		}
	}
	return err
}

// GetParentAuthorityID 获取角色的父角色ID
// 功能说明：根据角色ID查询该角色的父角色ID
// 设计思路：简单的查询操作，根据角色ID查询角色信息，返回父角色ID
//
// 为什么这样写：
// 1. 先查询角色：需要先查询角色信息才能获取父角色ID
// 2. 错误处理：如果角色不存在，直接返回错误，避免空指针引用
// 3. 返回解引用值：ParentId 是指针类型，需要解引用返回实际值
//
// 好处：
// 1. 接口简单：提供简单的接口获取父角色ID，便于其他模块使用
// 2. 错误处理：明确的错误返回，调用方可以知道查询是否成功
// 3. 类型安全：返回具体的 uint 类型，而不是指针，使用更方便
func (authorityService *AuthorityService) GetParentAuthorityID(authorityID uint) (parentID uint, err error) {
	// 第一步：查询角色信息
	var authority system.SysAuthority
	err = global.GVA_DB.Where("authority_id = ?", authorityID).First(&authority).Error
	if err != nil {
		return // 如果查询失败，直接返回错误
	}

	// 第二步：返回父角色ID
	// 设计说明：ParentId 是指针类型，需要解引用返回实际值
	// 好处：返回具体的值类型，调用方使用更方便，无需处理指针
	return *authority.ParentId, nil
}
