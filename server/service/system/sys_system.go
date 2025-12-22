package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"go.uber.org/zap"
)

// SystemConfigService 系统配置服务结构体
// 使用结构体组织服务方法的好处：
// 1. 封装相关功能：将系统配置相关的所有业务逻辑集中在一个结构体下，便于管理和维护
// 2. 便于扩展：虽然当前是空结构体，但未来如果需要添加服务级别的状态或依赖，可以直接在结构体中添加字段
// 3. 方法分组：通过接收者将方法分组，代码结构更清晰，符合面向对象的设计思想
// 4. 易于测试：结构体服务模式便于进行单元测试和依赖注入
type SystemConfigService struct{}

// SystemConfigServiceApp 系统配置服务的单例实例
// 使用单例模式的好处：
// 1. 全局唯一实例：确保整个应用中只有一个服务实例，避免重复创建对象造成的资源浪费
// 2. 简化调用：通过导出的变量可以直接调用，无需每次创建新实例，使用更方便
// 3. 统一管理：所有对系统配置服务的调用都通过同一个实例，便于统一管理和监控
// 4. 线程安全：在Go中，如果服务本身是无状态的，单例模式天然是线程安全的（此结构体为空结构体）
var SystemConfigServiceApp = new(SystemConfigService)

// GetSystemConfig 获取系统配置
// 设计思路：
//  1. 直接返回全局配置：系统配置是应用级别的配置，存储在全局变量中，直接返回即可
//  2. 简单的读取操作：此方法只是读取配置，不涉及复杂逻辑，保持简单直接
//  3. 统一的配置访问入口：通过服务层提供统一的配置访问接口，而不是直接访问全局变量
//     好处：如果未来需要添加配置缓存、配置验证等逻辑，只需修改此方法，调用方无需改动
func (systemConfigService *SystemConfigService) GetSystemConfig() (conf config.Server, err error) {
	return global.GVA_CONFIG, nil
}

// @description   set system config,
//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetSystemConfig
//@description: 设置配置文件
//@param: system model.System
//@return: err error

// SetSystemConfig 设置系统配置
// 设计思路：
//  1. 结构体转Map：使用 utils.StructToMap 将配置结构体转换为 map，这样可以通过循环统一处理
//     好处：避免手动逐个字段设置，代码更简洁，易于维护
//  2. 批量设置配置：通过循环遍历 map，将所有配置项批量设置到全局配置管理器
//     好处：如果配置项增加，无需修改此方法，自动适配
//  3. 持久化配置：调用 WriteConfig() 将配置写入配置文件，确保配置修改持久化
//     好处：配置修改后立即生效并保存，应用重启后配置仍然有效
//  4. 错误处理：返回错误信息，让调用方知道配置写入是否成功
func (systemConfigService *SystemConfigService) SetSystemConfig(system system.System) (err error) {
	// 将结构体转换为 map，便于统一处理配置项
	cs := utils.StructToMap(system.Config)
	// 遍历配置项，批量设置到全局配置管理器
	for k, v := range cs {
		global.GVA_VP.Set(k, v)
	}
	// 将配置持久化到配置文件
	err = global.GVA_VP.WriteConfig()
	return err
}

//@author: [SliverHorn](https://github.com/SliverHorn)
//@function: GetServerInfo
//@description: 获取服务器信息
//@return: server *utils.Server, err error

// GetServerInfo 获取服务器信息（操作系统、CPU、内存、磁盘信息）
// 设计思路：
//  1. 分步骤初始化：分别初始化操作系统、CPU、内存、磁盘信息，每个步骤独立
//     好处：即使某个步骤失败，也能返回已成功获取的部分信息，不会完全失败
//  2. 提前返回错误处理模式：每个初始化步骤后立即检查错误，一旦失败立即返回
//     好处：避免继续执行后续可能也会失败的操作，提高效率；同时便于定位具体是哪个步骤出错
//  3. 错误日志记录：每次初始化失败时都记录详细的错误日志
//     好处：便于问题排查和监控，运维人员可以通过日志快速定位服务器信息获取失败的原因
//  4. 返回指针而非值：返回 &s 而非 s，避免大结构体的值拷贝，提高性能
//     好处：Server 结构体可能包含较多数据，返回指针可以避免不必要的内存拷贝
//  5. 部分失败处理：即使某个组件初始化失败，也返回已初始化的 Server 对象
//     好处：调用方可以根据需要处理部分信息，而不是完全无法获取服务器信息
func (systemConfigService *SystemConfigService) GetServerInfo() (server *utils.Server, err error) {
	var s utils.Server
	// 初始化操作系统信息（通常不会失败，无需错误检查）
	s.Os = utils.InitOS()
	// 初始化CPU信息，如果失败则记录日志并返回
	if s.Cpu, err = utils.InitCPU(); err != nil {
		global.GVA_LOG.Error("func utils.InitCPU() Failed", zap.String("err", err.Error()))
		return &s, err
	}
	// 初始化内存信息，如果失败则记录日志并返回
	if s.Ram, err = utils.InitRAM(); err != nil {
		global.GVA_LOG.Error("func utils.InitRAM() Failed", zap.String("err", err.Error()))
		return &s, err
	}
	// 初始化磁盘信息，如果失败则记录日志并返回
	if s.Disk, err = utils.InitDisk(); err != nil {
		global.GVA_LOG.Error("func utils.InitDisk() Failed", zap.String("err", err.Error()))
		return &s, err
	}

	return &s, nil
}
