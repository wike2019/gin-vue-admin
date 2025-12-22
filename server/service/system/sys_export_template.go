// Package system 提供导出模板服务，支持动态配置的Excel导入导出功能
// 该服务允许用户通过配置模板来定义导出规则，支持多表关联、条件过滤、软删除过滤等功能
package system

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// SysExportTemplateService 导出模板服务结构体
// 采用空结构体设计，所有方法都是值接收者，避免不必要的内存分配
// 好处：轻量级设计，符合Go语言最佳实践，所有方法共享同一个服务实例
type SysExportTemplateService struct {
}

// SysExportTemplateServiceApp 全局服务实例
// 使用单例模式，确保整个应用只有一个服务实例
// 好处：统一管理，避免重复创建，节省内存
var SysExportTemplateServiceApp = new(SysExportTemplateService)

// CreateSysExportTemplate 创建导出模板记录
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 使用指针接收者 *system.SysExportTemplate，允许GORM自动填充ID、CreatedAt等字段
// - 直接使用 GORM 的 Create 方法，简洁高效
// - 好处：代码简洁，GORM会自动处理关联数据的创建（如果配置了关联关系）
func (sysExportTemplateService *SysExportTemplateService) CreateSysExportTemplate(sysExportTemplate *system.SysExportTemplate) (err error) {
	err = global.GVA_DB.Create(sysExportTemplate).Error
	return err
}

// DeleteSysExportTemplate 删除导出模板记录
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 使用值接收者，因为只需要ID字段即可删除
// - GORM的Delete方法支持软删除（如果模型实现了DeletedAt字段）
// - 好处：自动处理软删除逻辑，数据不会真正丢失，可以恢复
func (sysExportTemplateService *SysExportTemplateService) DeleteSysExportTemplate(sysExportTemplate system.SysExportTemplate) (err error) {
	err = global.GVA_DB.Delete(&sysExportTemplate).Error
	return err
}

// DeleteSysExportTemplateByIds 批量删除导出模板记录
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 使用 IN 查询批量删除，比循环删除效率高
// - 使用切片类型 &[]system.SysExportTemplate{} 作为模型，GORM会自动识别
// - 好处：一次SQL执行完成批量删除，减少数据库交互次数，提高性能
func (sysExportTemplateService *SysExportTemplateService) DeleteSysExportTemplateByIds(ids request.IdsReq) (err error) {
	err = global.GVA_DB.Delete(&[]system.SysExportTemplate{}, "id in ?", ids.Ids).Error
	return err
}

// UpdateSysExportTemplate 更新导出模板记录
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
//   - 使用数据库事务确保数据一致性：要么全部成功，要么全部回滚
//   - 采用"先删除后创建"策略处理关联数据（Conditions和JoinTemplate）
//     原因：GORM的Updates方法不会自动处理关联数据的更新，需要手动管理
//     好处：避免新旧数据混合，保证关联数据的完整性
//   - 在更新主表前，先将关联数据置为nil，避免GORM尝试更新关联数据
//   - 重新创建关联数据前，将ID置为0，确保创建新记录而不是更新旧记录
//   - 好处：事务保证原子性，任何步骤失败都会回滚，避免数据不一致
func (sysExportTemplateService *SysExportTemplateService) UpdateSysExportTemplate(sysExportTemplate system.SysExportTemplate) (err error) {
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 保存关联数据引用，因为后续会清空
		conditions := sysExportTemplate.Conditions
		// 先删除旧的关联条件，避免数据残留
		e := tx.Delete(&[]system.Condition{}, "template_id = ?", sysExportTemplate.TemplateID).Error
		if e != nil {
			return e
		}
		// 清空关联数据，避免GORM在Updates时尝试更新关联数据
		sysExportTemplate.Conditions = nil

		// 保存关联数据引用
		joins := sysExportTemplate.JoinTemplate
		// 先删除旧的关联表配置
		e = tx.Delete(&[]system.JoinTemplate{}, "template_id = ?", sysExportTemplate.TemplateID).Error
		if e != nil {
			return e
		}
		// 清空关联数据
		sysExportTemplate.JoinTemplate = nil

		// 更新主表数据
		e = tx.Updates(&sysExportTemplate).Error
		if e != nil {
			return e
		}
		// 重新创建条件数据
		if len(conditions) > 0 {
			// 将ID置为0，确保创建新记录
			for i := range conditions {
				conditions[i].ID = 0
			}
			e = tx.Create(&conditions).Error
		}
		// 重新创建关联表配置
		if len(joins) > 0 {
			// 将ID置为0，确保创建新记录
			for i := range joins {
				joins[i].ID = 0
			}
			e = tx.Create(&joins).Error
		}
		return e
	})
}

