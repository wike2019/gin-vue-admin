// Package system 提供系统参数管理的服务层实现
// 采用服务层模式（Service Layer Pattern），将业务逻辑与数据访问层分离
// 好处：1. 提高代码可维护性 2. 便于单元测试 3. 业务逻辑复用 4. 符合单一职责原则
package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
)

// SysParamsService 系统参数服务结构体
// 使用空结构体作为接收器，因为不需要存储状态信息
// 好处：1. 内存占用最小 2. 所有方法共享同一类型 3. 符合Go语言最佳实践
type SysParamsService struct{}

// CreateSysParams 创建参数记录
// 使用指针类型 *system.SysParams 作为参数，避免值拷贝，提高性能
// 使用 global.GVA_DB 全局数据库连接，统一管理数据库操作
// 好处：1. 代码简洁 2. 统一错误处理 3. 便于后续添加事务支持
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) CreateSysParams(sysParams *system.SysParams) (err error) {
	// GORM的Create方法会自动处理主键生成、时间戳等
	// .Error 获取操作结果，统一错误处理方式
	err = global.GVA_DB.Create(sysParams).Error
	return err
}

// DeleteSysParams 删除参数记录
// 使用占位符 "id = ?" 防止SQL注入攻击
// 使用 &system.SysParams{} 指定表结构，GORM会自动推断表名
// 好处：1. 安全性高 2. 代码可读性强 3. 类型安全
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) DeleteSysParams(ID string) (err error) {
	// Delete方法的第二个参数是WHERE条件，使用参数化查询确保安全
	err = global.GVA_DB.Delete(&system.SysParams{}, "id = ?", ID).Error
	return err
}

// DeleteSysParamsByIds 批量删除参数记录
// 使用 "id in ?" 实现批量删除，比循环删除效率更高
// 使用 []string 作为ID集合，GORM会自动处理IN查询
// 好处：1. 减少数据库交互次数 2. 提高性能 3. 原子性操作
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) DeleteSysParamsByIds(IDs []string) (err error) {
	// 批量删除使用IN子句，一次SQL执行完成所有删除操作
	// &[]system.SysParams{} 指定要删除的记录类型
	err = global.GVA_DB.Delete(&[]system.SysParams{}, "id in ?", IDs).Error
	return err
}

// UpdateSysParams 更新参数记录
// 使用 Model().Where().Updates() 链式调用，构建更新查询
// 使用 Updates 而不是 Update，只更新非零值字段，避免覆盖未提供的字段
// 好处：1. 部分更新支持 2. 防止误更新 3. 代码语义清晰
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) UpdateSysParams(sysParams system.SysParams) (err error) {
	// Model() 指定要操作的表模型
	// Where() 指定更新条件，确保只更新指定ID的记录
	// Updates() 只更新非零值字段，零值字段不会被更新（如空字符串、0等）
	err = global.GVA_DB.Model(&system.SysParams{}).Where("id = ?", sysParams.ID).Updates(&sysParams).Error
	return err
}

// GetSysParams 根据ID获取参数记录
// 使用 First() 方法获取单条记录，如果不存在会返回 gorm.ErrRecordNotFound 错误
// 使用命名返回值，代码更简洁
// 好处：1. 错误处理明确 2. 代码简洁 3. 符合Go语言习惯
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) GetSysParams(ID string) (sysParams system.SysParams, err error) {
	// Where() 构建查询条件，First() 获取第一条匹配的记录
	// 如果记录不存在，err 会是 gorm.ErrRecordNotFound
	err = global.GVA_DB.Where("id = ?", ID).First(&sysParams).Error
	return
}

// GetSysParamsInfoList 分页获取参数记录
// 实现分页查询和条件搜索功能
// 使用链式查询构建器，动态构建SQL查询
// 好处：1. 灵活的条件组合 2. 性能优化（先Count再查询）3. 支持可选分页
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) GetSysParamsInfoList(info systemReq.SysParamsSearch) (list []system.SysParams, total int64, err error) {
	// 计算分页偏移量：offset = (页码 - 1) * 每页数量
	// 例如：第2页，每页10条，offset = (2-1) * 10 = 10
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)

	// 创建独立的查询构建器，避免修改全局DB连接
	// 好处：1. 线程安全 2. 可以复用基础查询 3. 支持链式调用
	db := global.GVA_DB.Model(&system.SysParams{})
	var sysParamss []system.SysParams

	// 动态构建查询条件：只有提供的条件才会添加到查询中
	// 好处：1. 灵活性高 2. 避免不必要的条件判断 3. 代码可维护性强
	// 时间范围查询：使用 BETWEEN 提高查询效率
	if info.StartCreatedAt != nil && info.EndCreatedAt != nil {
		db = db.Where("created_at BETWEEN ? AND ?", info.StartCreatedAt, info.EndCreatedAt)
	}
	// 名称模糊查询：使用 LIKE 和通配符 % 实现模糊匹配
	// 注意：大量数据时建议使用全文索引优化
	if info.Name != "" {
		db = db.Where("name LIKE ?", "%"+info.Name+"%")
	}
	// Key模糊查询：支持按参数键搜索
	if info.Key != "" {
		db = db.Where("key LIKE ?", "%"+info.Key+"%")
	}

	// 先执行Count查询获取总数，用于前端分页显示
	// 好处：1. 分页信息准确 2. 用户体验好 3. 可以提前发现错误
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	// 只有当limit不为0时才应用分页，支持"获取全部"的场景
	// 好处：1. 灵活性高 2. 避免不必要的分页限制
	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	// 执行查询并填充结果到 sysParamss 切片
	// Find() 方法查询所有匹配的记录
	err = db.Find(&sysParamss).Error
	return sysParamss, total, err
}

// GetSysParam 根据key获取参数value
// 使用结构体作为查询条件，GORM会自动将非零值字段转换为WHERE条件
// 这种方式比字符串拼接更安全、更优雅
// 好处：1. 类型安全 2. 防止SQL注入 3. 代码简洁易读
// Author [Mr.奇淼](https://github.com/pixelmaxQm)
func (sysParamsService *SysParamsService) GetSysParam(key string) (param system.SysParams, err error) {
	// 使用结构体作为查询条件，GORM会将 Key: key 转换为 WHERE key = ?
	// 这种方式比字符串拼接更安全，且代码更清晰
	err = global.GVA_DB.Where(system.SysParams{Key: key}).First(&param).Error
	return
}
