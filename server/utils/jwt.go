package utils

import (
	"context"
	"errors"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	jwt "github.com/golang-jwt/jwt/v5"
)

// JWT JWT工具结构体
// 使用结构体封装签名密钥，便于管理和扩展
// 好处：可以支持多个JWT实例（如不同业务使用不同的签名密钥），提高代码的可维护性和灵活性
type JWT struct {
	SigningKey []byte // 签名密钥，用于签名和验证token
}

// JWT相关的错误定义
// 使用预定义的错误变量而不是直接返回错误字符串，有以下好处：
// 1. 错误类型统一，便于上层调用者进行错误判断和处理
// 2. 错误信息集中管理，修改时只需改一处
// 3. 可以使用 errors.Is() 进行错误类型判断，代码更清晰
var (
	TokenValid            = errors.New("未知错误")       // 通用token验证错误
	TokenExpired          = errors.New("token已过期")   // token已过期
	TokenNotValidYet      = errors.New("token尚未激活")  // token尚未生效（NotBefore时间未到）
	TokenMalformed        = errors.New("这不是一个token") // token格式错误
	TokenSignatureInvalid = errors.New("无效签名")       // token签名验证失败
	TokenInvalid          = errors.New("无法处理此token") // 其他无法处理的token错误
)

// NewJWT 创建JWT实例
// 从全局配置中读取签名密钥并初始化JWT结构体
// 好处：统一从配置读取，便于管理和修改签名密钥
func NewJWT() *JWT {
	return &JWT{
		[]byte(global.GVA_CONFIG.JWT.SigningKey),
	}
}

// CreateClaims 创建JWT Claims（声明）
// 将用户基础信息封装成JWT标准Claims结构
// 设计要点：
//  1. BufferTime（缓冲时间）：在token即将过期前允许刷新，避免频繁重新登录
//     好处：提升用户体验，在缓冲时间内刷新token时，旧token仍然有效，实现平滑过渡
//     注意：缓冲时间内会同时存在新旧两个token，前端只保留一个，另一个会丢失（这是正常的设计）
//  2. NotBefore设置为过去时间：确保token立即生效，避免时间同步问题导致的token无法使用
//  3. 使用标准JWT Claims字段（Audience、Issuer等）：符合JWT规范，便于与其他系统集成
func (j *JWT) CreateClaims(baseClaims request.BaseClaims) request.CustomClaims {
	bf, _ := ParseDuration(global.GVA_CONFIG.JWT.BufferTime)  // 解析缓冲时间配置
	ep, _ := ParseDuration(global.GVA_CONFIG.JWT.ExpiresTime) // 解析过期时间配置
	claims := request.CustomClaims{
		BaseClaims: baseClaims,              // 用户基础信息（UUID、ID、用户名等）
		BufferTime: int64(bf / time.Second), // 缓冲时间（秒），用于token刷新机制
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"GVA"},                   // 受众：标识token的目标接收者
			NotBefore: jwt.NewNumericDate(time.Now().Add(-1000)), // 签名生效时间：设置为过去时间，确保立即生效
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ep)),    // 过期时间：从配置读取，如7天
			Issuer:    global.GVA_CONFIG.JWT.Issuer,              // 签名的发行者：标识token的签发方
		},
	}
	return claims
}

// CreateToken 创建一个token
// 使用HS256算法（HMAC-SHA256）对Claims进行签名生成token字符串
// 为什么使用HS256：
// 1. 对称加密算法，性能好，适合高并发场景
// 2. 签名和验证使用同一个密钥，实现简单
// 3. 对于单服务架构，HS256足够安全且高效
func (j *JWT) CreateToken(claims request.CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.SigningKey)
}

// CreateTokenByOldToken 使用旧token换取新token
// 使用singleflight模式（归并回源）避免并发问题
// 设计原因和好处：
// 1. 防止并发刷新：同一用户可能同时发起多个刷新token请求
// 2. 避免重复生成：使用singleflight.Do确保同一个oldToken只执行一次token生成
// 3. 性能优化：多个并发请求共享同一个结果，减少不必要的计算和数据库操作
// 4. 数据一致性：确保同一oldToken在同一时刻只生成一个新token，避免token混乱
// 使用场景：用户在前端快速连续点击刷新按钮，或网络重试导致多个并发请求
func (j *JWT) CreateTokenByOldToken(oldToken string, claims request.CustomClaims) (string, error) {
	// 使用oldToken作为key，确保同一token的并发刷新请求被归并
	v, err, _ := global.GVA_Concurrency_Control.Do("JWT:"+oldToken, func() (interface{}, error) {
		return j.CreateToken(claims)
	})
	return v.(string), err
}

// ParseToken 解析和验证token
// 验证token的签名、过期时间、格式等，并提取Claims信息
// 错误处理策略：
// 1. 使用switch-case精确匹配JWT库返回的错误类型
// 2. 将JWT库的错误转换为业务层友好的错误信息
// 3. 好处：上层调用者可以根据不同错误类型采取不同的处理策略
//   - TokenExpired：可以引导用户刷新token
//   - TokenMalformed：可能是伪造的token，需要记录日志
//   - TokenSignatureInvalid：签名错误，可能是密钥泄露，需要告警
func (j *JWT) ParseToken(tokenString string) (*request.CustomClaims, error) {
	// 解析token，使用自定义的Claims结构体和签名密钥验证函数
	token, err := jwt.ParseWithClaims(tokenString, &request.CustomClaims{}, func(token *jwt.Token) (i interface{}, e error) {
		return j.SigningKey, nil // 返回签名密钥用于验证
	})

	// 根据JWT库返回的错误类型，转换为业务层错误
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, TokenExpired
		case errors.Is(err, jwt.ErrTokenMalformed):
			return nil, TokenMalformed
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, TokenSignatureInvalid
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, TokenNotValidYet
		default:
			return nil, TokenInvalid
		}
	}
	// 验证token是否有效，并提取Claims
	if token != nil {
		if claims, ok := token.Claims.(*request.CustomClaims); ok && token.Valid {
			return claims, nil
		}
	}
	return nil, TokenValid
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: SetRedisJWT
//@description: jwt存入redis并设置过期时间
//@param: jwt string, userName string
//@return: err error

// SetRedisJWT 将JWT token存储到Redis中
// 设计目的和好处：
// 1. 实现token黑名单机制：可以将失效的token存入Redis，用于快速验证token是否被撤销
// 2. 支持单点登录（SSO）：同一用户只能有一个有效token，新token会覆盖旧token
// 3. 快速验证：Redis查询速度快，可以快速判断token是否有效
// 4. 自动过期：Redis的过期机制与JWT过期时间同步，自动清理过期token，节省内存
// 5. 分布式支持：Redis作为共享存储，支持多实例部署场景下的token管理
//
// 注意：过期时间设置为与JWT过期时间相同，确保Redis中的token与JWT生命周期一致
func SetRedisJWT(jwt string, userName string) (err error) {
	// 解析JWT过期时间配置，用于设置Redis key的过期时间
	dr, err := ParseDuration(global.GVA_CONFIG.JWT.ExpiresTime)
	if err != nil {
		return err
	}
	timer := dr
	// 将token存入Redis，key为用户名校，value为token字符串，过期时间与JWT过期时间一致
	// 好处：Redis自动清理过期数据，无需手动维护
	err = global.GVA_REDIS.Set(context.Background(), userName, jwt, timer).Err()
	return err
}
