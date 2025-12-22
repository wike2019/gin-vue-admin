package system

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"go.uber.org/zap"
)

// 验证码存储配置
// 设计说明：
// 1. 单机模式：默认使用内存存储，适合单机部署
// 2. 集群模式：注释中提供了Redis存储方案，支持多服务器部署
// 3. 灵活切换：通过注释切换存储方式，代码修改简单
// 4. 好处：部署灵活、性能好、易于扩展
// 当开启多服务器部署时，替换下面的配置，使用redis共享存储验证码
// var store = captcha.NewDefaultRedisStore()
var store = base64Captcha.DefaultMemStore

// BaseApi 基础API结构体（验证码相关）
type BaseApi struct{}

// Captcha 生成验证码
// 设计说明：
// 1. 防暴力破解：根据IP记录请求次数，超过阈值才显示验证码
// 2. 动态开启：根据请求频率动态决定是否开启验证码，平衡安全性和用户体验
// 3. Base64编码：返回Base64编码的图片，前端可以直接显示
// 4. 配置化：验证码参数从配置文件读取，便于调整
// 5. 好处：安全性高、用户体验好、配置灵活
// @Tags      Base
// @Summary   生成验证码
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200  {object}  response.Response{data=systemRes.SysCaptchaResponse,msg=string}  "生成验证码,返回包括随机数id,base64,验证码长度,是否开启验证码"
// @Router    /base/captcha [post]
func (b *BaseApi) Captcha(c *gin.Context) {
	// 防暴力破解机制：根据IP记录请求次数
	openCaptcha := global.GVA_CONFIG.Captcha.OpenCaptcha               // 防爆阈值
	openCaptchaTimeOut := global.GVA_CONFIG.Captcha.OpenCaptchaTimeOut // 缓存超时时间
	key := c.ClientIP()
	v, ok := global.BlackCache.Get(key)
	if !ok {
		// 首次请求，设置初始计数
		global.BlackCache.Set(key, 1, time.Second*time.Duration(openCaptchaTimeOut))
	}

	// 判断是否需要显示验证码
	// 如果未设置阈值或请求次数超过阈值，则显示验证码
	var oc bool
	if openCaptcha == 0 || openCaptcha < interfaceToInt(v) {
		oc = true
	}
	
	// 创建数字验证码驱动，参数从配置读取
	driver := base64Captcha.NewDriverDigit(global.GVA_CONFIG.Captcha.ImgHeight, global.GVA_CONFIG.Captcha.ImgWidth, global.GVA_CONFIG.Captcha.KeyLong, 0.7, 80)
	// cp := base64Captcha.NewCaptcha(driver, store.UseWithCtx(c))   // v8下使用redis
	cp := base64Captcha.NewCaptcha(driver, store)
	id, b64s, _, err := cp.Generate()
	if err != nil {
		global.GVA_LOG.Error("验证码获取失败!", zap.Error(err))
		response.FailWithMessage("验证码获取失败", c)
		return
	}
	// 返回验证码ID、Base64图片、长度和是否开启标志
	response.OkWithDetailed(systemRes.SysCaptchaResponse{
		CaptchaId:     id,
		PicPath:       b64s,
		CaptchaLength: global.GVA_CONFIG.Captcha.KeyLong,
		OpenCaptcha:   oc,
	}, "验证码获取成功", c)
}

// interfaceToInt 将interface{}转换为int
// 设计说明：
// 1. 类型安全：使用类型断言确保类型安全
// 2. 默认值：无法转换时返回0，避免panic
// 3. 好处：类型安全、不会panic、代码简洁
func interfaceToInt(v interface{}) (i int) {
	switch v := v.(type) {
	case int:
		i = v
	default:
		i = 0
	}
	return
}
