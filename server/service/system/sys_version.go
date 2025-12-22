package system

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"gorm.io/gorm"
)

// SysVersionService 版本管理服务结构体
// 采用空结构体作为接收器，节省内存且语义清晰
// 好处：所有版本管理相关的业务逻辑都集中在这个服务中，便于维护和扩展
type SysVersionService struct{}

// CreateSysVersion 创建版本管理记录
// 参数说明：
//   - ctx: context上下文，用于传递请求级别的数据（如超时控制、取消信号、追踪信息等）
//     好处：支持请求链路追踪、超时控制、优雅取消等高级特性
//   - sysVersion: 版本管理记录指针，使用指针避免值拷贝，提高性能
//
// 返回值：error，统一错误处理，便于上层调用者处理异常
// Author [yourname](https://github.com/yourname)
func (sysVersionService *SysVersionService) CreateSysVersion(ctx context.Context, sysVersion *system.SysVersion) (err error) {
	// 直接使用 GORM 的 Create 方法，简洁高效
	// 使用 .Error 获取错误，符合 GORM 的错误处理模式
	err = global.GVA_DB.Create(sysVersion).Error
	return err
}

// DeleteSysVersion 删除版本管理记录
// 设计说明：
//   - 使用软删除（如果模型定义了 DeletedAt 字段）或硬删除
//   - 使用 "id = ?" 条件查询，避免 SQL 注入风险（GORM 的参数化查询）
//   - 传入模型结构体作为删除目标，类型安全
//
// 好处：参数化查询防止 SQL 注入，类型安全，代码简洁
// Author [yourname](https://github.com/yourname)
func (sysVersionService *SysVersionService) DeleteSysVersion(ctx context.Context, ID string) (err error) {
	err = global.GVA_DB.Delete(&system.SysVersion{}, "id = ?", ID).Error
	return err
}

// DeleteSysVersionByIds 批量删除版本管理记录
// 设计说明：
//   - 使用 "id in ?" 进行批量删除，一次 SQL 操作完成，性能优于循环删除
//   - GORM 会自动将切片转换为 IN 查询，如：WHERE id IN (1,2,3)
//
// 好处：
//   - 减少数据库交互次数，提高性能
//   - 原子性操作，要么全部删除成功，要么全部失败
//   - 代码简洁，避免手动拼接 SQL
//
// Author [yourname](https://github.com/yourname)
func (sysVersionService *SysVersionService) DeleteSysVersionByIds(ctx context.Context, IDs []string) (err error) {
	err = global.GVA_DB.Where("id in ?", IDs).Delete(&system.SysVersion{}).Error
	return err
}

// GetSysVersion 根据ID获取版本管理记录
// 设计说明：
//   - 使用 First 方法，如果记录不存在会返回 gorm.ErrRecordNotFound 错误
//   - 返回值使用命名返回值，代码更简洁
//   - 使用值类型返回，避免空指针问题
//
// 好处：
//   - 明确的错误处理，调用者可以区分"记录不存在"和"其他错误"
//   - 类型安全，返回值不会是 nil
//
// Author [yourname](https://github.com/yourname)
func (sysVersionService *SysVersionService) GetSysVersion(ctx context.Context, ID string) (sysVersion system.SysVersion, err error) {
	err = global.GVA_DB.Where("id = ?", ID).First(&sysVersion).Error
	return
}

