package system

import (
	"context"
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
)

// SysErrorService 系统错误日志服务结构体
// 使用空结构体作为接收器，好处：
// 1. 零内存占用，所有实例共享同一内存地址
// 2. 符合Go语言最佳实践，服务层通常不需要状态
// 3. 便于测试和依赖注入
type SysErrorService struct{}

// CreateSysError 创建错误日志记录
// Author [yourname](https://github.com/yourname)
//
// 设计说明：
// 1. 使用 context.Context 作为第一个参数：
//   - 支持请求超时控制和取消机制
//   - 便于链路追踪和日志记录
//   - 符合Go语言标准库设计规范
//
// 2. 使用指针类型 *system.SysError：
//   - 避免大结构体的值拷贝，提高性能
//   - GORM的Create方法会修改结构体（如填充ID、时间戳等），指针可确保修改生效
//
// 3. 直接返回 GORM 的 Error：
//   - 保持错误信息的完整性，便于上层处理
//   - 简化错误处理逻辑，避免不必要的错误包装
func (sysErrorService *SysErrorService) CreateSysError(ctx context.Context, sysError *system.SysError) (err error) {
	err = global.GVA_DB.Create(sysError).Error
	return err
}

// DeleteSysError 删除错误日志记录
// Author [yourname](https://github.com/yourname)
//
// 设计说明：
// 1. 使用 GORM 的 Delete 方法配合 Where 条件：
//   - 使用参数化查询 "id = ?" 防止SQL注入攻击
//   - 比先查询再删除更高效，减少数据库交互次数
//
// 2. 传入模型类型 &system.SysError{}：
//   - GORM 通过模型类型确定操作的表名
//   - 空结构体仅用于类型推断，不占用额外内存
//
// 3. 使用 string 类型的 ID：
//   - 兼容 UUID、字符串ID等多种ID格式
//   - 避免类型转换的复杂性
func (sysErrorService *SysErrorService) DeleteSysError(ctx context.Context, ID string) (err error) {
	err = global.GVA_DB.Delete(&system.SysError{}, "id = ?", ID).Error
	return err
}

// DeleteSysErrorByIds 批量删除错误日志记录
// Author [yourname](https://github.com/yourname)
//
// 设计说明：
// 1. 批量删除的优势：
//   - 减少数据库交互次数，提高性能
//   - 在单个事务中完成，保证原子性
//   - 降低网络开销，特别适合大量数据删除场景
//
// 2. 使用 "id in ?" 语法：
//   - GORM 会自动将切片转换为 SQL 的 IN 子句
//   - 比循环调用 DeleteSysError 效率高得多
//
// 3. 使用切片类型 []system.SysError：
//   - 告诉 GORM 这是批量操作
//   - 虽然传入空切片，但类型信息足够让 GORM 识别操作的表
func (sysErrorService *SysErrorService) DeleteSysErrorByIds(ctx context.Context, IDs []string) (err error) {
	err = global.GVA_DB.Delete(&[]system.SysError{}, "id in ?", IDs).Error
	return err
}

// UpdateSysError 更新错误日志记录
// Author [yourname](https://github.com/yourname)
//
// 设计说明：
// 1. 使用值类型 system.SysError 作为参数：
//   - 避免意外修改调用方的数据
//   - 明确表示这是输入参数，不会被修改
//
// 2. 使用 Updates 而非 Update：
//   - Updates 只更新非零值字段，更安全
//   - 避免误将零值更新到数据库
//   - 支持部分字段更新，灵活性更高
//
// 3. 先 Model 后 Where 的链式调用：
//   - 明确指定要操作的表和条件
//   - 代码可读性强，符合 GORM 的链式调用风格
//   - 便于后续扩展其他查询条件
func (sysErrorService *SysErrorService) UpdateSysError(ctx context.Context, sysError system.SysError) (err error) {
	err = global.GVA_DB.Model(&system.SysError{}).Where("id = ?", sysError.ID).Updates(&sysError).Error
	return err
}