// GetSysExportTemplate 根据id获取导出模板记录
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 使用Preload预加载关联数据（JoinTemplate和Conditions），避免N+1查询问题
// - Preload会在一次查询中加载所有关联数据，而不是为每条记录单独查询
// - 好处：减少数据库查询次数，提高性能，特别是在关联数据较多时
func (sysExportTemplateService *SysExportTemplateService) GetSysExportTemplate(id uint) (sysExportTemplate system.SysExportTemplate, err error) {
	err = global.GVA_DB.Where("id = ?", id).Preload("JoinTemplate").Preload("Conditions").First(&sysExportTemplate).Error
	return
}

// GetSysExportTemplateInfoList 分页获取导出模板记录
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 使用链式查询构建器，根据条件动态添加WHERE子句
// - 先执行Count获取总数，再执行分页查询，确保分页信息准确
// - 使用Model方法创建查询，保持查询链的灵活性
// - 好处：代码清晰，易于维护，支持动态条件组合
func (sysExportTemplateService *SysExportTemplateService) GetSysExportTemplateInfoList(info systemReq.SysExportTemplateSearch) (list []system.SysExportTemplate, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建查询构建器，使用Model方法保持链式调用的灵活性
	db := global.GVA_DB.Model(&system.SysExportTemplate{})
	var sysExportTemplates []system.SysExportTemplate
	// 动态构建查询条件：根据传入的搜索参数，有选择地添加WHERE子句
	// 好处：避免硬编码所有可能的条件组合，代码更简洁
	if info.StartCreatedAt != nil && info.EndCreatedAt != nil {
		db = db.Where("created_at BETWEEN ? AND ?", info.StartCreatedAt, info.EndCreatedAt)
	}
	if info.Name != "" {
		// 使用LIKE进行模糊查询，支持部分匹配
		db = db.Where("name LIKE ?", "%"+info.Name+"%")
	}
	if info.TableName != "" {
		// 精确匹配表名
		db = db.Where("table_name = ?", info.TableName)
	}
	if info.TemplateID != "" {
		// 精确匹配模板ID
		db = db.Where("template_id = ?", info.TemplateID)
	}
	// 先统计总数，用于分页计算
	err = db.Count(&total).Error
	if err != nil {
		return
	}

	// 只有在limit不为0时才添加分页，支持获取全部数据
	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Find(&sysExportTemplates).Error
	return sysExportTemplates, total, err
}

