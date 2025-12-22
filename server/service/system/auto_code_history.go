package system

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/utils/ast"
	"github.com/pkg/errors"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	common "github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	request "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"

	"go.uber.org/zap"
)

// AutocodeHistory 代码生成器历史记录服务实例
// 使用单例模式，确保全局只有一个服务实例，避免重复创建，节省内存
var AutocodeHistory = new(autoCodeHistory)

// autoCodeHistory 代码生成器历史记录服务结构体
// 使用空结构体作为接收器，不占用内存空间，只用于方法分组
type autoCodeHistory struct{}

// Create 创建代码生成器历史记录
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [songzhibin97](https://github.com/songzhibin97)
//
// 设计说明：
// 1. 使用 context.Context 参数：支持请求超时控制、取消操作和链路追踪，提高系统的可观测性和可控性
// 2. 使用 WithContext(ctx)：将 context 传递给数据库操作，确保数据库操作可以响应上下文取消信号
// 3. 使用 errors.Wrap：包装原始错误并添加上下文信息，便于错误定位和调试，保持错误堆栈信息
// 4. 通过 info.Create() 方法转换：将请求参数转换为数据库模型，实现数据层和业务层的解耦
func (s *autoCodeHistory) Create(ctx context.Context, info request.SysAutoHistoryCreate) error {
	// 将请求参数转换为数据库模型，实现数据转换的封装
	create := info.Create()
	// 使用 WithContext 确保数据库操作可以响应上下文取消，支持超时和取消机制
	err := global.GVA_DB.WithContext(ctx).Create(&create).Error
	if err != nil {
		// 使用 errors.Wrap 包装错误，保留原始错误信息并添加上下文，便于错误追踪
		return errors.Wrap(err, "创建失败!")
	}
	return nil
}

// First 根据id获取代码生成器历史的数据
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [songzhibin97](https://github.com/songzhibin97)
//
// 设计说明：
// 1. 使用 Pluck 方法：只查询需要的单个字段（request），避免查询整行数据，减少内存占用和网络传输
// 2. 使用 Model() 指定模型：明确指定查询的表结构，提高代码可读性和类型安全
// 3. 返回 string 类型：直接返回序列化的请求数据，调用方可以根据需要反序列化，保持接口简洁
// 4. 使用参数化查询（?）：防止 SQL 注入攻击，提高安全性
func (s *autoCodeHistory) First(ctx context.Context, info common.GetById) (string, error) {
	var meta string
	// 使用 Pluck 只查询 request 字段，而不是查询整行，节省内存和网络带宽
	// 使用 Model() 明确指定表模型，提高代码可读性
	// 使用参数化查询防止 SQL 注入
	err := global.GVA_DB.WithContext(ctx).Model(model.SysAutoCodeHistory{}).Where("id = ?", info.ID).Pluck("request", &meta).Error
	if err != nil {
		return "", errors.Wrap(err, "获取失败!")
	}
	return meta, nil
}

// Repeat 检测重复
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [songzhibin97](https://github.com/songzhibin97)
//
// 设计说明：
// 1. 检测逻辑：检查同一业务数据库、同一包下，是否存在相同结构体名或缩写的未回滚记录（flag=0）
// 2. 使用 OR 条件：struct_name 或 abbreviation 任一匹配即视为重复，因为两者都可能作为唯一标识
// 3. flag = 0 条件：只检查未回滚的记录（flag=0），已回滚的记录（flag=1）不参与重复检测，允许重新生成
// 4. 使用 Count 方法：只统计数量而不查询具体数据，性能更好，节省内存
// 5. 使用 Debug()：在开发环境可以输出 SQL 语句，便于调试和优化
// 6. 返回 bool：简化调用方的判断逻辑，语义清晰
//
// 好处：
// - 防止重复生成相同结构的代码，避免代码冲突
// - 支持软删除机制（通过 flag 标记），保留历史记录便于追溯
// - 性能优化：只统计数量，不查询完整数据
func (s *autoCodeHistory) Repeat(businessDB, structName, abbreviation, Package string) bool {
	var count int64
	// 检测同一业务数据库、同一包下是否存在相同结构体名或缩写的未回滚记录
	// flag=0 表示未回滚，flag=1 表示已回滚，只检测未回滚的记录避免误判
	// 使用 OR 条件：结构体名或缩写任一匹配即视为重复
	// Debug() 在开发环境输出 SQL，便于调试
	global.GVA_DB.Model(&model.SysAutoCodeHistory{}).Where("business_db = ? and (struct_name = ? OR abbreviation = ?) and package = ? and flag = ?", businessDB, structName, abbreviation, Package, 0).Count(&count).Debug()
	return count > 0
}

// RollBack 回滚代码生成操作
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [songzhibin97](https://github.com/songzhibin97)
//
// 设计说明：
// 回滚操作需要清理代码生成时创建的所有资源，包括：
// 1. 导出模板（如果存在）
// 2. API 接口（可选，由用户决定）
// 3. 菜单项（可选，由用户决定）
// 4. 数据库表（可选，由用户决定）
// 5. 注入到现有文件的代码（通过 AST 操作回滚）
// 6. 生成的新文件（移动到临时目录，而不是直接删除，便于恢复）
// 7. 标记历史记录为已回滚（flag=1）
//
// 好处：
// - 支持选择性回滚：用户可以选择回滚哪些资源，灵活性高
// - 文件不直接删除：移动到临时目录，可以恢复，降低误操作风险
// - 使用 AST 回滚注入代码：精确删除注入的代码片段，不影响其他代码
// - 原子性标记：最后更新 flag，确保回滚状态的一致性
func (s *autoCodeHistory) RollBack(ctx context.Context, info request.SysAutoHistoryRollBack) error {
	// 第一步：查询历史记录，获取回滚所需的所有信息
	var history model.SysAutoCodeHistory
	err := global.GVA_DB.Where("id = ?", info.ID).First(&history).Error
	if err != nil {
		return err
	}

	// 第二步：删除导出模板（如果存在）
	// 检查 ExportTemplateID 是否为 0，避免无效删除操作
	if history.ExportTemplateID != 0 {
		err = global.GVA_DB.Delete(&model.SysExportTemplate{}, "id = ?", history.ExportTemplateID).Error
		if err != nil {
			return err
		}
	}

	// 第三步：可选删除 API 接口
	// 使用 info.DeleteApi 标志让用户决定是否删除，提供灵活性
	// 如果删除失败只记录日志不中断流程，因为 API 可能已被其他方式删除
	if info.DeleteApi {
		ids := info.ApiIds(history)
		err = ApiServiceApp.DeleteApisByIds(ids)
		if err != nil {
			// 只记录错误不返回，因为 API 可能已被手动删除，不影响其他回滚操作
			global.GVA_LOG.Error("ClearTag DeleteApiByIds:", zap.Error(err))
		}
	}

	// 第四步：可选删除菜单项
	// 菜单删除失败需要返回错误，因为菜单可能影响系统功能
	if info.DeleteMenu {
		err = BaseMenuServiceApp.DeleteBaseMenu(int(history.MenuID))
		if err != nil {
			return errors.Wrap(err, "删除菜单失败!")
		}
	}

	// 第五步：可选删除数据库表
	// 表删除失败需要返回错误，因为表结构可能影响数据完整性
	if info.DeleteTable {
		err = s.DropTable(history.BusinessDB, history.Table)
		if err != nil {
			return errors.Wrap(err, "删除表失败!")
		}
	}

	// 第六步：处理模板文件路径
	// 历史记录中保存的文件路径可能是相对路径，需要转换为绝对路径以便删除
	// 使用 make 预分配 map 容量，避免扩容带来的性能开销
	templates := make(map[string]string, len(history.Templates))
	for key, template := range history.Templates {
		// 处理 key：将相对路径转换为相对于 server 目录的路径
		// 使用代码块 {} 限制变量作用域，避免变量污染
		{
			server := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server)
			// 使用 filepath.Join 处理路径分隔符，跨平台兼容（Windows/Linux/Mac）
			keys := strings.Split(key, "/")
			key = filepath.Join(keys...)
			// 移除 server 路径前缀，得到相对路径
			key = strings.TrimPrefix(key, server)
		}

		// 处理 value：根据文件扩展名确定文件属于 web 还是 server 目录
		// 使用代码块 {} 限制变量作用域
		{
			web := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.WebRoot())
			server := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server)
			slices := strings.Split(template, "/")
			template = filepath.Join(slices...)
			// 根据文件扩展名判断文件类型，确定正确的根目录
			ext := path.Ext(template)
			switch ext {
			case ".js", ".vue":
				// 前端文件放在 web 目录
				template = filepath.Join(web, template)
			case ".go":
				// 后端文件放在 server 目录
				template = filepath.Join(server, template)
			}
		}
		templates[key] = template
	}
	history.Templates = templates

	// 第七步：回滚注入到现有文件的代码
	// 使用 AST（抽象语法树）解析文件，精确删除注入的代码片段
	// 这种方式比字符串替换更安全，不会误删相似代码
	for key, value := range history.Injections {
		var injection ast.Ast
		// 根据注入类型反序列化为对应的 AST 实体
		// 使用策略模式，不同类型的注入使用不同的处理实体
		switch key {
		case ast.TypePackageApiEnter, ast.TypePackageRouterEnter, ast.TypePackageServiceEnter:
			// 这些类型不需要特殊处理，可能已经在其他地方处理

		case ast.TypePackageApiModuleEnter, ast.TypePackageRouterModuleEnter, ast.TypePackageServiceModuleEnter:
			// 包模块入口注入
			var entity ast.PackageModuleEnter
			_ = json.Unmarshal([]byte(value), &entity)
			injection = &entity
		case ast.TypePackageInitializeGorm:
			// 包 GORM 初始化注入
			var entity ast.PackageInitializeGorm
			_ = json.Unmarshal([]byte(value), &entity)
			injection = &entity
		case ast.TypePackageInitializeRouter:
			// 包路由初始化注入
			var entity ast.PackageInitializeRouter
			_ = json.Unmarshal([]byte(value), &entity)
			injection = &entity
		case ast.TypePluginGen:
			// 插件生成注入
			var entity ast.PluginGen
			_ = json.Unmarshal([]byte(value), &entity)
			injection = &entity
		case ast.TypePluginApiEnter, ast.TypePluginRouterEnter, ast.TypePluginServiceEnter:
			// 插件入口注入
			var entity ast.PluginEnter
			_ = json.Unmarshal([]byte(value), &entity)
			injection = &entity
		case ast.TypePluginInitializeGorm:
			// 插件 GORM 初始化注入
			var entity ast.PluginInitializeGorm
			_ = json.Unmarshal([]byte(value), &entity)
			injection = &entity
		case ast.TypePluginInitializeRouter:
			// 插件路由初始化注入
			var entity ast.PluginInitializeRouter
			_ = json.Unmarshal([]byte(value), &entity)
			injection = &entity
		}
		// 如果无法识别注入类型，跳过
		if injection == nil {
			continue
		}
		// 解析文件为 AST
		file, _ := injection.Parse("", nil)
		if file != nil {
			// 执行回滚操作，删除注入的代码
			_ = injection.Rollback(file)
			// 格式化文件，确保代码风格一致
			err = injection.Format("", nil, file)
			if err != nil {
				return err
			}
			fmt.Printf("[filepath:%s]回滚注入代码成功!\n", key)
		}
	}

	// 第八步：移动生成的文件到临时目录（而不是直接删除）
	// 使用时间戳纳秒作为目录名，确保唯一性，避免文件覆盖
	// 移动到临时目录而不是直接删除，便于恢复和调试
	removeBasePath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, "rm_file", strconv.FormatInt(int64(time.Now().Nanosecond()), 10))
	for _, value := range history.Templates {
		// 只处理绝对路径的文件，相对路径可能不存在或不需要处理
		if !filepath.IsAbs(value) {
			continue
		}
		// 保持文件在项目中的相对路径结构，便于识别和恢复
		removePath := filepath.Join(removeBasePath, strings.TrimPrefix(value, global.GVA_CONFIG.AutoCode.Root))
		err = utils.FileMove(value, removePath)
		if err != nil {
			// 使用 Wrapf 格式化错误信息，包含源路径和目标路径，便于定位问题
			return errors.Wrapf(err, "[src:%s][dst:%s]文件移动失败!", value, removePath)
		}
	}

	// 第九步：标记历史记录为已回滚（flag=1）
	// 最后更新 flag，确保只有所有回滚操作成功后才标记为已回滚
	// 这样即使中途失败，记录仍然是未回滚状态，可以重试
	err = global.GVA_DB.WithContext(ctx).Model(&model.SysAutoCodeHistory{}).Where("id = ?", info.ID).Update("flag", 1).Error
	if err != nil {
		return errors.Wrap(err, "更新失败!")
	}
	return nil
}

