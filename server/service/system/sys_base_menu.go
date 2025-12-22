package system

import (
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"gorm.io/gorm"
)

// BaseMenuService 基础菜单服务结构体
// 采用空结构体设计，不存储状态数据，所有方法都是无状态的，便于并发安全使用
type BaseMenuService struct{}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteBaseMenu
//@description: 删除基础路由
//@param: id float64
//@return: err error

// BaseMenuServiceApp 服务实例的单例对象
// 使用单例模式，全局共享同一个服务实例，减少内存占用，提高性能
var BaseMenuServiceApp = new(BaseMenuService)

// DeleteBaseMenu 删除基础菜单
// 删除操作涉及多个关联表，需要保证数据一致性，因此采用多层校验 + 事务的方式
func (baseMenuService *BaseMenuService) DeleteBaseMenu(id int) (err error) {
	// 第一步：检查是否存在子菜单
	// 原因：菜单采用树形结构，如果存在子菜单，直接删除会导致数据不完整
	// 好处：防止级联删除导致的业务逻辑错误，确保菜单层级关系的完整性
	err = global.GVA_DB.First(&system.SysBaseMenu{}, "parent_id = ?", id).Error
	if err == nil {
		return errors.New("此菜单存在子菜单不可删除")
	}
	// 注意：这里 err != nil 时继续执行是正常的，因为查询不到子菜单说明可以删除

	// 第二步：检查菜单记录是否存在
	// 原因：需要获取菜单的 Name 字段用于后续检查，同时避免删除不存在的记录
	// 好处：提前验证，返回明确的错误信息，提升用户体验
	var menu system.SysBaseMenu
	err = global.GVA_DB.First(&menu, id).Error
	if err != nil {
		return errors.New("记录不存在")
	}

	// 第三步：检查是否有角色正在使用此菜单作为首页
	// 原因：如果角色将菜单设置为默认首页，删除后会导致角色配置异常
	// 好处：防止删除正在使用的菜单，避免系统运行时错误，保证业务连续性
	err = global.GVA_DB.First(&system.SysAuthority{}, "default_router = ?", menu.Name).Error
	if err == nil {
		return errors.New("此菜单有角色正在作为首页，不可删除")
	}

	// 第四步：使用事务删除菜单及其所有关联数据
	// 原因：菜单与多个表存在关联关系（参数、按钮、权限等），必须保证要么全部删除成功，要么全部回滚
	// 好处：
	// 1. 保证数据一致性，避免出现"孤儿数据"（菜单删除了但关联数据还存在）
	// 2. 原子性操作，任何一步失败都会回滚，不会留下部分删除的脏数据
	// 3. 提高代码可维护性，所有删除操作集中在一个事务中
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 删除主菜单记录
		err = tx.Delete(&system.SysBaseMenu{}, "id = ?", id).Error
		if err != nil {
			return err
		}

		// 删除菜单参数（1对多关系）
		// 使用事务 tx 而不是 global.GVA_DB，确保在同一个事务中执行
		err = tx.Delete(&system.SysBaseMenuParameter{}, "sys_base_menu_id = ?", id).Error
		if err != nil {
			return err
		}

		// 删除菜单按钮（1对多关系）
		err = tx.Delete(&system.SysBaseMenuBtn{}, "sys_base_menu_id = ?", id).Error
		if err != nil {
			return err
		}

		// 删除权限按钮关联（多对多关系的中间表数据）
		// 清理角色对菜单按钮的权限配置
		err = tx.Delete(&system.SysAuthorityBtn{}, "sys_menu_id = ?", id).Error
		if err != nil {
			return err
		}

		// 删除权限菜单关联（多对多关系的中间表数据）
		// 清理角色对菜单的访问权限配置
		err = tx.Delete(&system.SysAuthorityMenu{}, "sys_base_menu_id = ?", id).Error
		if err != nil {
			return err
		}
		return nil
	})
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: UpdateBaseMenu
//@description: 更新路由
//@param: menu model.SysBaseMenu
//@return: err error

