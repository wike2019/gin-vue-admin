package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email/global"

	"github.com/jordan-wright/email"
)

// Email 发送邮件方法（支持多个收件人）
// 设计模式：适配器模式（Adapter Pattern）
// 好处：
// 1. 将多个收件人字符串（逗号分隔）转换为切片，适配底层 send 方法
// 2. 提供统一的接口，隐藏内部实现细节
// 3. 便于后续扩展，如添加抄送、密送等功能
// @author: [maplepie](https://github.com/maplepie)
// @function: Email
// @description: Email发送方法
// @param: To string 收件人，多个以逗号分隔
// @param: subject string 邮件主题
// @param: body string 邮件内容（HTML格式）
// @return: error
func Email(To, subject string, body string) error {
	// 将逗号分隔的收件人字符串拆分为切片
	// 好处：支持批量发送，提高使用灵活性
	to := strings.Split(To, ",")
	// 调用统一的发送方法，实现代码复用
	return send(to, subject, body)
}

// ErrorToEmail 将错误信息发送到配置的邮箱（用于错误监控）
// 设计模式：观察者模式（Observer Pattern）的变体
// 好处：
// 1. 自动将系统错误发送到指定邮箱，实现错误监控和告警
// 2. 从全局配置读取收件人，便于统一管理告警邮箱
// 3. 处理配置中的空字符串，提高代码健壮性
// 4. 可以用于中间件中，自动捕获和上报错误
// @author: [SliverHorn](https://github.com/SliverHorn)
// @function: ErrorToEmail
// @description: 给email中间件错误发送邮件到指定邮箱
// @param: subject string 邮件主题
// @param: body string 邮件内容（通常包含错误信息）
// @return: error
func ErrorToEmail(subject string, body string) error {
	// 从全局配置读取收件人列表
	// 好处：集中管理告警邮箱，便于运维人员统一接收错误通知
	to := strings.Split(global.GlobalConfig.To, ",")
	// 判断切片的最后一个元素是否为空，为空则移除
	// 原因：如果配置字符串以逗号结尾（如 "a@qq.com,b@qq.com,"），Split 会产生空字符串
	// 好处：提高代码健壮性，避免发送到空邮箱地址
	if to[len(to)-1] == "" {
		to = to[:len(to)-1]
	}
	return send(to, subject, body)
}

// EmailTest 发送测试邮件
// 设计模式：门面模式（Facade Pattern）
// 好处：
// 1. 简化测试流程，使用配置中的默认收件人
// 2. 隐藏配置读取细节，提供简洁的测试接口
// 3. 便于快速验证邮件功能是否正常
// @author: [maplepie](https://github.com/maplepie)
// @function: EmailTest
// @description: Email测试方法
// @param: subject string 邮件主题
// @param: body string 邮件内容
// @return: error
func EmailTest(subject string, body string) error {
	// 从全局配置读取测试收件人
	// 好处：测试时不需要指定收件人，使用配置中的默认值，简化测试流程
	to := []string{global.GlobalConfig.To}
	return send(to, subject, body)
}