// Delete 删除历史数据
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [songzhibin97](https://github.com/songzhibin97)
//
// 设计说明：
// 1. 使用 info.Uint() 方法：将 ID 转换为 uint 类型，类型安全，避免类型转换错误
// 2. 使用 WithContext：支持上下文取消和超时控制
// 3. 使用参数化查询：防止 SQL 注入
// 4. 使用 errors.Wrap：保留错误堆栈信息，便于调试
//
// 注意：这是物理删除，删除后无法恢复，建议在生产环境谨慎使用
func (s *autoCodeHistory) Delete(ctx context.Context, info common.GetById) error {
	// 使用 Uint() 方法确保类型安全，避免类型转换错误
	// 使用参数化查询防止 SQL 注入攻击
	err := global.GVA_DB.WithContext(ctx).Where("id = ?", info.Uint()).Delete(&model.SysAutoCodeHistory{}).Error
	if err != nil {
		return errors.Wrap(err, "删除失败!")
	}
	return nil
}

// GetList 获取系统历史数据列表（支持分页）
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [songzhibin97](https://github.com/songzhibin97)
//
// 设计说明：
// 1. 分两步查询：先查询总数，再查询分页数据，确保总数准确
// 2. 使用 Scopes 方法：将分页逻辑封装在 PageInfo.Paginate() 中，代码复用性高，符合 GORM 最佳实践
// 3. 使用链式调用：先应用分页，再排序，最后查询，逻辑清晰
// 4. 按 updated_at desc 排序：最新的记录在前，符合用户查看习惯
// 5. 返回 total 和 list：前端可以显示总数和分页信息，用户体验好
//
// 好处：
// - 分页逻辑封装在 PageInfo 中，所有列表查询都可以复用，减少重复代码
// - 先查总数再查数据，确保分页信息准确
// - 使用命名返回值，代码更清晰
func (s *autoCodeHistory) GetList(ctx context.Context, info common.PageInfo) (list []model.SysAutoCodeHistory, total int64, err error) {
	var entities []model.SysAutoCodeHistory
	// 创建查询构建器，使用 Model 明确指定表模型
	db := global.GVA_DB.WithContext(ctx).Model(&model.SysAutoCodeHistory{})
	// 第一步：查询总数，用于分页计算
	err = db.Count(&total).Error
	if err != nil {
		return nil, total, err
	}
	// 第二步：应用分页和排序，查询数据
	// Scopes 方法应用分页逻辑（LIMIT 和 OFFSET），封装在 PageInfo.Paginate() 中
	// Order 按更新时间倒序排列，最新的记录在前
	err = db.Scopes(info.Paginate()).Order("updated_at desc").Find(&entities).Error
	return entities, total, err
}

// DropTable 删除指定数据库和指定数据表
// @author: [piexlmax](https://github.com/piexlmax)
//
// 设计说明：
// 1. 支持多数据库：如果指定了 BusinessDb，使用对应的数据库连接；否则使用默认数据库
// 2. 使用 Exec 执行原生 SQL：DROP TABLE 是 DDL 操作，不需要 GORM 的 ORM 功能，直接执行 SQL 更高效
// 3. 简单条件判断：根据 BusinessDb 是否为空选择数据库连接
//
// 注意：
// - DROP TABLE 是危险操作，会永久删除表和数据，无法恢复
// - 这里直接拼接表名，如果表名来自用户输入需要做 SQL 注入防护
// - 建议在生产环境添加额外的安全检查
//
// 好处：
// - 支持多数据库架构，可以删除不同业务数据库中的表
// - 代码简洁，逻辑清晰
func (s *autoCodeHistory) DropTable(BusinessDb, tableName string) error {
	// 如果指定了业务数据库，使用对应的数据库连接
	// 支持多数据库架构，不同业务可以使用不同的数据库
	if BusinessDb != "" {
		// MustGetGlobalDBByDBName 根据数据库名称获取对应的数据库连接
		// 使用 Exec 执行原生 SQL，DROP TABLE 是 DDL 操作，不需要 ORM 功能
		return global.MustGetGlobalDBByDBName(BusinessDb).Exec("DROP TABLE " + tableName).Error
	} else {
		// 未指定业务数据库，使用默认数据库连接
		return global.GVA_DB.Exec("DROP TABLE " + tableName).Error
	}
}
