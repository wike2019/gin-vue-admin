package system

import (
	"errors"
	"fmt"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"gorm.io/gorm"
)

// ApiService API服务结构体
// 采用空结构体设计，好处：
// 1. 零内存占用，所有方法都是值接收者，不需要维护状态
// 2. 符合Go语言最佳实践，服务层通常不需要存储状态
// 3. 便于测试，可以轻松创建多个实例进行并发测试
type ApiService struct{}

// ApiServiceApp 全局单例实例
// 使用单例模式的好处：
// 1. 避免重复创建对象，节省内存
// 2. 统一管理服务实例，便于依赖注入
// 3. 符合Go语言包级别的单例模式习惯用法
var ApiServiceApp = new(ApiService)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: CreateApi
//@description: 新增基础api
//@param: api model.SysApi
//@return: err error

// CreateApi 创建新的API记录
// 设计思路：
// 1. 在创建前先检查是否存在相同的API（路径+方法组合唯一）
// 2. 使用 errors.Is 和 gorm.ErrRecordNotFound 进行精确的错误判断
// 3. 好处：避免重复数据，保证API的唯一性约束
// 为什么使用 errors.Is：
//   - 更准确的错误类型判断，不依赖错误消息字符串
//   - 即使错误被包装也能正确识别
//   - 符合Go 1.13+的错误处理最佳实践
func (apiService *ApiService) CreateApi(api system.SysApi) (err error) {
	// 检查是否存在相同的API（路径+HTTP方法的组合必须唯一）
	// 使用 First 查询，如果找到记录说明已存在，返回错误
	// 如果没找到记录（ErrRecordNotFound），说明可以创建
	if !errors.Is(global.GVA_DB.Where("path = ? AND method = ?", api.Path, api.Method).First(&system.SysApi{}).Error, gorm.ErrRecordNotFound) {
		return errors.New("存在相同api")
	}
	// 创建新记录
	return global.GVA_DB.Create(&api).Error
}

// GetApiGroups 获取所有API分组信息
// 返回值：
//   - groups: 所有不重复的API分组名称列表
//   - groupApiMap: 路径前缀到分组名称的映射（用于快速查找）
//   - err: 错误信息
//
// 设计思路：
//  1. 一次性查询所有API，在内存中处理分组逻辑（避免N+1查询问题）
//  2. 使用map存储路径前缀到分组的映射，提高查找效率
//  3. 手动去重分组列表，保证返回的分组不重复
//
// 好处：
//   - 减少数据库查询次数，提高性能
//   - 返回两种格式的数据，满足不同使用场景
//   - 路径前缀映射可以用于自动识别新API的分组
func (apiService *ApiService) GetApiGroups() (groups []string, groupApiMap map[string]string, err error) {
	var apis []system.SysApi
	// 一次性查询所有API，避免多次查询数据库
	err = global.GVA_DB.Find(&apis).Error
	if err != nil {
		return
	}
	// 初始化map，用于存储路径前缀到分组的映射
	groupApiMap = make(map[string]string, 0)

	// 遍历所有API，收集分组信息
	for i := range apis {
		// 通过路径的第一段（如 /api/v1/user 中的 "api"）来映射分组
		pathArr := strings.Split(apis[i].Path, "/")
		newGroup := true
		// 检查当前分组是否已经在groups列表中（手动去重）
		// 为什么不使用map去重？因为需要保持顺序，且分组数量通常不多
		for i2 := range groups {
			if groups[i2] == apis[i].ApiGroup {
				newGroup = false
				break // 找到后立即退出，提高效率
			}
		}
		// 如果是新分组，添加到列表中
		if newGroup {
			groups = append(groups, apis[i].ApiGroup)
		}
		// 建立路径前缀到分组的映射关系
		// 例如：pathArr[1] = "api" -> ApiGroup = "系统管理"
		// 注意：如果路径格式不规范，pathArr[1]可能为空，需要确保路径格式正确
		if len(pathArr) > 1 {
			groupApiMap[pathArr[1]] = apis[i].ApiGroup
		}
	}
	return
}

