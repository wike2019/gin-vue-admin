package system

import (
	"context"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderMenuAuthority 定义初始化顺序，确保在菜单和权限都初始化完成后再建立关联关系
// 好处：通过依赖顺序控制，避免在依赖数据未准备好时执行关联操作，保证数据一致性
const initOrderMenuAuthority = initOrderMenu + initOrderAuthority

// initMenuAuthority 菜单-权限关联初始化器
// 作用：为不同的系统角色分配对应的菜单访问权限，实现基于角色的访问控制(RBAC)
type initMenuAuthority struct{}

// init 自动注册初始化器到系统初始化流程中
// 好处：利用 Go 的 init 函数自动执行特性，无需手动调用，简化初始化流程
func init() {
	system.RegisterInit(initOrderMenuAuthority, &initMenuAuthority{})
}

// MigrateTable 表迁移方法，此处不需要创建表（因为是多对多关联表，由 GORM 自动管理）
// 返回 nil 表示跳过表创建步骤
// 好处：关联表由 GORM 的 Many2Many 关系自动管理，无需手动维护表结构
func (i *initMenuAuthority) MigrateTable(ctx context.Context) (context.Context, error) {
	return ctx, nil // do nothing
}

// TableCreated 检查表是否已创建
// 返回 false 表示总是执行替换操作，即使数据已存在也会重新初始化
// 好处：确保每次初始化都能得到正确的权限配置，避免历史数据残留导致权限不一致
func (i *initMenuAuthority) TableCreated(ctx context.Context) bool {
	return false // always replace
}

// InitializerName 返回初始化器名称，用于在 context 中标识和传递数据
// 好处：通过名称在 context 中查找依赖数据，实现松耦合的依赖注入
func (i *initMenuAuthority) InitializerName() string {
	return "sys_menu_authorities"
}

// InitializeData 初始化菜单-权限关联数据
// 核心功能：为不同角色分配对应的菜单访问权限，实现细粒度的权限控制
//
// 设计优势：
// 1. 通过 context 传递依赖数据，避免全局变量，提高代码可测试性和可维护性
// 2. 使用 GORM Association 的 Replace 方法，自动处理关联表的增删改，简化代码
// 3. 分层权限设计：超级管理员 > 测试角色 > 普通用户，满足不同场景需求
func (i *initMenuAuthority) InitializeData(ctx context.Context) (next context.Context, err error) {
	// 从 context 中获取数据库连接
	// 好处：通过依赖注入而非全局变量，使代码更易测试和扩展
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}

	// 从 context 中获取已初始化的权限数据
	// 好处：依赖其他初始化器的结果，通过 context 传递，实现初始化器之间的解耦
	initAuth := &initAuthority{}
	authorities, ok := ctx.Value(initAuth.InitializerName()).([]sysModel.SysAuthority)
	if !ok {
		return ctx, errors.Wrap(system.ErrMissingDependentContext, "创建 [菜单-权限] 关联失败, 未找到权限表初始化数据")
	}

	// 从 context 中获取已初始化的菜单数据
	// 好处：复用菜单初始化器的结果，避免重复查询数据库，提高初始化效率
	allMenus, ok := ctx.Value(new(initMenu).InitializerName()).([]sysModel.SysBaseMenu)
	if !ok {
		return next, errors.Wrap(errors.New(""), "创建 [菜单-权限] 关联失败, 未找到菜单表初始化数据")
	}
	next = ctx

	// 构建菜单ID映射，方便快速查找父菜单信息
	// 好处：使用 map 实现 O(1) 时间复杂度的查找，避免嵌套循环，提升性能
	// 当需要根据父菜单ID查找父菜单名称时，直接通过 map 查找比遍历数组快得多
	menuMap := make(map[uint]sysModel.SysBaseMenu)
	for _, menu := range allMenus {
		menuMap[menu.ID] = menu
	}

	// ========== 为不同角色分配不同权限 ==========
	// 采用分层权限设计，满足不同用户角色的访问需求

	// 1. 超级管理员角色(888) - 拥有所有菜单权限
	// 使用 Replace 方法的好处：
	// - 自动处理关联表的增删改，无需手动维护中间表
	// - 原子性操作，要么全部成功要么全部失败，保证数据一致性
	// - 如果之前有权限配置，会自动清除并替换为新配置
	if err = db.Model(&authorities[0]).Association("SysBaseMenus").Replace(allMenus); err != nil {
		return next, errors.Wrap(err, "为超级管理员分配菜单失败")
	}

	// 2. 普通用户角色(8881) - 仅拥有基础功能菜单
	// 设计目的：限制普通用户的访问范围，只允许访问基础功能，提高系统安全性
	// 仅选择部分父级菜单及其子菜单
	var menu8881 []sysModel.SysBaseMenu

	// 添加仪表盘、关于我们和个人信息菜单
	// 通过 ParentId == 0 判断是否为顶级菜单，通过 Name 匹配特定菜单
	// 好处：基于菜单名称的匹配方式，即使菜单ID变化也能正确识别
	for _, menu := range allMenus {
		if menu.ParentId == 0 && (menu.Name == "dashboard" || menu.Name == "about" || menu.Name == "person" || menu.Name == "state") {
			menu8881 = append(menu8881, menu)
		}
	}

	if err = db.Model(&authorities[1]).Association("SysBaseMenus").Replace(menu8881); err != nil {
		return next, errors.Wrap(err, "为普通用户分配菜单失败")
	}

	// 3. 测试角色(9528) - 拥有部分菜单权限
	// 设计目的：为测试人员提供足够的权限进行功能测试，但限制敏感操作
	var menu9528 []sysModel.SysBaseMenu

	// 添加所有父级菜单
	// 好处：让测试角色能看到所有功能模块，便于全面测试
	for _, menu := range allMenus {
		if menu.ParentId == 0 {
			menu9528 = append(menu9528, menu)
		}
	}

	// 添加部分子菜单 - 系统工具、示例文件等模块的子菜单
	// 通过 menuMap 快速查找父菜单名称，判断子菜单是否属于指定模块
	// 好处：
	// 1. 使用预先构建的 map 查找，时间复杂度 O(1)，性能优于嵌套循环
	// 2. 基于父菜单名称匹配，语义清晰，易于理解和维护
	for _, menu := range allMenus {
		parentName := ""
		// 安全检查：确保 ParentId 存在且有效，避免数组越界
		if menu.ParentId > 0 && menuMap[menu.ParentId].Name != "" {
			parentName = menuMap[menu.ParentId].Name
		}

		// 只添加系统工具和示例模块的子菜单
		// 这样测试角色可以看到这些模块，但其他敏感模块的子菜单不可见
		if menu.ParentId > 0 && (parentName == "systemTools" || parentName == "example") {
			menu9528 = append(menu9528, menu)
		}
	}

	if err = db.Model(&authorities[2]).Association("SysBaseMenus").Replace(menu9528); err != nil {
		return next, errors.Wrap(err, "为测试角色分配菜单失败")
	}

	return next, nil
}

// DataInserted 检查数据是否已插入
// 作用：在系统重启或重新初始化时，判断菜单-权限关联数据是否已存在
//
// 设计优势：
//  1. 通过检查测试角色(9528)的菜单数量来判断，因为该角色权限配置最复杂
//     如果该角色的菜单已配置，说明其他角色的配置也应该已完成
//  2. 使用 Preload 预加载关联数据，避免 N+1 查询问题，提高查询效率
//  3. 返回 bool 值，简化判断逻辑，符合 Go 语言的简洁风格
func (i *initMenuAuthority) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	auth := &sysModel.SysAuthority{}
	// 查询测试角色(9528)并预加载其关联的菜单
	// 为什么选择 9528：该角色的权限配置最复杂（包含父菜单和部分子菜单），
	// 如果它的配置存在，说明初始化流程已执行完成
	// Preload 的好处：一次性加载关联数据，避免后续访问时的额外查询
	if ret := db.Model(auth).
		Where("authority_id = ?", 9528).Preload("SysBaseMenus").Find(auth); ret != nil {
		if ret.Error != nil {
			return false
		}
		// 如果菜单数量大于0，说明关联数据已存在
		return len(auth.SysBaseMenus) > 0
	}
	return false
}