// ExportExcel 导出Excel
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 支持动态SQL构建：根据模板配置动态生成查询语句
// - 支持多数据库：通过DBName字段支持查询不同的数据库
// - 支持多表关联：通过JoinTemplate配置支持JOIN查询
// - 支持条件过滤：通过Conditions配置支持动态WHERE条件
// - 支持软删除过滤：自动检测并过滤已删除的记录
// - 支持排序验证：防止SQL注入，只允许排序存在的字段
// - 好处：高度灵活，用户可以通过配置实现复杂的导出需求，无需修改代码
//
// 使用示例：
//
// 示例1：基础导出（带简单条件）
//
//	values := url.Values{}
//	params := url.Values{}
//	params.Set("name", "张三")           // 设置查询条件：name = "张三"
//	params.Set("filterDeleted", "true")  // 过滤已删除记录
//	params.Set("limit", "100")           // 限制导出100条
//	values.Set("params", params.Encode())
//	file, name, err := service.ExportExcel("template-001", values)
//
// 示例2：范围查询（BETWEEN条件）
//
//	values := url.Values{}
//	params := url.Values{}
//	params.Set("startCreatedAt", "2024-01-01")  // 开始时间
//	params.Set("endCreatedAt", "2024-12-31")   // 结束时间
//	params.Set("status", "active")             // 状态条件
//	values.Set("params", params.Encode())
//	file, name, err := service.ExportExcel("template-002", values)
//
// 示例3：模糊查询（LIKE条件）和排序
//
//	values := url.Values{}
//	params := url.Values{}
//	params.Set("keyword", "测试")        // LIKE查询：%测试%
//	params.Set("order", "created_at desc")  // 按创建时间倒序
//	params.Set("offset", "0")            // 分页偏移量
//	params.Set("limit", "50")            // 每页50条
//	values.Set("params", params.Encode())
//	file, name, err := service.ExportExcel("template-003", values)
//
// 示例4：IN查询（多值条件）
//
//	values := url.Values{}
//	params := url.Values{}
//	params.Set("ids", "1,2,3,4,5")       // IN查询：id IN (1,2,3,4,5)
//	params.Set("filterDeleted", "true")
//	values.Set("params", params.Encode())
//	file, name, err := service.ExportExcel("template-004", values)
//
// 示例5：多表关联查询
//
//	values := url.Values{}
//	params := url.Values{}
//	params.Set("departmentId", "10")     // 部门ID条件
//	params.Set("order", "user.created_at desc")
//	params.Set("limit", "200")
//	values.Set("params", params.Encode())
//	// 注意：需要在模板中配置JoinTemplate来实现多表关联
//	file, name, err := service.ExportExcel("template-005", values)
//
// 示例6：完整参数示例
//
//	values := url.Values{}
//	params := url.Values{}
//	params.Set("name", "张三")
//	params.Set("age", "25")
//	params.Set("startCreatedAt", "2024-01-01")
//	params.Set("endCreatedAt", "2024-12-31")
//	params.Set("status", "active")
//	params.Set("filterDeleted", "true")
//	params.Set("order", "created_at desc")
//	params.Set("offset", "0")
//	params.Set("limit", "1000")
//	values.Set("params", params.Encode())
//	file, name, err := service.ExportExcel("template-006", values)
//	if err != nil {
//	    // 处理错误
//	}
//	// 使用file.Bytes()获取Excel文件内容，name为文件名
func (sysExportTemplateService *SysExportTemplateService) ExportExcel(templateID string, values url.Values) (file *bytes.Buffer, name string, err error) {
	// 解析URL参数中的params，params是经过URL编码的查询字符串
	// 设计原因：支持复杂的参数传递，避免URL过长
	var params = values.Get("params")
	paramsValues, err := url.ParseQuery(params)
	if err != nil {
		return nil, "", fmt.Errorf("解析 params 参数失败: %v", err)
	}
	// 加载模板配置，预加载关联数据避免N+1查询
	var template system.SysExportTemplate
	err = global.GVA_DB.Preload("Conditions").Preload("JoinTemplate").First(&template, "template_id = ?", templateID).Error
	if err != nil {
		return nil, "", err
	}
	// 创建Excel文件对象
	f := excelize.NewFile()
	// 使用defer确保文件资源被正确释放，避免内存泄漏
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	// 创建新的工作表
	index, err := f.NewSheet("Sheet1")
	if err != nil {
		fmt.Println(err)
		return
	}
	// 解析模板信息JSON，获取列名和列标题的映射关系
	// TemplateInfo格式：{"column_name": "列标题", ...}
	var templateInfoMap = make(map[string]string)
	columns, err := utils.GetJSONKeys(template.TemplateInfo)
	if err != nil {
		return nil, "", err
	}
	err = json.Unmarshal([]byte(template.TemplateInfo), &templateInfoMap)
	if err != nil {
		return nil, "", err
	}
	// 构建SELECT字段列表和Excel表头
	var tableTitle []string
	var selectKeyFmt []string
	for _, key := range columns {
		selectKeyFmt = append(selectKeyFmt, key)
		tableTitle = append(tableTitle, templateInfoMap[key])
	}

	// 将字段列表拼接成SELECT子句
	selects := strings.Join(selectKeyFmt, ", ")
	var tableMap []map[string]interface{}
	// 支持多数据库：如果模板指定了DBName，使用指定的数据库连接
	// 好处：可以跨数据库查询，支持数据仓库、分库等场景
	db := global.GVA_DB
	if template.DBName != "" {
		db = global.MustGetGlobalDBByDBName(template.DBName)
	}

	// 动态添加JOIN子句，支持多表关联查询
	// 设计原因：允许用户配置复杂的多表关联，无需硬编码
	if len(template.JoinTemplate) > 0 {
		for _, join := range template.JoinTemplate {
			db = db.Joins(join.JOINS + " " + join.Table + " ON " + join.ON)
		}
	}

	// 设置SELECT字段和主表
	db = db.Select(selects).Table(template.TableName)

	// 软删除过滤：根据参数决定是否过滤已删除的记录
	filterDeleted := false

	filterParam := paramsValues.Get("filterDeleted")
	if filterParam == "true" {
		filterDeleted = true
	}

	if filterDeleted {
		// 自动过滤主表的软删除记录
		// 使用表名前缀避免在多表JOIN时字段名冲突
		db = db.Where(fmt.Sprintf("%s.deleted_at IS NULL", template.TableName))

		// 过滤关联表的软删除记录（如果关联表也支持软删除）
		// 设计原因：关联表可能也实现了软删除，需要一并过滤
		if len(template.JoinTemplate) > 0 {
			for _, join := range template.JoinTemplate {
				// 动态检查关联表是否有deleted_at字段
				// 好处：避免对不支持软删除的表添加无效的WHERE条件
				hasDeletedAt := sysExportTemplateService.hasDeletedAtColumn(join.Table)
				if hasDeletedAt {
					db = db.Where(fmt.Sprintf("%s.deleted_at IS NULL", join.Table))
				}
			}
		}
	}

	// 动态添加WHERE条件：根据模板配置的条件和传入的参数值构建查询条件
	// 设计原因：支持灵活的查询条件配置，用户可以通过参数动态控制查询结果
	if len(template.Conditions) > 0 {
		for _, condition := range template.Conditions {
			// 构建基础SQL条件，使用占位符防止SQL注入
			sql := fmt.Sprintf("%s %s ?", condition.Column, condition.Operator)
			value := paramsValues.Get(condition.From)

			// 特殊处理IN和NOT IN操作符，需要括号包裹
			if condition.Operator == "IN" || condition.Operator == "NOT IN" {
				sql = fmt.Sprintf("%s %s (?)", condition.Column, condition.Operator)
			}

			// 特殊处理BETWEEN操作符，需要两个参数（起始值和结束值）
			// 设计原因：BETWEEN是范围查询，需要特殊处理参数获取方式
			if condition.Operator == "BETWEEN" {
				sql = fmt.Sprintf("%s BETWEEN ? AND ?", condition.Column)
				startValue := paramsValues.Get("start" + condition.From)
				endValue := paramsValues.Get("end" + condition.From)
				// 只有当两个值都存在时才添加条件
				if startValue != "" && endValue != "" {
					db = db.Where(sql, startValue, endValue)
				}
				continue
			}

			// 只有当参数值不为空时才添加条件，支持可选条件
			if value != "" {
				// LIKE操作符需要添加通配符
				if condition.Operator == "LIKE" {
					value = "%" + value + "%"
				}
				// 使用参数化查询，防止SQL注入
				db = db.Where(sql, value)
			}
		}
	}
	// 分页控制：优先使用参数传入的limit，其次使用模板默认值
	// 设计原因：支持动态控制导出数据量，避免一次性导出过多数据导致内存溢出
	limit := paramsValues.Get("limit")
	if limit != "" {
		l, e := strconv.Atoi(limit)
		if e == nil {
			db = db.Limit(l)
		}
	}
	// 模板的默认limit：当参数未传入时使用模板配置的默认值
	// 好处：可以为每个模板设置合理的默认导出数量
	if limit == "" && template.Limit != nil && *template.Limit != 0 {
		db = db.Limit(*template.Limit)
	}

	// 偏移量控制：支持分页导出
	offset := paramsValues.Get("offset")
	if offset != "" {
		o, e := strconv.Atoi(offset)
		if e == nil {
			db = db.Offset(o)
		}
	}

	// SQL注入防护：获取表的实际字段列表，用于验证排序字段
	// 设计原因：防止用户通过order参数注入恶意SQL代码
	table := template.TableName
	orderColumns, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		return nil, "", err
	}

	// 创建字段名映射表，用于快速查找字段是否存在
	// 使用map提高查找效率，O(1)时间复杂度
	fields := make(map[string]bool)

	for _, column := range orderColumns {
		fields[column.Name()] = true
	}

	// 排序控制：优先使用参数传入的排序，其次使用模板默认排序
	order := paramsValues.Get("order")

	if order == "" && template.Order != "" {
		// 如果没有order入参，使用模板的默认排序
		order = template.Order
	}

	if order != "" {
		checkOrderArr := strings.Split(order, " ")
		orderStr := ""
		// 安全检查：验证排序字段是否存在于表的字段列表中
		// 好处：防止SQL注入，只允许排序实际存在的字段
		if _, ok := fields[checkOrderArr[0]]; !ok {
			return nil, "", fmt.Errorf("order by %s is not in the fields", order)
		}
		orderStr = checkOrderArr[0]
		// 验证排序方向，只允许ASC或DESC
		// 好处：防止注入恶意SQL代码
		if len(checkOrderArr) > 1 {
			if checkOrderArr[1] != "asc" && checkOrderArr[1] != "desc" {
				return nil, "", fmt.Errorf("order by %s is not secure", order)
			}
			orderStr = orderStr + " " + checkOrderArr[1]
		}
		db = db.Order(orderStr)
	}

	// 执行查询，使用Debug模式便于调试（生产环境可移除）
	// 查询结果存储为map切片，支持动态字段
	err = db.Debug().Find(&tableMap).Error
	if err != nil {
		return nil, "", err
	}
	// 构建Excel行数据：先添加表头，再添加数据行
	var rows [][]string
	rows = append(rows, tableTitle)
	for _, exTable := range tableMap {
		var row []string
		for _, column := range columns {
			// 清理列名中的引号，统一处理
			column = strings.ReplaceAll(column, "\"", "")
			column = strings.ReplaceAll(column, "`", "")
			// 处理多表JOIN时的列名：可能是"table.column"或"column as alias"格式
			// 设计原因：JOIN查询时列名可能包含表前缀或别名，需要提取实际列名
			if len(template.JoinTemplate) > 0 {
				columnAs := strings.Split(column, " as ")
				if len(columnAs) > 1 {
					// 提取别名
					column = strings.TrimSpace(strings.Split(column, " as ")[1])
				} else {
					// 提取表前缀后的列名
					columnArr := strings.Split(column, ".")
					if len(columnArr) > 1 {
						column = strings.Split(column, ".")[1]
					}
				}
			}
			// 时间类型特殊处理：格式化为标准时间字符串
			// 设计原因：Excel需要字符串格式的时间，不能直接使用time.Time类型
			if t, ok := exTable[column].(time.Time); ok {
				row = append(row, t.Format("2006-01-02 15:04:05"))
			} else {
				// 其他类型转换为字符串
				row = append(row, fmt.Sprintf("%v", exTable[column]))
			}
		}
		rows = append(rows, row)
	}
	// 将数据写入Excel单元格
	// 设计说明：尝试将字符串解析为数字类型，提高Excel的数据类型识别
	// 好处：数字类型在Excel中可以参与计算，文本类型则不能
	for i, row := range rows {
		for j, colCell := range row {
			// 计算Excel单元格地址（如A1, B2等）
			cell := fmt.Sprintf("%s%d", getColumnName(j+1), i+1)

			var sErr error
			// 尝试解析为浮点数，保持精度
			if v, err := strconv.ParseFloat(colCell, 64); err == nil {
				sErr = f.SetCellValue("Sheet1", cell, v)
			} else if v, err := strconv.ParseInt(colCell, 10, 64); err == nil {
				// 尝试解析为整数
				sErr = f.SetCellValue("Sheet1", cell, v)
			} else {
				// 无法解析为数字，作为文本处理
				sErr = f.SetCellValue("Sheet1", cell, colCell)
			}

			if sErr != nil {
				return nil, "", sErr
			}
		}
	}
	// 激活工作表并写入缓冲区
	f.SetActiveSheet(index)
	file, err = f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	return file, template.Name, nil
}

