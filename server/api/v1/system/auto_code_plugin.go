package system

import (
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AutoCodePluginApi struct{}

// Install 安装插件
// 设计说明：
// 1. 文件上传：使用multipart/form-data接收文件
// 2. 分离安装：web和server分别安装，返回各自的安装结果
// 3. 友好提示：根据安装结果返回不同的提示信息
// 4. 状态码：使用-1表示安装失败，便于前端判断
// 5. 好处：安装灵活、提示清晰、支持分离部署
// @Tags      AutoCodePlugin
// @Summary   安装插件
// @Security  ApiKeyAuth
// @accept    multipart/form-data
// @Produce   application/json
// @Param     plug  formData  file                                              true  "this is a test file"
// @Success   200   {object}  response.Response{data=[]interface{},msg=string}  "安装插件成功"
// @Router    /autoCode/installPlugin [post]
func (a *AutoCodePluginApi) Install(c *gin.Context) {
	header, err := c.FormFile("plug")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// Service层返回web和server的安装结果
	web, server, err := autoCodePluginService.Install(header)
	webStr := "web插件安装成功"
	serverStr := "server插件安装成功"
	// 根据安装结果设置不同的提示信息
	if web == -1 {
		webStr = "web端插件未成功安装，请按照文档自行解压安装，如果为纯后端插件请忽略此条提示"
	}
	if server == -1 {
		serverStr = "server端插件未成功安装，请按照文档自行解压安装，如果为纯前端插件请忽略此条提示"
	}
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 返回web和server的安装结果，前端可以分别处理
	response.OkWithData([]interface{}{
		gin.H{
			"code": web,
			"msg":  webStr,
		},
		gin.H{
			"code": server,
			"msg":  serverStr,
		}}, c)
}

// Packaged
// @Tags      AutoCodePlugin
// @Summary   打包插件
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     plugName  query    string  true  "插件名称"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "打包插件成功"
// @Router    /autoCode/pubPlug [post]
func (a *AutoCodePluginApi) Packaged(c *gin.Context) {
	plugName := c.Query("plugName")
	zipPath, err := autoCodePluginService.PubPlug(plugName)
	if err != nil {
		global.GVA_LOG.Error("打包失败!", zap.Error(err))
		response.FailWithMessage("打包失败"+err.Error(), c)
		return
	}
	response.OkWithMessage(fmt.Sprintf("打包成功,文件路径为:%s", zipPath), c)
}

// InitMenu
// @Tags      AutoCodePlugin
// @Summary   打包插件
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "打包插件成功"
// @Router    /autoCode/initMenu [post]
func (a *AutoCodePluginApi) InitMenu(c *gin.Context) {
	var menuInfo request.InitMenu
	err := c.ShouldBindJSON(&menuInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = autoCodePluginService.InitMenu(menuInfo)
	if err != nil {
		global.GVA_LOG.Error("创建初始化Menu失败!", zap.Error(err))
		response.FailWithMessage("创建初始化Menu失败"+err.Error(), c)
		return
	}
	response.OkWithMessage("文件变更成功", c)
}

// InitAPI
// @Tags      AutoCodePlugin
// @Summary   打包插件
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "打包插件成功"
// @Router    /autoCode/initAPI [post]
func (a *AutoCodePluginApi) InitAPI(c *gin.Context) {
	var apiInfo request.InitApi
	err := c.ShouldBindJSON(&apiInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = autoCodePluginService.InitAPI(apiInfo)
	if err != nil {
		global.GVA_LOG.Error("创建初始化API失败!", zap.Error(err))
		response.FailWithMessage("创建初始化API失败"+err.Error(), c)
		return
	}
	response.OkWithMessage("文件变更成功", c)
}
