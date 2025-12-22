// Package system 提供系统相关的服务层实现
// 服务层（Service Layer）是业务逻辑的核心，负责处理具体的业务操作
// 将业务逻辑从控制器（Controller）中分离出来，使代码结构更清晰，便于维护和测试
package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
)

// OperationRecordService 操作记录服务结构体
// 使用空结构体的原因：
// 1. 节省内存：空结构体不占用任何内存空间（0字节）
// 2. 语义清晰：表明这是一个服务类，不需要存储状态，所有方法都是无状态的
// 3. 符合Go语言最佳实践：当只需要方法集合而不需要字段时，使用空结构体
// 好处：轻量级、高效，适合作为纯函数式服务的容器
type OperationRecordService struct{}

// OperationRecordServiceApp 服务单例实例
// 使用单例模式的原因：
// 1. 全局唯一：确保整个应用中只有一个服务实例，避免重复创建
// 2. 便于访问：其他包可以直接通过此变量访问服务，无需传递依赖
// 3. 资源节约：避免多次实例化带来的内存开销
// 好处：简化依赖注入，提高代码可维护性，符合Go语言的包级别单例模式
var OperationRecordServiceApp = new(OperationRecordService)

// DeleteSysOperationRecordByIds 批量删除操作记录
// @author: [granty1](https://github.com/granty1)
// @author: [piexlmax](https://github.com/piexlmax)
// @function: DeleteSysOperationRecordByIds
// @description: 批量删除记录
// @param: ids request.IdsReq 包含要删除的ID数组
// @return: err error 删除操作的错误信息
//
// 设计说明：
// 1. 使用 IN 查询的原因：
//   - 性能优势：一次SQL查询可以删除多条记录，比循环删除效率高得多
//   - 原子性：单次事务操作，要么全部成功要么全部失败，保证数据一致性
//   - 减少数据库连接开销：避免多次数据库往返
//
// 2. 使用 &[]system.SysOperationRecord{} 的原因：
//   - GORM要求传入模型类型来确定操作的表
//   - 使用切片类型告诉GORM这是批量操作
//   - 空切片仅用于类型推断，不会影响删除逻辑
//
// 3. 使用参数化查询 "id in (?)" 的原因：
//   - 防止SQL注入：GORM会自动处理参数，确保安全性
//   - 类型安全：编译时检查，避免运行时错误
//
// 好处：
// - 高效：单次SQL执行，性能优于循环删除
// - 安全：参数化查询防止SQL注入攻击
// - 简洁：代码量少，逻辑清晰
func (operationRecordService *OperationRecordService) DeleteSysOperationRecordByIds(ids request.IdsReq) (err error) {
	// 使用 IN 查询批量删除，ids.Ids 是 []uint 类型
	// GORM 会将 ids.Ids 展开为 IN (id1, id2, id3, ...) 的形式
	err = global.GVA_DB.Delete(&[]system.SysOperationRecord{}, "id in (?)", ids.Ids).Error
	return err
}

// DeleteSysOperationRecord 删除单条操作记录
// @author: [granty1](https://github.com/granty1)
// @function: DeleteSysOperationRecord
// @description: 删除操作记录
// @param: sysOperationRecord model.SysOperationRecord 包含要删除记录的信息（通常只需要ID字段）
// @return: err error 删除操作的错误信息
//
// 设计说明：
// 1. 为什么传入完整结构体而不是只传ID：
//   - 灵活性：可以根据主键或其他唯一字段删除（GORM支持）
//   - 一致性：与批量删除方法形成对比，提供两种删除方式
//   - 扩展性：未来如果需要根据多个条件删除，结构体方式更容易扩展
//
// 2. GORM Delete 方法的工作原理：
//   - 如果结构体有主键值，GORM会根据主键删除
//   - 如果结构体有多个字段值，GORM会将这些字段作为WHERE条件
//   - 使用 &sysOperationRecord 传递指针，GORM可以修改结构体（如软删除时）
//
// 3. 与批量删除的区别：
//   - 批量删除：适合删除多条记录，使用 IN 查询
//   - 单个删除：适合删除单条记录，使用主键或条件查询
//
// 好处：
// - 灵活：支持多种删除条件
// - 类型安全：使用结构体而非原始SQL
// - 可维护：代码清晰，易于理解
func (operationRecordService *OperationRecordService) DeleteSysOperationRecord(sysOperationRecord system.SysOperationRecord) (err error) {
	// GORM 会根据结构体的主键字段（通常是ID）执行删除操作
	// 如果 sysOperationRecord.ID 有值，则删除对应ID的记录
	err = global.GVA_DB.Delete(&sysOperationRecord).Error
	return err
}

