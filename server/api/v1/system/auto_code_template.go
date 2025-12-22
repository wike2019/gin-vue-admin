package system

// 自动代码生成模板API层
// 设计说明：
// 1. 预览功能：提供预览功能，生成代码前可以先查看
// 2. 参数预处理：使用Pretreatment方法预处理参数，统一处理逻辑
// 3. 参数验证：使用AutoCodeVerify验证规则，保证参数有效性
// 4. 方法注入：支持动态添加方法到已有代码中
// 5. 好处：功能完整、参数验证、支持预览

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AutoCodeTemplateApi 自动代码模板API结构体
type AutoCodeTemplateApi struct{}

// Preview 预览自动生成的代码
// 设计说明：
// 1. 预览功能：生成代码前先预览，确认无误后再创建
// 2. 参数验证：使用AutoCodeVerify验证规则，保证参数有效性
// 3. 参数预处理：使用Pretreatment方法统一处理参数，如包名转换等
// 4. 包名转换：使用FirstUpper将包名首字母大写，符合Go命名规范
// 5. 好处：避免错误生成、参数规范、用户体验好
// @Tags      AutoCodeTemplate
// @Summary   预览创建后的代码
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.AutoCode                                      true  "预览创建代码"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "预览创建后的代码"
// @Router    /autoCode/preview [post]
func (a *AutoCodeTemplateApi) Preview(c *gin.Context) {
	var info request.AutoCode
	err := c.ShouldBindJSON(&info)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 使用统一的验证规则，保证参数有效性
	err = utils.Verify(info, utils.AutoCodeVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 参数预处理：统一处理参数格式，如包名、路径等
	err = info.Pretreatment()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 包名首字母大写，符合Go命名规范
	info.PackageT = utils.FirstUpper(info.Package)
	autoCode, err := autoCodeTemplateService.Preview(c.Request.Context(), info)
	if err != nil {
		global.GVA_LOG.Error(err.Error(), zap.Error(err))
		response.FailWithMessage("预览失败:"+err.Error(), c)
	} else {
		response.OkWithDetailed(gin.H{"autoCode": autoCode}, "预览成功", c)
	}
}

// Create
// @Tags      AutoCodeTemplate
// @Summary   自动代码模板
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.AutoCode  true  "创建自动代码"
// @Success   200   {string}  string                 "{"success":true,"data":{},"msg":"创建成功"}"
// @Router    /autoCode/createTemp [post]
func (a *AutoCodeTemplateApi) Create(c *gin.Context) {
	var info request.AutoCode
	err := c.ShouldBindJSON(&info)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(info, utils.AutoCodeVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = info.Pretreatment()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = autoCodeTemplateService.Create(c.Request.Context(), info)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
	} else {
		response.OkWithMessage("创建成功", c)
	}
}

// AddFunc 向已有代码中添加方法
// 设计说明：
// 1. 预览模式：支持预览模式，可以先查看生成的代码再决定是否添加
// 2. 占位符填充：预览模式下使用占位符填充，不影响实际代码
// 3. 代码注入：将新方法注入到已有的API和Service文件中
// 4. 好处：支持预览、代码注入、功能扩展
// @Tags      AddFunc
// @Summary   增加方法
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.AutoCode  true  "增加方法"
// @Success   200   {string}  string                 "{"success":true,"data":{},"msg":"创建成功"}"
// @Router    /autoCode/addFunc [post]
func (a *AutoCodeTemplateApi) AddFunc(c *gin.Context) {
	var info request.AutoFunc
	err := c.ShouldBindJSON(&info)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	var tempMap map[string]string
	if info.IsPreview {
		// 预览模式：使用占位符填充，不实际修改代码
		// 好处：可以先查看效果，再决定是否真正添加
		info.Router = "填充router"
		info.FuncName = "填充funcName"
		info.Method = "填充method"
		info.Description = "填充description"
		tempMap, err = autoCodeTemplateService.GetApiAndServer(info)
	} else {
		// 实际添加：将方法注入到代码中
		err = autoCodeTemplateService.AddFunc(info)
	}
	if err != nil {
		global.GVA_LOG.Error("注入失败!", zap.Error(err))
		response.FailWithMessage("注入失败", c)
	} else {
		if info.IsPreview {
			// 预览模式返回生成的代码预览
			response.OkWithDetailed(tempMap, "注入成功", c)
			return
		}
		response.OkWithMessage("注入成功", c)
	}
}