// GetSysError 根据ID获取错误日志记录
// Author [yourname](https://github.com/yourname)
//
// 设计说明：
// 1. 使用命名返回值 (sysError system.SysError, err error)：
//   - 代码更简洁，可以直接 return 而不需要显式返回变量
//   - 返回值语义清晰，便于理解函数签名
//
// 2. 使用 First 方法：
//   - 如果记录不存在会返回 gorm.ErrRecordNotFound 错误
//   - 比 Find 更明确表达"获取单条记录"的意图
//   - 自动限制查询结果为1条，提高查询效率
//
// 3. 使用值类型返回 sysError：
//   - 调用方获得数据的副本，避免意外修改
//   - 如果记录不存在，返回零值结构体，配合错误信息使用
func (sysErrorService *SysErrorService) GetSysError(ctx context.Context, ID string) (sysError system.SysError, err error) {
	err = global.GVA_DB.Where("id = ?", ID).First(&sysError).Error
	return
}

// GetSysErrorInfoList 分页获取错误日志记录
// Author [yourname](https://github.com/yourname)
//
// 设计说明：
// 1. 链式构建查询条件的设计模式：
//   - 先创建基础查询 db := global.GVA_DB.Model(...).Order(...)
//   - 然后根据条件动态添加 Where 子句
//   - 好处：代码清晰、易于维护、支持条件组合
//   - 避免写复杂的 SQL 拼接逻辑
//
// 2. 先 Count 后查询数据：
//   - 先统计总数，便于前端分页组件计算总页数
//   - 如果 Count 失败，提前返回，避免无效的数据查询
//   - 注意：Count 和 Find 使用同一个 db 实例，保证条件一致
//
// 3. 条件判断的设计：
//   - 使用指针类型 *string 判断是否为 nil，区分"未设置"和"空字符串"
//   - 先判断 nil 再判断空字符串，避免空指针解引用
//   - 使用 LIKE 进行模糊查询，提高搜索灵活性
//
// 4. limit != 0 的判断：
//   - 允许 limit=0 表示查询全部数据（不分页）
//   - 提供灵活性，支持"导出全部"等场景
//
// 5. 时间范围查询使用 BETWEEN：
//   - 比两个独立的 >= 和 <= 条件更高效
//   - 数据库可以更好地优化索引使用
//
// 6. 默认按创建时间倒序：
//   - 最新的错误日志在前，符合用户查看习惯
//   - 使用 created_at desc 而非 id desc，更符合业务语义
func (sysErrorService *SysErrorService) GetSysErrorInfoList(ctx context.Context, info systemReq.SysErrorSearch) (list []system.SysError, total int64, err error) {
	// 计算分页参数：limit 限制每页数量，offset 计算跳过的记录数
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)

	// 创建基础查询：指定模型和默认排序
	// 使用链式调用，后续可以继续添加条件
	db := global.GVA_DB.Model(&system.SysError{}).Order("created_at desc")
	var sysErrors []system.SysError

	// 动态添加查询条件：根据传入的搜索参数构建 WHERE 子句
	// 这种设计的好处是：条件可选，代码清晰，易于扩展

	// 时间范围查询：如果提供了开始和结束时间，添加 BETWEEN 条件
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	// 错误来源精确匹配：使用指针判断是否设置了该条件
	// info.Form != nil 表示前端传入了该参数
	// *info.Form != "" 表示参数值不为空
	if info.Form != nil && *info.Form != "" {
		db = db.Where("form = ?", *info.Form)
	}

	// 错误内容模糊搜索：使用 LIKE 进行部分匹配
	// % 通配符表示任意字符，支持搜索错误信息中的关键词
	if info.Info != nil && *info.Info != "" {
		db = db.Where("info LIKE ?", "%"+*info.Info+"%")
	}

	// 先统计符合条件的总记录数
	// 必须在添加 Limit/Offset 之前执行，否则统计的是分页后的数量
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	// 如果 limit 不为 0，才添加分页限制
	// limit=0 表示查询全部数据，常用于导出功能
	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	// 执行查询，将结果填充到 sysErrors 切片
	err = db.Find(&sysErrors).Error
	return sysErrors, total, err
}

