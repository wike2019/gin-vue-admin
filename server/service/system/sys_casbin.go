package system

import (
	"errors"
	"strconv"

	"gorm.io/gorm"

	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	_ "github.com/go-sql-driver/mysql"
)

// CasbinService Casbin权限管理服务
// 使用结构体封装权限相关操作，便于统一管理和扩展
type CasbinService struct{}

// CasbinServiceApp 单例模式的服务实例
// 好处：
// 1. 全局唯一实例，避免重复创建，节省内存
// 2. 统一入口，便于维护和调用
// 3. 符合Go语言的服务层设计模式
var CasbinServiceApp = new(CasbinService)

// UpdateCasbin 更新指定角色的Casbin权限策略
// 设计思路：
// 1. 先验证操作者是否有权限修改目标角色（防止越权操作）
// 2. 严格模式下验证所有API是否在系统API列表中（防止分配不存在的API权限）
// 3. 采用"先清空再添加"的全量更新策略，保证权限数据一致性
// 4. 使用map进行去重处理，避免重复权限导致的问题
// 5. 空权限列表时直接返回，避免不必要的数据库操作
// @param adminAuthorityID 操作者的权限ID，用于权限验证
// @param AuthorityID 要更新的目标权限ID
// @param casbinInfos 新的权限列表
// @return error 操作失败时返回错误
func (casbinService *CasbinService) UpdateCasbin(adminAuthorityID, AuthorityID uint, casbinInfos []request.CasbinInfo) error {

	// 权限验证：检查操作者是否有权限修改目标角色的权限
	// 好处：防止低权限用户修改高权限角色的权限，确保权限管理的安全性
	// 例如：普通管理员不能修改超级管理员的权限
	err := AuthorityServiceApp.CheckAuthorityIDAuth(adminAuthorityID, AuthorityID)
	if err != nil {
		return err
	}

	// 严格权限模式检查：确保分配的API权限都在系统API列表中
	// 设计目的：
	// 1. 防止分配不存在的API权限（可能是前端传错或API已删除）
	// 2. 确保权限数据的有效性和一致性
	// 3. 可配置开关，灵活控制是否启用严格检查
	if global.GVA_CONFIG.System.UseStrictAuth {
		// 获取操作者可见的所有API列表（基于操作者的权限范围）
		apis, e := ApiServiceApp.GetAllApis(adminAuthorityID)
		if e != nil {
			return e
		}

		// 遍历待分配的权限，验证每个API是否在系统API列表中
		// 使用双重循环但配合break优化，时间复杂度为O(n*m)，但实际场景中API数量有限
		for i := range casbinInfos {
			hasApi := false
			for j := range apis {
				// 同时匹配路径和方法，确保精确匹配
				// 例如：GET /api/user 和 POST /api/user 是不同的权限
				if apis[j].Path == casbinInfos[i].Path && apis[j].Method == casbinInfos[i].Method {
					hasApi = true
					break // 找到即退出，提高效率
				}
			}
			if !hasApi {
				return errors.New("存在api不在权限列表中")
			}
		}
	}

	// 将权限ID转换为字符串（Casbin使用字符串作为策略标识）
	// Casbin的策略格式是字符串数组，需要统一类型
	authorityId := strconv.Itoa(int(AuthorityID))

	// 先清除该角色的所有现有权限
	// 设计原因：
	// 1. 全量更新策略：避免复杂的增量更新逻辑（需要对比新增、删除、修改）
	// 2. 保证数据一致性：清除后添加，确保最终状态与输入完全一致
	// 3. 简化实现：不需要处理部分更新可能带来的数据不一致问题
	// 注意：ClearCasbin使用的是内存中的Casbin实例，会同步到数据库
	casbinService.ClearCasbin(0, authorityId)

	// 准备新的权限规则列表
	rules := [][]string{}

	// 权限去重处理：使用map记录已添加的权限
	// 为什么需要去重：
	// 1. 前端可能传递重复的权限数据（用户误操作或前端bug）
	// 2. 防止重复权限导致Casbin策略冲突
	// 3. 使用map的O(1)查找特性，提高去重效率（比数组遍历O(n)快）
	deduplicateMap := make(map[string]bool)
	for _, v := range casbinInfos {
		// 使用"权限ID+路径+方法"作为唯一键
		// 这样可以确保同一个角色的同一个API权限只添加一次
		key := authorityId + v.Path + v.Method
		if _, ok := deduplicateMap[key]; !ok {
			deduplicateMap[key] = true
			// Casbin策略格式：[角色ID, 路径, HTTP方法]
			// 对应Casbin模型中的 p, sub, obj, act
			// 这里使用3个字段：v0=角色ID, v1=路径, v2=HTTP方法
			rules = append(rules, []string{authorityId, v.Path, v.Method})
		}
	}

	// 空权限列表优化：如果清空后没有新权限，直接返回
	// 好处：
	// 1. 避免调用AddPolicies时传入空数组可能的问题
	// 2. 减少不必要的Casbin操作，提高性能
	// 3. 逻辑清晰：空权限就是清空所有权限，已经通过ClearCasbin完成
	if len(rules) == 0 {
		return nil
	}

	// 获取Casbin实例并批量添加权限策略
	// 使用AddPolicies批量添加，比逐个AddPolicy效率更高
	// 批量操作可以减少数据库交互次数，提高性能
	e := utils.GetCasbin()
	success, _ := e.AddPolicies(rules)
	if !success {
		// 虽然已经去重，但Casbin内部可能还有其他检查（如策略冲突）
		// 这种情况比较少见，但需要处理以提供友好的错误提示
		return errors.New("存在相同api,添加失败,请联系管理员")
	}
	return nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: UpdateCasbinApi
//@description: API更新随动
//@param: oldPath string, newPath string, oldMethod string, newMethod string
//@return: error

// UpdateCasbinApi 当系统API路径或方法变更时，同步更新Casbin权限策略
// 使用场景：
// 1. API重构：路径从 /api/v1/user 改为 /api/v2/user
// 2. HTTP方法变更：从 GET 改为 POST
// 3. 批量更新：系统升级时批量更新API路径
//
// 设计思路：
// 1. 直接在数据库层面批量更新，效率高
// 2. 更新后重新加载策略到内存，保证一致性
// 3. 使用GORM的Updates方法，支持批量更新
//
// 为什么需要这个方法：
// - 如果API路径变更，但权限策略不更新，会导致权限失效
// - 手动更新每个角色的权限效率低且容易遗漏
// - 通过数据库批量更新，然后刷新内存缓存，保证数据一致性
func (casbinService *CasbinService) UpdateCasbinApi(oldPath string, newPath string, oldMethod string, newMethod string) error {
	// 直接在数据库层面更新所有匹配的Casbin规则
	// v1对应路径(Path)，v2对应HTTP方法(Method)
	// 使用WHERE条件精确匹配，只更新匹配的规则
	// 好处：批量更新，一次SQL操作完成，效率高
	err := global.GVA_DB.Model(&gormadapter.CasbinRule{}).Where("v1 = ? AND v2 = ?", oldPath, oldMethod).Updates(map[string]interface{}{
		"v1": newPath,   // 更新路径
		"v2": newMethod, // 更新HTTP方法
	}).Error
	if err != nil {
		return err
	}

	// 重新加载策略到内存
	// 原因：Casbin使用内存缓存策略以提高性能，数据库更新后需要刷新缓存
	// 如果不刷新，内存中的策略还是旧的，权限检查会失败
	e := utils.GetCasbin()
	return e.LoadPolicy()
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetPolicyPathByAuthorityId
//@description: 获取权限列表
//@param: authorityId string
//@return: pathMaps []request.CasbinInfo

// GetPolicyPathByAuthorityId 根据权限ID获取该角色的所有权限列表
// 使用场景：
// 1. 前端权限管理页面展示角色的权限列表
// 2. 权限编辑时回显当前权限
// 3. 权限审计：查看某个角色拥有哪些权限
//
// 设计说明：
// 1. 使用GetFilteredPolicy过滤策略，只获取指定角色的权限
// 2. 将Casbin的策略格式转换为业务层的CasbinInfo格式
// 3. 从内存中读取，性能高（Casbin策略已加载到内存）
func (casbinService *CasbinService) GetPolicyPathByAuthorityId(AuthorityID uint) (pathMaps []request.CasbinInfo) {
	e := utils.GetCasbin()
	// 转换为字符串，因为Casbin使用字符串作为策略标识
	authorityId := strconv.Itoa(int(AuthorityID))

	// GetFilteredPolicy(0, authorityId) 表示：
	// - 第一个参数0：从策略的第一个字段开始匹配（即v0，角色ID）
	// - 第二个参数authorityId：匹配值，即要查询的角色ID
	// 返回所有匹配的策略列表，每个策略是一个字符串数组
	list, _ := e.GetFilteredPolicy(0, authorityId)

	// 将Casbin策略格式转换为业务层的数据结构
	// Casbin策略格式：[角色ID, 路径, HTTP方法]
	// 所以 v[0]是角色ID，v[1]是路径，v[2]是HTTP方法
	for _, v := range list {
		pathMaps = append(pathMaps, request.CasbinInfo{
			Path:   v[1], // 路径
			Method: v[2], // HTTP方法
		})
	}
	return pathMaps
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: ClearCasbin
//@description: 清除匹配的权限
//@param: v int 从第几个字段开始匹配（0表示从v0开始）
//@param: p ...string 可变参数，匹配的值
//@return: bool 是否成功

// ClearCasbin 清除匹配的Casbin权限策略
// 使用场景：
// 1. 更新权限前先清空旧权限（如UpdateCasbin中的使用）
// 2. 删除角色时清除该角色的所有权限
// 3. 批量清理特定条件的权限
//
// 设计说明：
// 1. 使用RemoveFilteredPolicy，支持按条件过滤删除
// 2. 操作会同步到数据库（通过gorm-adapter）
// 3. 从内存中删除，性能高
//
// 参数说明：
// - v: 从策略的第几个字段开始匹配（0表示从v0开始，即角色ID）
// - p: 可变参数，要匹配的值
// 例如：ClearCasbin(0, "1") 表示删除所有v0="1"的策略（即角色ID为1的所有权限）
func (casbinService *CasbinService) ClearCasbin(v int, p ...string) bool {
	e := utils.GetCasbin()
	// RemoveFilteredPolicy会删除所有匹配的策略
	// 返回值success表示是否有策略被删除（true表示有策略被删除）
	success, _ := e.RemoveFilteredPolicy(v, p...)
	return success
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: RemoveFilteredPolicy
//@description: 使用数据库方法清理筛选的politicy 此方法需要调用FreshCasbin方法才可以在系统中即刻生效
//@param: db *gorm.DB 数据库连接，支持事务
//@param: authorityId string 要删除的权限ID
//@return: error

// RemoveFilteredPolicy 使用数据库方法删除指定角色的所有权限策略
// 与ClearCasbin的区别：
// 1. ClearCasbin：通过Casbin API操作，会同步更新内存和数据库
// 2. RemoveFilteredPolicy：直接操作数据库，需要手动刷新内存缓存
//
// 使用场景：
// 1. 在事务中批量操作权限（需要传入事务的db对象）
// 2. 需要精确控制数据库操作，不立即刷新内存缓存
// 3. 批量删除操作，提高性能
//
// 重要提示：
// 此方法只更新数据库，不会立即更新内存中的Casbin策略
// 调用后必须调用FreshCasbin()刷新内存缓存，否则权限检查会使用旧数据
//
// 为什么需要这个方法：
// - 在事务中操作时，需要传入事务的db对象
// - 批量操作时，可以先操作数据库，最后统一刷新缓存，提高性能
func (casbinService *CasbinService) RemoveFilteredPolicy(db *gorm.DB, authorityId string) error {
	// 直接使用GORM删除数据库中的Casbin规则
	// v0字段存储角色ID，删除所有v0=authorityId的记录
	// 使用传入的db对象，支持事务操作
	return db.Delete(&gormadapter.CasbinRule{}, "v0 = ?", authorityId).Error
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: SyncPolicy
//@description: 同步目前数据库的policy 此方法需要调用FreshCasbin方法才可以在系统中即刻生效
//@param: db *gorm.DB 数据库连接，支持事务
//@param: authorityId string 权限ID
//@param: rules [][]string 新的权限规则列表
//@return: error

// SyncPolicy 同步指定角色的权限策略到数据库
// 功能：先删除该角色的所有旧权限，再添加新权限（全量更新）
//
// 与UpdateCasbin的区别：
// 1. UpdateCasbin：通过Casbin API操作，自动同步内存和数据库
// 2. SyncPolicy：直接操作数据库，需要手动刷新内存缓存
//
// 使用场景：
// 1. 在事务中批量同步权限（需要传入事务的db对象）
// 2. 数据迁移：从其他系统导入权限数据
// 3. 批量更新：系统升级时批量更新权限
//
// 设计思路：
// 1. 先删除旧权限，再添加新权限（全量更新策略）
// 2. 使用同一个db对象，支持事务，保证原子性
// 3. 操作完成后需要调用FreshCasbin()刷新内存缓存
//
// 为什么需要这个方法：
// - 在事务中操作时，需要传入事务的db对象，保证数据一致性
// - 批量操作时，可以先操作数据库，最后统一刷新缓存，提高性能
// - 数据迁移场景，需要直接操作数据库，不经过Casbin API
func (casbinService *CasbinService) SyncPolicy(db *gorm.DB, authorityId string, rules [][]string) error {
	// 第一步：删除该角色的所有旧权限
	// 使用传入的db对象，支持事务操作
	err := casbinService.RemoveFilteredPolicy(db, authorityId)
	if err != nil {
		return err
	}
	// 第二步：添加新权限
	// 使用同一个db对象，保证在同一个事务中
	return casbinService.AddPolicies(db, rules)
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: AddPolicies
//@description: 添加匹配的权限
//@param: db *gorm.DB 数据库连接，支持事务
//@param: rules [][]string 权限规则列表，每个规则格式为[角色ID, 路径, HTTP方法]
//@return: error

// AddPolicies 批量添加权限策略到数据库
// 与Casbin的AddPolicies API的区别：
// 1. Casbin API：会同时更新内存和数据库
// 2. 本方法：只操作数据库，需要手动刷新内存缓存
//
// 使用场景：
// 1. 在事务中批量添加权限（需要传入事务的db对象）
// 2. 数据迁移：从其他系统导入权限数据
// 3. 批量操作：系统升级时批量添加权限
//
// 设计说明：
// 1. 使用GORM的批量插入（Create），比逐条插入效率高
// 2. 支持事务，保证数据一致性
// 3. 需要调用FreshCasbin()刷新内存缓存
//
// Casbin规则字段说明：
// - Ptype: "p" 表示策略类型（policy），还有"g"表示角色继承（group）
// - V0: 角色ID（subject）
// - V1: API路径（object）
// - V2: HTTP方法（action）
func (casbinService *CasbinService) AddPolicies(db *gorm.DB, rules [][]string) error {
	var casbinRules []gormadapter.CasbinRule
	// 将字符串数组格式的规则转换为CasbinRule结构体
	// 批量构建，提高效率
	for i := range rules {
		casbinRules = append(casbinRules, gormadapter.CasbinRule{
			Ptype: "p",         // 策略类型：p表示权限策略
			V0:    rules[i][0], // 角色ID
			V1:    rules[i][1], // API路径
			V2:    rules[i][2], // HTTP方法
		})
	}
	// 使用GORM的批量插入，一次SQL操作插入多条记录
	// 比逐条插入效率高，减少数据库交互次数
	return db.Create(&casbinRules).Error
}

// FreshCasbin 刷新Casbin策略缓存，从数据库重新加载策略到内存
// 使用场景：
// 1. 使用RemoveFilteredPolicy、AddPolicies等直接操作数据库的方法后
// 2. 批量更新权限后，需要刷新缓存使新权限生效
// 3. 系统启动时加载权限策略
// 4. 权限数据被外部修改后，需要刷新缓存
//
// 为什么需要这个方法：
// - Casbin为了提高性能，将策略加载到内存中
// - 直接操作数据库后，内存中的策略不会自动更新
// - 必须调用LoadPolicy()重新从数据库加载策略到内存
// - 否则权限检查会使用旧的缓存数据，导致权限失效
//
// 性能考虑：
// - LoadPolicy会从数据库读取所有策略，如果策略数量很大，可能较慢
// - 建议在批量操作完成后统一刷新，而不是每次操作都刷新
// - 在高并发场景下，可以考虑使用读写锁保护
func (casbinService *CasbinService) FreshCasbin() (err error) {
	e := utils.GetCasbin()
	// LoadPolicy会从数据库重新加载所有策略到内存
	// 这会覆盖内存中的旧策略，使用最新的数据库数据
	err = e.LoadPolicy()
	return err
}
