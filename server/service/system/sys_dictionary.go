package system

import (
	"encoding/json"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/gin-gonic/gin"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"gorm.io/gorm"
)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: CreateSysDictionary
//@description: 创建字典数据
//@param: sysDictionary model.SysDictionary
//@return: err error

// DictionaryService 字典服务结构体
// 使用空结构体作为服务类，好处：
// 1. 零内存占用，所有实例共享相同的方法集
// 2. 符合Go语言最佳实践，不需要存储状态的服务使用空结构体
// 3. 便于单例模式实现，避免重复创建服务实例
type DictionaryService struct{}

// DictionaryServiceApp 字典服务单例实例
// 使用包级变量提供全局访问点，好处：
// 1. 单例模式，整个应用只有一个服务实例，节省内存
// 2. 便于依赖注入和测试
// 3. 统一的服务访问入口，代码更清晰
var DictionaryServiceApp = new(DictionaryService)

func (dictionaryService *DictionaryService) CreateSysDictionary(sysDictionary system.SysDictionary) (err error) {
	// 在创建前检查字典类型是否已存在
	// 使用 errors.Is 和 gorm.ErrRecordNotFound 的好处：
	// 1. 精确判断错误类型，避免误判其他数据库错误
	// 2. 符合Go错误处理最佳实践，使用errors.Is进行错误比较
	// 3. 如果查询到记录（非RecordNotFound），说明type已存在，不允许重复创建
	// 这样设计保证了字典类型的唯一性，type作为业务唯一标识符
	if (!errors.Is(global.GVA_DB.First(&system.SysDictionary{}, "type = ?", sysDictionary.Type).Error, gorm.ErrRecordNotFound)) {
		return errors.New("存在相同的type，不允许创建")
	}
	// 通过验证后创建字典记录
	// 使用GORM的Create方法，会自动处理ID生成、时间戳等字段
	err = global.GVA_DB.Create(&sysDictionary).Error
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: DeleteSysDictionary
//@description: 删除字典数据
//@param: sysDictionary model.SysDictionary
//@return: err error

func (dictionaryService *DictionaryService) DeleteSysDictionary(sysDictionary system.SysDictionary) (err error) {
	// 先查询字典记录，同时预加载关联的字典详情
	// 使用Preload的好处：
	// 1. 一次性加载关联数据，避免N+1查询问题
	// 2. 在删除前获取完整数据，便于后续级联删除
	// 3. 可以验证字典是否存在，防止删除不存在的记录
	err = global.GVA_DB.Where("id = ?", sysDictionary.ID).Preload("SysDictionaryDetails").First(&sysDictionary).Error
	// 如果记录不存在，返回友好错误提示
	// 这样设计的好处：
	// 1. 防止误删除或重复删除操作
	// 2. 提供明确的错误信息，便于前端展示
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("请不要搞事")
	}
	if err != nil {
		return err
	}
	// 先删除主字典记录
	// GORM的Delete方法会进行软删除（如果模型定义了DeletedAt字段）
	err = global.GVA_DB.Delete(&sysDictionary).Error
	if err != nil {
		return err
	}

	// 级联删除关联的字典详情
	// 这样设计的好处：
	// 1. 保证数据一致性，避免产生孤儿数据
	// 2. 手动删除关联数据，确保完全清理
	// 3. 即使主记录删除失败，也不会留下不一致的数据
	if sysDictionary.SysDictionaryDetails != nil {
		return global.GVA_DB.Where("sys_dictionary_id=?", sysDictionary.ID).Delete(sysDictionary.SysDictionaryDetails).Error
	}
	return
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: UpdateSysDictionary
//@description: 更新字典数据
//@param: sysDictionary *model.SysDictionary
//@return: err error

func (dictionaryService *DictionaryService) UpdateSysDictionary(sysDictionary *system.SysDictionary) (err error) {
	var dict system.SysDictionary
	// 使用map[string]interface{}构造更新字段，好处：
	// 1. 只更新指定的字段，避免覆盖未传入的字段
	// 2. 可以灵活控制哪些字段需要更新
	// 3. 使用Updates方法时，GORM会忽略零值字段，但map方式更明确
	sysDictionaryMap := map[string]interface{}{
		"Name":     sysDictionary.Name,
		"Type":     sysDictionary.Type,
		"Status":   sysDictionary.Status,
		"Desc":     sysDictionary.Desc,
		"ParentID": sysDictionary.ParentID,
	}
	// 先查询原记录，验证记录是否存在
	// 这样设计的好处：
	// 1. 在更新前验证数据存在性，避免更新不存在的记录
	// 2. 获取原始数据，便于后续业务逻辑判断（如type是否改变）
	err = global.GVA_DB.Where("id = ?", sysDictionary.ID).First(&dict).Error
	if err != nil {
		global.GVA_LOG.Debug(err.Error())
		return errors.New("查询字典数据失败")
	}
	// 如果type字段被修改，需要检查新type是否已存在
	// 这样设计保证了type的唯一性约束，即使更新时也要遵守业务规则
	if dict.Type != sysDictionary.Type {
		if !errors.Is(global.GVA_DB.First(&system.SysDictionary{}, "type = ?", sysDictionary.Type).Error, gorm.ErrRecordNotFound) {
			return errors.New("存在相同的type，不允许创建")
		}
	}

	// 检查是否会形成循环引用
	// 这是树形结构的关键保护机制，好处：
	// 1. 防止父子关系形成循环，如A->B->C->A这样的死循环
	// 2. 保证树形结构的有效性，避免数据逻辑错误
	// 3. 在更新前检查，提前发现问题，避免数据损坏
	if sysDictionary.ParentID != nil && *sysDictionary.ParentID != 0 {
		if err := dictionaryService.checkCircularReference(sysDictionary.ID, *sysDictionary.ParentID); err != nil {
			return err
		}
	}

	// 使用Model(&dict)配合Updates的好处：
	// 1. 基于已查询到的记录进行更新，更安全
	// 2. Updates方法只更新非零值字段，避免误更新
	err = global.GVA_DB.Model(&dict).Updates(sysDictionaryMap).Error
	return err
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: GetSysDictionary
//@description: 根据id或者type获取字典单条数据
//@param: Type string, Id uint
//@return: err error, sysDictionary model.SysDictionary

func (dictionaryService *DictionaryService) GetSysDictionary(Type string, Id uint, status *bool) (sysDictionary system.SysDictionary, err error) {
	// 处理status参数的默认值逻辑
	// 使用指针类型*bool的好处：
	// 1. 可以区分"未传入"（nil）和"传入false"两种情况
	// 2. 提供灵活的查询选项，支持查询启用/禁用/全部状态
	// 3. 当status为nil时，默认查询启用状态（flag=true），符合业务常见需求
	var flag = false
	if status == nil {
		flag = true // 默认查询启用状态的字典
	} else {
		flag = *status // 使用传入的状态值
	}
	// 使用(type = ? OR id = ?)的查询条件，好处：
	// 1. 支持通过type或id两种方式查询，提供灵活的查询接口
	// 2. 同时过滤status状态，只返回符合状态要求的记录
	// 3. 使用Preload预加载关联数据，避免N+1查询问题
	// 4. 在Preload中使用回调函数，可以进一步过滤和排序详情数据：
	//    - 只加载启用状态的详情（status = true）
	//    - 排除已软删除的记录（deleted_at is null）
	//    - 按sort字段排序，保证返回顺序的一致性
	err = global.GVA_DB.Where("(type = ? OR id = ?) and status = ?", Type, Id, flag).Preload("SysDictionaryDetails", func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ? and deleted_at is null", true).Order("sort")
	}).First(&sysDictionary).Error
	return
}

//@author: [piexlmax](https://github.com/piexlmax)
//@author: [SliverHorn](https://github.com/SliverHorn)
//@function: GetSysDictionaryInfoList
//@description: 分页获取字典列表
//@param: info request.SysDictionarySearch
//@return: err error, list interface{}, total int64

func (dictionaryService *DictionaryService) GetSysDictionaryInfoList(c *gin.Context, req request.SysDictionarySearch) (list interface{}, err error) {
	var sysDictionarys []system.SysDictionary
	// 使用WithContext(c)传递gin.Context，好处：
	// 1. 将请求上下文传递给数据库操作，便于日志追踪和超时控制
	// 2. 在分布式系统中，可以传递trace信息，便于链路追踪
	// 3. 符合Go语言context最佳实践，支持请求取消和超时
	query := global.GVA_DB.WithContext(c)
	// 条件查询：如果提供了Name参数，进行模糊搜索
	// 使用LIKE查询name和type字段，好处：
	// 1. 支持模糊匹配，提升用户体验
	// 2. 同时搜索name和type，扩大搜索范围
	// 3. 使用参数化查询（?占位符），防止SQL注入
	if req.Name != "" {
		query = query.Where("name LIKE ? OR type LIKE ?", "%"+req.Name+"%", "%"+req.Name+"%")
	}
	// 预加载子字典（Children关联）
	// 这样设计的好处：
	// 1. 一次性加载树形结构的子节点，避免多次查询
	// 2. 返回完整的树形结构数据，前端可以直接使用
	// 3. 减少数据库查询次数，提升性能
	query = query.Preload("Children")
	err = query.Find(&sysDictionarys).Error
	return sysDictionarys, err
}

// checkCircularReference 检查是否会形成循环引用
// 这是树形结构数据的关键保护函数，用于防止父子关系形成死循环
// 设计思路：递归向上遍历父级链条，检查是否最终指向当前节点
// 好处：
// 1. 保证树形结构的有效性，避免数据逻辑错误
// 2. 防止出现A->B->C->A这样的循环引用
// 3. 在更新操作前进行检查，提前发现问题
func (dictionaryService *DictionaryService) checkCircularReference(currentID uint, parentID uint) error {
	// 第一层检查：不能将自己设置为父级
	// 这是最直接的循环引用情况，提前拦截
	if currentID == parentID {
		return errors.New("不能将字典设置为自己的父级")
	}

	// 递归检查父级链条
	// 查询父级记录，验证父级是否存在
	var parent system.SysDictionary
	err := global.GVA_DB.Where("id = ?", parentID).First(&parent).Error
	if err != nil {
		// 如果父级不存在，允许设置（可能是新建的父级）
		// 这样设计的好处：支持先创建子节点，后创建父节点的场景
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 父级不存在，允许设置
		}
		return err
	}

	// 如果父级还有父级，继续向上递归检查
	// 递归的好处：
	// 1. 代码简洁，逻辑清晰
	// 2. 可以处理任意深度的树形结构
	// 3. 递归深度通常不会太深（树形结构一般不会超过10层），性能可接受
	if parent.ParentID != nil && *parent.ParentID != 0 {
		return dictionaryService.checkCircularReference(currentID, *parent.ParentID)
	}

	// 如果父级链条中没有找到currentID，说明不会形成循环，允许设置
	return nil
}

