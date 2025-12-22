package main

import (
	"github.com/flipped-aurora/gin-vue-admin/server/core"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/initialize"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
)

//go:generate go env -w GO111MODULE=on
//go:generate go env -w GOPROXY=https://goproxy.cn,direct
//go:generate go mod tidy
//go:generate go mod download

// 这部分 @Tag 设置用于排序, 需要排序的接口请按照下面的格式添加
// swag init 对 @Tag 只会从入口文件解析, 默认 main.go
// 也可通过 --generalInfo flag 指定其他文件
// @Tag.Name        Base
// @Tag.Name        SysUser
// @Tag.Description 用户

// @title                       Gin-Vue-Admin Swagger API接口文档
// @version                     v2.8.7
// @description                 使用gin+vue进行极速开发的全栈开发基础平台
// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        x-token
// @BasePath                    /
func main() {
	// 初始化系统
	// 为什么先初始化再启动服务器？
	// 1. 依赖顺序：配置 -> 日志 -> 数据库 -> 路由，必须按顺序初始化
	// 2. 错误处理：初始化失败可以在启动前发现，避免启动后才发现配置错误
	// 3. 资源准备：确保所有依赖（数据库、Redis、定时任务等）就绪后再接受请求
	initializeSystem()
	// 运行服务器
	// 服务器启动必须在所有初始化完成后执行，确保服务启动时所有组件都已就绪
	core.RunServer()
}

// initializeSystem 初始化系统所有组件
//
// 设计思路：
// 将初始化逻辑提取为独立函数，而不是直接写在 main() 中
// 好处：
// 1. 代码复用：系统重载（reload）时可以复用此函数，无需重复代码
// 2. 可测试性：可以单独测试初始化逻辑，便于单元测试
// 3. 可维护性：初始化逻辑集中管理，修改时只需改一处
// 4. 清晰性：main() 函数保持简洁，只关注程序流程
//
// 初始化顺序的重要性（必须严格按照此顺序）：
// 1. Viper配置 -> 2. 其他初始化 -> 3. 日志 -> 4. 数据库 -> 5. 定时任务 -> 6. 表注册
// 原因：
// - 配置必须先加载，因为后续组件都需要读取配置
// - 日志必须在配置之后，因为日志系统需要读取日志配置
// - 数据库连接需要配置和日志都已就绪
// - 定时任务和表注册依赖数据库连接
func initializeSystem() {
	// 第一步：初始化配置文件管理器 Viper
	// 为什么最先初始化配置？
	// - 所有后续组件（日志、数据库、Redis等）都需要读取配置
	// - 配置错误应该在启动时立即发现，而不是运行时才发现
	// - 支持配置文件热更新，修改配置后自动重新加载
	global.GVA_VP = core.Viper() // 初始化Viper

	// 第二步：其他初始化（可能包括一些基础设置）
	// 在日志和数据库之前执行一些不依赖这些组件的初始化
	initialize.OtherInit()

	// 第三步：初始化日志系统 Zap
	// 为什么在配置之后、数据库之前？
	// - 日志系统需要读取日志配置（日志级别、输出路径等）
	// - 数据库连接可能产生日志，需要日志系统先就绪
	// - 后续所有组件的错误都需要通过日志记录
	global.GVA_LOG = core.Zap() // 初始化zap日志库

	// 替换全局日志实例
	// 为什么使用 ReplaceGlobals？
	// - 让项目中所有使用 zap.L() 的地方都能使用统一的日志实例
	// - 避免传递日志实例，简化代码
	// - 确保整个应用使用相同的日志配置
	zap.ReplaceGlobals(global.GVA_LOG)

	// 第四步：初始化数据库连接 GORM
	// 为什么在日志之后？
	// - 数据库连接可能失败，需要日志记录错误信息
	// - 数据库操作需要记录日志（慢查询、错误等）
	global.GVA_DB = initialize.Gorm() // gorm连接数据库

	// 第五步：初始化定时任务
	// 定时任务可能需要访问数据库，所以放在数据库初始化之后
	initialize.Timer()

	// 第六步：初始化数据库列表（多数据库支持）
	// 如果系统需要连接多个数据库，在这里初始化
	initialize.DBList()

	// 第七步：注册全局函数和系统事件处理器
	// 设置系统重载、关闭等全局事件的处理函数
	initialize.SetupHandlers() // 注册全局函数

	// 第八步：注册数据库表（自动迁移）
	// 为什么放在最后且需要判断数据库是否为空？
	// - 表注册依赖数据库连接，必须确保数据库已连接成功
	// - 如果数据库连接失败（GVA_DB == nil），跳过表注册避免 panic
	// - 表注册是可选操作，某些场景下可能不需要自动迁移
	if global.GVA_DB != nil {
		initialize.RegisterTables() // 初始化表
	}
}
