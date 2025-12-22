package system

import (
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	systemRes "github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Login 用户登录接口
// 设计要点：
// 1. 验证码防护：防止暴力破解，支持防爆次数配置
// 2. 密码验证：验证用户名和密码
// 3. 用户状态检查：检查用户是否被禁用
// 4. JWT 签发：登录成功后签发 JWT token
// 5. 多点登录：支持单点登录和多点登录两种模式
//
// 为什么这么写：
// - 验证码机制：通过 BlackCache 记录失败次数，超过阈值后要求验证码
// - 分层验证：先验证验证码，再验证用户信息，提前失败
// - 状态检查：登录前检查用户状态，避免无效登录
// - Token 管理：根据配置决定是否支持多点登录
//
// 安全考虑：
// - 验证码：防止自动化攻击
// - 失败计数：记录失败次数，超过阈值后要求验证码
// - 密码加密：密码在传输和存储时都加密
// - Token 过期：JWT token 有过期时间，提高安全性
//
// @Tags     Base
// @Summary  用户登录
// @Produce   application/json
// @Param    data  body      systemReq.Login                                             true  "用户名, 密码, 验证码"
// @Success  200   {object}  response.Response{data=systemRes.LoginResponse,msg=string}  "返回包括用户信息,token,过期时间"
// @Router   /base/login [post]
func (b *BaseApi) Login(c *gin.Context) {
	var l systemReq.Login
	err := c.ShouldBindJSON(&l)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	// 验证登录参数（用户名、密码格式等）
	err = utils.Verify(l, utils.LoginVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	// 获取客户端 IP，用于防爆机制
	// 使用 IP 作为 key，可以防止同一 IP 的暴力破解
	key := c.ClientIP()
	
	// 验证码防爆机制
	// openCaptcha: 防爆阈值，超过此次数后要求验证码
	// openCaptchaTimeOut: 缓存超时时间，超过此时间后重置计数
	openCaptcha := global.GVA_CONFIG.Captcha.OpenCaptcha               // 是否开启防爆次数
	openCaptchaTimeOut := global.GVA_CONFIG.Captcha.OpenCaptchaTimeOut // 缓存超时时间
	
	// 获取该 IP 的失败次数
	v, ok := global.BlackCache.Get(key)
	if !ok {
		// 如果不存在，初始化为 1（当前这次失败）
		global.BlackCache.Set(key, 1, time.Second*time.Duration(openCaptchaTimeOut))
	}

	// 判断是否需要验证码
	// oc = true 表示需要验证码（失败次数超过阈值或配置为 0 表示总是需要）
	var oc bool = openCaptcha == 0 || openCaptcha < interfaceToInt(v)
	
	// 如果需要验证码，则验证验证码
	// 验证码验证包括：
	// 1. 验证码不能为空
	// 2. 验证码 ID 不能为空
	// 3. 验证码值必须正确
	if oc && (l.Captcha == "" || l.CaptchaId == "" || !store.Verify(l.CaptchaId, l.Captcha, true)) {
		// 验证码错误，失败次数+1
		global.BlackCache.Increment(key, 1)
		response.FailWithMessage("验证码错误", c)
		return
	}

	// 验证用户名和密码
	// service 层会：
	// 1. 根据用户名查找用户
	// 2. 验证密码（使用 bcrypt 等加密算法）
	// 3. 返回用户信息（不包含密码）
	u := &system.SysUser{Username: l.Username, Password: l.Password}
	user, err := userService.Login(u)
	if err != nil {
		global.GVA_LOG.Error("登陆失败! 用户名不存在或者密码错误!", zap.Error(err))
		// 登录失败，失败次数+1
		global.BlackCache.Increment(key, 1)
		// 返回通用错误信息，不暴露具体是用户名错误还是密码错误
		// 这样可以防止用户名枚举攻击
		response.FailWithMessage("用户名不存在或者密码错误", c)
		return
	}
	
	// 检查用户是否被禁用
	// Enable = 1 表示启用，Enable != 1 表示禁用
	if user.Enable != 1 {
		global.GVA_LOG.Error("登陆失败! 用户被禁止登录!")
		// 即使密码正确，如果用户被禁用，也算作失败
		global.BlackCache.Increment(key, 1)
		response.FailWithMessage("用户被禁止登录", c)
		return
	}
	
	// 登录成功，签发 JWT token
	b.TokenNext(c, *user)
}

// TokenNext 登录成功后签发 JWT token
// 设计要点：
// 1. 单点登录 vs 多点登录：根据配置决定是否支持多点登录
// 2. Token 管理：将 token 存储到 Redis，支持 token 黑名单
// 3. 过期时间：返回 token 的过期时间，前端可以根据此时间刷新 token
//
// 为什么这么写：
// - 单点登录：同一用户只能在一个地方登录，新登录会踢掉旧登录
// - 多点登录：同一用户可以在多个地方同时登录
// - Redis 存储：将 token 存储到 Redis，便于管理和撤销
// - 黑名单机制：支持 token 撤销，提高安全性
//
// 工作流程（多点登录）：
// 1. 检查 Redis 中是否已有该用户的 token
// 2. 如果有，将旧 token 加入黑名单
// 3. 生成新 token 并存储到 Redis
// 4. 返回新 token 和用户信息
func (b *BaseApi) TokenNext(c *gin.Context, user system.SysUser) {
	// 生成 JWT token
	// LoginToken 会：
	// 1. 生成包含用户信息的 JWT token
	// 2. 设置过期时间
	// 3. 返回 token、claims 和错误
	token, claims, err := utils.LoginToken(&user)
	if err != nil {
		global.GVA_LOG.Error("获取token失败!", zap.Error(err))
		response.FailWithMessage("获取token失败", c)
		return
	}
	
	// 单点登录模式：不检查 Redis，直接返回 token
	// 这种模式简单，但不支持 token 撤销和强制下线
	if !global.GVA_CONFIG.System.UseMultipoint {
		// 设置 cookie（可选）
		utils.SetToken(c, token, int(claims.RegisteredClaims.ExpiresAt.Unix()-time.Now().Unix()))
		// 返回用户信息、token 和过期时间
		// ExpiresAt 转换为毫秒时间戳，便于前端使用
		response.OkWithDetailed(systemRes.LoginResponse{
			User:      user,
			Token:     token,
			ExpiresAt: claims.RegisteredClaims.ExpiresAt.Unix() * 1000,
		}, "登录成功", c)
		return
	}

	// 多点登录模式：检查 Redis 中是否已有该用户的 token
	if jwtStr, err := jwtService.GetRedisJWT(user.Username); err == redis.Nil {
		// 情况1：Redis 中没有该用户的 token（首次登录或 token 已过期）
		// 直接将新 token 存储到 Redis
		if err := utils.SetRedisJWT(token, user.Username); err != nil {
			global.GVA_LOG.Error("设置登录状态失败!", zap.Error(err))
			response.FailWithMessage("设置登录状态失败", c)
			return
		}
		utils.SetToken(c, token, int(claims.RegisteredClaims.ExpiresAt.Unix()-time.Now().Unix()))
		response.OkWithDetailed(systemRes.LoginResponse{
			User:      user,
			Token:     token,
			ExpiresAt: claims.RegisteredClaims.ExpiresAt.Unix() * 1000,
		}, "登录成功", c)
	} else if err != nil {
		// 情况2：Redis 查询出错（网络问题、Redis 故障等）
		global.GVA_LOG.Error("设置登录状态失败!", zap.Error(err))
		response.FailWithMessage("设置登录状态失败", c)
	} else {
		// 情况3：Redis 中已有该用户的 token（用户在其他地方已登录）
		// 将旧 token 加入黑名单，实现单点登录效果
		var blackJWT system.JwtBlacklist
		blackJWT.Jwt = jwtStr
		if err := jwtService.JsonInBlacklist(blackJWT); err != nil {
			response.FailWithMessage("jwt作废失败", c)
			return
		}
		// 将新 token 存储到 Redis，替换旧 token
		if err := utils.SetRedisJWT(token, user.GetUsername()); err != nil {
			response.FailWithMessage("设置登录状态失败", c)
			return
		}
		utils.SetToken(c, token, int(claims.RegisteredClaims.ExpiresAt.Unix()-time.Now().Unix()))
		response.OkWithDetailed(systemRes.LoginResponse{
			User:      user,
			Token:     token,
			ExpiresAt: claims.RegisteredClaims.ExpiresAt.Unix() * 1000,
		}, "登录成功", c)
	}
}

// Register 用户注册接口
// 设计要点：
// 1. 参数验证：使用 ShouldBindJSON 和 Verify 双重验证，确保数据安全
// 2. 多角色支持：支持一个用户拥有多个角色（Authorities），实现灵活的权限管理
// 3. 主角色设置：设置 AuthorityId 作为主角色，便于快速查询和权限判断
// 4. 错误处理：即使注册失败也返回部分用户信息，便于前端展示和调试
//
// 为什么这么写：
// - 双重验证：ShouldBindJSON 验证 JSON 格式，Verify 验证业务规则，提前发现错误
// - 多角色设计：通过 Authorities 数组支持用户拥有多个角色，满足复杂业务场景
// - 主角色设计：AuthorityId 作为主角色，简化常用查询场景
// - 错误信息：返回部分用户信息有助于前端了解注册进度和失败原因
//
// 好处：
// - 安全性：双重验证防止恶意数据和业务规则违规
// - 灵活性：支持多角色，满足复杂权限需求
// - 可维护性：清晰的错误处理和日志记录
// - 用户体验：详细的错误信息帮助用户理解问题
//
// @Tags     SysUser
// @Summary  用户注册账号
// @Produce   application/json
// @Param    data  body      systemReq.Register                                            true  "用户名, 昵称, 密码, 角色ID"
// @Success  200   {object}  response.Response{data=systemRes.SysUserResponse,msg=string}  "用户注册账号,返回包括用户信息"
// @Router   /user/admin_register [post]
func (b *BaseApi) Register(c *gin.Context) {
	// 步骤1：绑定请求参数
	// 使用 ShouldBindJSON 自动将 JSON 请求体解析到结构体
	// 好处：自动类型转换，减少手动解析代码
	var r systemReq.Register
	err := c.ShouldBindJSON(&r)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：业务规则验证
	// 使用统一的验证器验证业务规则（如密码强度、用户名格式等）
	// 好处：集中管理验证逻辑，便于维护和复用
	err = utils.Verify(r, utils.RegisterVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤3：构建多角色数组
	// 将请求中的角色ID数组转换为 SysAuthority 对象数组
	// 设计原因：支持用户拥有多个角色，实现细粒度权限控制
	var authorities []system.SysAuthority
	for _, v := range r.AuthorityIds {
		authorities = append(authorities, system.SysAuthority{
			AuthorityId: v,
		})
	}
	
	// 步骤4：构建用户对象
	// 将请求参数映射到用户模型，包含主角色和多角色信息
	// 设计原因：主角色（AuthorityId）用于快速查询，多角色（Authorities）用于细粒度权限
	user := &system.SysUser{
		Username:    r.Username,
		NickName:    r.NickName,
		Password:    r.Password,
		HeaderImg:   r.HeaderImg,
		AuthorityId: r.AuthorityId, // 主角色，用于快速查询
		Authorities: authorities,   // 多角色数组，用于细粒度权限控制
		Enable:      r.Enable,
		Phone:       r.Phone,
		Email:       r.Email,
	}
	
	// 步骤5：调用服务层注册用户
	// 将业务逻辑委托给 service 层，保持 API 层简洁
	// 好处：职责分离，便于测试和维护
	userReturn, err := userService.Register(*user)
	if err != nil {
		global.GVA_LOG.Error("注册失败!", zap.Error(err))
		// 即使失败也返回部分用户信息，便于前端了解注册进度
		response.FailWithDetailed(systemRes.SysUserResponse{User: userReturn}, "注册失败", c)
		return
	}
	response.OkWithDetailed(systemRes.SysUserResponse{User: userReturn}, "注册成功", c)
}

// ChangePassword 用户修改密码接口
// 设计要点：
// 1. 安全验证：从 JWT token 中获取用户ID，防止越权修改
// 2. 原密码验证：必须提供原密码才能修改，防止未授权修改
// 3. 密码加密：新密码在 service 层进行加密存储
// 4. 错误信息：不暴露具体错误原因，防止信息泄露
//
// 为什么这么写：
// - 从 token 获取用户ID：确保用户只能修改自己的密码，防止越权
// - 原密码验证：增加安全性，即使 token 泄露也需要原密码
// - 通用错误信息：不暴露是原密码错误还是其他原因，防止信息泄露
// - 最小权限原则：只传递必要的用户ID和密码，不传递完整用户对象
//
// 安全考虑：
// - 防止越权：从 token 获取用户ID，确保只能修改自己的密码
// - 防止暴力破解：需要原密码，增加攻击难度
// - 信息隐藏：不暴露具体错误原因，防止信息泄露
// - 密码加密：新密码在 service 层加密，不在 API 层处理
//
// @Tags      SysUser
// @Summary   用户修改密码
// @Security  ApiKeyAuth
// @Produce  application/json
// @Param     data  body      systemReq.ChangePasswordReq    true  "用户名, 原密码, 新密码"
// @Success   200   {object}  response.Response{msg=string}  "用户修改密码"
// @Router    /user/changePassword [post]
func (b *BaseApi) ChangePassword(c *gin.Context) {
	// 步骤1：绑定请求参数
	var req systemReq.ChangePasswordReq
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：验证业务规则（如新密码强度、原密码不能为空等）
	err = utils.Verify(req, utils.ChangePasswordVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤3：从 JWT token 中获取当前用户ID
	// 设计原因：确保用户只能修改自己的密码，防止越权操作
	// 好处：不需要在请求中传递用户ID，更安全
	uid := utils.GetUserID(c)
	
	// 步骤4：构建用户对象（只包含ID和原密码）
	// 设计原因：最小权限原则，只传递必要的字段
	// 好处：减少数据传输，降低安全风险
	u := &system.SysUser{
		GVA_MODEL: global.GVA_MODEL{ID: uid},
		Password:   req.Password, // 原密码，用于验证
	}
	
	// 步骤5：调用服务层修改密码
	// service 层会：
	// 1. 验证原密码是否正确
	// 2. 对新密码进行加密
	// 3. 更新数据库中的密码
	err = userService.ChangePassword(u, req.NewPassword)
	if err != nil {
		global.GVA_LOG.Error("修改失败!", zap.Error(err))
		// 使用通用错误信息，不暴露具体原因（原密码错误或其他）
		// 好处：防止信息泄露，增加攻击难度
		response.FailWithMessage("修改失败，原密码与当前账户不符", c)
		return
	}
	response.OkWithMessage("修改成功", c)
}

// GetUserList 分页获取用户列表接口
// 设计要点：
// 1. 分页查询：支持分页，避免一次性加载大量数据
// 2. 统一响应格式：使用 PageResult 统一分页响应格式
// 3. 参数验证：验证分页参数，防止无效查询
// 4. 数据安全：service 层会过滤敏感信息（如密码）
//
// 为什么这么写：
// - 分页设计：避免大数据量查询导致性能问题，提升用户体验
// - 统一格式：PageResult 包含 List、Total、Page、PageSize，前端易于处理
// - 参数验证：验证页码和每页大小，防止无效查询和 SQL 注入
// - 职责分离：API 层只负责参数验证和响应格式化，业务逻辑在 service 层
//
// 好处：
// - 性能：分页查询减少数据库负载和网络传输
// - 一致性：统一的响应格式便于前端统一处理
// - 安全性：参数验证防止恶意查询
// - 可维护性：清晰的职责分离便于维护
//
// @Tags      SysUser
// @Summary   分页获取用户列表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      systemReq.GetUserList                                        true  "页码, 每页大小"
// @Success   200   {object}  response.Response{data=response.PageResult,msg=string}  "分页获取用户列表,返回包括列表,总数,页码,每页数量"
// @Router    /user/getUserList [post]
func (b *BaseApi) GetUserList(c *gin.Context) {
	// 步骤1：绑定分页参数
	var pageInfo systemReq.GetUserList
	err := c.ShouldBindJSON(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：验证分页参数
	// 验证页码、每页大小等，防止无效查询和 SQL 注入
	err = utils.Verify(pageInfo, utils.PageInfoVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤3：调用服务层获取用户列表
	// service 层会：
	// 1. 根据分页参数查询数据库
	// 2. 过滤敏感信息（如密码）
	// 3. 返回列表和总数
	list, total, err := userService.GetUserInfoList(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	
	// 步骤4：构建统一的分页响应格式
	// 包含列表数据、总数、当前页码、每页大小
	// 好处：前端可以根据总数和每页大小计算总页数，实现分页组件
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// SetUserAuthority 更改用户权限接口
// 设计要点：
// 1. 权限验证：从 token 获取当前用户ID，确保操作合法性
// 2. 实时生效：修改权限后立即刷新 token，新权限立即生效
// 3. Token 刷新：通过响应头返回新 token，前端无需重新登录
// 4. 双 token 机制：同时设置 cookie 和响应头，兼容不同前端实现
//
// 为什么这么写：
// - 实时生效：权限修改后立即刷新 token，用户无需重新登录即可使用新权限
// - 响应头返回：通过 new-token 和 new-expires-at 响应头返回新 token
// - Cookie 设置：同时设置 cookie，兼容传统前端实现
// - 安全性：从 token 获取用户ID，防止越权操作
//
// 工作流程：
// 1. 验证请求参数
// 2. 更新数据库中的用户权限
// 3. 生成新的 JWT token（包含新权限）
// 4. 通过响应头和 cookie 返回新 token
//
// 好处：
// - 用户体验：权限修改后立即生效，无需重新登录
// - 兼容性：同时支持响应头和 cookie，兼容不同前端
// - 安全性：权限变更立即反映在 token 中，防止权限不一致
//
// @Tags      SysUser
// @Summary   更改用户权限
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      systemReq.SetUserAuth          true  "用户UUID, 角色ID"
// @Success   200   {object}  response.Response{msg=string}  "设置用户权限"
// @Router    /user/setUserAuthority [post]
func (b *BaseApi) SetUserAuthority(c *gin.Context) {
	// 步骤1：绑定请求参数
	var sua systemReq.SetUserAuth
	err := c.ShouldBindJSON(&sua)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：验证业务规则（如角色ID是否有效等）
	if UserVerifyErr := utils.Verify(sua, utils.SetUserAuthorityVerify); UserVerifyErr != nil {
		response.FailWithMessage(UserVerifyErr.Error(), c)
		return
	}
	
	// 步骤3：从 token 获取当前用户ID
	// 设计原因：确保用户只能修改自己的权限，或管理员修改其他用户权限
	userID := utils.GetUserID(c)
	
	// 步骤4：更新数据库中的用户权限
	err = userService.SetUserAuthority(userID, sua.AuthorityId)
	if err != nil {
		global.GVA_LOG.Error("修改失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤5：生成新的 JWT token（包含新权限）
	// 设计原因：权限修改后需要立即生效，生成新 token 包含新权限信息
	claims := utils.GetUserInfo(c)
	claims.AuthorityId = sua.AuthorityId // 更新权限ID
	token, err := utils.NewJWT().CreateToken(*claims)
	if err != nil {
		global.GVA_LOG.Error("修改失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤6：通过响应头返回新 token
	// 设计原因：前端可以通过响应头获取新 token，无需重新登录
	// 好处：提升用户体验，权限修改后立即生效
	c.Header("new-token", token)
	c.Header("new-expires-at", strconv.FormatInt(claims.ExpiresAt.Unix(), 10))
	
	// 步骤7：同时设置 cookie（兼容传统前端实现）
	utils.SetToken(c, token, int(claims.ExpiresAt.Unix()-time.Now().Unix()))
	
	response.OkWithMessage("修改成功", c)
}

// SetUserAuthorities
// @Tags      SysUser
// @Summary   设置用户权限
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      systemReq.SetUserAuthorities   true  "用户UUID, 角色ID"
// @Success   200   {object}  response.Response{msg=string}  "设置用户权限"
// @Router    /user/setUserAuthorities [post]
func (b *BaseApi) SetUserAuthorities(c *gin.Context) {
	var sua systemReq.SetUserAuthorities
	err := c.ShouldBindJSON(&sua)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	authorityID := utils.GetUserAuthorityId(c)
	err = userService.SetUserAuthorities(authorityID, sua.ID, sua.AuthorityIds)
	if err != nil {
		global.GVA_LOG.Error("修改失败!", zap.Error(err))
		response.FailWithMessage("修改失败", c)
		return
	}
	response.OkWithMessage("修改成功", c)
}

// DeleteUser 删除用户接口
// 设计要点：
// 1. 防误删：禁止用户删除自己，防止误操作导致账号丢失
// 2. 权限验证：从 token 获取当前用户ID，确保操作合法性
// 3. 参数验证：验证用户ID格式，防止无效删除
// 4. 级联删除：service 层会处理相关的级联删除（如用户角色关联等）
//
// 为什么这么写：
// - 防误删机制：禁止删除自己，防止误操作导致无法登录
// - 安全验证：从 token 获取用户ID，确保操作合法性
// - 参数验证：验证ID格式，防止无效删除和 SQL 注入
// - 级联处理：在 service 层处理相关数据删除，保持数据一致性
//
// 安全考虑：
// - 防止误删：禁止删除自己，保护账号安全
// - 权限控制：通过 token 验证操作权限
// - 数据一致性：service 层处理级联删除，保持数据完整性
//
// @Tags      SysUser
// @Summary   删除用户
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      request.GetById                true  "用户ID"
// @Success   200   {object}  response.Response{msg=string}  "删除用户"
// @Router    /user/deleteUser [delete]
func (b *BaseApi) DeleteUser(c *gin.Context) {
	// 步骤1：绑定请求参数
	var reqId request.GetById
	err := c.ShouldBindJSON(&reqId)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：验证用户ID格式
	err = utils.Verify(reqId, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤3：从 token 获取当前用户ID
	jwtId := utils.GetUserID(c)
	
	// 步骤4：防误删检查 - 禁止用户删除自己
	// 设计原因：防止误操作导致账号丢失，提升安全性
	// 好处：保护用户账号，防止恶意或误操作
	if jwtId == uint(reqId.ID) {
		response.FailWithMessage("删除失败, 无法删除自己。", c)
		return
	}
	
	// 步骤5：调用服务层删除用户
	// service 层会：
	// 1. 检查用户是否存在
	// 2. 处理级联删除（如用户角色关联、用户数据等）
	// 3. 删除用户记录
	err = userService.DeleteUser(reqId.ID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败", c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// SetUserInfo
// @Tags      SysUser
// @Summary   设置用户信息
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysUser                                             true  "ID, 用户名, 昵称, 头像链接"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "设置用户信息"
// @Router    /user/setUserInfo [put]
func (b *BaseApi) SetUserInfo(c *gin.Context) {
	var user systemReq.ChangeUserInfo
	err := c.ShouldBindJSON(&user)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = utils.Verify(user, utils.IdVerify)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if len(user.AuthorityIds) != 0 {
		authorityID := utils.GetUserAuthorityId(c)
		err = userService.SetUserAuthorities(authorityID, user.ID, user.AuthorityIds)
		if err != nil {
			global.GVA_LOG.Error("设置失败!", zap.Error(err))
			response.FailWithMessage("设置失败", c)
			return
		}
	}
	err = userService.SetUserInfo(system.SysUser{
		GVA_MODEL: global.GVA_MODEL{
			ID: user.ID,
		},
		NickName:  user.NickName,
		HeaderImg: user.HeaderImg,
		Phone:     user.Phone,
		Email:     user.Email,
		Enable:    user.Enable,
	})
	if err != nil {
		global.GVA_LOG.Error("设置失败!", zap.Error(err))
		response.FailWithMessage("设置失败", c)
		return
	}
	response.OkWithMessage("设置成功", c)
}

// SetSelfInfo
// @Tags      SysUser
// @Summary   设置用户信息
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      system.SysUser                                             true  "ID, 用户名, 昵称, 头像链接"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "设置用户信息"
// @Router    /user/SetSelfInfo [put]
func (b *BaseApi) SetSelfInfo(c *gin.Context) {
	var user systemReq.ChangeUserInfo
	err := c.ShouldBindJSON(&user)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	user.ID = utils.GetUserID(c)
	err = userService.SetSelfInfo(system.SysUser{
		GVA_MODEL: global.GVA_MODEL{
			ID: user.ID,
		},
		NickName:  user.NickName,
		HeaderImg: user.HeaderImg,
		Phone:     user.Phone,
		Email:     user.Email,
		Enable:    user.Enable,
	})
	if err != nil {
		global.GVA_LOG.Error("设置失败!", zap.Error(err))
		response.FailWithMessage("设置失败", c)
		return
	}
	response.OkWithMessage("设置成功", c)
}

// SetSelfSetting
// @Tags      SysUser
// @Summary   设置用户配置
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      map[string]interface{}  true  "用户配置数据"
// @Success   200   {object}  response.Response{data=map[string]interface{},msg=string}  "设置用户配置"
// @Router    /user/SetSelfSetting [put]
func (b *BaseApi) SetSelfSetting(c *gin.Context) {
	var req common.JSONMap
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err = userService.SetSelfSetting(req, utils.GetUserID(c))
	if err != nil {
		global.GVA_LOG.Error("设置失败!", zap.Error(err))
		response.FailWithMessage("设置失败", c)
		return
	}
	response.OkWithMessage("设置成功", c)
}

// GetUserInfo
// @Tags      SysUser
// @Summary   获取用户信息
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200  {object}  response.Response{data=map[string]interface{},msg=string}  "获取用户信息"
// @Router    /user/getUserInfo [get]
func (b *BaseApi) GetUserInfo(c *gin.Context) {
	uuid := utils.GetUserUuid(c)
	ReqUser, err := userService.GetUserInfo(uuid)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
		return
	}
	response.OkWithDetailed(gin.H{"userInfo": ReqUser}, "获取成功", c)
}

// ResetPassword
// @Tags      SysUser
// @Summary   重置用户密码
// @Security  ApiKeyAuth
// @Produce  application/json
// @Param     data  body      system.SysUser                 true  "ID"
// @Success   200   {object}  response.Response{msg=string}  "重置用户密码"
// @Router    /user/resetPassword [post]
func (b *BaseApi) ResetPassword(c *gin.Context) {
	var rps systemReq.ResetPassword
	err := c.ShouldBindJSON(&rps)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	err = userService.ResetPassword(rps.ID, rps.Password)
	if err != nil {
		global.GVA_LOG.Error("重置失败!", zap.Error(err))
		response.FailWithMessage("重置失败"+err.Error(), c)
		return
	}
	response.OkWithMessage("重置成功", c)
}
