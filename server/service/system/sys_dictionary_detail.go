package system

import (
	"fmt"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
)

// DictionaryDetailService 字典详情服务结构体
// 该服务实现了字典详情的树形结构管理，支持多层级嵌套
// 设计思路：
//  1. 使用 parent_id 建立父子关系，实现树形结构
//  2. 使用 level 字段记录层级深度，便于快速查询和限制层级
//  3. 使用 path 字段存储从根节点到当前节点的路径，格式为 "1,2,3"
//     好处：可以快速获取所有祖先节点，无需递归查询数据库，提高查询效率
//  4. 这种设计在数据量大的情况下，可以避免频繁的递归查询，提升性能
type DictionaryDetailService struct{}

// DictionaryDetailServiceApp 全局服务实例
// 使用单例模式，避免重复创建服务对象，节省内存
var DictionaryDetailServiceApp = new(DictionaryDetailService)

// CreateSysDictionaryDetail 创建字典详情数据
// 设计说明：
//  1. 在创建时自动计算层级（Level）和路径（Path），确保数据一致性
//     好处：避免手动设置错误，保证树形结构的正确性
//  2. 如果有父节点，层级 = 父节点层级 + 1，路径 = 父节点路径 + 父节点ID
//     好处：路径字段可以快速定位节点在树中的位置，无需递归查询
//  3. 如果没有父节点，则为根节点（Level=0, Path=""）
//     好处：明确区分根节点和子节点，便于树形结构的构建
func (dictionaryDetailService *DictionaryDetailService) CreateSysDictionaryDetail(sysDictionaryDetail system.SysDictionaryDetail) (err error) {
	// 计算层级和路径
	// 如果指定了父节点，需要查询父节点信息来计算当前节点的层级和路径
	if sysDictionaryDetail.ParentID != nil {
		var parent system.SysDictionaryDetail
		err = global.GVA_DB.First(&parent, *sysDictionaryDetail.ParentID).Error
		if err != nil {
			return err
		}
		// 层级计算：子节点的层级 = 父节点层级 + 1
		// 这样设计的好处：可以快速判断节点深度，便于限制树的最大深度
		sysDictionaryDetail.Level = parent.Level + 1
		// 路径计算：如果父节点是根节点（Path为空），则路径就是父节点ID
		// 否则路径 = 父节点路径 + "," + 父节点ID
		// 路径格式示例："1,2,3" 表示从根节点1 -> 节点2 -> 节点3的路径
		// 这样设计的好处：
		// - 可以通过路径快速获取所有祖先节点ID，无需递归查询
		// - 可以快速判断两个节点的关系（是否在同一分支）
		// - 可以快速查询某个节点的所有后代（使用 LIKE 查询）
		if parent.Path == "" {
			sysDictionaryDetail.Path = strconv.Itoa(int(parent.ID))
		} else {
			sysDictionaryDetail.Path = parent.Path + "," + strconv.Itoa(int(parent.ID))
		}
	} else {
		// 根节点：层级为0，路径为空
		// 这样设计的好处：根节点有明确的标识，便于查询和展示
		sysDictionaryDetail.Level = 0
		sysDictionaryDetail.Path = ""
	}

	err = global.GVA_DB.Create(&sysDictionaryDetail).Error
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteSysDictionaryDetail
//@description: 删除字典详情数据
//@param: sysDictionaryDetail model.SysDictionaryDetail
//@return: err error

// DeleteSysDictionaryDetail 删除字典详情数据
// 设计说明：
//  1. 删除前检查是否有子项，如果有子项则不允许删除
//     好处：防止误删导致数据丢失，保证树形结构的完整性
//  2. 这种"级联保护"的设计可以避免孤儿节点，确保数据一致性
//     好处：维护数据的完整性，避免出现无父节点的子节点
//  3. 如果需要级联删除，可以修改此逻辑，但需要谨慎处理
func (dictionaryDetailService *DictionaryDetailService) DeleteSysDictionaryDetail(sysDictionaryDetail system.SysDictionaryDetail) (err error) {
	// 检查是否有子项
	// 使用 Count 查询而不是 Find，性能更好，只需要获取数量
	var count int64
	err = global.GVA_DB.Model(&system.SysDictionaryDetail{}).Where("parent_id = ?", sysDictionaryDetail.ID).Count(&count).Error
	if err != nil {
		return err
	}
	// 如果有子项，不允许删除
	// 这样设计的好处：保护数据完整性，防止误删导致的数据丢失
	if count > 0 {
		return fmt.Errorf("该字典详情下还有子项，无法删除")
	}

	err = global.GVA_DB.Delete(&sysDictionaryDetail).Error
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: UpdateSysDictionaryDetail
//@description: 更新字典详情数据
//@param: sysDictionaryDetail *model.SysDictionaryDetail
//@return: err error

// UpdateSysDictionaryDetail 更新字典详情数据
// 设计说明：
//  1. 如果更新了父级ID，需要重新计算当前节点的层级和路径
//     好处：保证树形结构的一致性，确保层级和路径始终正确
//  2. 更新前检查循环引用，防止节点成为自己或子项的父级
//     好处：避免树形结构出现循环，导致无限递归等问题
//  3. 更新当前节点后，需要递归更新所有子项的层级和路径
//     好处：当父节点移动时，所有子节点自动更新，保持树形结构正确
//  4. 这种"级联更新"的设计确保了树形结构的一致性
func (dictionaryDetailService *DictionaryDetailService) UpdateSysDictionaryDetail(sysDictionaryDetail *system.SysDictionaryDetail) (err error) {
	// 如果更新了父级ID，需要重新计算层级和路径
	// 这样设计的好处：无论父节点如何变化，子节点的层级和路径都能正确更新
	if sysDictionaryDetail.ParentID != nil {
		var parent system.SysDictionaryDetail
		err = global.GVA_DB.First(&parent, *sysDictionaryDetail.ParentID).Error
		if err != nil {
			return err
		}

		// 检查循环引用
		// 防止将节点设置为自己或其子项的父级，避免树形结构出现循环
		// 这样设计的好处：
		// - 防止无限递归导致的程序崩溃
		// - 保证树形结构的有效性
		// - 提供清晰的错误提示
		if dictionaryDetailService.checkCircularReference(sysDictionaryDetail.ID, *sysDictionaryDetail.ParentID) {
			return fmt.Errorf("不能将字典详情设置为自己或其子项的父级")
		}

		// 重新计算层级和路径（逻辑与创建时相同）
		sysDictionaryDetail.Level = parent.Level + 1
		if parent.Path == "" {
			sysDictionaryDetail.Path = strconv.Itoa(int(parent.ID))
		} else {
			sysDictionaryDetail.Path = parent.Path + "," + strconv.Itoa(int(parent.ID))
		}
	} else {
		// 如果设置为根节点，重置层级和路径
		sysDictionaryDetail.Level = 0
		sysDictionaryDetail.Path = ""
	}

	// 先保存当前节点的更新
	err = global.GVA_DB.Save(sysDictionaryDetail).Error
	if err != nil {
		return err
	}

	// 更新所有子项的层级和路径
	// 这样设计的好处：当父节点移动时，所有子节点自动跟随更新
	// 保证了树形结构的一致性，无需手动更新每个子节点
	return dictionaryDetailService.updateChildrenLevelAndPath(sysDictionaryDetail.ID)
}

// checkCircularReference 检查循环引用
// 设计说明：
//  1. 使用递归方式检查是否存在循环引用
//     好处：代码简洁，逻辑清晰，能够检测任意深度的循环
//  2. 检查逻辑：如果目标节点ID等于父节点ID，或者父节点的祖先链中包含目标节点，则存在循环
//     好处：防止节点成为自己或子项的父级，保证树形结构的有效性
//  3. 递归终止条件：
//     - 如果 id == parentID，直接返回 true（自己不能是自己的父级）
//     - 如果父节点不存在，返回 false（没有循环）
//     - 如果父节点是根节点（ParentID == nil），返回 false（到达根节点，没有循环）
//  4. 这种设计可以检测任意深度的循环，例如：A -> B -> C -> A
func (dictionaryDetailService *DictionaryDetailService) checkCircularReference(id, parentID uint) bool {
	// 如果目标节点ID等于父节点ID，说明试图将节点设置为自己的父级，存在循环
	if id == parentID {
		return true
	}

	// 查询父节点信息
	var parent system.SysDictionaryDetail
	err := global.GVA_DB.First(&parent, parentID).Error
	if err != nil {
		// 如果父节点不存在，说明没有循环（可能是数据错误，但不属于循环引用）
		return false
	}

	// 如果父节点是根节点（没有父级），说明已经到达树顶，没有循环
	if parent.ParentID == nil {
		return false
	}

	// 递归检查父节点的父级，看是否在祖先链中找到目标节点
	// 这样设计的好处：可以检测任意深度的循环引用
	return dictionaryDetailService.checkCircularReference(id, *parent.ParentID)
}

// updateChildrenLevelAndPath 更新子项的层级和路径
// 设计说明：
//  1. 当父节点的层级或路径发生变化时，需要递归更新所有子节点
//     好处：保证整个子树的一致性，确保所有节点的层级和路径都正确
//  2. 使用递归方式更新，可以处理任意深度的树形结构
//     好处：代码简洁，能够处理多层级嵌套的情况
//  3. 先更新直接子节点，再递归更新子节点的子节点
//     好处：确保更新顺序正确，子节点的路径基于父节点的新路径计算
//  4. 这种"级联更新"的设计确保了树形结构的一致性
//     好处：当父节点移动时，整个子树自动跟随更新，无需手动处理
func (dictionaryDetailService *DictionaryDetailService) updateChildrenLevelAndPath(parentID uint) error {
	// 查询所有直接子节点
	var children []system.SysDictionaryDetail
	err := global.GVA_DB.Where("parent_id = ?", parentID).Find(&children).Error
	if err != nil {
		return err
	}

	// 查询父节点信息，用于计算子节点的新层级和路径
	var parent system.SysDictionaryDetail
	err = global.GVA_DB.First(&parent, parentID).Error
	if err != nil {
		return err
	}

	// 遍历所有子节点，更新它们的层级和路径
	for _, child := range children {
		// 重新计算子节点的层级：子节点层级 = 父节点层级 + 1
		child.Level = parent.Level + 1
		// 重新计算子节点的路径：基于父节点的新路径
		if parent.Path == "" {
			child.Path = strconv.Itoa(int(parent.ID))
		} else {
			child.Path = parent.Path + "," + strconv.Itoa(int(parent.ID))
		}

		// 保存子节点的更新
		err = global.GVA_DB.Save(&child).Error
		if err != nil {
			return err
		}

		// 递归更新子项的子项
		// 这样设计的好处：无论树有多深，都能正确更新所有层级的节点
		// 确保整个子树的一致性
		err = dictionaryDetailService.updateChildrenLevelAndPath(child.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetSysDictionaryDetail
//@description: 根据id获取字典详情单条数据
//@param: id uint
//@return: sysDictionaryDetail system.SysDictionaryDetail, err error

// GetSysDictionaryDetail 根据id获取字典详情单条数据
// 设计说明：
//  1. 使用 First 方法获取单条记录，如果不存在会返回错误
//     好处：明确区分"记录不存在"和"查询出错"两种情况
//  2. 使用 Where 条件查询，性能优于直接使用 First(id)
//     好处：代码更清晰，便于后续扩展查询条件
func (dictionaryDetailService *DictionaryDetailService) GetSysDictionaryDetail(id uint) (sysDictionaryDetail system.SysDictionaryDetail, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&sysDictionaryDetail).Error
	return
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetSysDictionaryDetailInfoList
//@description: 分页获取字典详情列表
//@param: info request.SysDictionaryDetailSearch
//@return: list interface{}, total int64, err error

// GetSysDictionaryDetailInfoList 分页获取字典详情列表
// 设计说明：
//  1. 使用链式查询构建器，根据条件动态添加查询条件
//     好处：代码灵活，支持多条件组合查询，避免写多个查询方法
//  2. 先查询总数，再查询分页数据
//     好处：前端可以显示总记录数和分页信息
//  3. 使用 LIKE 查询支持模糊搜索（Label字段）
//     好处：提供更好的用户体验，支持部分匹配搜索
//  4. 使用精确匹配查询（Value、Status等字段）
//     好处：查询性能更好，结果更准确
//  5. 支持按层级（Level）和父节点（ParentID）查询
//     好处：可以快速筛选特定层级的节点或某个节点的子节点
//  6. 排序规则：先按 sort 排序，再按 id 排序
//     好处：支持自定义排序，同时保证相同 sort 值的记录有稳定的排序
func (dictionaryDetailService *DictionaryDetailService) GetSysDictionaryDetailInfoList(info request.SysDictionaryDetailSearch) (list interface{}, total int64, err error) {
	// 计算分页参数
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建查询构建器
	db := global.GVA_DB.Model(&system.SysDictionaryDetail{})
	var sysDictionaryDetails []system.SysDictionaryDetail
	// 根据条件动态添加查询条件
	// 这样设计的好处：支持多条件组合查询，代码复用性高
	if info.Label != "" {
		// 使用 LIKE 进行模糊搜索，支持部分匹配
		// 好处：提供更好的搜索体验，用户不需要输入完整标签
		db = db.Where("label LIKE ?", "%"+info.Label+"%")
	}
	if info.Value != "" {
		// 精确匹配查询，性能更好
		db = db.Where("value = ?", info.Value)
	}
	if info.Status != nil {
		// 状态筛选，支持启用/禁用状态过滤
		db = db.Where("status = ?", info.Status)
	}
	if info.SysDictionaryID != 0 {
		// 按字典ID筛选，获取特定字典的所有详情
		db = db.Where("sys_dictionary_id = ?", info.SysDictionaryID)
	}
	if info.ParentID != nil {
		// 按父节点筛选，获取特定节点的所有子节点
		// 好处：可以快速查询某个节点的直接子节点
		db = db.Where("parent_id = ?", *info.ParentID)
	}
	if info.Level != nil {
		// 按层级筛选，获取特定层级的所有节点
		// 好处：可以快速查询某一层的所有节点，例如只查询第一层或第二层
		db = db.Where("level = ?", *info.Level)
	}
	// 先查询总数，用于分页显示
	err = db.Count(&total).Error
	if err != nil {
		return
	}
	// 查询分页数据，按 sort 和 id 排序
	// 排序规则：先按 sort 排序（支持自定义排序），再按 id 排序（保证稳定性）
	err = db.Limit(limit).Offset(offset).Order("sort").Order("id").Find(&sysDictionaryDetails).Error
	return sysDictionaryDetails, total, err
}

// GetDictionaryList 按照字典id获取字典全部内容的方法
// 设计说明：
//  1. 获取指定字典的所有详情项，不区分层级，返回扁平列表
//     好处：简单直接，适用于不需要树形结构的场景
//  2. 使用 Find 方法查询所有匹配的记录
//     好处：一次性获取所有数据，减少数据库查询次数
//  3. 注意：此方法返回的是扁平列表，不包含树形结构
//     如果需要树形结构，应使用 GetDictionaryTreeList 方法
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryList(dictionaryID uint) (list []system.SysDictionaryDetail, err error) {
	var sysDictionaryDetails []system.SysDictionaryDetail
	err = global.GVA_DB.Find(&sysDictionaryDetails, "sys_dictionary_id = ?", dictionaryID).Error
	return sysDictionaryDetails, err
}

// GetDictionaryTreeList 获取字典树形结构列表
// 设计说明：
//  1. 只查询顶级节点（parent_id IS NULL），然后递归加载所有子节点
//     好处：避免一次性查询所有数据，按需加载，性能更好
//  2. 使用递归方式构建完整的树形结构
//     好处：代码简洁，能够处理任意深度的树形结构
//  3. 自动设置 disabled 属性，便于前端控制节点是否可操作
//     好处：前端可以直接使用 disabled 属性，无需额外处理
//  4. 按 sort 字段排序，保证树形结构的展示顺序
//     好处：支持自定义排序，树形结构展示更有序
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryTreeList(dictionaryID uint) (list []system.SysDictionaryDetail, err error) {
	var sysDictionaryDetails []system.SysDictionaryDetail
	// 只获取顶级项目（parent_id为空）
	// 这样设计的好处：
	// - 只查询根节点，减少初始查询的数据量
	// - 按需递归加载子节点，性能更好
	// - 支持按 sort 排序，保证根节点的展示顺序
	err = global.GVA_DB.Where("sys_dictionary_id = ? AND parent_id IS NULL", dictionaryID).Order("sort").Find(&sysDictionaryDetails).Error
	if err != nil {
		return nil, err
	}

	// 递归加载子项并设置disabled属性
	for i := range sysDictionaryDetails {
		// 设置disabled属性：当status为false时，disabled为true
		// 这样设计的好处：
		// - 前端可以直接使用 disabled 属性控制UI状态
		// - 禁用状态的节点通常不可选择或不可操作
		// - 提供更好的用户体验
		if sysDictionaryDetails[i].Status != nil {
			sysDictionaryDetails[i].Disabled = !*sysDictionaryDetails[i].Status
		} else {
			sysDictionaryDetails[i].Disabled = false // 默认不禁用
		}

		// 递归加载子节点，构建完整的树形结构
		err = dictionaryDetailService.loadChildren(&sysDictionaryDetails[i])
		if err != nil {
			return nil, err
		}
	}

	return sysDictionaryDetails, nil
}

// loadChildren 递归加载子项
// 设计说明：
//  1. 使用递归方式加载所有层级的子节点
//     好处：代码简洁，能够处理任意深度的树形结构
//  2. 每次只查询直接子节点，然后递归查询子节点的子节点
//     好处：按需加载，避免一次性查询大量数据，性能更好
//  3. 自动设置 disabled 属性，保持与父节点一致的处理逻辑
//     好处：整个树形结构的节点都有统一的 disabled 属性
//  4. 按 sort 字段排序，保证子节点的展示顺序
//     好处：支持自定义排序，树形结构展示更有序
//  5. 将子节点赋值给 detail.Children，构建树形结构
//     好处：前端可以直接使用树形结构，无需额外处理
func (dictionaryDetailService *DictionaryDetailService) loadChildren(detail *system.SysDictionaryDetail) error {
	// 查询当前节点的直接子节点
	var children []system.SysDictionaryDetail
	err := global.GVA_DB.Where("parent_id = ?", detail.ID).Order("sort").Find(&children).Error
	if err != nil {
		return err
	}

	// 遍历子节点，设置 disabled 属性并递归加载子节点的子节点
	for i := range children {
		// 设置disabled属性：当status为false时，disabled为true
		// 这样设计的好处：前端可以直接使用 disabled 属性控制UI状态
		if children[i].Status != nil {
			children[i].Disabled = !*children[i].Status
		} else {
			children[i].Disabled = false // 默认不禁用
		}

		// 递归加载子节点的子节点
		// 这样设计的好处：无论树有多深，都能正确加载所有层级的节点
		err = dictionaryDetailService.loadChildren(&children[i])
		if err != nil {
			return err
		}
	}

	// 将子节点赋值给当前节点的 Children 字段，构建树形结构
	detail.Children = children
	return nil
}

// GetDictionaryDetailsByParent 根据父级ID获取字典详情
// 设计说明：
//  1. 支持查询指定父节点的子节点，也支持查询根节点（ParentID为nil）
//     好处：灵活性强，可以查询任意层级的节点
//  2. 可选的 IncludeChildren 参数，控制是否递归加载所有子节点
//     好处：按需加载，如果只需要直接子节点，可以避免不必要的递归查询，提升性能
//  3. 自动设置 disabled 属性，保持与其他查询方法的一致性
//     好处：前端处理逻辑统一，无需针对不同方法做特殊处理
//  4. 按 sort 字段排序，保证返回结果的顺序
//     好处：支持自定义排序，结果展示更有序
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryDetailsByParent(req request.GetDictionaryDetailsByParentRequest) (list []system.SysDictionaryDetail, err error) {
	// 先按字典ID筛选
	db := global.GVA_DB.Model(&system.SysDictionaryDetail{}).Where("sys_dictionary_id = ?", req.SysDictionaryID)

	// 根据是否指定父节点ID，查询子节点或根节点
	if req.ParentID != nil {
		// 查询指定父节点的直接子节点
		db = db.Where("parent_id = ?", *req.ParentID)
	} else {
		// 查询根节点（没有父节点的节点）
		db = db.Where("parent_id IS NULL")
	}

	// 按 sort 排序查询
	err = db.Order("sort").Find(&list).Error
	if err != nil {
		return list, err
	}

	// 设置disabled属性
	// 这样设计的好处：前端可以直接使用 disabled 属性控制UI状态
	for i := range list {
		if list[i].Status != nil {
			list[i].Disabled = !*list[i].Status
		} else {
			list[i].Disabled = false // 默认不禁用
		}
	}

	// 如果需要包含子级数据，使用递归方式加载所有层级的子项
	// 这样设计的好处：
	// - 按需加载：如果只需要直接子节点，可以避免不必要的递归查询，提升性能
	// - 灵活性：需要完整树形结构时，可以通过 IncludeChildren=true 获取
	if req.IncludeChildren {
		for i := range list {
			err = dictionaryDetailService.loadChildren(&list[i])
			if err != nil {
				return list, err
			}
		}
	}

	return list, err
}

// GetDictionaryListByType 按照字典type获取字典全部内容的方法
// 设计说明：
//  1. 通过 JOIN 查询关联字典表，根据字典类型（type）获取所有详情
//     好处：不需要先查询字典ID，直接通过类型查询，使用更方便
//  2. 使用 JOIN 而不是子查询，性能更好
//     好处：数据库可以优化 JOIN 查询，比子查询效率更高
//  3. 返回扁平列表，不包含树形结构
//     好处：简单直接，适用于不需要树形结构的场景
//  4. 注意：此方法返回的是扁平列表，如果需要树形结构，应使用 GetDictionaryTreeListByType 方法
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryListByType(t string) (list []system.SysDictionaryDetail, err error) {
	var sysDictionaryDetails []system.SysDictionaryDetail
	// 使用 JOIN 关联字典表，根据字典类型查询
	// 这样设计的好处：
	// - 不需要先查询字典ID，直接通过类型查询，使用更方便
	// - JOIN 查询性能优于子查询
	db := global.GVA_DB.Model(&system.SysDictionaryDetail{}).Joins("JOIN sys_dictionaries ON sys_dictionaries.id = sys_dictionary_details.sys_dictionary_id")
	err = db.Find(&sysDictionaryDetails, "type = ?", t).Error
	return sysDictionaryDetails, err
}

// GetDictionaryTreeListByType 根据字典类型获取树形结构
// 设计说明：
//  1. 通过 JOIN 查询关联字典表，根据字典类型（type）获取树形结构
//     好处：不需要先查询字典ID，直接通过类型查询，使用更方便
//  2. 只查询根节点（parent_id IS NULL），然后递归加载所有子节点
//     好处：避免一次性查询所有数据，按需加载，性能更好
//  3. 使用递归方式构建完整的树形结构
//     好处：代码简洁，能够处理任意深度的树形结构
//  4. 自动设置 disabled 属性，便于前端控制节点是否可操作
//     好处：前端可以直接使用 disabled 属性，无需额外处理
//  5. 按 sort 字段排序，保证树形结构的展示顺序
//     好处：支持自定义排序，树形结构展示更有序
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryTreeListByType(t string) (list []system.SysDictionaryDetail, err error) {
	var sysDictionaryDetails []system.SysDictionaryDetail
	// 使用 JOIN 关联字典表，查询指定类型的根节点
	// 这样设计的好处：
	// - 不需要先查询字典ID，直接通过类型查询，使用更方便
	// - 只查询根节点，减少初始查询的数据量
	// - 按 sort 排序，保证根节点的展示顺序
	db := global.GVA_DB.Model(&system.SysDictionaryDetail{}).
		Joins("JOIN sys_dictionaries ON sys_dictionaries.id = sys_dictionary_details.sys_dictionary_id").
		Where("sys_dictionaries.type = ? AND sys_dictionary_details.parent_id IS NULL", t).
		Order("sys_dictionary_details.sort")

	err = db.Find(&sysDictionaryDetails).Error
	if err != nil {
		return nil, err
	}

	// 递归加载子项并设置disabled属性
	for i := range sysDictionaryDetails {
		// 设置disabled属性：当status为false时，disabled为true
		// 这样设计的好处：前端可以直接使用 disabled 属性控制UI状态
		if sysDictionaryDetails[i].Status != nil {
			sysDictionaryDetails[i].Disabled = !*sysDictionaryDetails[i].Status
		} else {
			sysDictionaryDetails[i].Disabled = false // 默认不禁用
		}

		// 递归加载子节点，构建完整的树形结构
		err = dictionaryDetailService.loadChildren(&sysDictionaryDetails[i])
		if err != nil {
			return nil, err
		}
	}

	return sysDictionaryDetails, nil
}

// GetDictionaryInfoByValue 按照字典id+字典内容value获取单条字典内容
// 设计说明：
//  1. 通过字典ID和值（value）精确查询单条记录
//     好处：value 通常是唯一标识，可以精确定位到具体的字典项
//  2. 使用 First 方法获取单条记录，如果不存在会返回错误
//     好处：明确区分"记录不存在"和"查询出错"两种情况
//  3. 适用于需要根据值查找字典项的场景，例如：根据状态值查找状态名称
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryInfoByValue(dictionaryID uint, value string) (detail system.SysDictionaryDetail, err error) {
	var sysDictionaryDetail system.SysDictionaryDetail
	err = global.GVA_DB.First(&sysDictionaryDetail, "sys_dictionary_id = ? and value = ?", dictionaryID, value).Error
	return sysDictionaryDetail, err
}

// GetDictionaryInfoByTypeValue 按照字典type+字典内容value获取单条字典内容
// 设计说明：
//  1. 通过字典类型（type）和值（value）精确查询单条记录
//     好处：不需要先查询字典ID，直接通过类型和值查询，使用更方便
//  2. 使用 JOIN 关联字典表，根据字典类型查询
//     好处：不需要先查询字典ID，直接通过类型查询，使用更方便
//  3. 使用 First 方法获取单条记录，如果不存在会返回错误
//     好处：明确区分"记录不存在"和"查询出错"两种情况
//  4. 适用于需要根据类型和值查找字典项的场景
//     好处：使用更便捷，不需要知道字典ID，只需要知道类型即可
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryInfoByTypeValue(t string, value string) (detail system.SysDictionaryDetail, err error) {
	var sysDictionaryDetails system.SysDictionaryDetail
	// 使用 JOIN 关联字典表，根据字典类型和值查询
	// 这样设计的好处：
	// - 不需要先查询字典ID，直接通过类型和值查询，使用更方便
	// - JOIN 查询性能优于子查询
	db := global.GVA_DB.Model(&system.SysDictionaryDetail{}).Joins("JOIN sys_dictionaries ON sys_dictionaries.id = sys_dictionary_details.sys_dictionary_id")
	err = db.First(&sysDictionaryDetails, "sys_dictionaries.type = ? and sys_dictionary_details.value = ?", t, value).Error
	return sysDictionaryDetails, err
}

// GetDictionaryPath 获取字典详情的完整路径
// 设计说明：
//  1. 使用递归方式从当前节点向上追溯到根节点，获取完整路径
//     好处：代码简洁，能够处理任意深度的树形结构
//  2. 返回路径数组，从根节点到当前节点的顺序
//     好处：前端可以直接使用路径数组展示面包屑导航或层级信息
//  3. 虽然可以使用 Path 字段（如 "1,2,3"）来获取路径，但此方法返回的是完整的节点对象
//     好处：包含节点的所有信息（label、value等），而不仅仅是ID
//  4. 适用于需要展示完整路径的场景，例如：面包屑导航、层级选择器等
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryPath(id uint) (path []system.SysDictionaryDetail, err error) {
	// 查询当前节点
	var detail system.SysDictionaryDetail
	err = global.GVA_DB.First(&detail, id).Error
	if err != nil {
		return nil, err
	}

	// 将当前节点添加到路径中
	path = append(path, detail)

	// 如果当前节点有父节点，递归获取父节点的路径
	if detail.ParentID != nil {
		parentPath, err := dictionaryDetailService.GetDictionaryPath(*detail.ParentID)
		if err != nil {
			return nil, err
		}
		// 将父节点路径放在前面，当前节点放在后面
		// 这样设计的好处：路径数组从根节点到当前节点的顺序，符合常见的展示需求
		path = append(parentPath, path...)
	}

	return path, nil
}

// GetDictionaryPathByValue 根据值获取字典详情的完整路径
// 设计说明：
//  1. 先根据字典ID和值查找节点，再获取该节点的完整路径
//     好处：提供更便捷的接口，不需要先查询节点ID
//  2. 复用 GetDictionaryInfoByValue 和 GetDictionaryPath 方法
//     好处：代码复用，避免重复逻辑，易于维护
//  3. 适用于需要根据值查找节点并获取路径的场景
//     好处：使用更便捷，只需要知道字典ID和值即可获取完整路径
func (dictionaryDetailService *DictionaryDetailService) GetDictionaryPathByValue(dictionaryID uint, value string) (path []system.SysDictionaryDetail, err error) {
	// 先根据字典ID和值查找节点
	detail, err := dictionaryDetailService.GetDictionaryInfoByValue(dictionaryID, value)
	if err != nil {
		return nil, err
	}

	// 再获取该节点的完整路径
	// 这样设计的好处：复用现有方法，代码简洁，易于维护
	return dictionaryDetailService.GetDictionaryPath(detail.ID)
}
