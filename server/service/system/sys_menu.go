package system

import (
	"errors"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"gorm.io/gorm"
)

// MenuService 菜单服务结构体
// 采用单例模式，通过 MenuServiceApp 全局访问，避免重复创建实例，提高性能
type MenuService struct{}

// MenuServiceApp 菜单服务单例实例
// 好处：全局唯一实例，减少内存占用，统一管理菜单相关业务逻辑
var MenuServiceApp = new(MenuService)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: getMenuTreeMap
//@description: 获取路由总树map
//@param: authorityId string
//@return: treeMap map[string][]system.SysMenu, err error

// getMenuTreeMap 获取指定角色的菜单树映射表
// 设计思路：使用 map[uint][]SysMenu 结构，key 为父菜单ID，value 为该父菜单下的所有子菜单列表
// 好处：
// 1. 扁平化存储树形结构，避免嵌套查询，提高查询效率 O(1) 时间复杂度查找子菜单
// 2. 一次性加载所有菜单数据，减少数据库查询次数（N+1 问题优化）
// 3. 便于后续递归构建树形结构，只需根据 ParentId 快速定位子菜单
//
// 使用示例：
//
//	示例1：基本用法
//	treeMap, err := menuService.getMenuTreeMap(1) // authorityId = 1
//	if err != nil {
//		log.Println("获取菜单树失败:", err)
//		return
//	}
//	// treeMap[0] 包含所有顶级菜单（ParentId = 0 的菜单）
//	topMenus := treeMap[0]
//	for _, menu := range topMenus {
//		fmt.Printf("顶级菜单: %s (ID: %d)\n", menu.Name, menu.MenuId)
//		// 获取该菜单的所有子菜单
//		children := treeMap[menu.MenuId]
//		for _, child := range children {
//			fmt.Printf("  └─ 子菜单: %s (ID: %d)\n", child.Name, child.MenuId)
//		}
//	}
//
//	示例2：遍历所有菜单（包括嵌套层级）
//	func printMenuTree(treeMap map[uint][]system.SysMenu, parentId uint, indent string) {
//		menus := treeMap[parentId]
//		for _, menu := range menus {
//			fmt.Printf("%s- %s (ID: %d, ParentId: %d)\n", indent, menu.Name, menu.MenuId, menu.ParentId)
//			// 递归打印子菜单
//			if children, exists := treeMap[menu.MenuId]; exists && len(children) > 0 {
//				printMenuTree(treeMap, menu.MenuId, indent+"  ")
//			}
//		}
//	}
//	printMenuTree(treeMap, 0, "") // 从顶级菜单开始打印
//
//	示例3：查找特定菜单的所有子菜单
//	menuID := uint(10) // 要查找的菜单ID
//	if children, exists := treeMap[menuID]; exists {
//		fmt.Printf("菜单 %d 有 %d 个子菜单:\n", menuID, len(children))
//		for _, child := range children {
//			fmt.Printf("  - %s\n", child.Name)
//		}
//	} else {
//		fmt.Printf("菜单 %d 没有子菜单\n", menuID)
//	}
//
//	示例4：检查菜单的按钮权限
//	for _, menu := range treeMap[0] {
//		fmt.Printf("菜单: %s\n", menu.Name)
//		if menu.Btns != nil {
//			// menu.Btns 是一个 map[string]uint，key 是按钮名称，value 是角色ID
//			for btnName, authorityId := range menu.Btns {
//				fmt.Printf("  按钮权限: %s (角色ID: %d)\n", btnName, authorityId)
//			}
//		}
//		// 检查是否有特定按钮权限
//		if menu.Btns != nil && menu.Btns["add"] != 0 {
//			fmt.Printf("  该菜单有 'add' 按钮权限\n")
//		}
//	}
//
//	示例5：统计菜单数量和层级
//	totalMenus := 0
//	maxLevel := 0
//	var countMenus func(uint, int)
//	countMenus = func(parentId uint, level int) {
//		if level > maxLevel {
//			maxLevel = level
//		}
//		if menus, exists := treeMap[parentId]; exists {
//			totalMenus += len(menus)
//			for _, menu := range menus {
//				countMenus(menu.MenuId, level+1)
//			}
//		}
//	}
//	countMenus(0, 0) // 从顶级菜单开始统计
//	fmt.Printf("总菜单数: %d, 最大层级: %d\n", totalMenus, maxLevel)
func (menuService *MenuService) getMenuTreeMap(authorityId uint) (treeMap map[uint][]system.SysMenu, err error) {
	var allMenus []system.SysMenu
	var baseMenu []system.SysBaseMenu
	var btns []system.SysAuthorityBtn
	// 初始化树映射表，key 为父菜单ID，value 为子菜单列表
	// 使用 map 而非 slice，可以快速通过 ParentId 查找对应的子菜单集合
	treeMap = make(map[uint][]system.SysMenu)

	// 第一步：查询该角色关联的所有菜单权限记录
	// 通过中间表 sys_authority_menus 获取角色与菜单的关联关系
	var SysAuthorityMenus []system.SysAuthorityMenu
	err = global.GVA_DB.Where("sys_authority_authority_id = ?", authorityId).Find(&SysAuthorityMenus).Error
	if err != nil {
		return
	}

	// 第二步：提取菜单ID列表，用于后续批量查询
	// 好处：使用 IN 查询一次性获取所有菜单，避免循环查询（N+1 问题）
	var MenuIds []string
	for i := range SysAuthorityMenus {
		MenuIds = append(MenuIds, SysAuthorityMenus[i].MenuId)
	}

	// 第三步：批量查询基础菜单信息，按 sort 排序，预加载参数信息
	// Order("sort")：保证菜单按排序字段有序，前端展示时顺序正确
	// Preload("Parameters")：预加载菜单参数，避免后续单独查询（N+1 优化）
	err = global.GVA_DB.Where("id in (?)", MenuIds).Order("sort").Preload("Parameters").Find(&baseMenu).Error
	if err != nil {
		return
	}

	// 第四步：将基础菜单转换为角色菜单，添加角色相关信息
	// SysMenu 包含 SysBaseMenu 的所有信息，同时增加了 AuthorityId 和 MenuId
	// 这样设计的好处：区分基础菜单（所有角色共享）和角色菜单（带权限信息）
	for i := range baseMenu {
		allMenus = append(allMenus, system.SysMenu{
			SysBaseMenu: baseMenu[i],
			AuthorityId: authorityId,
			MenuId:      baseMenu[i].ID,
			Parameters:  baseMenu[i].Parameters,
		})
	}

	// 第五步：查询该角色的按钮权限
	// 按钮权限是细粒度的权限控制，每个菜单可能有多个按钮（如：新增、编辑、删除）
	err = global.GVA_DB.Where("authority_id = ?", authorityId).Preload("SysBaseMenuBtn").Find(&btns).Error
	if err != nil {
		return
	}

	// 第六步：构建按钮权限映射表 btnMap[菜单ID][按钮名称] = 角色ID
	// 使用嵌套 map 的好处：
	// 1. 快速判断某个菜单是否有某个按钮权限 O(1) 查找
	// 2. 结构清晰，便于前端使用（直接通过 menu.Btns["add"] 判断是否有新增权限）
	var btnMap = make(map[uint]map[string]uint)
	for _, v := range btns {
		// 懒加载：只有当菜单首次出现时才初始化内层 map，节省内存
		if btnMap[v.SysMenuID] == nil {
			btnMap[v.SysMenuID] = make(map[string]uint)
		}
		btnMap[v.SysMenuID][v.SysBaseMenuBtn.Name] = authorityId
	}

	// 第七步：将菜单按父菜单ID分组，构建树映射表
	// 同时将按钮权限信息附加到对应菜单上
	for _, v := range allMenus {
		// 将按钮权限映射表附加到菜单对象
		v.Btns = btnMap[v.SysBaseMenu.ID]
		// 按父菜单ID分组：treeMap[0] 存储顶级菜单，treeMap[parentId] 存储对应父菜单的子菜单
		treeMap[v.ParentId] = append(treeMap[v.ParentId], v)
	}
	return treeMap, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetMenuTree
//@description: 获取动态菜单树
//@param: authorityId string
//@return: menus []system.SysMenu, err error

// GetMenuTree 获取指定角色的完整菜单树（递归结构）
// 设计思路：先获取扁平化的菜单映射表，然后递归构建树形结构
// 好处：
// 1. 分离数据获取和树构建逻辑，代码结构清晰
// 2. 扁平化映射表便于后续扩展（如：查找特定菜单、批量操作等）
// 3. 递归构建树形结构，符合前端组件树形展示的需求
func (menuService *MenuService) GetMenuTree(authorityId uint) (menus []system.SysMenu, err error) {
	// 获取扁平化的菜单映射表
	menuTree, err := menuService.getMenuTreeMap(authorityId)
	// treeMap[0] 存储所有顶级菜单（ParentId = 0 的菜单）
	menus = menuTree[0]
	// 递归为每个顶级菜单构建子菜单树
	for i := 0; i < len(menus); i++ {
		err = menuService.getChildrenList(&menus[i], menuTree)
	}
	return menus, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: getChildrenList
//@description: 获取子菜单
//@param: menu *model.SysMenu, treeMap map[string][]model.SysMenu
//@return: err error

// getChildrenList 递归构建菜单的子树
// 设计思路：深度优先遍历，从根节点开始，逐层递归构建子菜单
// 好处：
// 1. 递归算法简洁，易于理解和维护
// 2. 利用 map 的 O(1) 查找特性，快速定位子菜单
// 3. 支持无限层级的菜单嵌套（理论上）
// 注意：如果菜单层级过深，可能导致栈溢出，但实际业务中菜单层级通常不超过 3-4 层
func (menuService *MenuService) getChildrenList(menu *system.SysMenu, treeMap map[uint][]system.SysMenu) (err error) {
	// 从映射表中获取当前菜单的所有子菜单（通过 MenuId 作为 key 查找）
	menu.Children = treeMap[menu.MenuId]
	// 递归为每个子菜单构建其子树
	for i := 0; i < len(menu.Children); i++ {
		err = menuService.getChildrenList(&menu.Children[i], treeMap)
	}
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetInfoList
//@description: 获取路由分页
//@return: list interface{}, total int64,err error

// GetInfoList 获取基础菜单树列表（用于管理界面展示）
// 与 GetMenuTree 的区别：
// 1. GetMenuTree 返回 SysMenu（带角色权限信息），用于前端动态路由
// 2. GetInfoList 返回 SysBaseMenu（基础菜单信息），用于后台管理界面
// 好处：区分业务场景，避免数据冗余，提高接口语义清晰度
func (menuService *MenuService) GetInfoList(authorityID uint) (list interface{}, err error) {
	var menuList []system.SysBaseMenu
	// 获取基础菜单的扁平化映射表（会根据权限进行筛选）
	treeMap, err := menuService.getBaseMenuTreeMap(authorityID)
	// 获取顶级菜单
	menuList = treeMap[0]
	// 递归构建菜单树
	for i := 0; i < len(menuList); i++ {
		err = menuService.getBaseChildrenList(&menuList[i], treeMap)
	}
	return menuList, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: getBaseChildrenList
//@description: 获取菜单的子菜单
//@param: menu *model.SysBaseMenu, treeMap map[string][]model.SysBaseMenu
//@return: err error

// getBaseChildrenList 递归构建基础菜单的子树
// 与 getChildrenList 的区别：操作的是 SysBaseMenu 而非 SysMenu
// 好处：代码复用，相同的递归逻辑适用于不同类型的菜单结构
func (menuService *MenuService) getBaseChildrenList(menu *system.SysBaseMenu, treeMap map[uint][]system.SysBaseMenu) (err error) {
	// 通过菜单ID查找子菜单
	menu.Children = treeMap[menu.ID]
	// 递归构建每个子菜单的子树
	for i := 0; i < len(menu.Children); i++ {
		err = menuService.getBaseChildrenList(&menu.Children[i], treeMap)
	}
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: AddBaseMenu
//@description: 添加基础路由
//@param: menu model.SysBaseMenu
//@return: error

// AddBaseMenu 添加基础菜单
// 设计思路：使用数据库事务确保数据一致性，在添加前进行多重校验
// 好处：
// 1. 事务保证原子性：要么全部成功，要么全部回滚，避免数据不一致
// 2. 多重校验保证数据完整性：防止重复、无效的菜单数据
// 3. 权限清理机制：当叶子菜单变成枝干菜单时，自动清理权限分配，避免权限混乱
func (menuService *MenuService) AddBaseMenu(menu system.SysBaseMenu) error {
	// 使用事务包装所有操作，确保数据一致性
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 第一步：检查菜单名称是否重复
		// 使用 errors.Is 判断是否为记录不存在错误，如果不是则说明已存在同名菜单
		// 好处：精确判断错误类型，避免误判其他数据库错误
		if !errors.Is(tx.Where("name = ?", menu.Name).First(&system.SysBaseMenu{}).Error, gorm.ErrRecordNotFound) {
			return errors.New("存在重复name，请修改name")
		}

		// 第二步：如果是有父菜单的子菜单，需要进行额外校验
		if menu.ParentId != 0 {
			// 2.1 检查父菜单是否存在
			// 防止引用不存在的父菜单，导致数据不一致
			var parentMenu system.SysBaseMenu
			if err := tx.First(&parentMenu, menu.ParentId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("父菜单不存在")
				}
				return err
			}

			// 2.2 检查父菜单下现有子菜单数量
			// 用于判断父菜单当前是叶子菜单还是枝干菜单
			var existingChildrenCount int64
			err := tx.Model(&system.SysBaseMenu{}).Where("parent_id = ?", menu.ParentId).Count(&existingChildrenCount).Error
			if err != nil {
				return err
			}

			// 2.3 如果父菜单原本是叶子菜单（没有子菜单），现在要变成枝干菜单
			// 业务规则：叶子菜单可以被分配权限，但枝干菜单不能直接分配权限（只能通过子菜单）
			// 因此需要清空父菜单的权限分配，避免权限混乱
			if existingChildrenCount == 0 {
				// 2.3.1 检查父菜单是否被其他角色设置为首页
				// 如果父菜单是某个角色的默认首页，不能直接清空权限，需要先处理
				var defaultRouterCount int64
				err := tx.Model(&system.SysAuthority{}).Where("default_router = ?", parentMenu.Name).Count(&defaultRouterCount).Error
				if err != nil {
					return err
				}
				if defaultRouterCount > 0 {
					return errors.New("父菜单已被其他角色的首页占用，请先释放父菜单的首页权限")
				}

				// 2.3.2 清空父菜单的所有权限分配
				// 因为父菜单即将变成枝干菜单，不再需要直接分配权限
				// 好处：自动维护权限一致性，避免管理员手动清理
				err = tx.Where("sys_base_menu_id = ?", menu.ParentId).Delete(&system.SysAuthorityMenu{}).Error
				if err != nil {
					return err
				}
			}
		}

		// 第三步：创建菜单
		// 所有校验通过后，执行实际的创建操作
		return tx.Create(&menu).Error
	})
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: getBaseMenuTreeMap
//@description: 获取路由总树map
//@return: treeMap map[string][]system.SysBaseMenu, err error

// getBaseMenuTreeMap 获取基础菜单的扁平化映射表
// 与 getMenuTreeMap 的区别：
// 1. 返回的是 SysBaseMenu（基础菜单），不包含角色权限信息
// 2. 支持严格权限模式下的菜单筛选（子角色只能看到父角色授权的菜单）
// 好处：
// 1. 支持灵活的权限控制策略（严格模式 vs 宽松模式）
// 2. 预加载关联数据（MenuBtn、Parameters），避免 N+1 查询问题
func (menuService *MenuService) getBaseMenuTreeMap(authorityID uint) (treeMap map[uint][]system.SysBaseMenu, err error) {
	// 第一步：获取当前角色的父角色ID
	// 用于判断是否启用严格权限模式
	parentAuthorityID, err := AuthorityServiceApp.GetParentAuthorityID(authorityID)
	if err != nil {
		return nil, err
	}

	var allMenus []system.SysBaseMenu
	treeMap = make(map[uint][]system.SysBaseMenu)
	// 构建查询对象，按 sort 排序，预加载按钮和参数
	// 使用链式调用，便于后续条件追加
	db := global.GVA_DB.Order("sort").Preload("MenuBtn").Preload("Parameters")

	// 第二步：严格权限模式下的菜单筛选
	// 业务场景：在严格的角色树权限体系下，子角色只能看到父角色授权的菜单
	// 好处：
	// 1. 防止权限越级：子角色不能看到父角色未授权的菜单
	// 2. 支持多级角色继承：角色树中的每个节点都有独立的菜单权限
	// 3. 提高安全性：避免权限泄露
	if global.GVA_CONFIG.System.UseStrictAuth && parentAuthorityID != 0 {
		// 查询当前角色已授权的菜单列表
		var authorityMenus []system.SysAuthorityMenu
		err = global.GVA_DB.Where("sys_authority_authority_id = ?", authorityID).Find(&authorityMenus).Error
		if err != nil {
			return nil, err
		}
		// 提取菜单ID列表
		var menuIds []string
		for i := range authorityMenus {
			menuIds = append(menuIds, authorityMenus[i].MenuId)
		}
		// 只查询已授权的菜单，过滤未授权的菜单
		db = db.Where("id in (?)", menuIds)
	}

	// 第三步：执行查询并构建映射表
	err = db.Find(&allMenus).Error
	// 按父菜单ID分组，构建扁平化映射表
	for _, v := range allMenus {
		treeMap[v.ParentId] = append(treeMap[v.ParentId], v)
	}
	return treeMap, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetBaseMenuTree
//@description: 获取基础路由树
//@return: menus []system.SysBaseMenu, err error

// GetBaseMenuTree 获取基础菜单树（递归结构）
// 与 GetMenuTree 的区别：返回基础菜单而非角色菜单
// 使用场景：后台管理界面展示菜单树，用于菜单管理、权限分配等
func (menuService *MenuService) GetBaseMenuTree(authorityID uint) (menus []system.SysBaseMenu, err error) {
	// 获取扁平化映射表（已根据权限筛选）
	treeMap, err := menuService.getBaseMenuTreeMap(authorityID)
	// 获取顶级菜单
	menus = treeMap[0]
	// 递归构建菜单树
	for i := 0; i < len(menus); i++ {
		err = menuService.getBaseChildrenList(&menus[i], treeMap)
	}
	return menus, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: AddMenuAuthority
//@description: 为角色增加menu树
//@param: menus []model.SysBaseMenu, authorityId string
//@return: err error

// AddMenuAuthority 为指定角色分配菜单权限
// 设计思路：先进行权限校验，再执行权限分配
// 好处：
// 1. 防止权限越级：子角色不能分配父角色未授权的菜单
// 2. 支持角色树权限继承：在严格模式下，子角色只能分配父角色已有的菜单
// 3. 安全性：操作前校验，避免非法权限分配
func (menuService *MenuService) AddMenuAuthority(menus []system.SysBaseMenu, adminAuthorityID, authorityId uint) (err error) {
	// 构建权限对象，准备分配菜单
	var auth system.SysAuthority
	auth.AuthorityId = authorityId
	auth.SysBaseMenus = menus

	// 第一步：检查操作权限
	// 确保当前管理员有权限为指定角色分配菜单（防止权限越级操作）
	err = AuthorityServiceApp.CheckAuthorityIDAuth(adminAuthorityID, authorityId)
	if err != nil {
		return err
	}

	// 第二步：获取当前管理员的角色信息
	var authority system.SysAuthority
	_ = global.GVA_DB.First(&authority, "authority_id = ?", adminAuthorityID).Error
	var menuIds []string

	// 第三步：严格权限模式下的菜单校验
	// 业务规则：在严格模式下，子角色只能分配父角色已拥有的菜单
	// 好处：
	// 1. 防止权限泄露：子角色不能获得父角色没有的菜单权限
	// 2. 维护权限层级：确保权限树的一致性
	// 3. 提高安全性：避免权限分配错误
	if global.GVA_CONFIG.System.UseStrictAuth && *authority.ParentId != 0 {
		// 3.1 查询当前管理员角色已拥有的菜单列表
		var authorityMenus []system.SysAuthorityMenu
		err = global.GVA_DB.Where("sys_authority_authority_id = ?", adminAuthorityID).Find(&authorityMenus).Error
		if err != nil {
			return err
		}
		// 提取菜单ID列表
		for i := range authorityMenus {
			menuIds = append(menuIds, authorityMenus[i].MenuId)
		}

		// 3.2 校验待分配的菜单是否都在当前管理员角色的菜单列表中
		// 如果存在不在列表中的菜单，说明是跨级操作，拒绝执行
		for i := range menus {
			hasMenu := false
			// 将菜单ID转换为字符串进行比较（因为 menuIds 是 []string）
			idStr := strconv.Itoa(int(menus[i].ID))
			for j := range menuIds {
				if idStr == menuIds[j] {
					hasMenu = true
					break
				}
			}
			// 如果菜单不在当前管理员角色的菜单列表中，拒绝分配
			if !hasMenu {
				return errors.New("添加失败,请勿跨级操作")
			}
		}
	}

	// 第四步：执行菜单权限分配
	// 所有校验通过后，调用权限服务设置菜单权限
	err = AuthorityServiceApp.SetMenuAuthority(&auth)
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetMenuAuthority
//@description: 查看当前角色树
//@param: info *request.GetAuthorityId
//@return: menus []system.SysMenu, err error

// GetMenuAuthority 获取指定角色的菜单权限列表（扁平结构，非树形）
// 与 GetMenuTree 的区别：
// 1. GetMenuTree 返回树形结构，用于前端动态路由渲染
// 2. GetMenuAuthority 返回扁平列表，用于权限管理界面展示
// 好处：根据使用场景返回不同结构，提高接口语义清晰度
func (menuService *MenuService) GetMenuAuthority(info *request.GetAuthorityId) (menus []system.SysMenu, err error) {
	var baseMenu []system.SysBaseMenu
	// 第一步：查询角色与菜单的关联关系
	var SysAuthorityMenus []system.SysAuthorityMenu
	err = global.GVA_DB.Where("sys_authority_authority_id = ?", info.AuthorityId).Find(&SysAuthorityMenus).Error
	if err != nil {
		return
	}

	// 第二步：提取菜单ID列表
	var MenuIds []string
	for i := range SysAuthorityMenus {
		MenuIds = append(MenuIds, SysAuthorityMenus[i].MenuId)
	}

	// 第三步：批量查询菜单信息，按 sort 排序
	// 使用 IN 查询，避免 N+1 问题
	err = global.GVA_DB.Where("id in (?) ", MenuIds).Order("sort").Find(&baseMenu).Error

	// 第四步：将基础菜单转换为角色菜单
	// 添加角色ID和菜单ID信息，便于前端使用
	for i := range baseMenu {
		menus = append(menus, system.SysMenu{
			SysBaseMenu: baseMenu[i],
			AuthorityId: info.AuthorityId,
			MenuId:      baseMenu[i].ID,
			Parameters:  baseMenu[i].Parameters,
		})
	}
	return menus, err
}

// UserAuthorityDefaultRouter 用户角色默认路由检查
// 功能：验证用户角色的默认路由是否在用户拥有的菜单权限中
// 设计思路：如果默认路由不在权限范围内，自动设置为 404 页面，避免用户访问无权限页面
// 好处：
// 1. 防止权限泄露：确保用户只能访问有权限的页面
// 2. 自动修复：当菜单权限变更导致默认路由失效时，自动降级到 404
// 3. 提升用户体验：避免用户登录后看到空白页或错误页
//
//	Author [SliverHorn](https://github.com/SliverHorn)
func (menuService *MenuService) UserAuthorityDefaultRouter(user *system.SysUser) {
	// 第一步：获取用户角色拥有的所有菜单ID
	// 使用 Pluck 直接提取菜单ID列表，比 Find 更高效
	var menuIds []string
	err := global.GVA_DB.Model(&system.SysAuthorityMenu{}).Where("sys_authority_authority_id = ?", user.AuthorityId).Pluck("sys_base_menu_id", &menuIds).Error
	if err != nil {
		return
	}

	// 第二步：检查默认路由是否在用户拥有的菜单列表中
	// 查询条件：菜单名称匹配默认路由，且菜单ID在权限列表中
	var am system.SysBaseMenu
	err = global.GVA_DB.First(&am, "name = ? and id in (?)", user.Authority.DefaultRouter, menuIds).Error
	// 如果默认路由不在权限范围内，设置为 404
	// 好处：自动修复无效的默认路由，避免用户访问无权限页面
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user.Authority.DefaultRouter = "404"
	}
}