// SyncApi 同步API：对比内存中的路由和数据库中的API，找出差异
// 返回值：
//   - newApis: 内存中存在但数据库中不存在的API（需要新增）
//   - deleteApis: 数据库中存在但内存中不存在的API（需要删除）
//   - ignoreApis: 被标记为忽略的API列表
//   - err: 错误信息
//
// 设计思路：
//  1. 通过对比内存路由（global.GVA_ROUTERS）和数据库API，实现自动同步
//  2. 支持忽略列表，允许某些API不被同步到数据库
//  3. 返回差异列表而不是直接操作，让调用方决定是否执行同步
//
// 好处：
//   - 自动化管理API，减少手动维护成本
//   - 支持忽略机制，灵活控制哪些API需要管理
//   - 只返回差异，不直接修改数据，更安全可控
//   - 可以用于API变更的审计和预览
func (apiService *ApiService) SyncApi() (newApis, deleteApis, ignoreApis []system.SysApi, err error) {
	// 初始化三个结果切片，使用make指定容量为0，后续通过append动态扩容
	newApis = make([]system.SysApi, 0)
	deleteApis = make([]system.SysApi, 0)
	ignoreApis = make([]system.SysApi, 0)

	// 第一步：从数据库查询所有已存在的API
	var apis []system.SysApi
	err = global.GVA_DB.Find(&apis).Error
	if err != nil {
		return
	}

	// 第二步：查询所有被忽略的API配置
	var ignores []system.SysIgnoreApi
	err = global.GVA_DB.Find(&ignores).Error
	if err != nil {
		return
	}

	// 第三步：将忽略列表转换为SysApi格式，便于统一处理
	for i := range ignores {
		ignoreApis = append(ignoreApis, system.SysApi{
			Path:        ignores[i].Path,
			Description: "",
			ApiGroup:    "",
			Method:      ignores[i].Method,
		})
	}

	// 第四步：从内存路由中提取API，排除被忽略的API
	// global.GVA_ROUTERS 是应用启动时注册的所有路由
	var cacheApis []system.SysApi
	for i := range global.GVA_ROUTERS {
		ignoresFlag := false
		// 检查当前路由是否在忽略列表中
		for j := range ignores {
			if ignores[j].Path == global.GVA_ROUTERS[i].Path && ignores[j].Method == global.GVA_ROUTERS[i].Method {
				ignoresFlag = true
				break // 找到匹配项后立即退出，提高效率
			}
		}
		// 如果不在忽略列表中，添加到待同步的API列表
		if !ignoresFlag {
			cacheApis = append(cacheApis, system.SysApi{
				Path:   global.GVA_ROUTERS[i].Path,
				Method: global.GVA_ROUTERS[i].Method,
			})
		}
	}

	// 第五步：找出需要新增的API（存在于内存路由，但不存在于数据库）
	// 算法：遍历内存中的API，检查是否在数据库中存在
	// 时间复杂度：O(n*m)，n为内存API数量，m为数据库API数量
	// 优化建议：如果API数量很大，可以使用map提高查找效率到O(n+m)
	for i := range cacheApis {
		var flag bool
		// 在数据库API列表中查找匹配项
		for j := range apis {
			if cacheApis[i].Path == apis[j].Path && cacheApis[i].Method == apis[j].Method {
				flag = true
				break // 找到匹配项后立即退出
			}
		}
		// 如果没找到匹配项，说明这是新API，需要添加到数据库
		if !flag {
			newApis = append(newApis, system.SysApi{
				Path:        cacheApis[i].Path,
				Description: "", // 新API的描述和分组为空，需要后续手动补充
				ApiGroup:    "",
				Method:      cacheApis[i].Method,
			})
		}
	}

	// 第六步：找出需要删除的API（存在于数据库，但不存在于内存路由）
	// 这通常发生在：代码中删除了某个路由，但数据库中还保留着该API记录
	for i := range apis {
		var flag bool
		// 在内存API列表中查找匹配项
		for j := range cacheApis {
			if cacheApis[j].Path == apis[i].Path && cacheApis[j].Method == apis[i].Method {
				flag = true
				break // 找到匹配项后立即退出
			}
		}
		// 如果没找到匹配项，说明这个API已经不在代码中了，需要从数据库删除
		if !flag {
			deleteApis = append(deleteApis, apis[i])
		}
	}
	return
}

