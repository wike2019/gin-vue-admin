package utils

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

// RegisterApis 注册 API 到系统
// 设计模式：注册模式（Registry Pattern） + 事务模式（Transaction Pattern）
// 好处：
// 1. 插件可以将自己的 API 自动注册到系统中，无需手动配置
// 2. 使用数据库事务，确保所有 API 要么全部注册成功，要么全部失败
// 3. 使用 FirstOrCreate，避免重复注册，支持插件多次初始化
// 4. 通过路径、方法和 API 组唯一标识 API，避免重复
// 5. 详细的错误日志，包含 API 路径、方法和组信息，便于问题排查
// @param apis ...system.SysApi 可变参数，可以传入多个 API 进行批量注册
func RegisterApis(apis ...system.SysApi) {
	// 使用数据库事务，确保原子性
	// 好处：
	// 1. 如果任何一个 API 注册失败，所有操作都会回滚
	// 2. 保证数据一致性，避免部分 API 注册成功、部分失败的情况
	err := global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 遍历所有要注册的 API
		for _, api := range apis {
			// 使用 FirstOrCreate 方法，如果不存在则创建，存在则返回现有记录
			// 查询条件：路径、方法和 API 组必须完全匹配
			// 好处：
			// 1. 避免重复注册相同的 API
			// 2. 支持插件多次初始化，不会产生重复数据
			// 3. 如果 API 已存在，会更新为最新的配置
			err := tx.Model(system.SysApi{}).Where("path = ? AND method = ? AND api_group = ? ", api.Path, api.Method, api.ApiGroup).FirstOrCreate(&api).Error
			if err != nil {
				// 使用结构化日志记录错误，包含详细的上下文信息
				// 好处：便于问题排查和监控告警
				zap.L().Error("注册API失败", zap.Error(err), zap.String("api", api.Path), zap.String("method", api.Method), zap.String("apiGroup", api.ApiGroup))
				return err
			}
		}
		return nil
	})
	if err != nil {
		// 事务失败，记录错误日志
		zap.L().Error("注册API失败", zap.Error(err))
	}
}

// RegisterMenus 注册菜单到系统
// 设计模式：注册模式（Registry Pattern） + 事务模式（Transaction Pattern） + 树形结构模式
// 好处：
// 1. 插件可以将自己的菜单自动注册到系统中，无需手动配置
// 2. 支持父子菜单结构，第一个菜单作为父菜单，其余作为子菜单
// 3. 使用数据库事务，确保所有菜单要么全部注册成功，要么全部失败
// 4. 使用 FirstOrCreate，避免重复注册，支持插件多次初始化
// 5. 自动建立父子关系，简化菜单配置
// @param menus ...system.SysBaseMenu 可变参数，第一个菜单为父菜单，其余为子菜单
func RegisterMenus(menus ...system.SysBaseMenu) {
	// 分离父菜单和子菜单
	// 设计模式：策略模式（Strategy Pattern）
	// 好处：
	// 1. 第一个菜单作为父菜单，其余作为子菜单，简化配置
	// 2. 自动建立菜单层级关系，无需手动设置 ParentId
	parentMenu := menus[0]
	otherMenus := menus[1:]
	
	// 使用数据库事务，确保原子性
	err := global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 先注册父菜单
		// 使用 FirstOrCreate，如果不存在则创建，存在则返回现有记录
		// 查询条件：菜单名称必须匹配
		// 好处：避免重复注册，支持插件多次初始化
		err := tx.Model(system.SysBaseMenu{}).Where("name = ? ", parentMenu.Name).FirstOrCreate(&parentMenu).Error
		if err != nil {
			zap.L().Error("注册菜单失败", zap.Error(err))
			// 使用 errors.Wrap 包装错误，保留原始错误信息和堆栈
			// 好处：便于错误追踪和调试
			return errors.Wrap(err, "注册菜单失败")
		}
		
		// 获取父菜单的 ID，用于设置子菜单的 ParentId
		pid := parentMenu.ID
		
		// 遍历注册所有子菜单
		for i := range otherMenus {
			// 设置子菜单的父菜单 ID
			// 好处：自动建立菜单层级关系，无需手动配置
			otherMenus[i].ParentId = pid
			// 注册子菜单，使用 FirstOrCreate 避免重复
			err = tx.Model(system.SysBaseMenu{}).Where("name = ? ", otherMenus[i].Name).FirstOrCreate(&otherMenus[i]).Error
			if err != nil {
				zap.L().Error("注册菜单失败", zap.Error(err))
				return errors.Wrap(err, "注册菜单失败")
			}
		}

		return nil
	})
	if err != nil {
		// 事务失败，记录错误日志
		zap.L().Error("注册菜单失败", zap.Error(err))
	}
}
