package service

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/model"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/model/request"
)

// Info 是公告服务的全局实例
// 设计模式：单例模式（Singleton Pattern）
// 好处：确保整个插件只有一个服务实例，便于统一管理
var Info = new(info)

// info 是公告相关的业务服务
// 设计模式：服务层模式（Service Layer Pattern）
// 好处：
// 1. 将业务逻辑从 API 层和数据库层分离，实现关注点分离
// 2. 业务逻辑集中管理，便于维护和测试
// 3. 可以轻松替换底层实现（如从 GORM 切换到其他 ORM）
// 4. 便于实现业务规则验证、事务管理等复杂逻辑
// 5. 通过空结构体实现，节省内存
type info struct{}

// CreateInfo 创建公告记录
// 设计模式：数据访问对象模式（DAO Pattern） + GORM ORM 模式
// 好处：
// 1. 使用 GORM 的 Create 方法，自动处理字段映射和 SQL 生成
// 2. 自动填充创建时间、更新时间等字段（通过 GVA_MODEL）
// 3. 使用指针传递，GORM 会自动填充 ID 等生成字段
// 4. 统一的错误处理，便于上层调用者处理错误
// Author [piexlmax](https://github.com/piexlmax)
func (s *info) CreateInfo(info *model.Info) (err error) {
	// 使用 GORM 的 Create 方法插入记录
	// 好处：
	// 1. 自动生成 SQL，避免手动拼接 SQL 语句
	// 2. 自动处理字段映射，支持结构体标签（如 gorm:"column:title"）
	// 3. 自动填充创建时间、更新时间等字段
	// 4. 返回错误信息，便于上层处理异常情况
	err = global.GVA_DB.Create(info).Error
	return err
}

// DeleteInfo 删除公告记录
// 设计模式：软删除模式（Soft Delete Pattern） + 参数化查询模式
// 好处：
// 1. 使用参数化查询（id = ?），防止 SQL 注入攻击
// 2. GORM 支持软删除，如果模型定义了 DeletedAt 字段，会执行软删除
// 3. 统一的错误处理，便于上层调用者处理错误
// 4. 如果使用软删除，数据不会真正删除，便于数据恢复和审计
// Author [piexlmax](https://github.com/piexlmax)
func (s *info) DeleteInfo(ID string) (err error) {
	// 使用 GORM 的 Delete 方法删除记录
	// 好处：
	// 1. 使用参数化查询，防止 SQL 注入攻击
	// 2. 如果模型定义了 DeletedAt 字段，会执行软删除（逻辑删除）
	// 3. 软删除的好处：数据不会真正删除，便于数据恢复和审计
	// 4. 自动处理删除时间字段（如果使用软删除）
	err = global.GVA_DB.Delete(&model.Info{}, "id = ?", ID).Error
	return err
}

// DeleteInfoByIds 批量删除公告记录
// 设计模式：批量操作模式（Batch Operation Pattern） + IN 查询模式
// 好处：
// 1. 使用 IN 查询一次删除多条记录，比多次单独删除效率更高
// 2. 减少数据库交互次数，提高性能
// 3. 在同一个事务中执行，保证数据一致性
// 4. 使用参数化查询，防止 SQL 注入攻击
// Author [piexlmax](https://github.com/piexlmax)
func (s *info) DeleteInfoByIds(IDs []string) (err error) {
	// 使用 GORM 的 Delete 方法批量删除记录
	// 好处：
	// 1. 使用 IN 查询（id in ?），一次删除多条记录
	// 2. 比多次单独删除效率更高，减少数据库交互次数
	// 3. 使用参数化查询，防止 SQL 注入攻击
	// 4. 如果使用软删除，所有记录会同时标记为删除
	err = global.GVA_DB.Delete(&[]model.Info{}, "id in ?", IDs).Error
	return err
}

// UpdateInfo 更新公告记录
// 设计模式：选择性更新模式（Selective Update Pattern） + 参数化查询模式
// 好处：
// 1. 使用 Updates 方法，只更新非零值字段，避免覆盖未修改的字段
// 2. 自动处理更新时间字段（通过 GVA_MODEL）
// 3. 使用参数化查询，防止 SQL 注入攻击
// 4. 先通过 Where 定位记录，再更新，保证更新准确性
// Author [piexlmax](https://github.com/piexlmax)
func (s *info) UpdateInfo(info model.Info) (err error) {
	// 使用 GORM 的 Updates 方法更新记录
	// 好处：
	// 1. Updates 方法只更新非零值字段，避免覆盖未修改的字段
	// 2. 自动处理更新时间字段（如果模型定义了 UpdatedAt）
	// 3. 先通过 Where 定位记录，再更新，保证更新准确性
	// 4. 使用参数化查询，防止 SQL 注入攻击
	// 注意：如果字段值为零值（如 0、""、false），该字段不会被更新
	err = global.GVA_DB.Model(&model.Info{}).Where("id = ?", info.ID).Updates(&info).Error
	return err
}

