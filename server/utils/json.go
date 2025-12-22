package utils

import (
	"encoding/json"
	"strings"
)

// GetJSONKeys 从JSON字符串中提取所有键名，并保持键的原始顺序
//
// 为什么使用 json.Decoder 而不是 json.Unmarshal？
//  1. 保持键的顺序：json.Unmarshal 使用 map[string]interface{} 时会丢失键的顺序（Go的map是无序的）
//     而 json.Decoder 通过流式解析可以按照JSON原始顺序读取键
//  2. 内存效率：不需要将整个JSON结构解析到内存中，只需要流式读取键名即可
//  3. 灵活性：可以处理任意大小的JSON，不需要预先知道值的结构类型
//
// 参数:
//   - jsonStr: 待解析的JSON字符串，必须是有效的JSON对象格式
//
// 返回:
//   - keys: JSON对象中所有键名的切片，按照在JSON中出现的顺序排列
//   - err: 解析过程中的错误，如果JSON格式无效或不是对象类型则返回错误
func GetJSONKeys(jsonStr string) (keys []string, err error) {
	// 使用 json.Decoder 进行流式解析
	// strings.NewReader 将字符串转换为 io.Reader，满足 Decoder 的输入要求
	// 这种方式可以逐个token读取JSON，而不是一次性解析整个结构
	dec := json.NewDecoder(strings.NewReader(jsonStr))

	// 读取第一个token，应该是 '{' 表示JSON对象的开始
	// Token() 方法返回下一个JSON token，可能是：
	// - json.Delim: '{', '}', '[', ']' 等分隔符
	// - string: JSON键名或字符串值
	// - float64: JSON数字
	// - bool: JSON布尔值
	// - nil: JSON null值
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}

	// 类型检查：确保第一个token是对象开始符 '{'
	// json.Delim 是 rune 类型的别名，用于表示JSON的分隔符
	// 如果不是对象类型（比如是数组 '[' 或其他），则返回错误
	// 这样可以提前发现输入格式错误，避免后续解析失败
	if t != json.Delim('{') {
		return nil, err
	}

	// dec.More() 检查当前对象或数组中是否还有更多元素
	// 在对象中，它会检查是否还有更多的键值对
	// 这个循环会按照JSON中键出现的顺序逐个处理
	for dec.More() {
		// 读取键名（在对象中，Token() 会交替返回键名和值）
		// 在 '{' 之后，第一个Token() 返回的是键名（string类型）
		t, err = dec.Token()
		if err != nil {
			return nil, err
		}

		// 类型断言：将token转换为string（JSON对象的键必须是字符串）
		// 这里可以安全地断言，因为JSON规范要求对象的键必须是字符串
		keys = append(keys, t.(string))

		// 关键步骤：必须解析值，即使我们不需要它
		// 原因：
		// 1. Decoder 是流式解析器，它需要知道当前值在哪里结束，才能继续读取下一个键
		// 2. 如果不解析值，Decoder 会停留在当前键的值上，无法前进到下一个键值对
		// 3. 使用 interface{} 可以接收任意类型的值（对象、数组、字符串、数字等）
		//    这样无论值的结构如何复杂，都能正确跳过，继续读取下一个键
		var value interface{}
		err = dec.Decode(&value)
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}
