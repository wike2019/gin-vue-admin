package utils

import (
	"crypto/md5"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// BcryptHash 使用 bcrypt 对密码进行加密
//
// 为什么使用 bcrypt 而不是简单的哈希算法（如 MD5/SHA）：
// 1. 安全性：bcrypt 是专门为密码设计的哈希算法，具有以下优势：
//   - 内置盐值（salt）：每次加密都会自动生成随机盐值，即使相同密码也会产生不同的哈希值
//   - 可调节成本因子：通过 DefaultCost 控制计算复杂度，可以随着硬件性能提升而增加成本
//   - 抗彩虹表攻击：由于每次加密都使用不同的盐值，无法使用预计算的彩虹表进行破解
//   - 抗暴力破解：计算成本高，使得暴力破解变得非常困难
//
// 2. 为什么使用 bcrypt.DefaultCost：
//   - DefaultCost 通常为 10，这是经过安全性和性能平衡后的推荐值
//   - 在保证安全性的同时，不会对系统性能造成过大影响
//   - 可以根据实际需求调整成本因子（范围通常是 4-31）
//
// 3. 设计考虑：
//   - 忽略错误：这是简化版本，bcrypt.GenerateFromPassword 在正常情况下不会失败
//   - 生产环境建议：如果需要更严格的错误处理，可以检查并返回错误
//
// 使用场景：用户注册、密码修改等需要存储密码的场景
func BcryptHash(password string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes)
}

// BcryptCheck 对比明文密码和数据库的哈希值
//
// 为什么使用 bcrypt.CompareHashAndPassword 而不是简单的字符串比较：
// 1. 安全性：
//   - 常量时间比较：使用专门的函数可以防止时序攻击（timing attack）
//   - 如果使用简单的字符串比较（如 ==），攻击者可以通过测量比较时间来判断密码的相似度
//   - bcrypt 的比较函数在内部实现了安全的常量时间比较算法
//
// 2. 正确性：
//   - bcrypt 哈希值包含算法版本、成本因子、盐值等信息
//   - 只有使用专门的比较函数才能正确解析和比较这些信息
//   - 直接字符串比较无法正确处理 bcrypt 的哈希格式
//
// 3. 设计考虑：
//   - 返回 bool 而不是 error：简化调用方的使用，只需要判断 true/false
//   - 如果密码匹配返回 true，否则返回 false
//
// 使用场景：用户登录、密码验证等需要验证密码的场景
func BcryptCheck(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// @author: [piexlmax](https://github.com/piexlmax)
// @function: MD5V
// @description: md5加密
// @param: str []byte
// @return: string
//
// # MD5V 使用 MD5 算法对数据进行哈希处理
//
// 为什么使用 MD5（注意：MD5 不适用于密码加密）：
// 1. 使用场景：
//   - 文件完整性校验：快速计算文件的 MD5 值，用于验证文件是否被篡改
//   - 生成唯一标识：为数据生成固定长度的唯一标识符（如缓存键、文件名等）
//   - 数据去重：通过 MD5 值快速判断数据是否重复
//   - 非敏感数据的快速哈希：对于不需要高安全性的场景，MD5 仍然是一个快速的选择
//
// 2. 为什么不适合密码加密：
//   - MD5 已被证明存在碰撞漏洞，安全性不足
//   - MD5 没有盐值机制，相同输入总是产生相同输出
//   - 计算速度快，容易被暴力破解
//   - 密码加密应该使用 BcryptHash 函数
//
// 3. 设计特点：
//   - 可变参数 b ...byte：允许在哈希计算时添加额外的字节
//   - 这种设计提供了灵活性，可以在哈希时附加额外的数据（如密钥、随机数等）
//   - 使用方式：MD5V(data) 或 MD5V(data, additionalBytes...)
//
// 4. 返回值：
//   - 返回十六进制编码的字符串，便于存储和传输
//   - 固定长度为 32 个字符（128 位）
//
// 使用场景：文件校验、生成唯一标识、缓存键生成等非密码场景
func MD5V(str []byte, b ...byte) string {
	h := md5.New()
	h.Write(str)
	return hex.EncodeToString(h.Sum(b))
}
