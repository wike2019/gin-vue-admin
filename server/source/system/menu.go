package system

import (
	"context"

	. "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderMenu 定义菜单表的初始化顺序
// 设置为 initOrderAuthority + 1 确保权限表先于菜单表初始化
// 这样设计的好处：
// 1. 明确依赖关系：菜单表虽然不直接依赖权限表，但菜单权限关联表需要两者都存在
// 2. 保证初始化顺序：菜单初始化完成后，后续的菜单权限关联初始化器可以安全执行
// 3. 支持依赖注入：菜单数据可以通过 context 传递给后续初始化器使用
const initOrderMenu = initOrderAuthority + 1

// initMenu 菜单表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
// 使用结构体而不是函数的好处：
// 1. 可以保存状态（如果需要）
// 2. 可以实现接口方法
// 3. 符合 Go 的面向对象设计模式
// 4. 便于扩展：未来可以添加配置或缓存等字段
type initMenu struct{}

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用
// 2. 解耦：初始化逻辑与注册逻辑分离
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理
func init() {
	system.RegisterInit(initOrderMenu, &initMenu{})
}

// InitializerName 返回初始化器的唯一标识名称
// 用于：
// 1. 日志输出：标识当前初始化的模块
// 2. Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
// 3. 去重检查：防止同名初始化器重复注册
//
// 使用表名作为标识的好处：
// - 语义清晰：直接对应数据库表
// - 自动获取：通过模型方法获取，避免硬编码
// - 类型安全：编译期检查，避免拼写错误
func (i *initMenu) InitializerName() string {
	return SysBaseMenu{}.TableName()
}

// MigrateTable 执行数据库表结构迁移
// 参数：ctx - 上下文，包含数据库连接等初始化所需信息
// 返回：next context - 可以用于传递数据给后续初始化步骤
//
//	error - 迁移过程中的错误
//
// 设计说明：
// 1. 使用 context 传递 db 连接，避免全局变量，提高可测试性
//   - 好处：可以在测试中轻松替换数据库连接，无需修改全局状态
//
// 2. 类型断言 + ok 模式确保安全获取数据库连接
//   - 好处：避免 panic，提供清晰的错误信息
//
// 3. AutoMigrate 自动根据模型结构创建/更新表结构，支持迭代开发
//   - 好处：模型变更时自动同步表结构，无需手动编写 SQL
//
// 4. 一次性迁移三个相关表，保证表结构一致性
//   - SysBaseMenu: 菜单主表
//   - SysBaseMenuParameter: 菜单参数表（扩展字段）
//   - SysBaseMenuBtn: 菜单按钮表（按钮级权限）
func (i *initMenu) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(
		&SysBaseMenu{},
		&SysBaseMenuParameter{},
		&SysBaseMenuBtn{},
	)
}

// TableCreated 检查表是否已存在
// 用于判断是否需要执行表迁移和数据初始化
//
// 返回值：bool - true 表示所有相关表都已存在，false 表示至少有一个表不存在
//
// 设计说明：
// 1. 幂等性检查：避免重复初始化导致的错误
//   - 好处：支持多次运行初始化流程，不会因为表已存在而失败
//
// 2. 支持增量初始化：已存在的表可以跳过迁移步骤
//   - 好处：在已有数据库上初始化时，只创建缺失的表
//
// 3. 使用 Migrator().HasTable() 方法，兼容不同数据库类型
//   - 好处：MySQL、PostgreSQL、SQLite 等数据库都能正确检查
//
// 4. 检查所有相关表，确保表结构完整性
//   - 好处：避免只创建了主表但缺少关联表的情况
func (i *initMenu) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	m := db.Migrator()
	return m.HasTable(&SysBaseMenu{}) &&
		m.HasTable(&SysBaseMenuParameter{}) &&
		m.HasTable(&SysBaseMenuBtn{})
}