//@author: [pixelMax]
//@function: ExportSysDictionary
//@description: 导出字典JSON（包含字典详情）
//@param: id uint
//@return: exportData map[string]interface{}, err error

func (dictionaryService *DictionaryService) ExportSysDictionary(id uint) (exportData map[string]interface{}, err error) {
	var dictionary system.SysDictionary
	// 查询字典及其所有详情
	// 使用Preload预加载关联数据，好处：
	// 1. 一次性加载所有相关数据，避免N+1查询
	// 2. 在Preload回调中按sort排序，保证导出数据的顺序一致性
	err = global.GVA_DB.Where("id = ?", id).Preload("SysDictionaryDetails", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort")
	}).First(&dictionary).Error
	if err != nil {
		return nil, err
	}

	// 清空字典详情中的ID、创建时间、更新时间等字段
	// 这样设计的好处：
	// 1. 导出数据不包含数据库内部字段（ID、时间戳等），数据更纯净
	// 2. 导出的JSON可以跨环境导入，不受原数据库ID影响
	// 3. 只导出业务相关字段，减少数据体积
	// 4. 使用map[string]interface{}构造，便于JSON序列化
	var cleanDetails []map[string]interface{}
	for _, detail := range dictionary.SysDictionaryDetails {
		cleanDetail := map[string]interface{}{
			"label":  detail.Label,
			"value":  detail.Value,
			"extend": detail.Extend,
			"status": detail.Status,
			"sort":   detail.Sort,
			"level":  detail.Level,
			"path":   detail.Path,
		}
		cleanDetails = append(cleanDetails, cleanDetail)
	}

	// 构造导出数据
	// 使用map[string]interface{}的好处：
	// 1. 灵活的数据结构，便于后续扩展
	// 2. 可以直接序列化为JSON，无需额外的结构体定义
	// 3. 字段名使用小写，符合JSON命名规范
	exportData = map[string]interface{}{
		"name":                 dictionary.Name,
		"type":                 dictionary.Type,
		"status":               dictionary.Status,
		"desc":                 dictionary.Desc,
		"sysDictionaryDetails": cleanDetails,
	}

	return exportData, nil
}

