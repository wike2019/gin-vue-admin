package request

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
)

// HttpRequest 是一个通用的HTTP请求函数，支持GET、POST、PUT、DELETE等多种HTTP方法
//
// 设计优势：
// 1. 统一封装：将HTTP请求的通用逻辑集中管理，避免代码重复
// 2. 灵活性强：支持自定义请求方法、请求头、查询参数和请求体
// 3. 类型安全：使用泛型any类型接收请求体，支持任意可序列化的数据结构
// 4. 职责分离：只负责构建和发送请求，响应处理交给调用者，提高复用性
//
// 参数说明：
//   - urlStr: 目标URL地址（支持带或不带查询参数）
//   - method: HTTP方法（GET、POST、PUT、DELETE等）
//   - headers: 自定义请求头（如Authorization、User-Agent等）
//   - params: URL查询参数（会与URL中原有参数合并）
//   - data: 请求体数据（任意类型，会自动序列化为JSON）
//
// 返回值：
//   - *http.Response: HTTP响应对象，调用者需要自行关闭Body并处理响应
//   - error: 请求过程中的错误（URL解析、JSON序列化、网络请求等）
func HttpRequest(
	urlStr string,
	method string,
	headers map[string]string,
	params map[string]string,
	data any) (*http.Response, error) {
	// 使用url.Parse解析URL字符串
	// 好处：
	// 1. 自动处理URL编码，避免手动拼接导致的编码错误
	// 2. 分离URL的各个组成部分（scheme、host、path、query等），便于后续操作
	// 3. 验证URL格式的有效性，提前发现URL错误
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// 处理URL查询参数
	// 使用u.Query()获取现有查询参数的副本（返回Values类型）
	// 好处：
	// 1. 保留URL中已有的查询参数，避免丢失
	// 2. 支持参数合并：如果URL中已有同名参数，会被新参数覆盖
	// 3. Query()返回的是副本，不会直接修改原始URL对象，更安全
	query := u.Query()
	for k, v := range params {
		query.Set(k, v) // Set方法会自动处理参数名的编码
	}
	// 使用Encode()将查询参数编码为URL格式（如：key1=value1&key2=value2）
	// 好处：自动处理特殊字符的URL编码，确保参数值中的特殊字符（如空格、中文等）被正确编码
	u.RawQuery = query.Encode()

	// 处理请求体数据
	// 先创建一个空的Buffer，如果data为nil则使用空Buffer（适用于GET等无请求体的方法）
	// 好处：
	// 1. 统一处理有/无请求体两种情况，代码更简洁
	// 2. 避免在data为nil时创建不必要的Buffer对象
	buf := new(bytes.Buffer)
	if data != nil {
		// 将任意类型的数据序列化为JSON格式
		// 使用any类型的好处：
		// 1. 类型灵活，可以接收struct、map、slice等任意可序列化类型
		// 2. 调用者无需手动转换，减少样板代码
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		// 使用bytes.NewBuffer将JSON字节数组包装为Buffer
		// 好处：Buffer实现了io.Reader接口，可以直接作为http.NewRequest的body参数
		buf = bytes.NewBuffer(b)
	}

	// 使用http.NewRequest创建请求对象（而不是直接使用http.Get/Post等方法）
	// 好处：
	// 1. 支持所有HTTP方法（GET、POST、PUT、DELETE、PATCH等），不局限于Get/Post
	// 2. 可以在发送前自定义请求头、超时等属性，灵活性更高
	// 3. 统一的请求创建方式，代码更一致
	req, err := http.NewRequest(method, u.String(), buf)

	if err != nil {
		return nil, err
	}

	// 设置自定义请求头
	// 使用Header.Set而不是Header.Add的好处：
	// 1. Set会覆盖同名header，避免重复设置导致的问题
	// 2. 如果header已存在，Set会替换它；Add会追加，可能导致重复
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 当有请求体数据时，自动设置Content-Type为application/json
	// 好处：
	// 1. 自动处理，调用者无需手动设置，减少出错可能
	// 2. 只在有数据时设置，避免GET等无请求体方法设置不必要的header
	// 3. 确保服务端能正确解析JSON格式的请求体
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 使用http.DefaultClient发送请求
	// 好处：
	// 1. 简单直接，无需创建和管理Client实例
	// 2. DefaultClient内部有连接池，可以复用TCP连接，提高性能
	// 3. 适合大多数场景，如果需要对超时、重试等有特殊要求，可以传入自定义Client
	// 注意：DefaultClient没有设置超时，如果需要超时控制，应该使用自定义Client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// 返回响应对象，而不是直接读取响应体
	// 好处：
	// 1. 调用者可以根据需要处理响应（读取Body、检查状态码、读取Header等）
	// 2. 调用者负责关闭resp.Body，避免资源泄漏
	// 3. 提高函数的通用性，不同的调用场景可以有不同的处理方式
	// 注意：调用者必须调用resp.Body.Close()来释放连接资源
	return resp, nil
}