// InitializeData 初始化菜单表的初始数据
// 这是核心的数据初始化逻辑，包括：
// 1. 创建所有父级菜单（ParentId = 0）
// 2. 创建所有子菜单，并正确关联父菜单
//
// 设计亮点：
//
//  1. 分步创建策略：先创建父菜单，再创建子菜单
//     好处：
//     - 解决外键依赖问题：子菜单的 ParentId 必须引用已存在的父菜单ID
//     - 数据库自动生成ID：父菜单创建后，数据库会分配自增ID，子菜单才能正确引用
//     - 避免硬编码ID：不依赖固定的ID值，适配不同数据库的自增策略
//
//  2. 使用 Name 作为映射键建立父子关系
//     好处：
//     - 语义清晰：通过业务名称（如 "superAdmin"）建立关联，而非数字ID
//     - 易于维护：新增菜单时只需修改 Name，无需关心ID
//     - 避免错误：减少因ID写错导致的关联错误
//
//  3. 批量创建提高性能
//     好处：
//     - 减少数据库交互次数：父菜单和子菜单分别批量插入
//     - 事务一致性：GORM 的 Create 在事务中执行，保证原子性
//     - 提高初始化速度：特别是在网络延迟较高的环境中
//
//  4. Context 传递数据给后续初始化器
//     好处：
//     - 菜单权限关联初始化器可以直接使用菜单数据，无需查询数据库
//     - 减少数据库查询，提高初始化效率
//     - 实现初始化器之间的数据共享机制
//
//  5. 菜单层级设计（MenuLevel）
//     好处：
//     - 支持多级菜单：MenuLevel 0 为顶级，1 为二级，可扩展
//     - 前端路由生成：可以根据层级自动生成嵌套路由结构
//     - 权限控制：可以按层级控制菜单显示权限
//
//  6. Hidden 字段控制菜单可见性
//     好处：
//     - 隐藏系统菜单：如 "person"（个人信息）菜单，不在导航中显示但可通过路由访问
//     - 动态路由：如 "autoCodeEdit/:id" 是动态路由，需要隐藏但可访问
//     - 灵活控制：根据业务需求灵活控制菜单的显示/隐藏
func (i *initMenu) InitializeData(ctx context.Context) (next context.Context, err error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}

	// 定义所有父级菜单（ParentId = 0 表示顶级菜单）
	// 这些菜单是系统的基础功能模块，包括：
	// - dashboard: 仪表盘（首页）
	// - about: 关于我们
	// - superAdmin: 超级管理员模块（包含用户、角色、菜单等管理功能）
	// - person: 个人信息（隐藏菜单，不在导航显示）
	// - example: 示例文件（演示功能）
	// - systemTools: 系统工具（代码生成、表单生成等）
	// - plugin: 插件系统
	// - state: 服务器状态
	// - 官方网站: 外部链接
	allMenus := []SysBaseMenu{
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "dashboard", Name: "dashboard", Component: "view/dashboard/index.vue", Sort: 1, Meta: Meta{Title: "仪表盘", Icon: "odometer"}},
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "about", Name: "about", Component: "view/about/index.vue", Sort: 9, Meta: Meta{Title: "关于我们", Icon: "info-filled"}},
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "admin", Name: "superAdmin", Component: "view/superAdmin/index.vue", Sort: 3, Meta: Meta{Title: "超级管理员", Icon: "user"}},
		{MenuLevel: 0, Hidden: true, ParentId: 0, Path: "person", Name: "person", Component: "view/person/person.vue", Sort: 4, Meta: Meta{Title: "个人信息", Icon: "message"}},
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "example", Name: "example", Component: "view/example/index.vue", Sort: 7, Meta: Meta{Title: "示例文件", Icon: "management"}},
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "systemTools", Name: "systemTools", Component: "view/systemTools/index.vue", Sort: 5, Meta: Meta{Title: "系统工具", Icon: "tools"}},
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "https://www.gin-vue-admin.com", Name: "https://www.gin-vue-admin.com", Component: "/", Sort: 0, Meta: Meta{Title: "官方网站", Icon: "customer-gva"}},
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "state", Name: "state", Component: "view/system/state.vue", Sort: 8, Meta: Meta{Title: "服务器状态", Icon: "cloudy"}},
		{MenuLevel: 0, Hidden: false, ParentId: 0, Path: "plugin", Name: "plugin", Component: "view/routerHolder.vue", Sort: 6, Meta: Meta{Title: "插件系统", Icon: "cherry"}},
	}

	// 先创建父级菜单（ParentId = 0 的菜单）
	// 这一步完成后，数据库会为每个菜单分配自增ID
	// 使用批量创建提高性能，减少数据库交互次数
	if err = db.Create(&allMenus).Error; err != nil {
		return ctx, errors.Wrap(err, SysBaseMenu{}.TableName()+"父级菜单初始化失败!")
	}

	// 建立菜单映射 - 通过Name查找已创建的菜单及其ID
	// 设计思路：
	// 1. 使用 map[string]uint 建立 Name -> ID 的映射关系
	// 2. 遍历已创建的父菜单，提取 Name 和数据库分配的 ID
	// 3. 后续子菜单可以通过 Name 找到对应的父菜单 ID
	//
	// 为什么使用 Name 而不是直接使用 ID？
	// - Name 是业务语义，更易读易维护
	// - ID 是数据库生成的，在创建前无法预知
	// - 通过 Name 映射，代码更清晰，减少硬编码
	menuNameMap := make(map[string]uint)
	for _, menu := range allMenus {
		menuNameMap[menu.Name] = menu.ID
	}

	// 定义子菜单，并设置正确的ParentId
	// 通过 menuNameMap 查找父菜单ID，建立父子关系
	// 子菜单按功能模块分组：
	// 1. superAdmin 子菜单：角色管理、菜单管理、API管理、用户管理等
	// 2. example 子菜单：上传下载、断点续传、客户列表等示例功能
	// 3. systemTools 子菜单：代码生成器、表单生成器、系统配置等工具
	// 4. plugin 子菜单：插件市场、插件安装、打包插件等
	childMenus := []SysBaseMenu{
		// superAdmin子菜单
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["superAdmin"], Path: "authority", Name: "authority", Component: "view/superAdmin/authority/authority.vue", Sort: 1, Meta: Meta{Title: "角色管理", Icon: "avatar"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["superAdmin"], Path: "menu", Name: "menu", Component: "view/superAdmin/menu/menu.vue", Sort: 2, Meta: Meta{Title: "菜单管理", Icon: "tickets", KeepAlive: true}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["superAdmin"], Path: "api", Name: "api", Component: "view/superAdmin/api/api.vue", Sort: 3, Meta: Meta{Title: "api管理", Icon: "platform", KeepAlive: true}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["superAdmin"], Path: "user", Name: "user", Component: "view/superAdmin/user/user.vue", Sort: 4, Meta: Meta{Title: "用户管理", Icon: "coordinate"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["superAdmin"], Path: "dictionary", Name: "dictionary", Component: "view/superAdmin/dictionary/sysDictionary.vue", Sort: 5, Meta: Meta{Title: "字典管理", Icon: "notebook"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["superAdmin"], Path: "operation", Name: "operation", Component: "view/superAdmin/operation/sysOperationRecord.vue", Sort: 6, Meta: Meta{Title: "操作历史", Icon: "pie-chart"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["superAdmin"], Path: "sysParams", Name: "sysParams", Component: "view/superAdmin/params/sysParams.vue", Sort: 7, Meta: Meta{Title: "参数管理", Icon: "compass"}},

		// example子菜单
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["example"], Path: "upload", Name: "upload", Component: "view/example/upload/upload.vue", Sort: 5, Meta: Meta{Title: "媒体库（上传下载）", Icon: "upload"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["example"], Path: "breakpoint", Name: "breakpoint", Component: "view/example/breakpoint/breakpoint.vue", Sort: 6, Meta: Meta{Title: "断点续传", Icon: "upload-filled"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["example"], Path: "customer", Name: "customer", Component: "view/example/customer/customer.vue", Sort: 7, Meta: Meta{Title: "客户列表（资源示例）", Icon: "avatar"}},

		// systemTools子菜单
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "autoCode", Name: "autoCode", Component: "view/systemTools/autoCode/index.vue", Sort: 1, Meta: Meta{Title: "代码生成器", Icon: "cpu", KeepAlive: true}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "formCreate", Name: "formCreate", Component: "view/systemTools/formCreate/index.vue", Sort: 3, Meta: Meta{Title: "表单生成器", Icon: "magic-stick", KeepAlive: true}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "system", Name: "system", Component: "view/systemTools/system/system.vue", Sort: 4, Meta: Meta{Title: "系统配置", Icon: "operation"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "autoCodeAdmin", Name: "autoCodeAdmin", Component: "view/systemTools/autoCodeAdmin/index.vue", Sort: 2, Meta: Meta{Title: "自动化代码管理", Icon: "magic-stick"}},
		{MenuLevel: 1, Hidden: true, ParentId: menuNameMap["systemTools"], Path: "autoCodeEdit/:id", Name: "autoCodeEdit", Component: "view/systemTools/autoCode/index.vue", Sort: 0, Meta: Meta{Title: "自动化代码-${id}", Icon: "magic-stick"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "autoPkg", Name: "autoPkg", Component: "view/systemTools/autoPkg/autoPkg.vue", Sort: 0, Meta: Meta{Title: "模板配置", Icon: "folder"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "exportTemplate", Name: "exportTemplate", Component: "view/systemTools/exportTemplate/exportTemplate.vue", Sort: 5, Meta: Meta{Title: "导出模板", Icon: "reading"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "picture", Name: "picture", Component: "view/systemTools/autoCode/picture.vue", Sort: 6, Meta: Meta{Title: "AI页面绘制", Icon: "picture-filled"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "mcpTool", Name: "mcpTool", Component: "view/systemTools/autoCode/mcp.vue", Sort: 7, Meta: Meta{Title: "Mcp Tools模板", Icon: "magnet"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "mcpTest", Name: "mcpTest", Component: "view/systemTools/autoCode/mcpTest.vue", Sort: 7, Meta: Meta{Title: "Mcp Tools测试", Icon: "partly-cloudy"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "sysVersion", Name: "sysVersion", Component: "view/systemTools/version/version.vue", Sort: 8, Meta: Meta{Title: "版本管理", Icon: "server"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["systemTools"], Path: "sysError", Name: "sysError", Component: "view/systemTools/sysError/sysError.vue", Sort: 9, Meta: Meta{Title: "错误日志", Icon: "warn"}},

		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["plugin"], Path: "https://plugin.gin-vue-admin.com/", Name: "https://plugin.gin-vue-admin.com/", Component: "https://plugin.gin-vue-admin.com/", Sort: 0, Meta: Meta{Title: "插件市场", Icon: "shop"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["plugin"], Path: "installPlugin", Name: "installPlugin", Component: "view/systemTools/installPlugin/index.vue", Sort: 1, Meta: Meta{Title: "插件安装", Icon: "box"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["plugin"], Path: "pubPlug", Name: "pubPlug", Component: "view/systemTools/pubPlug/pubPlug.vue", Sort: 3, Meta: Meta{Title: "打包插件", Icon: "files"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["plugin"], Path: "plugin-email", Name: "plugin-email", Component: "plugin/email/view/index.vue", Sort: 4, Meta: Meta{Title: "邮件插件", Icon: "message"}},
		{MenuLevel: 1, Hidden: false, ParentId: menuNameMap["plugin"], Path: "anInfo", Name: "anInfo", Component: "plugin/announcement/view/info.vue", Sort: 5, Meta: Meta{Title: "公告管理[示例]", Icon: "scaleToOriginal"}},
	}

	// 创建子菜单
	// 此时所有父菜单已创建并分配了ID，子菜单可以正确引用父菜单ID
	// 批量创建提高性能，减少数据库交互
	if err = db.Create(&childMenus).Error; err != nil {
		return ctx, errors.Wrap(err, SysBaseMenu{}.TableName()+"子菜单初始化失败!")
	}

	// 组合所有菜单作为返回结果
	// 将父菜单和子菜单合并，存入 context 供后续初始化器使用
	// 好处：
	// 1. 菜单权限关联初始化器可以直接使用这些数据，无需查询数据库
	// 2. 减少数据库查询，提高初始化效率
	// 3. 实现初始化器之间的数据共享机制
	allEntities := append(allMenus, childMenus...)
	next = context.WithValue(ctx, i.InitializerName(), allEntities)
	return next, nil
}

// DataInserted 检查初始化数据是否已存在
// 用于判断是否需要重新初始化数据
//
// 检查策略：
// 1. 检查特定菜单（"autoPkg"）是否存在
// 2. 如果该菜单存在，说明初始化数据已完整
//
// 设计说明：
// 1. 使用 errors.Is 判断是否为记录不存在错误，兼容性好
//   - 好处：兼容不同版本的 GORM，避免因错误类型变化导致的判断失败
//
// 2. 选择 "autoPkg" 作为检查点
//   - 原因：这是一个相对靠后的子菜单，如果它存在，说明前面的菜单都已创建
//   - 好处：避免检查所有菜单，提高检查效率
//
// 3. 幂等性保证
//   - 好处：支持多次运行初始化流程，已存在的数据不会重复创建
//
// 4. 类型断言 + ok 模式确保安全获取数据库连接
//   - 好处：避免 panic，提供清晰的错误处理
func (i *initMenu) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	// 检查 "autoPkg" 菜单是否存在
	// 选择这个菜单作为检查点的原因：
	// - 它是 systemTools 的子菜单，相对靠后
	// - 如果它存在，说明父菜单和前面的子菜单都已创建
	// - 避免检查所有菜单，提高检查效率
	if errors.Is(db.Where("path = ?", "autoPkg").First(&SysBaseMenu{}).Error, gorm.ErrRecordNotFound) { // 判断是否存在数据
		return false
	}
	return true
}