// UpdateBaseMenu 更新基础菜单
// 更新操作需要处理主表和关联表的数据，采用"先删除关联数据再创建"的策略保证数据一致性
func (baseMenuService *BaseMenuService) UpdateBaseMenu(menu system.SysBaseMenu) (err error) {
	var oldMenu system.SysBaseMenu

	// 使用 map[string]interface{} 构建更新字段映射
	// 原因：
	// 1. GORM 的 Updates 方法使用 map 时，只会更新 map 中指定的字段
	// 2. 如果直接使用 struct，零值字段（false, 0, ""）不会被更新，可能导致无法将字段重置为零值
	// 好处：
	// 1. 精确控制需要更新的字段，避免误更新
	// 2. 支持将字段设置为零值（如将 hidden 设置为 false）
	// 3. 只更新提供的字段，提高更新效率
	upDateMap := make(map[string]interface{})
	upDateMap["keep_alive"] = menu.KeepAlive
	upDateMap["transition_type"] = menu.TransitionType
	upDateMap["close_tab"] = menu.CloseTab
	upDateMap["default_menu"] = menu.DefaultMenu
	upDateMap["parent_id"] = menu.ParentId
	upDateMap["path"] = menu.Path
	upDateMap["name"] = menu.Name
	upDateMap["hidden"] = menu.Hidden
	upDateMap["component"] = menu.Component
	upDateMap["title"] = menu.Title
	upDateMap["active_name"] = menu.ActiveName
	upDateMap["icon"] = menu.Icon
	upDateMap["sort"] = menu.Sort

	// 使用事务保证整个更新过程的原子性
	// 原因：更新操作涉及主表和多个关联表，必须保证要么全部成功，要么全部回滚
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 获取原始菜单数据，用于后续的名称重复检查和更新操作
		tx.Where("id = ?", menu.ID).Find(&oldMenu)

		// 如果菜单名称发生变化，需要检查新名称是否已存在
		// 原因：菜单名称通常是业务唯一标识，不能重复
		// 好处：防止名称冲突，保证业务逻辑的正确性
		if oldMenu.Name != menu.Name {
			// 查询是否存在其他菜单使用相同名称（排除当前菜单本身）
			// 使用 errors.Is 检查是否为 ErrRecordNotFound，如果不是说明找到了同名菜单
			if !errors.Is(tx.Where("id <> ? AND name = ?", menu.ID, menu.Name).First(&system.SysBaseMenu{}).Error, gorm.ErrRecordNotFound) {
				global.GVA_LOG.Debug("存在相同name修改失败")
				return errors.New("存在相同name修改失败")
			}
		}

		// 删除旧的菜单参数关联数据（使用 Unscoped 彻底删除，而非软删除）
		// 原因：菜单参数是 1 对多关系，采用"先删后增"策略来更新
		// 好处：
		// 1. 简化更新逻辑，不需要逐个比对新增、删除、修改
		// 2. 保证数据完全替换，避免遗漏删除不需要的参数
		// 3. Unscoped().Delete() 确保物理删除，避免软删除导致的冗余数据
		txErr := tx.Unscoped().Delete(&system.SysBaseMenuParameter{}, "sys_base_menu_id = ?", menu.ID).Error
		if txErr != nil {
			global.GVA_LOG.Debug(txErr.Error())
			return txErr
		}

		// 删除旧的菜单按钮关联数据（同样使用 Unscoped 彻底删除）
		txErr = tx.Unscoped().Delete(&system.SysBaseMenuBtn{}, "sys_base_menu_id = ?", menu.ID).Error
		if txErr != nil {
			global.GVA_LOG.Debug(txErr.Error())
			return txErr
		}

		// 如果提供了新的参数数据，则创建新的参数记录
		// 原因：只有在参数数组不为空时才创建，避免不必要的数据库操作
		if len(menu.Parameters) > 0 {
			// 设置每个参数的菜单ID关联
			// 使用 range 只遍历索引，修改原数组元素，避免复制
			for k := range menu.Parameters {
				menu.Parameters[k].SysBaseMenuID = menu.ID
			}
			txErr = tx.Create(&menu.Parameters).Error
			if txErr != nil {
				global.GVA_LOG.Debug(txErr.Error())
				return txErr
			}
		}

		// 如果提供了新的按钮数据，则创建新的按钮记录
		if len(menu.MenuBtn) > 0 {
			// 设置每个按钮的菜单ID关联
			for k := range menu.MenuBtn {
				menu.MenuBtn[k].SysBaseMenuID = menu.ID
			}
			txErr = tx.Create(&menu.MenuBtn).Error
			if txErr != nil {
				global.GVA_LOG.Debug(txErr.Error())
				return txErr
			}
		}

		// 最后更新主菜单记录
		// 放在最后执行，因为关联数据的创建依赖菜单ID，如果菜单更新失败，前面的操作也会回滚
		txErr = tx.Model(&oldMenu).Updates(upDateMap).Error
		if txErr != nil {
			global.GVA_LOG.Debug(txErr.Error())
			return txErr
		}
		return nil
	})
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetBaseMenuById
//@description: 返回当前选中menu
//@param: id float64
//@return: menu system.SysBaseMenu, err error

// GetBaseMenuById 根据ID获取基础菜单及其关联数据
// 使用 Preload 预加载关联数据，避免 N+1 查询问题
func (baseMenuService *BaseMenuService) GetBaseMenuById(id int) (menu system.SysBaseMenu, err error) {
	// 使用 Preload 预加载关联数据
	// 原因：
	// 1. MenuBtn 和 Parameters 是菜单的关联数据，通常查询菜单时需要一并获取
	// 2. 如果不用 Preload，后续访问关联数据时会触发额外的查询（N+1 问题）
	// 好处：
	// 1. 性能优化：一次性加载所有关联数据，减少数据库查询次数
	//    - 不使用 Preload：1次主查询 + N次关联查询（N为关联数据数量）
	//    - 使用 Preload：1次主查询 + 1次关联查询（使用 IN 查询批量加载）
	// 2. 代码简洁：直接访问 menu.MenuBtn 和 menu.Parameters 即可，无需手动查询
	// 3. 数据完整性：保证返回的菜单对象包含完整的关联数据，避免空指针异常
	err = global.GVA_DB.Preload("MenuBtn").Preload("Parameters").Where("id = ?", id).First(&menu).Error
	return
}
