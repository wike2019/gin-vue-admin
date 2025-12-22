package system

// 系统配置管理API层
// 设计说明：
// 1. 采用分层架构：API层只负责请求处理、参数校验和响应封装，业务逻辑在Service层
// 2. 统一错误处理：所有错误都通过response统一返回，保证API响应格式一致性
// 3. 使用空结构体作为接收者：Go语言最佳实践，避免不必要的内存分配
// 4. 好处：职责清晰、易于测试、便于维护、符合RESTful设计原则

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SystemApi 系统配置API结构体
// 使用空结构体作为接收者，避免内存浪费
// 所有方法都挂载在此结构体上，便于统一管理和路由注册
type SystemApi struct{}

// GetSystemConfig 获取系统配置
// 设计说明：
// 1. 使用POST而非GET：虽然语义上是查询，但使用POST可以避免URL参数过长，且更安全
// 2. 错误处理模式：先记录日志再返回错误，便于问题追踪和调试
// 3. 使用专门的响应结构体：systemRes.SysConfigResponse封装返回数据，保证数据结构清晰
// 4. 好处：统一日志记录、便于问题排查、响应格式规范
// @Tags      System
// @Summary   获取配置文件内容
// @Security  ApiKeyAuth
// @Produce   application/json
// @Success   200  {object}  response.Response{data=systemRes.SysConfigResponse,msg=string}  "获取配置文件内容,返回包括系统配置"
// @Router    /system/getSystemConfig [post]
func (s *SystemApi) GetSystemConfig(c *gin.Context) {
	// 调用Service层获取配置，API层不处理业务逻辑
	config, err := systemConfigService.GetSystemConfig()
	if err != nil {
		// 先记录错误日志，再返回错误响应
		// 这样设计的好处：日志系统可以记录详细错误信息，而用户只看到友好提示
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	// 使用专门的响应结构体封装数据，保证返回格式统一
	response.OkWithDetailed(systemRes.SysConfigResponse{Config: config}, "获取成功", c)
}

// SetSystemConfig 设置系统配置
// 设计说明：
// 1. 参数绑定：使用ShouldBindJSON自动将JSON请求体绑定到结构体，减少手动解析代码
// 2. 参数校验：Gin框架自动进行类型校验，无效数据会在绑定阶段返回错误
// 3. 错误分层：参数错误直接返回，业务错误记录日志后返回
// 4. 好处：代码简洁、类型安全、错误信息明确
// @Tags      System
// @Summary   设置配置文件内容
// @Security  ApiKeyAuth
// @Produce   application/json
// @Param     data  body      system.System                   true  "设置配置文件内容"
// @Success   200   {object}  response.Response{data=string}  "设置配置文件内容"
// @Router    /system/setSystemConfig [post]
func (s *SystemApi) SetSystemConfig(c *gin.Context) {
	var sys system.System
	// 使用Gin的ShouldBindJSON自动绑定JSON请求体到结构体
	// 好处：自动类型转换、自动校验、代码简洁
	err := c.ShouldBindJSON(&sys)
	if err != nil {
		// 参数绑定错误直接返回，不需要记录日志（通常是客户端问题）
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 调用Service层处理业务逻辑
	err = systemConfigService.SetSystemConfig(sys)
	if err != nil {
		// 业务错误需要记录日志，便于排查问题
		global.GVA_LOG.Error("设置失败!", zap.Error(err))
		response.FailWithMessage("设置失败", c)
		return
	}
	response.OkWithMessage("设置成功", c)
}

// ReloadSystem 重载系统配置
// 设计说明：
// 1. 事件驱动模式：使用事件机制触发系统重载，解耦重载逻辑
// 2. 全局事件管理器：utils.GlobalSystemEvents统一管理系统事件，便于扩展和维护
// 3. 错误信息包含详情：将错误信息附加到响应中，便于前端展示和调试
// 4. 好处：解耦、可扩展、错误信息完整
// @Tags      System
// @Summary   重载系统
// @Security  ApiKeyAuth
// @Produce   application/json
// @Success   200  {object}  response.Response{msg=string}  "重载系统"
// @Router    /system/reloadSystem [post]
func (s *SystemApi) ReloadSystem(c *gin.Context) {
	// 使用事件驱动模式触发系统重载
	// 好处：解耦重载逻辑，支持多个监听者，便于扩展
	err := utils.GlobalSystemEvents.TriggerReload()
	if err != nil {
		global.GVA_LOG.Error("重载系统失败!", zap.Error(err))
		// 将详细错误信息返回给前端，便于调试和问题定位
		response.FailWithMessage("重载系统失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("重载系统成功", c)
}

// GetServerInfo 获取服务器信息
// 设计说明：
// 1. 使用gin.H构建响应：对于简单的键值对响应，gin.H比定义结构体更灵活
// 2. 统一错误处理：遵循统一的错误处理模式，保证代码一致性
// 3. 好处：代码简洁、响应灵活、易于扩展
// @Tags      System
// @Summary   获取服务器信息
// @Security  ApiKeyAuth
// @Produce   application/json
// @Success   200  {object}  response.Response{data=map[string]interface{},msg=string}  "获取服务器信息"
// @Router    /system/getServerInfo [post]
func (s *SystemApi) GetServerInfo(c *gin.Context) {
	server, err := systemConfigService.GetServerInfo()
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	// 使用gin.H构建简单的响应结构，比定义专门的结构体更灵活
	// 适用于数据结构不固定或临时响应的场景
	response.OkWithDetailed(gin.H{"server": server}, "获取成功", c)
}