// GetInfo 根据ID获取公告记录
// 设计模式：单记录查询模式（Single Record Query Pattern） + 参数化查询模式
// 好处：
// 1. 使用 First 方法，如果记录不存在会返回错误，便于判断记录是否存在
// 2. 使用参数化查询，防止 SQL 注入攻击
// 3. 自动处理字段映射，支持结构体标签
// 4. 统一的错误处理，便于上层调用者处理错误（如记录不存在）
// Author [piexlmax](https://github.com/piexlmax)
func (s *info) GetInfo(ID string) (info model.Info, err error) {
	// 使用 GORM 的 First 方法查询单条记录
	// 好处：
	// 1. First 方法如果记录不存在会返回 gorm.ErrRecordNotFound 错误
	// 2. 便于上层调用者判断记录是否存在
	// 3. 使用参数化查询，防止 SQL 注入攻击
	// 4. 自动处理字段映射，支持结构体标签（如 gorm:"column:title"）
	err = global.GVA_DB.Where("id = ?", ID).First(&info).Error
	return
}

// GetInfoInfoList 分页获取公告记录
// 设计模式：查询构建器模式（Query Builder Pattern） + 分页模式
// 好处：
// 1. 使用 GORM 的链式调用构建查询，代码清晰易读
// 2. 支持动态查询条件，根据参数自动添加 WHERE 子句
// 3. 先查询总数，再查询分页数据，保证分页信息的准确性
// 4. 支持条件搜索（如时间范围），提高查询灵活性
// 5. 使用 limit 和 offset 实现分页，减少数据库查询压力
// Author [piexlmax](https://github.com/piexlmax)
func (s *info) GetInfoInfoList(info request.InfoSearch) (list []model.Info, total int64, err error) {
	// 计算分页参数
	// limit：每页记录数
	// offset：偏移量，计算公式：每页记录数 * (页码 - 1)
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	
	// 创建数据库查询对象
	// 好处：使用 GORM 的 Model 方法，自动处理表名和字段映射
	db := global.GVA_DB.Model(&model.Info{})
	var infos []model.Info
	
	// 根据查询条件动态添加 WHERE 子句
	// 设计模式：条件构建模式（Conditional Builder Pattern）
	// 好处：
	// 1. 如果提供了时间范围，自动添加时间过滤条件
	// 2. 如果没有提供条件，则查询所有记录
	// 3. 支持扩展更多查询条件（如关键词搜索、状态过滤等）
	if info.StartCreatedAt != nil && info.EndCreatedAt != nil {
		// 使用 BETWEEN 查询时间范围内的记录
		// 好处：SQL 查询效率高，索引友好
		db = db.Where("created_at BETWEEN ? AND ?", info.StartCreatedAt, info.EndCreatedAt)
	}
	
	// 先查询总记录数
	// 好处：在分页查询前获取总数，保证分页信息的准确性
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	// 如果 limit 不为 0，则应用分页限制
	// 好处：支持查询所有记录（limit=0）和分页查询两种模式
	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}
	
	// 执行查询，获取分页数据
	err = db.Find(&infos).Error
	return infos, total, err
}
// GetInfoDataSource 获取公告的数据源
// 设计模式：数据源模式（DataSource Pattern） + 原生 SQL 查询模式
// 好处：
// 1. 为前端提供下拉框、选择器等组件所需的数据源
// 2. 统一管理数据源，避免前端硬编码
// 3. 支持动态数据源，数据变化时前端自动更新
// 4. 使用原生 SQL 查询，可以灵活选择字段和别名
// 5. 返回格式化的数据（label/value），便于前端直接使用
func (s *info) GetInfoDataSource() (res map[string][]map[string]any, err error) {
	// 初始化返回结果，使用 map 存储不同类型的数据源
	// 好处：可以同时返回多种数据源（如用户列表、状态选项等）
	res = make(map[string][]map[string]any)

	// 查询用户列表作为数据源
	// 设计模式：原生 SQL 查询模式（Raw SQL Query Pattern）
	// 好处：
	// 1. 使用 Select 方法选择特定字段，减少数据传输量
	// 2. 使用别名（as label, as value），格式化数据便于前端使用
	// 3. 使用 Scan 方法将查询结果映射到 map 切片
	// 4. 前端可以直接使用 label 作为显示文本，value 作为实际值
	userID := make([]map[string]any, 0)
	global.GVA_DB.Table("sys_users").Select("nick_name as label,id as value").Scan(&userID)
	res["userID"] = userID
	
	// 可以继续添加其他数据源，如状态选项、分类列表等
	// 示例：
	// statusOptions := []map[string]any{
	//     {"label": "启用", "value": 1},
	//     {"label": "禁用", "value": 0},
	// }
	// res["status"] = statusOptions
	
	return
}