// send 核心邮件发送方法
// 设计模式：模板方法模式（Template Method Pattern）
// 好处：
// 1. 统一邮件发送流程，避免代码重复
// 2. 集中处理配置读取、认证、连接等逻辑
// 3. 支持多种认证方式和连接方式（SSL/非SSL），提高兼容性
// 4. 便于统一添加重试、超时等机制
// @author: [maplepie](https://github.com/maplepie)
// @function: send
// @description: Email发送方法（核心实现）
// @param: to []string 收件人列表
// @param: subject string 邮件主题
// @param: body string 邮件内容（HTML格式）
// @return: error
func send(to []string, subject string, body string) error {
	// 从全局配置读取邮件服务器配置
	// 好处：配置集中管理，便于修改和维护
	from := global.GlobalConfig.From
	nickname := global.GlobalConfig.Nickname
	secret := global.GlobalConfig.Secret
	host := global.GlobalConfig.Host
	port := global.GlobalConfig.Port
	isSSL := global.GlobalConfig.IsSSL
	isLoginAuth := global.GlobalConfig.IsLoginAuth

	// 根据配置选择认证方式
	// 设计模式：策略模式（Strategy Pattern）
	// 好处：
	// 1. 支持多种认证方式（LOGIN 和 PLAIN），提高兼容性
	// 2. 根据配置动态选择认证策略，灵活可配置
	// 3. 便于扩展新的认证方式
	var auth smtp.Auth
	if isLoginAuth {
		// 使用 LOGIN 认证（适用于 IBM、微软等邮箱服务器）
		auth = LoginAuth(from, secret)
	} else {
		// 使用标准的 PLAIN 认证（适用于大多数邮箱服务器）
		auth = smtp.PlainAuth("", from, secret, host)
	}

	// 创建邮件对象
	e := email.NewEmail()

	// 设置发件人，如果配置了昵称则使用昵称格式
	// 好处：提高邮件可读性，收件人可以看到友好的发件人名称
	if nickname != "" {
		// 格式：昵称 <邮箱地址>，如 "系统管理员 <admin@example.com>"
		e.From = fmt.Sprintf("%s <%s>", nickname, from)
	} else {
		// 如果没有配置昵称，直接使用邮箱地址
		e.From = from
	}

	// 设置收件人、主题和内容
	e.To = to
	e.Subject = subject
	e.HTML = []byte(body) // 使用 HTML 格式，支持富文本邮件

	var err error
	hostAddr := fmt.Sprintf("%s:%d", host, port)

	// 根据配置选择连接方式（SSL 或非 SSL）
	// 设计模式：策略模式（Strategy Pattern）
	// 好处：
	// 1. 支持 SSL/TLS 加密连接，提高安全性
	// 2. 兼容不支持 SSL 的旧服务器
	// 3. 根据配置动态选择连接策略
	if isSSL {
		// 使用 TLS 加密连接
		// 好处：加密传输，防止邮件内容被窃听
		err = e.SendWithTLS(hostAddr, auth, &tls.Config{ServerName: host})
	} else {
		// 使用普通连接（不推荐，但兼容旧服务器）
		err = e.Send(hostAddr, auth)
	}
	return err
}

// LoginAuth 用于 IBM、微软邮箱服务器的 LOGIN 认证方式
// 设计模式：适配器模式（Adapter Pattern）
// 好处：
// 1. 实现 smtp.Auth 接口，适配标准库的认证机制
// 2. 支持非标准的 LOGIN 认证方式，提高兼容性
// 3. 封装认证细节，上层代码无需关心具体认证流程
// 说明：LOGIN 认证是某些邮箱服务器（如 IBM、微软）使用的非标准认证方式
// 与标准的 PLAIN 认证不同，LOGIN 认证需要分步发送用户名和密码
type loginAuth struct {
	username, password string
}

// LoginAuth 创建 LOGIN 认证对象
// 好处：提供工厂方法，隐藏内部结构体实现
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

// Start 开始认证流程
// 实现 smtp.Auth 接口，返回认证机制名称
// 好处：符合 Go 接口规范，可以无缝集成到标准库的认证流程中
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// 返回 "LOGIN" 表示使用 LOGIN 认证方式
	return "LOGIN", []byte{}, nil
}

// Next 处理服务器返回的认证提示
// 设计模式：状态机模式（State Machine Pattern）
// 好处：
// 1. 根据服务器返回的不同提示，返回相应的认证信息
// 2. 支持多种服务器提示格式，提高兼容性
// 3. 使用字符串匹配，兼容不同服务器的提示文本
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		// 服务器要求更多信息，根据提示返回用户名或密码
		switch string(fromServer) {
		case "Username:":
			// 标准提示格式：返回用户名
			return []byte(a.username), nil
		case "Password:":
			// 标准提示格式：返回密码
			return []byte(a.password), nil
		default:
			// 处理非标准提示格式（不同服务器可能使用不同的提示文本）
			// 好处：提高兼容性，支持更多邮箱服务器
			prompt := strings.ToLower(string(fromServer))
			// 使用模糊匹配，兼容不同服务器的提示文本
			if strings.Contains(prompt, "username") || strings.Contains(prompt, "user") {
				return []byte(a.username), nil
			}
			if strings.Contains(prompt, "password") || strings.Contains(prompt, "pass") {
				return []byte(a.password), nil
			}
		}
	}
	// 认证完成，返回 nil
	return nil, nil
}
