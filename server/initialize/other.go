package initialize

import (
	"bufio"
	"os"
	"strings"

	"github.com/songzhibin97/gkit/cache/local_cache"
	"go.uber.org/zap"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
)

// OtherInit 初始化其他杂项配置和组件
//
// 设计思路：
// 此函数负责初始化一些不依赖数据库、日志等核心组件的配置
// 主要包括：JWT 配置验证、黑名单缓存初始化、代码生成模块名自动检测
//
// 为什么单独提取为 OtherInit？
// - 代码组织：将杂项初始化逻辑集中管理，保持 main.go 的简洁
// - 依赖顺序：这些初始化不依赖数据库和日志，可以在配置加载后立即执行
// - 可维护性：修改杂项初始化逻辑时只需修改此函数
func OtherInit() {
	// 第一步：解析并验证 JWT 过期时间配置
	// 为什么需要解析时间字符串？
	// - 配置文件中的时间是字符串格式（如 "24h"、"720h"），需要转换为 time.Duration
	// - 解析失败说明配置错误，应该立即发现并终止程序
	// - 解析后的时间用于设置黑名单缓存的过期时间
	dr, err := utils.ParseDuration(global.GVA_CONFIG.JWT.ExpiresTime)
	if err != nil {
		// 为什么使用 panic 而不是返回错误？
		// - JWT 配置错误是致命错误，程序无法正常运行
		// - 启动时发现错误比运行时发现更好，问题更明显
		// - 避免程序在错误配置下运行，导致安全问题
		panic(err)
	}

	// 第二步：解析并验证 JWT BufferTime（缓冲时间）配置
	// 什么是 BufferTime？
	// - BufferTime 是 token 刷新缓冲时间，用于在 token 即将过期前自动刷新
	// - 例如：token 还有 5 分钟过期，如果 BufferTime 是 10 分钟，则自动刷新 token
	// - 这样设计的好处：用户无需感知 token 刷新，提升用户体验
	//
	// 为什么只解析不保存？
	// - BufferTime 在 JWT 中间件中直接使用配置值，不需要全局变量
	// - 这里解析主要是为了验证配置格式是否正确
	// - 配置验证：确保 BufferTime 格式正确，避免运行时错误
	_, err = utils.ParseDuration(global.GVA_CONFIG.JWT.BufferTime)
	if err != nil {
		// 同样，配置错误应该立即终止程序
		panic(err)
	}

	// 第三步：初始化黑名单缓存（BlackCache）
	// 什么是 BlackCache？
	// - 本地内存缓存，用于存储 JWT 黑名单和验证码防爆次数
	// - JWT 黑名单：用户退出登录或 token 被拉黑后，存储在此缓存中
	// - 验证码防爆：记录每个 IP 的验证码请求次数，防止暴力破解
	//
	// 为什么使用本地缓存而不是 Redis？
	// - 性能：本地缓存访问速度更快，无需网络开销
	// - 简单：不需要额外的 Redis 依赖，降低系统复杂度
	// - 适用场景：黑名单数据量不大，本地缓存足够使用
	//
	// 为什么设置默认过期时间为 JWT 过期时间？
	// - JWT 黑名单的过期时间应该与 token 过期时间一致
	// - token 过期后，对应的黑名单记录也应该自动清除，释放内存
	// - 自动过期机制：无需手动清理，减少内存占用
	global.BlackCache = local_cache.NewCache(
		local_cache.SetDefaultExpire(dr), // 设置默认过期时间为 JWT 过期时间
	)

	// 第四步：自动检测 Go 模块名（用于代码生成功能）
	// 为什么需要读取 go.mod 文件？
	// - 代码生成功能需要知道项目的模块名，用于生成正确的 import 路径
	// - 如果配置文件中没有指定模块名，则自动从 go.mod 读取
	// - 好处：减少配置项，提高开发体验

	/**
		有个疑问 如果 defer file.Close() 写在这里是不是更好

	**/

	// 为什么使用 err == nil 判断？
	// - 如果文件打开失败（文件不存在、权限不足等），跳过自动检测
	// - 容错处理：即使 go.mod 不存在，也不影响程序启动
	// - 同时检查配置是否已设置：如果配置中已有模块名，则不覆盖
	if global.GVA_CONFIG.AutoCode.Module == "" {
		file, err := os.Open("go.mod")
		if err != nil {
			zap.L().Debug("open go.mod file failed", zap.Error(err))
			return
		}
		// 使用 defer 确保文件正确关闭
		// 为什么使用 defer？
		// - 确保资源释放：无论函数如何返回，文件都会被关闭
		// - 防止资源泄漏：避免文件句柄泄漏
		// - 最佳实践：打开文件后立即使用 defer，养成良好的编程习惯
		defer file.Close()

		// 使用 bufio.Scanner 逐行读取文件
		// 为什么使用 Scanner 而不是一次性读取？
		// - go.mod 文件通常很小，只需要第一行
		// - Scanner 更高效，只读取需要的部分
		// - 代码简洁：不需要处理文件关闭等复杂逻辑
		scanner := bufio.NewScanner(file)
		scanner.Scan() // 读取第一行，go.mod 的第一行是 "module xxx"

		// 提取模块名
		// go.mod 第一行格式：module github.com/flipped-aurora/gin-vue-admin
		// 使用 TrimPrefix 去除 "module " 前缀，得到模块名
		// 为什么使用 TrimPrefix？
		// - 简单直接：只需要去除固定前缀即可
		// - 性能好：比正则表达式或字符串分割更高效
		// - 容错性：如果格式不对，至少不会 panic，只是得到错误的值
		global.GVA_CONFIG.AutoCode.Module = strings.TrimPrefix(scanner.Text(), "module ")
	}
}
