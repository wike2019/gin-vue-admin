package core

import (
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/initialize"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"go.uber.org/zap"
	"time"
)

// RunServer 启动 HTTP 服务器
//
// 设计思路：
// 此函数负责在系统初始化完成后启动 Web 服务器
// 按照依赖关系顺序初始化可选组件（Redis、MongoDB），然后启动 HTTP 服务
//
// 为什么这样设计：
// 1. 条件初始化：Redis、MongoDB 是可选的，根据配置决定是否初始化
//    - 好处：不需要这些组件时不会浪费资源
//    - 灵活性：不同环境可以使用不同的技术栈
// 2. 错误容忍：MongoDB 初始化失败只记录日志，不中断服务
//    - 好处：即使 MongoDB 不可用，其他功能仍可正常使用
//    - 容错性：提高系统的健壮性
// 3. 数据预加载：启动时加载 JWT 黑名单等数据到内存
//    - 好处：提高运行时性能，避免频繁查询数据库
//    - 一致性：确保启动时数据已就绪
func RunServer() {
	// 第一步：条件初始化 Redis（如果配置启用）
	// 为什么使用条件判断？
	// - Redis 是可选的，不是所有环境都需要（开发环境可能不需要）
	// - 避免不必要的资源消耗和连接开销
	// - 支持渐进式部署，可以先不使用 Redis，后续再启用
	if global.GVA_CONFIG.System.UseRedis {
		// 初始化redis服务
		initialize.Redis()
		// 如果启用多点登录（多设备登录控制），初始化 Redis 列表
		// 多点登录需要 Redis 存储每个用户的登录设备信息
		if global.GVA_CONFIG.System.UseMultipoint {
			initialize.RedisList()
		}
	}

	// 第二步：条件初始化 MongoDB（如果配置启用）
	// 为什么只记录错误而不中断服务？
	// - MongoDB 可能是可选的，某些功能依赖它，但不是核心功能
	// - 即使 MongoDB 初始化失败，HTTP 服务仍可正常启动
	// - 错误已记录到日志，运维人员可以及时发现并处理
	if global.GVA_CONFIG.System.UseMongo {
		err := initialize.Mongo.Initialization()
		if err != nil {
			zap.L().Error(fmt.Sprintf("%+v", err))
		}
	}

	// 第三步：从数据库预加载系统数据到内存
	// 为什么在启动时加载？
	// - JWT 黑名单、API 列表等数据需要频繁查询
	// - 启动时加载到内存，避免每次请求都查询数据库
	// - 提高响应速度，减少数据库压力
	// 为什么需要判断数据库是否为空？
	// - 如果数据库连接失败，跳过数据加载避免 panic
	// - 某些测试场景可能不需要数据库
	if global.GVA_DB != nil {
		system.LoadAll()
	}

	// 第四步：初始化路由
	// 路由初始化必须在所有依赖组件就绪后进行
	// 因为路由注册可能依赖数据库、Redis 等组件
	Router := initialize.Routers()

	// 第五步：构建服务器监听地址
	// 使用 fmt.Sprintf 格式化地址字符串，支持配置化的端口号
	// 好处：不同环境可以使用不同端口，通过配置文件统一管理
	address := fmt.Sprintf(":%d", global.GVA_CONFIG.System.Addr)

	// 第六步：打印启动信息
	// 为什么打印这些信息？
	// - 方便开发者和运维人员快速了解服务状态
	// - 提供关键访问地址（Swagger、MCP、前端）
	// - 版权声明确保合规使用
	fmt.Printf(`
	欢迎使用 gin-vue-admin
	当前版本:%s
	加群方式:微信号：shouzi_1994 QQ群：470239250
	项目地址：https://github.com/flipped-aurora/gin-vue-admin
	插件市场:https://plugin.gin-vue-admin.com
	GVA讨论社区:https://support.qq.com/products/371961
	默认自动化文档地址:http://127.0.0.1%s/swagger/index.html
	默认MCP SSE地址:http://127.0.0.1%s%s
	默认MCP Message地址:http://127.0.0.1%s%s
	默认前端文件运行地址:http://127.0.0.1:8080
	--------------------------------------版权声明--------------------------------------
	** 版权所有方：flipped-aurora开源团队 **
	** 版权持有公司：北京翻转极光科技有限责任公司 **
	** 剔除授权标识需购买商用授权：https://gin-vue-admin.com/empower/index.html **
	** 感谢您对Gin-Vue-Admin的支持与关注 合法授权使用更有利于项目的长久发展**
`, global.Version, address, address, global.GVA_CONFIG.MCP.SSEPath, address, global.GVA_CONFIG.MCP.MessagePath)

	// 第七步：启动 HTTP 服务器
	// 为什么设置读写超时时间为 10 分钟？
	// - ReadTimeout：防止客户端长时间不发送数据占用连接
	// - WriteTimeout：防止响应时间过长导致连接超时
	// - 10 分钟是一个合理的值，既能处理大文件上传，又能防止资源耗尽
	// - 优雅关闭机制确保正在处理的请求能够完成
	initServer(address, Router, 10*time.Minute, 10*time.Minute)
}