// GetSysOperationRecord 根据ID获取单条操作记录
// @author: [granty1](https://github.com/granty1)
// @function: GetSysOperationRecord
// @description: 根据id获取单条操作记录
// @param: id uint 操作记录的ID
// @return: sysOperationRecord system.SysOperationRecord 查询到的操作记录
// @return: err error 查询操作的错误信息（如果记录不存在也会返回错误）
//
// 设计说明：
// 1. 使用 Where + First 的原因：
//   - First 方法：查询第一条匹配的记录，如果没找到返回 gorm.ErrRecordNotFound
//   - 明确性：比 Find 更明确表达"只查询一条"的意图
//   - 错误处理：可以通过错误判断记录是否存在，便于业务层处理
//
// 2. 使用参数化查询 "id = ?" 的原因：
//   - 安全性：防止SQL注入攻击
//   - 性能：数据库可以缓存执行计划
//   - 可读性：代码清晰，易于维护
//
// 3. 使用命名返回值的原因：
//   - 简洁：return 语句不需要显式返回变量名
//   - 约定：Go语言中，命名返回值常用于错误处理场景
//
// 好处：
// - 安全：参数化查询防止SQL注入
// - 明确：First方法明确表达查询单条记录的意图
// - 易用：通过错误可以判断记录是否存在
func (operationRecordService *OperationRecordService) GetSysOperationRecord(id uint) (sysOperationRecord system.SysOperationRecord, err error) {
	// Where 构建查询条件，First 查询第一条匹配的记录
	// 如果记录不存在，First 会返回 gorm.ErrRecordNotFound 错误
	err = global.GVA_DB.Where("id = ?", id).First(&sysOperationRecord).Error
	return
}

