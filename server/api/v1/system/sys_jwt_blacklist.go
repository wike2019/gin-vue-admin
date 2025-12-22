package system

// JWT黑名单管理API层
// 设计说明：
// 1. 单一职责：只负责JWT加入黑名单的操作，职责清晰
// 2. 安全设计：登出时将token加入黑名单，防止token被继续使用
// 3. 清理token：操作成功后清除客户端token，双重保障
// 4. 好处：安全性高、职责清晰、操作完整

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// JwtApi JWT管理API结构体
type JwtApi struct{}

// JsonInBlacklist 将JWT加入黑名单（登出功能）
// 设计说明：
// 1. 双重安全机制：先加入黑名单，再清除客户端token，确保token失效
// 2. 工具函数封装：使用utils.GetToken和utils.ClearToken，代码复用性好
// 3. 操作顺序：先服务端处理，再客户端清理，保证数据一致性
// 4. 好处：安全性高、代码复用、操作可靠
// @Tags      Jwt
// @Summary   jwt加入黑名单
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200  {object}  response.Response{msg=string}  "jwt加入黑名单"
// @Router    /jwt/jsonInBlacklist [post]
func (j *JwtApi) JsonInBlacklist(c *gin.Context) {
	// 从请求中提取token，使用工具函数统一处理
	token := utils.GetToken(c)
	jwt := system.JwtBlacklist{Jwt: token}
	// 先将token加入黑名单，服务端记录失效token
	err := jwtService.JsonInBlacklist(jwt)
	if err != nil {
		global.GVA_LOG.Error("jwt作废失败!", zap.Error(err))
		response.FailWithMessage("jwt作废失败", c)
		return
	}
	// 清除客户端token，双重保障
	// 好处：即使客户端token未清除，服务端黑名单也能防止token被使用
	utils.ClearToken(c)
	response.OkWithMessage("jwt作废成功", c)
}