// GetSysVersionInfoList 分页获取版本管理记录
// 设计说明：
//  1. 链式查询构建：使用 db.Model() 创建查询构建器，支持链式调用添加条件
//  2. 条件搜索：动态添加 WHERE 条件，只添加有值的搜索条件，避免无效查询
//  3. 分页计算：offset = (page - 1) * pageSize，标准的数据库分页公式
//  4. 先统计总数：在分页前先 Count，确保返回准确的 total 值
//  5. 条件判断：limit != 0 时才添加分页，支持获取全部数据（limit=0）
//
// 好处：
//   - 链式查询：代码可读性强，易于维护和扩展
//   - 动态条件：避免拼接 SQL，防止注入，只查询需要的条件
//   - 返回总数：前端可以计算总页数，实现完整的分页功能
//   - 灵活分页：支持分页和全量查询两种模式
//
// Author [yourname](https://github.com/yourname)
func (sysVersionService *SysVersionService) GetSysVersionInfoList(ctx context.Context, info systemReq.SysVersionSearch) (list []system.SysVersion, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建查询构建器，使用 Model 指定查询的表
	// 好处：类型安全，GORM 会自动推断表名，避免硬编码表名
	db := global.GVA_DB.Model(&system.SysVersion{})
	var sysVersions []system.SysVersion

	// 动态添加搜索条件：只添加有值的条件，避免无效查询
	// 时间范围查询：使用 BETWEEN 进行范围查询，性能优于多个 OR 条件
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	// 版本名称模糊查询：使用 LIKE 和通配符 %，支持部分匹配
	// 先判断指针不为 nil 再判断值不为空，避免空指针异常
	if info.VersionName != nil && *info.VersionName != "" {
		db = db.Where("version_name LIKE ?", "%"+*info.VersionName+"%")
	}
	// 版本号精确查询：使用 = 进行精确匹配，性能优于 LIKE
	if info.VersionCode != nil && *info.VersionCode != "" {
		db = db.Where("version_code = ?", *info.VersionCode)
	}

	// 先统计总数：在分页前执行 Count，确保返回准确的 total
	// 好处：前端可以根据 total 和 pageSize 计算总页数
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	// 条件分页：只有当 limit != 0 时才添加分页限制
	// 好处：支持两种模式：limit=0 获取全部数据，limit>0 分页查询
	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	// 执行查询：使用 Find 方法查询多条记录
	err = db.Find(&sysVersions).Error
	return sysVersions, total, err
}

// GetSysVersionPublic 获取公开的版本管理数据
// 此方法为获取数据源定义的数据
// 请自行实现
func (sysVersionService *SysVersionService) GetSysVersionPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}

// GetMenusByIds 根据ID列表获取菜单数据
// 设计说明：
//   - 使用 "id in ?" 批量查询，一次 SQL 获取多条记录，性能优于循环查询
//   - 使用 Preload 预加载关联数据（Parameters 和 MenuBtn），避免 N+1 查询问题
//   - Preload 会在一次查询中加载所有关联数据，而不是为每条菜单单独查询
//
// 好处：
//   - 批量查询：减少数据库交互次数，提高性能
//   - 预加载关联：避免 N+1 查询问题（如果不用 Preload，查询 N 条菜单会产生 N+1 次查询）
//   - 数据完整性：一次性获取菜单及其关联的参数和按钮，数据完整
func (sysVersionService *SysVersionService) GetMenusByIds(ctx context.Context, ids []uint) (menus []system.SysBaseMenu, err error) {
	err = global.GVA_DB.Where("id in ?", ids).Preload("Parameters").Preload("MenuBtn").Find(&menus).Error
	return
}

// GetApisByIds 根据ID列表获取API数据
// 设计说明：
//   - API 数据通常没有复杂的关联关系，所以不需要 Preload
//   - 使用批量查询提高性能
//
// 好处：简洁高效，只查询需要的数据
func (sysVersionService *SysVersionService) GetApisByIds(ctx context.Context, ids []uint) (apis []system.SysApi, err error) {
	err = global.GVA_DB.Where("id in ?", ids).Find(&apis).Error
	return
}

// GetDictionariesByIds 根据ID列表获取字典数据
// 设计说明：
//   - 字典数据包含字典详情（SysDictionaryDetails），使用 Preload 预加载
//   - 避免 N+1 查询：如果不用 Preload，查询 N 个字典会产生 N+1 次查询（1次查字典 + N次查详情）
//
// 好处：
//   - 一次查询获取字典及其所有详情，性能优异
//   - 数据完整，便于前端直接使用
func (sysVersionService *SysVersionService) GetDictionariesByIds(ctx context.Context, ids []uint) (dictionaries []system.SysDictionary, err error) {
	err = global.GVA_DB.Where("id in ?", ids).Preload("SysDictionaryDetails").Find(&dictionaries).Error
	return
}