// GetSysOperationRecordInfoList 分页获取操作记录列表（支持多条件搜索）
// @author: [granty1](https://github.com/granty1)
// @author: [piexlmax](https://github.com/piexlmax)
// @function: GetSysOperationRecordInfoList
// @description: 分页获取操作记录列表
// @param: info systemReq.SysOperationRecordSearch 包含分页信息和搜索条件
// @return: list interface{} 操作记录列表
// @return: total int64 符合条件的总记录数（用于前端分页计算）
// @return: err error 查询操作的错误信息
//
// 设计说明：
// 1. 分页计算逻辑：
//   - limit = PageSize：每页显示的记录数
//   - offset = PageSize * (Page - 1)：跳过的记录数
//   - 例如：第2页，每页10条，offset = 10 * (2-1) = 10，跳过前10条
//     好处：标准的分页算法，适用于所有数据库
//
// 2. 链式查询构建（db := global.GVA_DB.Model(...)）：
//   - Model 方法：指定要查询的表模型
//   - 链式调用：每个 Where 条件都返回新的查询对象，可以继续链式调用
//   - 条件叠加：多个 Where 条件会自动组合成 AND 关系
//     好处：
//   - 灵活性：可以根据不同条件动态构建查询
//   - 可读性：代码清晰，条件一目了然
//   - 可维护性：添加新条件只需增加一个 if 语句
//
// 3. 条件判断的设计：
//   - Method != ""：空字符串不添加条件，避免无效查询
//   - Path LIKE：使用模糊查询，支持部分匹配
//   - Status != 0：0通常表示未设置，非0才添加条件
//     好处：避免添加无意义的查询条件，提高性能
//
// 4. 为什么先 Count 再查询数据：
//   - 分页需要总数：前端需要知道总记录数来计算总页数
//   - 提前错误检查：如果 Count 失败，可以提前返回，避免无效的数据查询
//   - 性能考虑：Count 查询通常比数据查询快，先执行可以快速发现错误
//     注意：这里 Count 和 Find 使用的是同一个 db 对象，确保查询条件一致
//
// 5. Order("id desc") 的原因：
//   - 按ID倒序：最新的记录在前，符合操作日志的查看习惯
//   - 使用主键排序：ID是主键，有索引，排序性能好
//
// 6. Preload("User") 的作用：
//   - 预加载关联：SysOperationRecord 有 User 关联字段
//   - 避免N+1问题：不使用 Preload 会导致每条记录都查询一次 User
//   - 性能优化：一次查询获取所有关联数据，而不是循环查询
//     好处：
//   - 性能：减少数据库查询次数，从 N+1 次减少到 2 次（Count + Find with Preload）
//   - 简洁：代码中自动加载关联数据，无需手动处理
//
// 7. 返回 interface{} 的原因：
//   - 灵活性：可以返回不同类型的列表
//   - 兼容性：便于未来扩展，返回不同类型的数据结构
//     注意：虽然返回 interface{}，但实际类型是 []system.SysOperationRecord
//
// 整体设计优势：
// - 高效：使用索引字段排序，预加载关联数据，减少查询次数
// - 灵活：支持多条件组合搜索，易于扩展
// - 安全：所有查询都使用参数化，防止SQL注入
// - 可维护：代码结构清晰，条件判断明确
func (operationRecordService *OperationRecordService) GetSysOperationRecordInfoList(info systemReq.SysOperationRecordSearch) (list interface{}, total int64, err error) {
	// 计算分页参数
	// limit: 每页显示的记录数
	limit := info.PageSize
	// offset: 跳过的记录数，用于实现分页
	// 例如：第1页 offset=0，第2页 offset=PageSize，第3页 offset=2*PageSize
	offset := info.PageSize * (info.Page - 1)

	// 创建查询对象，指定要查询的模型
	// Model 方法返回一个查询构建器，可以链式调用添加条件
	db := global.GVA_DB.Model(&system.SysOperationRecord{})
	var sysOperationRecords []system.SysOperationRecord

	// 动态构建查询条件
	// 这种设计的好处是：只有传入的条件才会添加到查询中，避免无效查询
	// 如果条件为空，则不添加该条件，实现灵活的多条件搜索

	// 按请求方法筛选（精确匹配）
	// 只有当 Method 不为空时才添加条件，避免空字符串的无效查询
	if info.Method != "" {
		db = db.Where("method = ?", info.Method)
	}

	// 按请求路径筛选（模糊匹配）
	// 使用 LIKE 查询，支持部分匹配，例如搜索 "/api" 可以匹配 "/api/user" 和 "/api/admin"
	// "%"+info.Path+"%" 表示路径中包含 info.Path 的所有记录
	if info.Path != "" {
		db = db.Where("path LIKE ?", "%"+info.Path+"%")
	}

	// 按状态筛选（精确匹配）
	// Status != 0 的原因：0 通常表示未设置或全部，非0才表示具体的状态值
	// 这样可以区分"查询所有状态"和"查询状态为0的记录"
	if info.Status != 0 {
		db = db.Where("status = ?", info.Status)
	}

	// 先执行 Count 查询获取总记录数
	// 这样做的好处：
	// 1. 前端需要总数来计算总页数
	// 2. 如果查询条件有问题，可以提前发现并返回错误
	// 3. Count 查询通常比数据查询快，先执行可以快速验证查询条件
	err = db.Count(&total).Error
	if err != nil {
		return // 如果 Count 失败，直接返回，不执行后续的数据查询
	}

	// 执行数据查询
	// Order("id desc"): 按ID倒序排列，最新的记录在前
	// Limit(limit): 限制返回的记录数
	// Offset(offset): 跳过前面的记录，实现分页
	// Preload("User"): 预加载关联的 User 数据，避免 N+1 查询问题
	// Find: 查询多条记录
	err = db.Order("id desc").Limit(limit).Offset(offset).Preload("User").Find(&sysOperationRecords).Error
	return sysOperationRecords, total, err
}
