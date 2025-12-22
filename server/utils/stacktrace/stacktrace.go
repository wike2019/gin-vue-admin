package stacktrace

import (
	"regexp"
	"strconv"
	"strings"
)

// Frame 表示一次“栈帧”的解析结果
// 之所以只保留 File / Line / Func 三个字段，是因为：
//  1. 业务排查问题时，最核心关心的是“哪个文件哪一行、在哪个函数里”；
//  2. 结构体保持精简，便于在日志、监控里直接序列化输出；
//  3. 与 zap 栈信息的格式解耦，后续如更换日志库，只要能解析出这三项即可兼容。
type Frame struct {
	File string // 触发日志的源代码文件完整路径
	Line int    // 源代码行号
	Func string // 对应的函数名（从上一行 stack 文本中提取）
}

// fileLineRe 用来从一行 stack 文本中提取 `xxx.go:行号`
// 使用正则而不是简单的 strings.Split 的原因：
//   - zap 的 Stack 文本前后可能带有空格、Tab，需要统一 Trim 掉；
//   - 只匹配以 `.go:数字` 结尾的行，避免误将普通字符串识别为文件行。
var fileLineRe = regexp.MustCompile(`\s*(.+\.go):(\d+)\s*$`)

// FindFinalCaller 从 zap 的 entry.Stack 文本中，解析“最终业务调用方”的文件与行号。
//
// 设计思路与好处：
//  1. **只返回“第一条业务相关的栈帧”**：
//     - 一般来说，最上层（靠前）的项目代码行，就是最接近真实报错位置的地方；
//     - 这样日志里能直接看到“命中的业务代码”，而不是漫长的中间件/框架调用链。
//  2. **主动过滤第三方库、标准库、框架中间件**（见 shouldSkip）：
//     - 这些调用往往是“基础设施”，对排查业务 Bug 帮助不大；
//     - 过滤掉之后，日志定位更干净，研发看到的就是自己写的代码。
//  3. **从上到下单次遍历**：
//     - 时间复杂度 O(n)，n 为栈文本行数，性能开销极小，适合在高 QPS 场景下使用；
//     - 一旦命中即可立即返回，不需要继续向下扫描。
func FindFinalCaller(stack string) (Frame, bool) {
	// 空栈直接返回 false，避免后续 Split / 遍历的无意义开销
	if stack == "" {
		return Frame{}, false
	}

	// zap 的 Stack 是一段多行文本：函数名行 + 文件:行号行 交替出现
	lines := strings.Split(stack, "\n")

	// currFunc 用来保存“最近一次出现的函数名行”，下一行通常就是对应的文件:行号
	var currFunc string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			// 空行直接跳过，既可兼容不同日志格式，又避免额外分支逻辑
			continue
		}

		// 第一种情况：当前行是 `xxx.go:123` 这种文件行
		if m := fileLineRe.FindStringSubmatch(line); m != nil {
			file := m[1]
			ln, _ := strconv.Atoi(m[2]) // 正则已保证是数字，这里忽略错误简化逻辑

			// 如果命中的文件属于“框架/三方库/标准库”，则认为不是“最终业务调用方”
			if shouldSkip(file) {
				// 为防止下一个业务文件行错误地绑定到当前的 currFunc，这里将其重置。
				// 好处：避免“函数名行来自三方库，但文件行来自业务代码”的错误配对。
				currFunc = ""
				continue
			}

			// 命中第一条业务相关栈帧，立即返回。
			// 返回值中带上 Func，方便在日志中直接打印“函数 + 文件 + 行号”三元组。
			return Frame{File: file, Line: ln, Func: currFunc}, true
		}

		// 第二种情况：当前行不是文件:行号，通常是函数签名，如：
		//   go.uber.org/zap/logger.(*Logger).Error
		//   github.com/flipped-aurora/gin-vue-admin/server/api/v1.(*Api).Create...
		// 我们先保存下来，等待下一行的文件:行号行出现时进行绑定。
		currFunc = line
	}

	// 整个栈都没有找到合适的业务帧，返回 false 让调用者自行决定降级策略
	return Frame{}, false
}

// shouldSkip 用来判断某个文件路径是否应该被排除在“最终业务调用方”之外。
//
// 这样拆成独立函数有几个好处：
//  1. 过滤规则集中管理，后续要新增/修改规则只改这一处，逻辑更清晰；
//  2. 与解析主流程解耦，FindFinalCaller 代码更容易阅读；
//  3. 可以在单元测试中针对各种路径做边界校验，而不影响主逻辑。
func shouldSkip(file string) bool {
	// 第三方库与 Go 模块缓存（module cache）
	// 这些路径下的代码通常是依赖库，出现问题大概率是“调用方式不对”而非库本身有 Bug，
	// 对业务错误排查意义不大，因此默认过滤。
	if strings.Contains(file, "/go/pkg/mod/") {
		return true
	}
	if strings.Contains(file, "/go.uber.org/") {
		return true
	}
	if strings.Contains(file, "/gorm.io/") {
		return true
	}

	// 标准库路径：
	//   - 在很多环境下类似：/usr/local/go/src/net/http/server.go
	//   - 在某些开发者本地可能是：/Users/name/go/go1.24.2/src/net/http/server.go
	// 这里用包含关系而不是前缀判断，是为了兼容不同安装路径。
	if strings.Contains(file, "/go/go") && strings.Contains(file, "/src/") { // e.g. /Users/name/go/go1.24.2/src/net/http/server.go
		return true
	}

	// 框架内部通用模块：
	//   - core: 服务启动、日志封装等基础设施；
	//   - errorhook: 统一错误钩子；
	//   - middleware: 中间件链路；
	//   - router: 路由分发。
	// 这些地方虽然也可能打印日志，但通常不是“真正的业务入口”，过滤掉能让定位更聚焦。
	if strings.Contains(file, "/server/core/zap.go") {
		return true
	}
	if strings.Contains(file, "/server/core/") {
		return true
	}
	if strings.Contains(file, "/server/utils/errorhook/") {
		return true
	}
	if strings.Contains(file, "/server/middleware/") {
		return true
	}
	if strings.Contains(file, "/server/router/") {
		return true
	}

	// 其他情况一律视为“可能是业务代码”，不做过滤
	return false
}
