package system

import (
	"errors"
	"fmt"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Register
//@description: 用户注册
//@param: u model.SysUser
//@return: userInter system.SysUser, err error

// UserService 用户服务结构体
// 好处：使用结构体方法而非全局函数，便于扩展和测试，符合面向对象设计原则
type UserService struct{}

// UserServiceApp 用户服务单例实例
// 好处：全局单例模式，避免重复创建服务对象，节省内存，统一管理服务实例
var UserServiceApp = new(UserService)

// Register 用户注册功能
// 设计思路：先检查用户名唯一性，再进行密码加密和用户创建，确保数据完整性
func (userService *UserService) Register(u system.SysUser) (userInter system.SysUser, err error) {
	var user system.SysUser
	// 使用 errors.Is 判断是否为记录不存在的错误，而不是直接判断 err != nil
	// 好处：更精确的错误判断，避免误判其他类型的错误（如数据库连接错误）
	// 使用 First 查询而不是 Count，因为我们需要知道用户是否存在，Count 需要全表扫描，效率较低
	if !errors.Is(global.GVA_DB.Where("username = ?", u.Username).First(&user).Error, gorm.ErrRecordNotFound) {
		return userInter, errors.New("用户名已注册")
	}

	// 密码加密：使用 Bcrypt 算法进行哈希加密
	// 好处：Bcrypt 是专门为密码设计的哈希算法，包含随机盐值，即使相同密码也会产生不同哈希，安全性高
	// 注意：必须在数据库操作前加密，而不是在数据库中加密，确保密码不会以明文形式传输或存储
	u.Password = utils.BcryptHash(u.Password)

	// 为每个用户生成唯一标识 UUID
	// 好处：UUID 是全局唯一标识符，不依赖数据库自增ID，便于分布式系统和数据迁移
	// 即使暴露给前端也不会暴露数据库内部ID，增强安全性
	u.UUID = uuid.New()

	// 创建用户记录
	err = global.GVA_DB.Create(&u).Error
	return u, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@author: [SliverHorn](https://github.com/SliverHorn)
//@function: Login
//@description: 用户登录
//@param: u *model.SysUser
//@return: err error, userInter *model.SysUser

// Login 用户登录功能
// 设计思路：先验证数据库连接，再查询用户信息并验证密码，最后设置用户默认路由
func (userService *UserService) Login(u *system.SysUser) (userInter *system.SysUser, err error) {
	// 前置检查：确保数据库已初始化
	// 好处：在数据库操作前进行校验，避免空指针异常，提前发现问题，给出明确的错误提示
	if nil == global.GVA_DB {
		return nil, fmt.Errorf("db not init")
	}

	var user system.SysUser
	// 使用 Preload 预加载关联数据（Authorities 和 Authority）
	// 好处：避免 N+1 查询问题，一次性加载所有需要的数据，减少数据库查询次数，提高性能
	// 如果使用懒加载，每次访问权限信息都会触发一次查询，性能较差
	err = global.GVA_DB.Where("username = ?", u.Username).Preload("Authorities").Preload("Authority").First(&user).Error

	if err == nil {
		// 密码验证：使用 BcryptCheck 比较明文密码和哈希密码
		// 好处：Bcrypt 的比较函数是时间安全的，可以防止时序攻击（timing attack）
		// 直接比较哈希值可能导致安全漏洞，使用专门的比较函数更安全
		if ok := utils.BcryptCheck(u.Password, user.Password); !ok {
			return nil, errors.New("密码错误")
		}

		// 设置用户权限对应的默认路由
		// 好处：在登录时统一处理路由信息，确保用户登录后能正确跳转到对应的默认页面
		MenuServiceApp.UserAuthorityDefaultRouter(&user)
	}
	return &user, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: ChangePassword
//@description: 修改用户密码
//@param: u *model.SysUser, newPassword string
//@return: err error

// ChangePassword 修改用户密码功能
// 设计思路：先验证原密码正确性，再更新为新密码，确保安全性
func (userService *UserService) ChangePassword(u *system.SysUser, newPassword string) (err error) {
	var user system.SysUser
	// 使用 Select 指定只查询需要的字段（id, password）
	// 好处：减少数据传输量，提高查询效率，避免查询不必要的敏感字段（如密码哈希），增强安全性
	// 同时减少内存占用，特别是在字段较多的大表中，性能提升明显
	err = global.GVA_DB.Select("id, password").Where("id = ?", u.ID).First(&user).Error
	if err != nil {
		return err
	}

	// 验证原密码是否正确
	// 好处：防止未授权用户修改密码，确保只有知道原密码的用户才能修改，增强账户安全性
	if ok := utils.BcryptCheck(u.Password, user.Password); !ok {
		return errors.New("原密码错误")
	}

	// 对新密码进行加密
	pwd := utils.BcryptHash(newPassword)

	// 使用 Model + Update 进行更新，而不是 Save
	// 好处：只更新指定字段（password），不会影响其他字段，避免误操作覆盖其他数据
	// 使用 Model(&user) 可以复用已查询到的用户对象，减少一次查询
	err = global.GVA_DB.Model(&user).Update("password", pwd).Error
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetUserInfoList
//@description: 分页获取数据
//@param: info request.PageInfo
//@return: err error, list interface{}, total int64

// GetUserInfoList 分页获取用户列表
// 设计思路：构建动态查询条件，先统计总数，再分页查询，支持多条件组合搜索
func (userService *UserService) GetUserInfoList(info systemReq.GetUserList) (list interface{}, total int64, err error) {
	// 计算分页参数
	// 好处：将分页逻辑封装在服务层，前端只需传入页码和每页大小，降低耦合度
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)

	// 使用 Model 方法获取基础查询对象
	// 好处：复用查询对象，可以通过链式调用动态构建查询条件，代码更清晰
	db := global.GVA_DB.Model(&system.SysUser{})
	var userList []system.SysUser

	// 动态构建查询条件：只有当搜索条件不为空时才添加 WHERE 子句
	// 好处：避免无意义的查询条件，提高查询效率；支持多条件组合搜索，灵活性高
	// 使用链式调用构建查询，代码可读性强，易于维护和扩展
	if info.NickName != "" {
		// 使用 LIKE 进行模糊查询，前后都加 % 表示包含匹配
		// 注意：虽然 LIKE 可能影响索引使用，但对于搜索功能是必需的，可以在这些字段上建立全文索引优化
		db = db.Where("nick_name LIKE ?", "%"+info.NickName+"%")
	}
	if info.Phone != "" {
		db = db.Where("phone LIKE ?", "%"+info.Phone+"%")
	}
	if info.Username != "" {
		db = db.Where("username LIKE ?", "%"+info.Username+"%")
	}
	if info.Email != "" {
		db = db.Where("email LIKE ?", "%"+info.Email+"%")
	}

	// 先统计符合条件的总记录数
	// 好处：在分页查询前获取总数，前端可以正确显示总页数和进行分页导航
	// 使用 Count 而不是先查询所有数据再计算长度，性能更好
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	// 执行分页查询，同时预加载权限信息
	// 使用 Limit 和 Offset 进行分页：简单直观，适合大多数场景
	// 注意：在大数据量场景下，Offset 性能可能较差，可以考虑使用游标分页（cursor-based pagination）
	// Preload 预加载关联数据，避免 N+1 查询问题
	err = db.Limit(limit).Offset(offset).Preload("Authorities").Preload("Authority").Find(&userList).Error
	return userList, total, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetUserAuthority
//@description: 设置一个用户的权限
//@param: uuid uuid.UUID, authorityId string
//@return: err error

// SetUserAuthority 设置单个用户的权限（切换用户的主要角色）
// 设计思路：先验证用户是否拥有该角色，再验证角色的默认路由是否存在，最后更新用户的主要角色
func (userService *UserService) SetUserAuthority(id uint, authorityId uint) (err error) {
	// 第一步：验证用户是否已经拥有该角色权限
	// 好处：防止将用户切换到不存在的角色，确保数据一致性和业务逻辑正确性
	// 查询用户角色关联表，确认用户与该角色的关联关系是否存在
	assignErr := global.GVA_DB.Where("sys_user_id = ? AND sys_authority_authority_id = ?", id, authorityId).First(&system.SysUserAuthority{}).Error
	if errors.Is(assignErr, gorm.ErrRecordNotFound) {
		return errors.New("该用户无此角色")
	}

	// 第二步：查询权限（角色）信息，获取默认路由
	var authority system.SysAuthority
	err = global.GVA_DB.Where("authority_id = ?", authorityId).First(&authority).Error
	if err != nil {
		return err
	}

	// 第三步：查询该角色关联的所有菜单
	var authorityMenu []system.SysAuthorityMenu
	var authorityMenuIDs []string
	err = global.GVA_DB.Where("sys_authority_authority_id = ?", authorityId).Find(&authorityMenu).Error
	if err != nil {
		return err
	}

	// 提取菜单ID列表
	// 好处：先提取ID，再使用 IN 查询，比循环查询效率高，减少数据库交互次数
	for i := range authorityMenu {
		authorityMenuIDs = append(authorityMenuIDs, authorityMenu[i].MenuId)
	}

	// 第四步：查询菜单详情，预加载参数信息
	var authorityMenus []system.SysBaseMenu
	// 使用 Preload("Parameters") 预加载菜单参数，避免后续访问时触发额外查询
	// 使用 WHERE id IN (?) 批量查询，比循环查询效率高
	err = global.GVA_DB.Preload("Parameters").Where("id in (?)", authorityMenuIDs).Find(&authorityMenus).Error
	if err != nil {
		return err
	}

	// 第五步：验证默认路由是否存在于该角色的菜单中
	// 好处：确保切换角色后，用户能够正常访问默认页面，避免切换到无法使用的角色
	hasMenu := false
	for i := range authorityMenus {
		if authorityMenus[i].Name == authority.DefaultRouter {
			hasMenu = true
			break // 找到后立即退出循环，提高效率
		}
	}
	if !hasMenu {
		return errors.New("找不到默认路由,无法切换本角色")
	}

	// 第六步：更新用户的主要角色ID
	// 使用 Update 只更新 authority_id 字段，不影响其他字段
	err = global.GVA_DB.Model(&system.SysUser{}).Where("id = ?", id).Update("authority_id", authorityId).Error
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetUserAuthorities
//@description: 设置一个用户的权限
//@param: id uint, authorityIds []string
//@return: err error

// SetUserAuthorities 批量设置用户的多个角色权限
// 设计思路：使用事务确保数据一致性，先删除旧权限，再批量创建新权限，最后更新主要角色
func (userService *UserService) SetUserAuthorities(adminAuthorityID, id uint, authorityIds []uint) (err error) {
	// 使用事务包装整个操作
	// 好处：确保所有操作要么全部成功，要么全部回滚，避免数据不一致（如删除了旧权限但新权限创建失败）
	// 事务保证了原子性（Atomicity），符合 ACID 原则
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 第一步：查询用户是否存在
		// 使用事务中的 tx 而不是 global.GVA_DB，确保操作在同一个事务中
		var user system.SysUser
		TxErr := tx.Where("id = ?", id).First(&user).Error
		if TxErr != nil {
			global.GVA_LOG.Debug(TxErr.Error())
			return errors.New("查询用户数据失败")
		}

		// 第二步：删除用户的所有旧权限关联
		// 好处：采用"先删后加"的策略，简化逻辑，避免复杂的差异计算（哪些删除、哪些新增、哪些保留）
		// 虽然可能有性能损失，但逻辑清晰，维护成本低
		TxErr = tx.Delete(&[]system.SysUserAuthority{}, "sys_user_id = ?", id).Error
		if TxErr != nil {
			return TxErr // 返回错误会自动触发事务回滚
		}

		// 第三步：验证并构建新的权限关联列表
		var useAuthority []system.SysUserAuthority
		for _, v := range authorityIds {
			// 权限检查：验证管理员是否有权限分配该角色
			// 好处：实现权限分级管理，防止低权限管理员分配高权限角色，增强安全性
			e := AuthorityServiceApp.CheckAuthorityIDAuth(adminAuthorityID, v)
			if e != nil {
				return e // 权限检查失败，回滚事务
			}
			// 构建权限关联对象
			useAuthority = append(useAuthority, system.SysUserAuthority{
				SysUserId: id, SysAuthorityAuthorityId: v,
			})
		}

		// 第四步：批量创建新的权限关联
		// 好处：使用批量插入（Create 传入切片），比循环插入效率高，减少数据库交互次数
		TxErr = tx.Create(&useAuthority).Error
		if TxErr != nil {
			return TxErr
		}

		// 第五步：更新用户的主要角色为列表中的第一个
		// 好处：保持主要角色的同步更新，确保用户的主要角色与权限列表一致
		TxErr = tx.Model(&user).Update("authority_id", authorityIds[0]).Error
		if TxErr != nil {
			return TxErr
		}

		// 返回 nil 表示事务成功，会自动提交
		// 返回非 nil 错误会自动触发事务回滚
		return nil
	})
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteUser
//@description: 删除用户
//@param: id float64
//@return: err error

// DeleteUser 删除用户功能
// 设计思路：使用事务确保用户及其关联数据的一致性删除，防止产生孤立数据
func (userService *UserService) DeleteUser(id int) (err error) {
	// 使用事务包装删除操作
	// 好处：确保用户数据和关联的权限数据要么全部删除，要么全部保留，避免产生数据不一致
	// 如果只删除用户表数据而关联表删除失败，会产生孤立数据（orphan data）
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 第一步：删除用户主表数据
		// 使用软删除（如果模型定义了 DeletedAt 字段）或硬删除
		// 好处：GORM 的 Delete 方法会检查模型定义，支持软删除，保留历史数据便于审计
		if err := tx.Where("id = ?", id).Delete(&system.SysUser{}).Error; err != nil {
			return err // 删除失败，触发事务回滚
		}

		// 第二步：删除用户的权限关联数据
		// 好处：级联删除关联数据，避免外键约束错误，保持数据完整性
		// 注意：如果数据库设置了外键级联删除（ON DELETE CASCADE），这一步可能不是必需的
		// 但显式删除更清晰，不依赖数据库配置，提高代码可移植性
		if err := tx.Delete(&[]system.SysUserAuthority{}, "sys_user_id = ?", id).Error; err != nil {
			return err
		}

		// 返回 nil 提交事务
		return nil
	})
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetUserInfo
//@description: 设置用户信息
//@param: reqUser model.SysUser
//@return: err error, user model.SysUser

// SetUserInfo 管理员设置用户信息（受控更新）
// 设计思路：使用 Select 限制可更新字段，防止敏感字段被修改，增强安全性
func (userService *UserService) SetUserInfo(req system.SysUser) error {
	return global.GVA_DB.Model(&system.SysUser{}).
		// 使用 Select 明确指定允许更新的字段（白名单机制）
		// 好处：即使请求中包含其他字段（如 password、authority_id），也不会被更新
		// 这是一种安全最佳实践，防止恶意用户通过修改请求参数来提升权限或修改敏感信息
		// 只允许更新用户的基本信息和状态，不允许修改密码、权限等敏感字段
		Select("updated_at", "nick_name", "header_img", "phone", "email", "enable").
		Where("id=?", req.ID).
		// 使用 Updates 配合 map，只更新指定字段
		// 使用 map[string]interface{} 而不是结构体，避免零值字段被误更新
		// 好处：精确控制每个字段的更新，避免将字段更新为意外的零值
		Updates(map[string]interface{}{
			"updated_at": time.Now(), // 显式更新时间戳，确保记录最后修改时间
			"nick_name":  req.NickName,
			"header_img": req.HeaderImg,
			"phone":      req.Phone,
			"email":      req.Email,
			"enable":     req.Enable, // 允许启用/禁用用户账户
		}).Error
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetSelfInfo
//@description: 设置用户信息
//@param: reqUser model.SysUser
//@return: err error, user model.SysUser

// SetSelfInfo 用户自己设置个人信息（自由更新）
// 设计思路：用户修改自己的信息，相比 SetUserInfo 更宽松，但仍需在模型层或API层控制可更新字段
func (userService *UserService) SetSelfInfo(req system.SysUser) error {
	// 使用 Updates 方法，传入结构体
	// 好处：代码简洁，GORM 会自动忽略零值字段，只更新非零值字段
	// 注意：这种方法需要在模型定义中使用 gorm 标签控制哪些字段可以被更新
	// 或者在前置验证中过滤掉不允许修改的字段（如 password、authority_id、uuid 等）
	// 相比 SetUserInfo 的 Select 白名单，这种方式更灵活但需要额外的安全检查
	return global.GVA_DB.Model(&system.SysUser{}).
		Where("id=?", req.ID).
		Updates(req).Error
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetSelfSetting
//@description: 设置用户配置
//@param: req datatypes.JSON, uid uint
//@return: err error

// SetSelfSetting 设置用户的个性化配置
// 设计思路：使用 JSON 类型字段存储灵活的用户配置，支持任意结构的配置数据
func (userService *UserService) SetSelfSetting(req common.JSONMap, uid uint) error {
	// 使用 Update 方法更新单个 JSON 字段
	// 好处：JSON 类型字段可以存储任意结构的配置数据，不需要为每个配置项添加独立字段
	// 这样可以灵活扩展用户配置项，不需要频繁修改数据库表结构
	// 适合存储用户偏好设置、界面配置等非结构化数据
	return global.GVA_DB.Model(&system.SysUser{}).Where("id = ?", uid).Update("origin_setting", req).Error
}

//@author: [piexlmax](https://github.com/piexlmax)
//@author: [SliverHorn](https://github.com/SliverHorn)
//@function: GetUserInfo
//@description: 获取用户信息
//@param: uuid uuid.UUID
//@return: err error, user system.SysUser

// GetUserInfo 通过 UUID 获取用户完整信息
// 设计思路：使用 UUID 而非 ID 查询，预加载关联数据，设置默认路由
func (userService *UserService) GetUserInfo(uuid uuid.UUID) (user system.SysUser, err error) {
	var reqUser system.SysUser
	// 使用 UUID 查询而不是 ID
	// 好处：UUID 是对外暴露的安全标识符，不会暴露数据库内部ID，增强安全性
	// 即使 UUID 被截获，也无法通过它直接推断其他用户的信息或进行批量查询
	// 使用 Preload 预加载权限信息，避免 N+1 查询问题
	err = global.GVA_DB.Preload("Authorities").Preload("Authority").First(&reqUser, "uuid = ?", uuid).Error
	if err != nil {
		return reqUser, err
	}

	// 设置用户权限对应的默认路由
	// 好处：确保返回的用户信息包含正确的默认路由，前端可以直接使用，无需额外处理
	MenuServiceApp.UserAuthorityDefaultRouter(&reqUser)
	return reqUser, err
}

//@author: [SliverHorn](https://github.com/SliverHorn)
//@function: FindUserById
//@description: 通过id获取用户信息
//@param: id int
//@return: err error, user *model.SysUser

// FindUserById 通过数据库ID查询用户（内部使用）
// 设计思路：简单的ID查询，返回指针，适合内部服务调用
func (userService *UserService) FindUserById(id int) (user *system.SysUser, err error) {
	var u system.SysUser
	// 使用 ID 直接查询，效率最高（ID 是主键，有索引）
	// 好处：主键查询性能最优，不需要额外的索引查找
	// 注意：此方法只查询基本用户信息，不预加载关联数据，适合只需要用户基本信息的场景
	err = global.GVA_DB.Where("id = ?", id).First(&u).Error
	return &u, err
}

//@author: [SliverHorn](https://github.com/SliverHorn)
//@function: FindUserByUuid
//@description: 通过uuid获取用户信息
//@param: uuid string
//@return: err error, user *model.SysUser

// FindUserByUuid 通过 UUID 查询用户（对外接口使用）
// 设计思路：使用 UUID 查询，将数据库错误转换为业务错误，提供更友好的错误信息
func (userService *UserService) FindUserByUuid(uuid string) (user *system.SysUser, err error) {
	var u system.SysUser
	// 使用 UUID 查询，适合对外暴露的接口
	// 好处：UUID 是安全的对外标识符，不暴露内部ID
	if err = global.GVA_DB.Where("uuid = ?", uuid).First(&u).Error; err != nil {
		// 将数据库层的错误（如 gorm.ErrRecordNotFound）转换为业务层错误
		// 好处：对外屏蔽数据库实现细节，提供统一的业务错误信息，提高代码的可维护性
		// 调用方不需要关心是数据库连接错误还是记录不存在，统一处理为"用户不存在"
		return &u, errors.New("用户不存在")
	}
	return &u, nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: ResetPassword
//@description: 修改用户密码
//@param: ID uint
//@return: err error

// ResetPassword 重置用户密码（管理员操作，不需要原密码）
// 设计思路：直接更新密码，不需要验证原密码，适合管理员重置或忘记密码场景
func (userService *UserService) ResetPassword(ID uint, password string) (err error) {
	// 直接更新密码，不验证原密码
	// 好处：适合管理员重置用户密码或用户忘记密码后通过验证码重置的场景
	// 与 ChangePassword 的区别：ChangePassword 需要原密码验证（用户主动修改），
	// ResetPassword 不需要原密码（管理员操作或忘记密码场景）
	// 使用 BcryptHash 加密新密码，确保密码安全存储
	err = global.GVA_DB.Model(&system.SysUser{}).Where("id = ?", ID).Update("password", utils.BcryptHash(password)).Error
	return err
}