// IgnoreApi 添加或移除API忽略配置
// 参数：
//   - ignoreApi.Flag = true: 添加忽略配置
//   - ignoreApi.Flag = false: 移除忽略配置
//
// 设计思路：
//  1. 使用Flag字段控制是添加还是删除，一个函数处理两种操作
//  2. 删除时使用Unscoped()进行硬删除（物理删除），而不是软删除
//
// 好处：
//   - 简化API，一个函数处理两种操作，减少代码重复
//   - 忽略配置使用硬删除，避免软删除带来的数据冗余
//   - 通过Flag字段明确表达操作意图，代码可读性好
func (apiService *ApiService) IgnoreApi(ignoreApi system.SysIgnoreApi) (err error) {
	if ignoreApi.Flag {
		// Flag为true，添加忽略配置
		return global.GVA_DB.Create(&ignoreApi).Error
	}
	// Flag为false，移除忽略配置
	// 使用Unscoped()进行硬删除，直接从数据库中删除记录
	// 为什么不使用软删除？因为忽略配置是临时性的，不需要保留历史记录
	return global.GVA_DB.Unscoped().Delete(&ignoreApi, "path = ? AND method = ?", ignoreApi.Path, ignoreApi.Method).Error
}

// EnterSyncApi 执行API同步操作（新增和删除）
// 参数：syncApis 包含需要新增和删除的API列表
// 设计思路：
//  1. 使用数据库事务保证操作的原子性
//  2. 删除API时同时清理Casbin权限配置，保证数据一致性
//  3. 批量新增使用Create，批量删除使用循环Delete
//
// 好处：
//   - 事务保证：要么全部成功，要么全部回滚，避免数据不一致
//   - 权限同步：删除API时自动清理相关权限，避免脏数据
//   - 错误处理：任何步骤失败都会回滚整个事务
//
// 为什么删除时使用循环而不是批量删除？
//   - 因为需要在删除每个API时调用ClearCasbin清理权限
//   - 如果使用批量删除，无法在删除过程中执行额外的清理逻辑
func (apiService *ApiService) EnterSyncApi(syncApis systemRes.SysSyncApis) (err error) {
	// 使用事务包装所有操作，确保原子性
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		var txErr error

		// 第一步：批量新增API
		// 先检查是否有新API，避免不必要的数据库操作
		if len(syncApis.NewApis) > 0 {
			// 使用事务对象tx而不是global.GVA_DB，确保在同一个事务中
			txErr = tx.Create(&syncApis.NewApis).Error
			if txErr != nil {
				// 返回错误会自动触发事务回滚
				return txErr
			}
		}

		// 第二步：逐个删除API并清理权限
		// 为什么使用循环而不是批量删除？
		// 1. 需要在删除每个API时调用ClearCasbin清理Casbin权限配置
		// 2. 如果某个API删除失败，可以立即返回错误并回滚
		for i := range syncApis.DeleteApis {
			// 先清理Casbin权限配置
			// 参数1表示v0（角色/用户），Path和Method用于定位权限规则
			// 注意：ClearCasbin使用的是全局DB，不在事务中
			// 如果后续删除失败，Casbin已经清理了，这可能导致数据不一致
			// 优化建议：将Casbin操作也纳入事务，或使用补偿机制
			CasbinServiceApp.ClearCasbin(1, syncApis.DeleteApis[i].Path, syncApis.DeleteApis[i].Method)

			// 删除数据库中的API记录
			txErr = tx.Delete(&system.SysApi{}, "path = ? AND method = ?", syncApis.DeleteApis[i].Path, syncApis.DeleteApis[i].Method).Error
			if txErr != nil {
				// 返回错误会自动触发事务回滚
				return txErr
			}
		}

		// 返回nil表示事务成功提交
		return nil
	})
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteApi
//@description: 删除基础api
//@param: api model.SysApi
//@return: err error

