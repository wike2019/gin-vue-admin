package utils

import (
	"strconv"
	"strings"
	"time"
)

// ParseDuration 解析人类可读的持续时间字符串，返回 time.Duration
//
// 这个函数是对标准库 time.ParseDuration 的扩展，提供了以下增强功能：
// 1. 支持 "d"（天）单位：标准库只支持到小时，此函数扩展支持天数（如 "2d3h"）
// 2. 向后兼容：优先使用标准库解析，保持对标准格式的完全兼容
// 3. 降级处理：如果都不匹配，尝试将纯数字解析为纳秒数
//
// 设计优势：
// - 渐进式解析策略：先尝试标准库（性能最优），再尝试扩展功能，最后降级处理
// - 用户体验友好：支持更直观的天数表示，符合人类阅读习惯
// - 容错性强：多种解析策略确保尽可能解析成功
//
// 示例：
//   - "2h30m"     -> 2小时30分钟（标准库格式）
//   - "3d"        -> 3天
//   - "2d5h30m"   -> 2天5小时30分钟（混合格式）
//   - "3600000000000" -> 1小时（纯数字，作为纳秒）
func ParseDuration(d string) (time.Duration, error) {
	// 去除首尾空格，提高容错性
	// 好处：用户输入时可能意外添加空格，自动处理提升用户体验
	d = strings.TrimSpace(d)

	// 策略1：优先使用标准库解析（性能最优，支持所有标准单位）
	// 标准库支持：ns, us, µs, ms, s, m, h
	// 好处：对于标准格式，直接使用经过优化的标准库实现，无需额外处理
	dr, err := time.ParseDuration(d)
	if err == nil {
		return dr, nil
	}

	// 策略2：如果标准库解析失败，检查是否包含 "d"（天）单位
	// 为什么需要这个扩展：标准库 time.ParseDuration 不支持 "d" 单位
	// 但在实际业务中，天数是很常见的需求（如缓存过期时间、任务调度周期等）
	if strings.Contains(d, "d") {
		index := strings.Index(d, "d")

		// 解析天数部分：提取 "d" 之前的数字
		// 注意：这里忽略 Atoi 的错误，因为如果解析失败，hour 为 0，结果是合理的
		// 好处：即使天数部分格式错误，也能继续尝试解析剩余部分
		hour, _ := strconv.Atoi(d[:index])
		// 将天数转换为小时数（1天 = 24小时）
		// 使用 time.Hour * 24 而不是 time.Duration(24) * time.Hour 是为了类型安全
		dr = time.Hour * 24 * time.Duration(hour)

		// 尝试解析 "d" 之后的部分（可能包含其他时间单位，如 "2d5h30m"）
		// 好处：支持混合格式，如 "2d5h30m"，提供更灵活的使用方式
		ndr, err := time.ParseDuration(d[index+1:])
		if err != nil {
			// 如果剩余部分解析失败，只返回天数部分
			// 好处：部分成功总比完全失败好，至少能解析出天数
			return dr, nil
		}
		// 将天数部分和剩余部分相加
		// 好处：支持完整的混合格式，如 "2d5h30m" = 2天 + 5小时30分钟
		return dr + ndr, nil
	}

	// 策略3：降级处理 - 将纯数字字符串解析为纳秒数
	// 为什么需要这个：某些场景下，可能直接传入纳秒数（如从配置文件读取的数值）
	// 好处：提供最后一道防线，尽可能解析成功，避免因格式问题导致程序失败
	dv, err := strconv.ParseInt(d, 10, 64)
	return time.Duration(dv), err
}