// GetSysErrorSolution 异步处理错误，使用AI生成解决方案
// Author [yourname](https://github.com/yourname)
//
// 设计说明：
// 1. 异步处理的设计理念：
//   - LLM 调用可能耗时较长（几秒到几十秒），不适合同步等待
//   - 使用 goroutine 异步处理，立即返回响应，提升用户体验
//   - 前端可以通过轮询或 WebSocket 获取处理结果
//
// 2. 状态机设计：
//   - "未处理" -> "处理中" -> "处理完成"/"处理失败"
//   - 立即更新为"处理中"，让用户知道任务已开始
//   - 最终更新为"处理完成"或"处理失败"，明确任务结果
//
// 3. 错误处理策略：
//   - 使用 _ 忽略部分错误，避免 goroutine 中的错误无法传递
//   - 即使 LLM 调用失败，也更新状态为"处理失败"，避免任务永远处于"处理中"
//   - 这种"尽力而为"的策略确保系统不会因为外部服务故障而卡住
//
// 4. 参数传递：
//   - goroutine 中使用 id 参数而非闭包捕获 ID，避免并发安全问题
//   - 如果直接使用 ID，多个 goroutine 可能共享同一个变量
//
// 5. Context 使用：
//   - 主流程使用 ctx，支持请求取消和超时
//   - goroutine 中使用 context.Background()，因为主请求已返回，需要独立上下文
//
// 6. 数据库操作：
//   - 使用 WithContext 传递上下文，支持链路追踪和超时控制
//   - 使用 Updates 更新多个字段，使用 Update 更新单个字段
func (sysErrorService *SysErrorService) GetSysErrorSolution(ctx context.Context, ID string) (err error) {
	// 立即更新状态为"处理中"
	// 使用 WithContext 传递上下文，支持请求超时和取消
	// 好处：用户立即看到状态变化，知道任务已开始处理
	err = global.GVA_DB.WithContext(ctx).Model(&system.SysError{}).Where("id = ?", ID).Update("status", "处理中").Error
	if err != nil {
		return err
	}

	// 启动异步协程处理 AI 生成方案
	// 使用独立的 goroutine，不阻塞主请求响应
	// 传入 id 参数而非闭包捕获，避免并发安全问题
	go func(id string) {
		// 查询当前错误信息，用于生成解决方案
		// 注意：这里重新查询是为了获取最新的错误信息
		// 如果直接使用传入的参数，可能不是最新的数据
		var se system.SysError
		_ = global.GVA_DB.Model(&system.SysError{}).Where("id = ?", id).First(&se).Error

		// 安全地提取字符串值：处理指针类型可能为 nil 的情况
		// 使用指针类型的好处：区分"未设置"和"空字符串"
		var form, info string
		if se.Form != nil {
			form = *se.Form
		}
		if se.Info != nil {
			info = *se.Info
		}

		// 构造 LLM 请求参数
		// mode: "solution" 表示使用解决方案生成模式
		// command: "solution" 具体的命令类型
		// info: 错误内容，用于 AI 分析
		// form: 错误来源，提供上下文信息
		llmReq := common.JSONMap{
			"mode":    "solution",
			"command": "solution",
			"info":    info,
			"form":    form,
		}

		// 调用 LLM 服务生成解决方案
		// 使用 context.Background() 因为主请求已返回，需要独立上下文
		var solution string
		if data, err := (&AutoCodeService{}).LLMAuto(context.Background(), llmReq); err == nil {
			// 生成成功：将结果转换为字符串并更新数据库
			solution = fmt.Sprintf("%v", data)
			// 使用 Updates 同时更新状态和解决方案
			_ = global.GVA_DB.Model(&system.SysError{}).Where("id = ?", id).Updates(map[string]interface{}{"status": "处理完成", "solution": solution}).Error
		} else {
			// 生成失败：标记为"处理失败"
			// 即使失败也更新状态，避免任务永远处于"处理中"状态
			// 这种设计确保系统的健壮性，不会因为外部服务故障而卡住
			_ = global.GVA_DB.Model(&system.SysError{}).Where("id = ?", id).Update("status", "处理失败").Error
		}
	}(ID)

	// 立即返回 nil，不等待异步处理完成
	// 前端可以通过查询接口获取最新的状态和解决方案
	return nil
}