// DeleteApi 删除单个API记录
// 设计思路：
//  1. 先查询API是否存在，避免删除不存在的记录
//  2. 删除数据库记录后，同步清理Casbin权限配置
//  3. 使用查询到的完整entity进行删除，确保删除的是正确的记录
//
// 好处：
//   - 先查询后删除，可以获取完整的API信息（如Path和Method）
//   - 删除后立即清理权限，保证数据一致性
//   - 如果API不存在，提前返回错误，避免无效操作
//
// 为什么不直接使用api参数删除？
//   - 因为需要获取完整的Path和Method信息用于清理Casbin
//   - 先查询可以验证记录是否存在，提供更好的错误提示
func (apiService *ApiService) DeleteApi(api system.SysApi) (err error) {
	var entity system.SysApi
	// 第一步：根据ID查询API记录
	// 为什么要先查询？
	// 1. 验证API是否存在，避免删除不存在的记录
	// 2. 获取完整的API信息（Path、Method等），用于后续清理权限
	err = global.GVA_DB.First(&entity, "id = ?", api.ID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// API记录不存在，直接返回错误
		return err
	}

	// 第二步：删除数据库记录
	// 使用查询到的entity进行删除，GORM会自动使用主键（ID）进行删除
	err = global.GVA_DB.Delete(&entity).Error
	if err != nil {
		return err
	}

	// 第三步：清理Casbin权限配置
	// 删除API后，相关的权限规则也需要清理，避免脏数据
	// 参数1表示v0（角色/用户标识），entity.Path和entity.Method用于定位权限规则
	CasbinServiceApp.ClearCasbin(1, entity.Path, entity.Method)
	return nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetAPIInfoList
//@description: 分页获取数据,
//@param: api model.SysApi, info request.PageInfo, order string, desc bool
//@return: list interface{}, total int64, err error

// GetAPIInfoList 分页查询API列表，支持多条件筛选和排序
// 参数说明：
//   - api: 用于筛选条件的API对象（Path、Description、Method、ApiGroup）
//   - info: 分页信息（Page、PageSize）
//   - order: 排序字段名
//   - desc: 是否降序排列
//
// 返回值：
//   - list: API列表
//   - total: 符合条件的总记录数（用于前端分页计算）
//   - err: 错误信息
//
// 设计思路：
//  1. 使用链式查询构建器，动态添加WHERE条件
//  2. 先统计总数，再查询分页数据（两次查询，但保证总数准确）
//  3. 使用白名单验证排序字段，防止SQL注入
//  4. 支持模糊查询（LIKE）和精确查询（=）混合使用
//
// 好处：
//   - 灵活的查询条件，支持多字段组合筛选
//   - 安全的排序字段验证，防止SQL注入攻击
//   - 返回总数和列表，满足前端分页需求
//   - 代码结构清晰，易于维护和扩展
func (apiService *ApiService) GetAPIInfoList(api system.SysApi, info request.PageInfo, order string, desc bool) (list interface{}, total int64, err error) {
	// 计算分页参数
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)

	// 创建查询构建器，使用Model指定查询的表
	db := global.GVA_DB.Model(&system.SysApi{})
	var apiList []system.SysApi

	// 动态添加WHERE条件
	// 使用链式调用，只有条件不为空时才添加WHERE子句
	// 好处：代码简洁，避免写大量的if-else判断

	// Path模糊查询：支持部分匹配
	if api.Path != "" {
		db = db.Where("path LIKE ?", "%"+api.Path+"%")
	}

	// Description模糊查询：支持部分匹配
	if api.Description != "" {
		db = db.Where("description LIKE ?", "%"+api.Description+"%")
	}

	// Method精确查询：HTTP方法必须完全匹配
	if api.Method != "" {
		db = db.Where("method = ?", api.Method)
	}

	// ApiGroup精确查询：分组必须完全匹配
	if api.ApiGroup != "" {
		db = db.Where("api_group = ?", api.ApiGroup)
	}

	// 先统计符合条件的总记录数
	// 为什么在分页查询前统计？
	// 1. 前端需要总数来计算总页数
	// 2. 如果先查询列表，需要额外查询一次才能得到总数
	// 3. 使用相同的查询条件，保证总数和列表的一致性
	err = db.Count(&total).Error
	if err != nil {
		return apiList, total, err
	}

	// 添加分页限制
	db = db.Limit(limit).Offset(offset)

	// 处理排序逻辑
	OrderStr := "id desc" // 默认按ID降序
	if order != "" {
		// 使用白名单验证排序字段，防止SQL注入
		// 只允许预定义的字段进行排序，其他字段直接拒绝
		orderMap := make(map[string]bool, 5)
		orderMap["id"] = true
		orderMap["path"] = true
		orderMap["api_group"] = true
		orderMap["description"] = true
		orderMap["method"] = true

		// 检查排序字段是否在白名单中
		if !orderMap[order] {
			err = fmt.Errorf("非法的排序字段: %v", order)
			return apiList, total, err
		}

		// 构建排序字符串
		OrderStr = order
		if desc {
			OrderStr = order + " desc"
		}
	}

	// 执行查询
	err = db.Order(OrderStr).Find(&apiList).Error
	return apiList, total, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetAllApis
//@description: 获取所有的api
//@return:  apis []model.SysApi, err error

// GetAllApis 获取所有API，根据权限配置进行过滤
// 参数：authorityID 权限ID，用于权限过滤
// 返回值：apis API列表，err 错误信息
// 设计思路：
//  1. 根据权限配置决定是否进行权限过滤
//  2. 如果启用严格权限模式且不是顶级权限，只返回有权限的API
//  3. 否则返回所有API（用于管理员或不需要权限控制的场景）
//
// 好处：
//   - 灵活的权限控制，支持严格模式和宽松模式
//   - 顶级权限（parentAuthorityID=0）可以查看所有API，便于管理
//   - 通过配置开关控制权限严格程度，适应不同业务场景
//
// 性能优化建议：
//   - 当前使用嵌套循环查找匹配的API，时间复杂度O(n*m)
//   - 如果API数量很大，可以使用map优化到O(n+m)
func (apiService *ApiService) GetAllApis(authorityID uint) (apis []system.SysApi, err error) {
	// 第一步：获取父权限ID
	// 如果parentAuthorityID=0，说明是顶级权限，通常不需要过滤
	parentAuthorityID, err := AuthorityServiceApp.GetParentAuthorityID(authorityID)
	if err != nil {
		return nil, err
	}

	// 第二步：查询所有API（先查询全部，后续根据权限过滤）
	err = global.GVA_DB.Order("id desc").Find(&apis).Error

	// 第三步：判断是否需要权限过滤
	// 条件1：parentAuthorityID == 0（顶级权限，不需要过滤）
	// 条件2：!UseStrictAuth（未启用严格权限模式，不需要过滤）
	// 如果满足任一条件，直接返回所有API
	if parentAuthorityID == 0 || !global.GVA_CONFIG.System.UseStrictAuth {
		return
	}

	// 第四步：启用严格权限模式，需要根据权限过滤API
	// 获取当前权限ID对应的所有权限路径（从Casbin中获取）
	paths := CasbinServiceApp.GetPolicyPathByAuthorityId(authorityID)

	// 第五步：筛选出有权限的API
	// 算法：遍历所有API，检查是否在权限路径列表中
	// 时间复杂度：O(n*m)，n为API数量，m为权限路径数量
	// 优化建议：将paths转换为map，查找效率提升到O(1)
	var authApis []system.SysApi
	for i := range apis {
		for j := range paths {
			// 同时匹配Path和Method，确保权限精确控制
			if paths[j].Path == apis[i].Path && paths[j].Method == apis[i].Method {
				authApis = append(authApis, apis[i])
				break // 找到匹配项后立即退出内层循环
			}
		}
	}
	return authApis, err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetApiById
//@description: 根据id获取api
//@param: id float64
//@return: api model.SysApi, err error

// GetApiById 根据ID查询单个API记录
// 参数：id API的主键ID
// 返回值：api API对象，err 错误信息（如果记录不存在，返回gorm.ErrRecordNotFound）
// 设计思路：
//  1. 使用First方法查询单条记录
//  2. 如果记录不存在，GORM会返回ErrRecordNotFound错误
//  3. 使用命名返回值，代码更简洁
//
// 好处：
//   - 代码简洁，一行完成查询
//   - 自动处理记录不存在的情况
//   - 使用命名返回值，调用方可以直接判断错误类型
func (apiService *ApiService) GetApiById(id int) (api system.SysApi, err error) {
	// First方法查询第一条匹配的记录
	// 如果记录不存在，返回gorm.ErrRecordNotFound错误
	err = global.GVA_DB.First(&api, "id = ?", id).Error
	return
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: UpdateApi
//@description: 根据id更新api
//@param: api model.SysApi
//@return: err error

// UpdateApi 更新API记录
// 设计思路：
//  1. 先查询旧记录，获取原始信息
//  2. 如果Path或Method发生变化，检查是否存在重复
//  3. 更新Casbin权限配置（如果Path或Method变化）
//  4. 最后更新数据库记录
//
// 好处：
//   - 更新前验证唯一性，避免数据冲突
//   - 同步更新权限配置，保证数据一致性
//   - 先查询旧记录，可以获取完整信息用于权限更新
//
// 为什么需要先查询旧记录？
//   - 需要对比Path和Method是否变化，决定是否需要更新权限
//   - 需要旧值来更新Casbin中的权限规则
func (apiService *ApiService) UpdateApi(api system.SysApi) (err error) {
	// 第一步：查询旧记录，获取原始信息
	var oldA system.SysApi
	err = global.GVA_DB.First(&oldA, "id = ?", api.ID).Error

	// 第二步：如果Path或Method发生变化，需要检查是否存在重复
	// 为什么只检查Path和Method变化的情况？
	// 因为Path+Method的组合必须唯一，如果这两个字段没变化，就不需要检查重复
	if oldA.Path != api.Path || oldA.Method != api.Method {
		var duplicateApi system.SysApi
		// 查询是否存在相同的Path+Method组合
		if ferr := global.GVA_DB.First(&duplicateApi, "path = ? AND method = ?", api.Path, api.Method).Error; ferr != nil {
			// 如果查询出错，且不是"记录不存在"的错误，返回错误
			if !errors.Is(ferr, gorm.ErrRecordNotFound) {
				return ferr
			}
			// 如果是"记录不存在"的错误，说明没有重复，可以继续
		} else {
			// 如果找到了记录，检查是否是同一条记录（允许自己更新自己）
			if duplicateApi.ID != api.ID {
				// 找到了其他记录，说明存在重复，返回错误
				return errors.New("存在相同api路径")
			}
			// 如果是同一条记录，说明只是更新其他字段，可以继续
		}
	}

	// 检查查询旧记录时是否有错误
	if err != nil {
		return err
	}

	// 第三步：如果Path或Method发生变化，需要同步更新Casbin权限配置
	// 为什么需要更新Casbin？
	// 因为Casbin中存储的权限规则使用的是Path和Method，如果这些值变化了，
	// 需要更新Casbin中的规则，否则权限控制会失效
	err = CasbinServiceApp.UpdateCasbinApi(oldA.Path, api.Path, oldA.Method, api.Method)
	if err != nil {
		return err
	}

	// 第四步：更新数据库记录
	// 使用Save方法，GORM会根据主键（ID）自动判断是更新还是插入
	return global.GVA_DB.Save(&api).Error
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteApisByIds
//@description: 删除选中API
//@param: apis []model.SysApi
//@return: err error

// DeleteApisByIds 批量删除API（根据ID列表）
// 参数：ids 包含要删除的API ID列表
// 返回值：err 错误信息
// 设计思路：
//  1. 使用事务保证操作的原子性
//  2. 先查询要删除的API，获取完整信息（用于清理权限）
//  3. 批量删除数据库记录
//  4. 循环清理每个API的Casbin权限配置
//
// 好处：
//   - 事务保证：要么全部成功，要么全部回滚
//   - 先查询后删除，可以获取完整信息用于权限清理
//   - 批量删除提高效率，减少数据库交互次数
//
// 注意：
//   - Casbin清理操作在事务外执行，如果后续删除失败，可能导致数据不一致
//   - 优化建议：将Casbin操作也纳入事务，或使用补偿机制
func (apiService *ApiService) DeleteApisByIds(ids request.IdsReq) (err error) {
	// 使用事务包装所有操作，确保原子性
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 第一步：根据ID列表查询要删除的API
		// 为什么要先查询？
		// 1. 获取完整的API信息（Path、Method），用于后续清理权限
		// 2. 验证ID是否有效，避免删除不存在的记录
		var apis []system.SysApi
		err = tx.Find(&apis, "id in ?", ids.Ids).Error
		if err != nil {
			return err
		}

		// 第二步：批量删除数据库记录
		// 使用IN查询批量删除，比循环删除效率更高
		// 注意：使用空切片[]system.SysApi{}作为模型，GORM会根据WHERE条件删除
		err = tx.Delete(&[]system.SysApi{}, "id in ?", ids.Ids).Error
		if err != nil {
			return err
		}

		// 第三步：清理每个API的Casbin权限配置
		// 为什么使用循环而不是批量操作？
		// 因为ClearCasbin方法需要逐个处理，且使用的是全局DB（不在事务中）
		// 注意：如果这里出错，数据库已经删除了，但Casbin可能还有残留
		// 优化建议：考虑将Casbin操作也纳入事务，或使用消息队列异步处理
		for _, sysApi := range apis {
			CasbinServiceApp.ClearCasbin(1, sysApi.Path, sysApi.Method)
		}

		// 返回err（此时应该是nil，表示成功）
		// 注意：这里返回的是外层err变量，如果前面都成功，err应该是nil
		return err
	})
}