// PreviewSQL 预览最终生成的 SQL（不执行查询，仅返回 SQL 字符串）
// Author [piexlmax](https://github.com/piexlmax) & [trae-ai]
//
// 设计说明：
// - 不执行实际查询，只生成SQL字符串用于预览和调试
// - 与ExportExcel使用相同的逻辑，确保预览的SQL与实际执行的SQL一致
// - 对于未传入的参数值，使用占位符显示，便于用户理解需要哪些参数
// - 好处：帮助用户理解查询逻辑，调试模板配置，避免执行错误查询
func (sysExportTemplateService *SysExportTemplateService) PreviewSQL(templateID string, values url.Values) (sqlPreview string, err error) {
	// 解析 params（与导出逻辑保持一致）
	// 设计原因：确保预览的SQL与实际执行的SQL使用相同的参数解析逻辑
	var params = values.Get("params")
	paramsValues, _ := url.ParseQuery(params)

	// 加载模板
	var template system.SysExportTemplate
	err = global.GVA_DB.Preload("Conditions").Preload("JoinTemplate").First(&template, "template_id = ?", templateID).Error
	if err != nil {
		return "", err
	}

	// 解析模板列
	var templateInfoMap = make(map[string]string)
	columns, err := utils.GetJSONKeys(template.TemplateInfo)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(template.TemplateInfo), &templateInfoMap)
	if err != nil {
		return "", err
	}
	var selectKeyFmt []string
	for _, key := range columns {
		selectKeyFmt = append(selectKeyFmt, key)
	}
	selects := strings.Join(selectKeyFmt, ", ")

	// 使用strings.Builder高效拼接SQL字符串
	// 好处：比字符串拼接性能更好，内存分配更少
	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(selects)
	sb.WriteString(" FROM ")
	sb.WriteString(template.TableName)

	// 动态添加JOIN子句
	if len(template.JoinTemplate) > 0 {
		for _, join := range template.JoinTemplate {
			sb.WriteString(" ")
			sb.WriteString(join.JOINS)
			sb.WriteString(" ")
			sb.WriteString(join.Table)
			sb.WriteString(" ON ")
			sb.WriteString(join.ON)
		}
	}

	// WHERE 条件：使用切片收集所有条件，最后用AND连接
	// 设计原因：条件可能来自多个来源（软删除、模板条件等），需要统一收集
	var wheres []string

	// 软删除过滤：与ExportExcel逻辑保持一致
	filterDeleted := false
	if paramsValues != nil {
		filterParam := paramsValues.Get("filterDeleted")
		if filterParam == "true" {
			filterDeleted = true
		}
	}
	if filterDeleted {
		wheres = append(wheres, fmt.Sprintf("%s.deleted_at IS NULL", template.TableName))
		if len(template.JoinTemplate) > 0 {
			for _, join := range template.JoinTemplate {
				if sysExportTemplateService.hasDeletedAtColumn(join.Table) {
					wheres = append(wheres, fmt.Sprintf("%s.deleted_at IS NULL", join.Table))
				}
			}
		}
	}

	// 模板条件：与ExportExcel保持完全同步的解析规则
	// 设计原因：确保预览的SQL与实际执行的SQL一致
	if len(template.Conditions) > 0 {
		for _, condition := range template.Conditions {
			// 统一转换为大写并去除空格，确保操作符匹配
			op := strings.ToUpper(strings.TrimSpace(condition.Operator))
			col := strings.TrimSpace(condition.Column)

			// 预览逻辑：优先展示传入值，没有则展示占位符
			// 好处：用户可以看到实际会执行的SQL，也可以看到需要哪些参数
			val := ""
			if paramsValues != nil {
				val = paramsValues.Get(condition.From)
			}

			switch op {
			case "BETWEEN":
				startValue := ""
				endValue := ""
				if paramsValues != nil {
					startValue = paramsValues.Get("start" + condition.From)
					endValue = paramsValues.Get("end" + condition.From)
				}
				if startValue != "" && endValue != "" {
					// 有值：显示实际SQL
					wheres = append(wheres, fmt.Sprintf("%s BETWEEN '%s' AND '%s'", col, startValue, endValue))
				} else {
					// 无值：显示占位符，提示用户需要传入start{From}和end{From}参数
					wheres = append(wheres, fmt.Sprintf("%s BETWEEN {start%s} AND {end%s}", col, condition.From, condition.From))
				}
			case "IN", "NOT IN":
				if val != "" {
					// 逗号分隔值做简单展示：将逗号分隔的字符串转换为SQL IN格式
					parts := strings.Split(val, ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					wheres = append(wheres, fmt.Sprintf("%s %s ('%s')", col, op, strings.Join(parts, "','")))
				} else {
					// 无值：显示占位符
					wheres = append(wheres, fmt.Sprintf("%s %s ({%s})", col, op, condition.From))
				}
			case "LIKE":
				if val != "" {
					// 有值：显示实际的LIKE模式
					wheres = append(wheres, fmt.Sprintf("%s LIKE '%%%s%%'", col, val))
				} else {
					// 无值：显示占位符，提示LIKE的模式
					wheres = append(wheres, fmt.Sprintf("%s LIKE {%%%s%%}", col, condition.From))
				}
			default:
				if val != "" {
					// 有值：显示实际值
					wheres = append(wheres, fmt.Sprintf("%s %s '%s'", col, op, val))
				} else {
					// 无值：显示占位符
					wheres = append(wheres, fmt.Sprintf("%s %s {%s}", col, op, condition.From))
				}
			}
		}
	}

	// 将所有WHERE条件用AND连接
	if len(wheres) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(wheres, " AND "))
	}

	// 排序：优先使用参数传入的排序，其次使用模板默认排序
	order := ""
	if paramsValues != nil {
		order = paramsValues.Get("order")
	}
	if order == "" && template.Order != "" {
		order = template.Order
	}
	if order != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(order)
	}

	// limit/offset处理：如果传入或默认值为0，则不生成
	// 设计原因：避免生成无意义的LIMIT 0或OFFSET 0
	limitStr := ""
	offsetStr := ""
	if paramsValues != nil {
		limitStr = paramsValues.Get("limit")
		offsetStr = paramsValues.Get("offset")
	}

	// 处理模板默认limit（仅当非0时）
	if limitStr == "" && template.Limit != nil && *template.Limit != 0 {
		limitStr = strconv.Itoa(*template.Limit)
	}

	// 解析为数值，用于判断是否生成LIMIT/OFFSET子句
	limitInt := 0
	offsetInt := 0
	if limitStr != "" {
		if v, e := strconv.Atoi(limitStr); e == nil {
			limitInt = v
		}
	}
	if offsetStr != "" {
		if v, e := strconv.Atoi(offsetStr); e == nil {
			offsetInt = v
		}
	}

	// 根据limit和offset的值决定是否生成SQL子句
	// 好处：避免生成无效的SQL片段，使预览的SQL更清晰
	if limitInt > 0 {
		sb.WriteString(" LIMIT ")
		sb.WriteString(strconv.Itoa(limitInt))
		if offsetInt > 0 {
			sb.WriteString(" OFFSET ")
			sb.WriteString(strconv.Itoa(offsetInt))
		}
	} else {
		// 当limit未设置或为0时，仅当offset>0才生成OFFSET
		// 设计原因：某些数据库不支持只有OFFSET没有LIMIT的语法
		if offsetInt > 0 {
			sb.WriteString(" OFFSET ")
			sb.WriteString(strconv.Itoa(offsetInt))
		}
	}

	return sb.String(), nil
}

