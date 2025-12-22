package system

// 数据库初始化API层
// 设计说明：
// 1. 初始化检查：提供检查接口，判断是否需要初始化数据库
// 2. 安全防护：检查是否已存在数据库配置，防止重复初始化
// 3. 错误提示：提供详细的错误信息，引导用户正确操作
// 4. 好处：安全性高、用户体验好、操作引导清晰

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

// DBApi 数据库初始化API结构体
type DBApi struct{}

// InitDB 初始化数据库
// 设计说明：
// 1. 安全检查：先检查是否已存在数据库配置，防止重复初始化
// 2. 详细错误提示：提供明确的错误信息和操作建议
// 3. 自动创建：支持自动创建数据库，简化部署流程
// 4. 好处：安全性高、操作简便、错误提示清晰
// @Tags     InitDB
// @Summary  初始化用户数据库
// @Produce  application/json
// @Param    data  body      request.InitDB                  true  "初始化数据库参数"
// @Success  200   {object}  response.Response{data=string}  "初始化用户数据库"
// @Router   /init/initdb [post]
func (i *DBApi) InitDB(c *gin.Context) {
	// 安全检查：防止重复初始化数据库
	// 好处：避免覆盖已有数据，保证数据安全
	if global.GVA_DB != nil {
		global.GVA_LOG.Error("已存在数据库配置!")
		response.FailWithMessage("已存在数据库配置", c)
		return
	}
	var dbInfo request.InitDB
	if err := c.ShouldBindJSON(&dbInfo); err != nil {
		global.GVA_LOG.Error("参数校验不通过!", zap.Error(err))
		response.FailWithMessage("参数校验不通过", c)
		return
	}
	// 调用Service层执行数据库初始化
	if err := initDBService.InitDB(dbInfo); err != nil {
		global.GVA_LOG.Error("自动创建数据库失败!", zap.Error(err))
		// 提供详细的错误提示和操作建议
		response.FailWithMessage("自动创建数据库失败，请查看后台日志，检查后在进行初始化", c)
		return
	}
	response.OkWithMessage("自动创建数据库成功", c)
}

// CheckDB 检查数据库是否需要初始化
// 设计说明：
// 1. 状态检查：检查数据库配置是否存在，返回初始化状态
// 2. 前端引导：返回needInit标志，前端可以根据此标志显示相应界面
// 3. 默认值设置：使用变量默认值，代码更清晰
// 4. 好处：前端友好、状态明确、代码简洁
// @Tags     CheckDB
// @Summary  初始化用户数据库
// @Produce  application/json
// @Success  200  {object}  response.Response{data=map[string]interface{},msg=string}  "初始化用户数据库"
// @Router   /init/checkdb [post]
func (i *DBApi) CheckDB(c *gin.Context) {
	// 设置默认值，假设需要初始化
	var (
		message  = "前往初始化数据库"
		needInit = true
	)

	// 如果数据库已配置，则不需要初始化
	if global.GVA_DB != nil {
		message = "数据库无需初始化"
		needInit = false
	}
	global.GVA_LOG.Info(message)
	// 返回初始化状态，前端可以根据needInit显示相应界面
	response.OkWithDetailed(gin.H{"needInit": needInit}, message, c)
}