// ImportMenus 导入菜单数据
// 设计说明：
//   - 使用数据库事务（Transaction）确保数据一致性
//   - 如果导入过程中任何一步失败，整个事务回滚，保证数据完整性
//   - 使用递归函数处理菜单的树形结构
//
// 好处：
//   - 事务保证：要么全部导入成功，要么全部回滚，不会出现部分导入的情况
//   - 数据一致性：避免因异常导致的数据不完整问题
//   - 支持树形结构：菜单通常是树形结构，递归处理是最自然的方式
func (sysVersionService *SysVersionService) ImportMenus(ctx context.Context, menus []system.SysBaseMenu) error {
	// 使用事务包装整个导入过程
	// 好处：如果任何一步失败，所有操作都会回滚，保证数据一致性
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 递归创建菜单，从根节点开始（parentId = 0）
		return sysVersionService.createMenusRecursively(tx, menus, 0)
	})
}

// createMenusRecursively 递归创建菜单
// 设计说明：
//  1. 去重检查：导入前检查菜单是否已存在（通过 name 和 path 唯一标识）
//     - 好处：避免重复导入，支持幂等性操作
//  2. 分离关联数据：先保存菜单主体，再保存关联数据（参数、按钮）
//     - 原因：菜单创建后才能获得 ID，关联数据需要这个 ID
//     - 好处：逻辑清晰，避免数据不一致
//  3. 递归处理子菜单：处理完当前菜单后，递归处理子菜单
//     - 原因：菜单是树形结构，需要先创建父菜单才能创建子菜单
//     - 好处：自然处理任意深度的菜单树
//  4. 使用事务中的 tx：所有操作使用同一个事务，保证原子性
//
// 参数说明：
//   - tx: 事务对象，所有操作都在这个事务中执行
//   - menus: 要创建的菜单列表
//   - parentId: 父菜单ID，根菜单的 parentId 为 0
func (sysVersionService *SysVersionService) createMenusRecursively(tx *gorm.DB, menus []system.SysBaseMenu, parentId uint) error {
	for _, menu := range menus {
		// 去重检查：通过 name 和 path 唯一标识菜单
		// 设计原因：同一菜单可能被多次导入，需要避免重复创建
		// 好处：支持幂等性，多次调用不会产生重复数据
		var existingMenu system.SysBaseMenu
		if err := tx.Where("name = ? AND path = ?", menu.Name, menu.Path).First(&existingMenu).Error; err == nil {
			// 菜单已存在，使用现有菜单ID继续处理子菜单
			// 设计原因：即使菜单已存在，子菜单可能不存在，需要继续处理
			// 好处：支持增量导入，只导入缺失的部分
			if len(menu.Children) > 0 {
				if err := sysVersionService.createMenusRecursively(tx, menu.Children, existingMenu.ID); err != nil {
					return err
				}
			}
			continue
		}

		// 保存关联数据到临时变量，稍后处理
		// 设计原因：需要先创建菜单获得 ID，才能创建关联数据
		// 好处：逻辑清晰，避免在创建菜单时处理关联数据导致的问题
		parameters := menu.Parameters
		menuBtns := menu.MenuBtn
		children := menu.Children

		// 创建新菜单（不包含关联数据）
		// 设计原因：只创建菜单主体，关联数据需要菜单的 ID
		// 好处：先获得 ID，再创建关联数据，逻辑清晰
		newMenu := system.SysBaseMenu{
			ParentId:  parentId, // 设置父菜单ID，建立树形关系
			Path:      menu.Path,
			Name:      menu.Name,
			Hidden:    menu.Hidden,
			Component: menu.Component,
			Sort:      menu.Sort,
			Meta:      menu.Meta,
		}

		if err := tx.Create(&newMenu).Error; err != nil {
			return err
		}

		// 创建菜单参数（关联数据）
		// 设计原因：菜单创建后获得 ID，现在可以创建关联的参数
		// 好处：保证外键关系正确，数据完整性
		if len(parameters) > 0 {
			for _, param := range parameters {
				newParam := system.SysBaseMenuParameter{
					SysBaseMenuID: newMenu.ID, // 使用新创建的菜单ID
					Type:          param.Type,
					Key:           param.Key,
					Value:         param.Value,
				}
				if err := tx.Create(&newParam).Error; err != nil {
					return err
				}
			}
		}

		// 创建菜单按钮（关联数据）
		// 设计原因：同参数，需要菜单ID才能创建关联的按钮
		if len(menuBtns) > 0 {
			for _, btn := range menuBtns {
				newBtn := system.SysBaseMenuBtn{
					SysBaseMenuID: newMenu.ID, // 使用新创建的菜单ID
					Name:          btn.Name,
					Desc:          btn.Desc,
				}
				if err := tx.Create(&newBtn).Error; err != nil {
					return err
				}
			}
		}

		// 递归处理子菜单
		// 设计原因：菜单是树形结构，需要先创建父菜单，再创建子菜单
		// 好处：自然处理任意深度的菜单树，代码简洁
		if len(children) > 0 {
			if err := sysVersionService.createMenusRecursively(tx, children, newMenu.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ImportApis 导入API数据
// 设计说明：
//  1. 使用事务：确保所有API要么全部导入成功，要么全部失败
//  2. 去重检查：通过 path 和 method 唯一标识API（同一路径不同方法视为不同API）
//  3. 跳过已存在：如果API已存在，跳过不创建，避免重复
//
// 好处：
//   - 事务保证：数据一致性，不会出现部分导入的情况
//   - 幂等性：多次调用不会产生重复数据
//   - 性能：批量导入，减少数据库交互次数
//
// 注意：此方法未使用 ctx 参数，可以考虑在事务中使用 ctx 支持超时控制
func (sysVersionService *SysVersionService) ImportApis(apis []system.SysApi) error {
	// 使用事务包装整个导入过程
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		for _, api := range apis {
			// 去重检查：通过 path 和 method 唯一标识API
			// 设计原因：同一路径可能支持多种HTTP方法（GET、POST等），需要区分
			// 好处：避免重复导入，支持幂等性操作
			var existingApi system.SysApi
			if err := tx.Where("path = ? AND method = ?", api.Path, api.Method).First(&existingApi).Error; err == nil {
				// API已存在，跳过
				// 好处：支持增量导入，只导入新API
				continue
			}

			// 创建新API：只复制必要的字段
			// 设计原因：只复制需要导入的字段，避免导入不需要的数据（如ID、创建时间等）
			// 好处：数据清晰，避免覆盖系统自动生成的字段
			newApi := system.SysApi{
				Path:        api.Path,
				Description: api.Description,
				ApiGroup:    api.ApiGroup,
				Method:      api.Method,
			}

			if err := tx.Create(&newApi).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ImportDictionaries 导入字典数据
// 设计说明：
//  1. 使用事务：确保字典及其详情要么全部导入成功，要么全部失败
//  2. 去重检查：通过 type 字段唯一标识字典（字典类型通常是唯一的）
//  3. 级联创建：同时创建字典和字典详情（SysDictionaryDetails）
//     - GORM 会自动处理关联数据的创建
//
// 好处：
//   - 事务保证：字典和详情一起创建，保证数据完整性
//   - 幂等性：多次调用不会产生重复数据
//   - 级联创建：一次操作创建字典及其所有详情，方便高效
//
// 注意：此方法未使用 ctx 参数，可以考虑在事务中使用 ctx 支持超时控制
func (sysVersionService *SysVersionService) ImportDictionaries(dictionaries []system.SysDictionary) error {
	// 使用事务包装整个导入过程
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		for _, dict := range dictionaries {
			// 去重检查：通过 type 字段唯一标识字典
			// 设计原因：字典类型（type）通常是业务中的唯一标识
			// 好处：避免重复导入，支持幂等性操作
			var existingDict system.SysDictionary
			if err := tx.Where("type = ?", dict.Type).First(&existingDict).Error; err == nil {
				// 字典已存在，跳过
				// 好处：支持增量导入，只导入新字典
				continue
			}

			// 创建新字典：包含字典详情
			// 设计原因：字典和详情是关联的，需要一起创建
			// 好处：GORM 会自动处理关联数据的创建，一次操作完成
			// 注意：SysDictionaryDetails 会在创建字典时自动创建（如果模型定义了关联）
			newDict := system.SysDictionary{
				Name:                 dict.Name,
				Type:                 dict.Type,
				Status:               dict.Status,
				Desc:                 dict.Desc,
				SysDictionaryDetails: dict.SysDictionaryDetails, // 关联的字典详情
			}

			if err := tx.Create(&newDict).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