// ExportTemplate 导出Excel模板（仅包含表头，无数据）
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 只导出表头，不包含数据，用于生成导入模板
// - 用户可以根据这个模板填写数据，然后使用ImportExcel导入
// - 好处：提供标准的导入格式，避免用户手动创建Excel时格式错误
func (sysExportTemplateService *SysExportTemplateService) ExportTemplate(templateID string) (file *bytes.Buffer, name string, err error) {
	var template system.SysExportTemplate
	err = global.GVA_DB.First(&template, "template_id = ?", templateID).Error
	if err != nil {
		return nil, "", err
	}
	// 创建Excel文件对象
	f := excelize.NewFile()
	// 使用defer确保文件资源被正确释放
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	// 创建新的工作表
	index, err := f.NewSheet("Sheet1")
	if err != nil {
		fmt.Println(err)
		return
	}
	// 解析模板信息，获取列标题
	var templateInfoMap = make(map[string]string)

	columns, err := utils.GetJSONKeys(template.TemplateInfo)

	err = json.Unmarshal([]byte(template.TemplateInfo), &templateInfoMap)
	if err != nil {
		return nil, "", err
	}
	// 构建表头行
	var tableTitle []string
	for _, key := range columns {
		tableTitle = append(tableTitle, templateInfoMap[key])
	}

	// 将表头写入Excel第一行
	for i := range tableTitle {
		fErr := f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", getColumnName(i+1), 1), tableTitle[i])
		if fErr != nil {
			return nil, "", fErr
		}
	}
	// 激活工作表并写入缓冲区
	f.SetActiveSheet(index)
	file, err = f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	return file, template.Name, nil
}