//@author: [pixelMax]
//@function: ImportSysDictionary
//@description: 导入字典JSON（包含字典详情）
//@param: jsonStr string
//@return: err error

func (dictionaryService *DictionaryService) ImportSysDictionary(jsonStr string) error {
	// 直接解析JSON到SysDictionary结构体
	// 使用结构体解析的好处：
	// 1. 类型安全，自动进行类型转换和验证
	// 2. 代码简洁，无需手动解析JSON字段
	// 3. 如果JSON格式错误，会立即返回错误，提前发现问题
	var importData system.SysDictionary
	if err := json.Unmarshal([]byte(jsonStr), &importData); err != nil {
		return errors.New("JSON 格式错误: " + err.Error())
	}

	// 验证必填字段
	// 在导入前进行数据验证，好处：
	// 1. 提前发现数据问题，避免部分导入后失败
	// 2. 提供明确的错误信息，便于用户修正数据
	// 3. 保证数据完整性，符合业务规则
	if importData.Name == "" {
		return errors.New("字典名称不能为空")
	}
	if importData.Type == "" {
		return errors.New("字典类型不能为空")
	}

	// 检查字典类型是否已存在
	// 保持与创建字典相同的唯一性约束，好处：
	// 1. 保证type字段的唯一性，避免数据冲突
	// 2. 防止重复导入相同类型的字典
	if !errors.Is(global.GVA_DB.First(&system.SysDictionary{}, "type = ?", importData.Type).Error, gorm.ErrRecordNotFound) {
		return errors.New("存在相同的type，不允许导入")
	}

	// 创建字典（清空导入数据的ID和时间戳）
	// 手动构造新对象，忽略导入数据中的ID和时间戳，好处：
	// 1. 让数据库自动生成新的ID，避免ID冲突
	// 2. 使用当前时间作为创建时间，更符合业务逻辑
	// 3. 只保留业务相关字段，数据更纯净
	dictionary := system.SysDictionary{
		Name:   importData.Name,
		Type:   importData.Type,
		Status: importData.Status,
		Desc:   importData.Desc,
	}

	// 开启数据库事务
	// 使用事务的好处：
	// 1. 保证原子性：要么全部成功，要么全部回滚
	// 2. 如果任何一步失败，整个导入操作都会回滚，保证数据一致性
	// 3. 避免部分导入导致的数据不完整问题
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 创建字典主记录
		if err := tx.Create(&dictionary).Error; err != nil {
			return err
		}

		// 处理字典详情（如果存在）
		if len(importData.SysDictionaryDetails) > 0 {
			// 创建一个映射来跟踪旧ID到新ID的对应关系
			// 这是处理树形结构parent_id关系的关键：
			// 1. 导入的JSON中可能包含旧的ID和parent_id关系
			// 2. 新创建的记录会有新的ID
			// 3. 需要建立旧ID到新ID的映射，才能正确恢复parent_id关系
			idMap := make(map[uint]uint)

			// 第一遍：创建所有详情记录
			// 分两遍处理的好处：
			// 1. 第一遍创建所有记录，获取新的ID
			// 2. 第二遍根据ID映射更新parent_id关系
			// 3. 这样可以正确处理任意深度的树形结构
			for _, detail := range importData.SysDictionaryDetails {
				// 验证必填字段，跳过无效数据
				// 使用continue而不是return的好处：
				// 1. 允许部分数据无效，继续处理其他有效数据
				// 2. 提高导入的容错性
				if detail.Label == "" || detail.Value == "" {
					continue
				}

				// 记录旧ID（用于后续建立parent_id关系）
				oldID := detail.ID

				// 创建新的详情记录（ID会被GORM自动设置）
				// 注意：这里不设置ParentID，因为新ID还未生成
				detailRecord := system.SysDictionaryDetail{
					Label:           detail.Label,
					Value:           detail.Value,
					Extend:          detail.Extend,
					Status:          detail.Status,
					Sort:            detail.Sort,
					Level:           detail.Level,
					Path:            detail.Path,
					SysDictionaryID: int(dictionary.ID),
				}

				// 创建详情记录
				if err := tx.Create(&detailRecord).Error; err != nil {
					return err
				}

				// 记录旧ID到新ID的映射
				// 只有当旧ID有效时才记录，避免无效映射
				if oldID > 0 {
					idMap[oldID] = detailRecord.ID
				}
			}

			// 第二遍：更新parent_id关系
			// 此时所有记录都已创建，新ID都已生成，可以建立parent_id关系
			for _, detail := range importData.SysDictionaryDetails {
				// 只有当存在parent_id且旧ID有效时才处理
				if detail.ParentID != nil && *detail.ParentID > 0 && detail.ID > 0 {
					// 查找当前记录的新ID
					if newID, exists := idMap[detail.ID]; exists {
						// 查找父级记录的新ID
						if newParentID, parentExists := idMap[*detail.ParentID]; parentExists {
							// 更新parent_id为新的ID
							// 这样设计的好处：
							// 1. 正确恢复树形结构的父子关系
							// 2. 即使导入数据的ID不同，也能正确建立关系
							// 3. 保证数据的完整性和一致性
							if err := tx.Model(&system.SysDictionaryDetail{}).
								Where("id = ?", newID).
								Update("parent_id", newParentID).Error; err != nil {
								return err
							}
						}
					}
				}
			}
		}

		return nil
	})
}
