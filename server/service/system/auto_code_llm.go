package system

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common"
	commonResp "github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/request"
	"github.com/goccy/go-json"
)

// LLMAuto 调用大模型服务，返回生成结果数据
//
// 设计思路：
// 1. 使用通用 JSONMap 作为入参，提供最大灵活性，支持不同业务场景的动态参数
// 2. 通过 mode 参数实现多模式支持（ai/butler/eye/painter 等），一个函数覆盖多种大模型能力
// 3. 采用配置化的 AiPath，支持不同环境、不同服务商的灵活切换
//
// 入参说明：
//   - llm["mode"]: 必填，指定调用的大模型模式（例如 "ai"/"butler"/"eye"/"painter"）
//   - llm 其他字段: 根据 mode 不同，包含相应的业务参数（如 prompt、payload 等）
//
// 返回值：
//   - interface{}: 大模型生成的结果数据，具体类型由业务决定
//   - error: 错误信息，包含详细的失败原因
func (s *AutoCodeService) LLMAuto(ctx context.Context, llm common.JSONMap) (interface{}, error) {
	// 前置配置检查：在发起请求前验证配置，避免无效的网络请求
	// 好处：快速失败（fail-fast），节省资源，提供清晰的错误提示
	if global.GVA_CONFIG.AutoCode.AiPath == "" {
		return nil, errors.New("请先前往插件市场个人中心获取AiPath并填入config.yaml中")
	}

	// 路径构建策略：使用占位符替换机制
	// 设计好处：
	// 1. 配置模板化：AiPath 配置如 "https://api.example.com/{FUNC}"，通过替换支持多端点
	// 2. 统一路径规范：所有模式统一使用 "api/chat/{mode}" 格式，便于服务端路由管理
	// 3. 类型安全：使用 fmt.Sprintf("%v") 统一转字符串，避免 nil 值导致路径拼接异常
	//    例如：如果 llm["mode"] 为 nil，%v 会转为 "nil" 字符串，而不是 panic
	mode := fmt.Sprintf("%v", llm["mode"]) // 统一转字符串，避免 nil 造成路径异常
	path := strings.ReplaceAll(global.GVA_CONFIG.AutoCode.AiPath, "{FUNC}", fmt.Sprintf("api/chat/%s", mode))

	// HTTP 请求调用：使用统一的请求工具，便于统一管理超时、重试、日志等
	// 参数说明：
	//   - path: 动态构建的完整请求路径
	//   - "POST": 大模型服务通常需要 POST 方法传递复杂参数
	//   - nil, nil: 请求头和查询参数为空，使用默认值
	//   - llm: 将整个 JSONMap 作为请求体，包含 mode 和所有业务参数
	res, err := request.HttpRequest(
		path,
		"POST",
		nil,
		nil,
		llm,
	)
	if err != nil {
		// 网络层错误：使用 fmt.Errorf 包装原始错误，保留错误链（%w）
		// 好处：便于错误追踪和调试，可以 unwrap 获取底层错误信息
		return nil, fmt.Errorf("大模型生成失败: %w", err)
	}
	// 资源管理：使用 defer 确保响应体在所有情况下都能正确关闭
	// 好处：防止资源泄漏，即使后续代码发生 panic 也能保证资源释放
	defer res.Body.Close()

	// 响应解析：分步骤处理，便于定位具体失败环节
	var resStruct commonResp.Response
	// 第一步：读取响应体
	// 使用 io.ReadAll 一次性读取，适合中小型响应（大文件场景需考虑流式处理）
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("读取大模型响应失败: %w", err)
	}
	// 第二步：JSON 反序列化
	// 使用统一的 Response 结构体，便于统一处理业务状态码和错误信息
	if err = json.Unmarshal(b, &resStruct); err != nil {
		return nil, fmt.Errorf("解析大模型响应失败: %w", err)
	}
	// 第三步：业务状态码检查
	// 设计说明：HTTP 200 不代表业务成功，需要检查业务层面的状态码
	// Code == 7 是业务约定，表示模型生成失败（如内容违规、生成超时等）
	// 好处：区分网络错误和业务错误，提供更精确的错误信息
	if resStruct.Code == 7 { // 业务约定：7 表示模型生成失败
		return nil, fmt.Errorf("大模型生成失败: %s", resStruct.Msg)
	}
	// 成功返回：直接返回 Data 字段，由调用方根据业务需要处理具体类型
	// 好处：保持接口简洁，调用方可以灵活处理不同类型的数据
	return resStruct.Data, nil
}