// hasDeletedAtColumn 辅助函数：检查表是否有deleted_at列
//
// 设计说明：
// - 使用INFORMATION_SCHEMA查询表的元数据，检查是否存在deleted_at列
// - 使用参数化查询防止SQL注入
// - 好处：动态检测表结构，避免对不支持软删除的表添加无效的WHERE条件
// - 注意：此方法依赖数据库的INFORMATION_SCHEMA，不同数据库可能有差异
func (s *SysExportTemplateService) hasDeletedAtColumn(tableName string) bool {
	var count int64
	// 查询INFORMATION_SCHEMA.COLUMNS表，检查是否存在deleted_at列
	// 使用参数化查询，tableName通过占位符传入，防止SQL注入
	global.GVA_DB.Raw("SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = 'deleted_at'", tableName).Count(&count)
	return count > 0
}

// ImportExcel 导入Excel数据到数据库
// Author [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// - 支持根据模板配置导入Excel数据
// - 使用事务确保数据一致性：要么全部成功，要么全部回滚
// - 自动处理时间戳字段（created_at、updated_at）
// - 支持批量插入，提高导入性能
// - 支持多数据库：可以导入到不同的数据库
// - 好处：提供标准化的数据导入功能，支持大量数据的高效导入
func (sysExportTemplateService *SysExportTemplateService) ImportExcel(templateID string, file *multipart.FileHeader) (err error) {
	var template system.SysExportTemplate
	err = global.GVA_DB.First(&template, "template_id = ?", templateID).Error
	if err != nil {
		return err
	}

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return err
	}
	// 使用defer确保文件资源被正确释放
	defer src.Close()

	// 使用excelize库读取Excel文件
	f, err := excelize.OpenReader(src)
	if err != nil {
		return err
	}

	// 读取Sheet1的所有行数据
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		return err
	}
	// 验证数据完整性：至少需要表头行和数据行
	if len(rows) < 2 {
		return errors.New("Excel data is not enough.\nIt should contain title row and data")
	}

	// 解析模板信息，获取列名和列标题的映射
	var templateInfoMap = make(map[string]string)
	err = json.Unmarshal([]byte(template.TemplateInfo), &templateInfoMap)
	if err != nil {
		return err
	}

	// 构建反向映射：从列标题到列名
	// 设计原因：Excel第一行是列标题，需要通过标题找到对应的数据库字段名
	var titleKeyMap = make(map[string]string)
	for key, title := range templateInfoMap {
		titleKeyMap[title] = key
	}

	// 支持多数据库：如果模板指定了DBName，使用指定的数据库连接
	db := global.GVA_DB
	if template.DBName != "" {
		db = global.MustGetGlobalDBByDBName(template.DBName)
	}

	// 使用事务确保数据一致性
	return db.Transaction(func(tx *gorm.DB) error {
		// 获取Excel表头行，去除前后空格
		excelTitle := rows[0]
		for i, str := range excelTitle {
			excelTitle[i] = strings.TrimSpace(str)
		}
		// 获取数据行（从第二行开始）
		values := rows[1:]
		// 预分配切片容量，提高性能
		items := make([]map[string]interface{}, 0, len(values))
		for _, row := range values {
			var item = make(map[string]interface{})
			// 将Excel行数据转换为map，key为数据库字段名
			for ii, value := range row {
				// 跳过Excel中存在但模板中不存在的列
				// 设计原因：Excel可能包含额外的列，这些列不应该导入
				if _, ok := titleKeyMap[excelTitle[ii]]; !ok {
					continue // excel中多余的标题，在模板信息中没有对应的字段，因此key为空，必须跳过
				}
				key := titleKeyMap[excelTitle[ii]]
				item[key] = value
			}

			// 动态检查表是否有时间戳字段，如果有则自动填充
			// 设计原因：不同表的时间戳字段可能不同，需要动态检测
			needCreated := tx.Migrator().HasColumn(template.TableName, "created_at")
			needUpdated := tx.Migrator().HasColumn(template.TableName, "updated_at")

			// 如果Excel中没有提供时间戳，且表需要这些字段，则自动填充当前时间
			if item["created_at"] == nil && needCreated {
				item["created_at"] = time.Now()
			}
			if item["updated_at"] == nil && needUpdated {
				item["updated_at"] = time.Now()
			}

			items = append(items, item)
		}
		// 批量插入数据，每批1000条
		// 好处：比逐条插入效率高，同时避免单次插入过多数据导致内存问题
		cErr := tx.Table(template.TableName).CreateInBatches(&items, 1000).Error
		return cErr
	})
}

// getColumnName 将数字转换为Excel列名（如1->A, 2->B, 27->AA等）
//
// 设计说明：
// - 使用26进制转换算法，将数字转换为Excel的列名格式
// - Excel列名是A-Z, AA-ZZ, AAA-ZZZ...的格式，类似于26进制
// - 算法说明：
//  1. 每次取n%26得到当前位的字符（0-25对应A-Z）
//  2. 将字符插入到结果字符串的前面（因为是从低位到高位计算）
//  3. n除以26继续处理下一位
//
// - 好处：高效地将列索引转换为Excel列名，支持任意数量的列
func getColumnName(n int) string {
	columnName := ""
	for n > 0 {
		n--
		// n%26得到0-25，对应A-Z（'A'+0='A', 'A'+25='Z'）
		// 将字符插入到前面，因为是从低位到高位计算
		columnName = string(rune('A'+n%26)) + columnName
		// 除以26继续处理下一位
		n /= 26
	}
	return columnName
}
