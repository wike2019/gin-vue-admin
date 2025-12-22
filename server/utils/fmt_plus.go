package utils

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common"
)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: StructToMap
//@description: 利用反射将结构体转化为map
//@param: obj interface{}
//@return: map[string]interface{}

// StructToMap 将结构体转换为 map[string]interface{} 格式
//
// 为什么使用反射：
// 1. 通用性：可以处理任意类型的结构体，无需为每个结构体单独编写转换代码
// 2. 灵活性：通过反射可以动态获取结构体的字段信息，包括字段名和标签
// 3. 可维护性：当结构体字段变化时，无需修改此函数，自动适配
//
// 设计考虑：
//   - 使用 reflect.TypeOf 获取类型信息，用于遍历字段
//   - 使用 reflect.ValueOf 获取值信息，用于提取字段值
//   - 优先使用 mapstructure 标签作为 map 的 key，如果没有则使用字段名
//     这样可以在结构体字段名和 map key 之间建立映射关系，支持字段重命名
//
// 好处：
// - 减少重复代码：一个函数处理所有结构体类型
// - 支持配置映射：通过 mapstructure 标签实现灵活的字段映射
// - 类型安全：通过 Interface() 方法获取字段值，保持类型信息
func StructToMap(obj interface{}) map[string]interface{} {
	// 获取结构体的类型信息，用于遍历字段
	obj1 := reflect.TypeOf(obj)
	// 获取结构体的值信息，用于提取字段值
	obj2 := reflect.ValueOf(obj)

	data := make(map[string]interface{})
	// 遍历结构体的所有字段
	for i := 0; i < obj1.NumField(); i++ {
		// 优先使用 mapstructure 标签作为 map 的 key
		// 这样可以在结构体字段名和 map key 之间建立映射关系
		if obj1.Field(i).Tag.Get("mapstructure") != "" {
			data[obj1.Field(i).Tag.Get("mapstructure")] = obj2.Field(i).Interface()
		} else {
			// 如果没有 mapstructure 标签，则使用字段名作为 key
			data[obj1.Field(i).Name] = obj2.Field(i).Interface()
		}
	}
	return data
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: ArrayToString
//@description: 将数组格式化为字符串
//@param: array []interface{}
//@return: string

// ArrayToString 将数组格式化为逗号分隔的字符串
//
// 为什么这样实现：
// 1. 使用 fmt.Sprint 将数组转换为字符串，会自动添加方括号和空格分隔
// 2. 使用 strings.Trim 去除首尾的方括号 "[]"
// 3. 使用 strings.Replace 将空格替换为逗号，实现逗号分隔
//
// 设计考虑：
// - 使用 -1 作为替换次数，表示替换所有匹配项
// - 先 Trim 再 Replace，避免处理方括号内的内容
//
// 好处：
// - 简洁高效：一行代码完成转换
// - 通用性强：可以处理任意类型的数组
// - 输出格式统一：始终是逗号分隔的字符串，便于后续处理
//
// 示例：[1 2 3] -> "1,2,3"
func ArrayToString(array []interface{}) string {
	// fmt.Sprint 将数组转为 "[1 2 3]" 格式
	// strings.Trim 去除方括号，得到 "1 2 3"
	// strings.Replace 将空格替换为逗号，得到 "1,2,3"
	return strings.Replace(strings.Trim(fmt.Sprint(array), "[]"), " ", ",", -1)
}

// Pointer 将任意类型的值转换为指针类型
//
// 为什么使用泛型：
// 1. 类型安全：泛型确保输入和输出类型一致，避免类型转换错误
// 2. 代码复用：一个函数可以处理所有类型，无需为每种类型单独编写函数
// 3. 编译时检查：类型不匹配会在编译时报错，而不是运行时
//
// 设计考虑：
// - 使用泛型约束 [T any] 表示接受任意类型
// - 返回类型 *T 是指向 T 的指针
// - 使用命名返回值 out *T 使代码更清晰
//
// 好处：
// - 简化代码：避免在每次需要指针时都写 &value
// - 提高可读性：函数名明确表达意图
// - 类型安全：泛型保证类型一致性
//
// 使用场景：
// - 需要将值类型转换为指针类型时
// - 函数参数需要指针类型，但只有值类型时
func Pointer[T any](in T) (out *T) {
	return &in
}

// FirstUpper 将字符串首字母转换为大写
//
// 为什么这样实现：
// 1. 先检查空字符串，避免索引越界
// 2. 使用切片 s[:1] 获取第一个字符（支持多字节字符）
// 3. 使用 strings.ToUpper 转换为大写
// 4. 使用字符串拼接组合首字母和剩余部分
//
// 设计考虑：
// - 空字符串检查：防止对空字符串进行切片操作导致 panic
// - 使用切片而非直接索引：虽然 Go 字符串支持索引，但切片更安全
// - 字符串拼接：Go 1.10+ 对字符串拼接有优化，性能足够
//
// 好处：
// - 安全性：空字符串检查避免运行时错误
// - 简洁性：代码清晰易懂
// - 性能：字符串操作高效
//
// 示例："hello" -> "Hello", "world" -> "World"
func FirstUpper(s string) string {
	// 空字符串检查，避免索引越界
	if s == "" {
		return ""
	}
	// s[:1] 获取第一个字符，strings.ToUpper 转大写
	// s[1:] 获取剩余部分，直接拼接
	return strings.ToUpper(s[:1]) + s[1:]
}

// FirstLower 将字符串首字母转换为小写
//
// 为什么这样实现：
// 1. 先检查空字符串，避免索引越界
// 2. 使用切片 s[:1] 获取第一个字符
// 3. 使用 strings.ToLower 转换为小写
// 4. 使用字符串拼接组合首字母和剩余部分
//
// 设计考虑：
// - 与 FirstUpper 保持一致的实现风格
// - 空字符串检查保证安全性
// - 使用标准库函数确保正确性
//
// 好处：
// - 安全性：空字符串检查避免运行时错误
// - 一致性：与 FirstUpper 函数实现风格一致
// - 可读性：代码意图明确
//
// 使用场景：
// - 将导出的结构体字段名转换为私有字段名
// - 命名规范转换（如驼峰命名首字母小写）
//
// 示例："Hello" -> "hello", "World" -> "world"
func FirstLower(s string) string {
	// 空字符串检查，避免索引越界
	if s == "" {
		return ""
	}
	// s[:1] 获取第一个字符，strings.ToLower 转小写
	// s[1:] 获取剩余部分，直接拼接
	return strings.ToLower(s[:1]) + s[1:]
}

// MaheHump 将连字符分隔的字符串转换为驼峰命名
//
// 为什么这样实现：
// 1. 使用 strings.Split 按 "-" 分割字符串，得到单词数组
// 2. 从第二个单词开始（i=1），将每个单词首字母大写
// 3. 第一个单词保持原样（通常首字母小写）
// 4. 使用 strings.Join 将单词拼接，无分隔符
//
// 设计考虑：
// - 只处理第二个及之后的单词：第一个单词通常保持小写（如 user-name -> userName）
// - 使用 strings.Title：将每个单词的首字母大写，其余小写
// - 使用空字符串拼接：实现无分隔符的驼峰命名
//
// 好处：
// - 命名规范转换：将 kebab-case 转换为 camelCase
// - 代码生成：在代码生成工具中用于命名转换
// - 可读性：输出符合 Go 语言的命名规范
//
// 注意：
// - strings.Title 已废弃（Go 1.18+），建议使用 golang.org/x/text/cases
// - 但为了兼容性，这里仍使用 strings.Title
//
// 示例："user-name" -> "userName", "my-component" -> "myComponent"
func MaheHump(s string) string {
	// 按 "-" 分割字符串，得到单词数组
	words := strings.Split(s, "-")

	// 从第二个单词开始，将每个单词首字母大写
	// 第一个单词保持原样（通常首字母小写）
	for i := 1; i < len(words); i++ {
		words[i] = strings.Title(words[i])
	}

	// 使用空字符串拼接，实现驼峰命名
	return strings.Join(words, "")
}

// HumpToUnderscore 将驼峰命名转换为下划线分隔的命名（snake_case）
//
// 为什么使用 strings.Builder：
// 1. 性能优化：在循环中拼接字符串时，Builder 比直接拼接效率高得多
// 2. 内存效率：Builder 内部使用字节缓冲区，减少内存分配
// 3. 避免字符串拷贝：直接写入缓冲区，而不是创建新字符串
//
// 为什么这样实现：
// 1. 遍历字符串的每个字符（rune），支持 Unicode
// 2. 检测大写字母：当遇到大写字母且不是首字符时，在其前添加下划线
// 3. 转换大小写：通过字符运算 'A' + 'a' - 'A' 将大写转为小写
// 4. 最后统一转小写：确保所有字符都是小写
//
// 设计考虑：
// - 使用 rune 而非 byte：支持 Unicode 字符（如中文、emoji）
// - i > 0 检查：首字符不需要添加下划线
// - 字符范围检查：只处理 A-Z 的大写字母
// - 最后统一转小写：处理首字符可能是大写的情况
//
// 好处：
// - 性能优秀：Builder 在字符串拼接场景下性能最佳
// - 支持 Unicode：使用 rune 处理多字节字符
// - 命名规范转换：将 camelCase 转换为 snake_case
//
// 使用场景：
// - 数据库字段名转换：Go 结构体字段名转数据库列名
// - API 参数转换：JSON 字段名转 URL 参数名
//
// 示例："UserName" -> "user_name", "MyComponent" -> "my_component"
func HumpToUnderscore(s string) string {
	// 使用 Builder 提高字符串拼接性能
	var result strings.Builder

	// 遍历字符串的每个字符（rune），支持 Unicode
	for i, char := range s {
		// 检测大写字母：当遇到大写字母且不是首字符时
		if i > 0 && char >= 'A' && char <= 'Z' {
			// 在大写字母前添加下划线
			result.WriteRune('_')
			// 将大写字母转换为小写：'A'(65) -> 'a'(97)
			// char - 'A' + 'a' 实现大小写转换
			result.WriteRune(char - 'A' + 'a')
		} else {
			// 非大写字母或首字符，直接写入
			result.WriteRune(char)
		}
	}

	// 最后统一转小写，处理首字符可能是大写的情况
	return strings.ToLower(result.String())
}

// RandomString 生成指定长度的随机字符串
//
// 为什么使用 rune 而非 byte：
// 1. 支持 Unicode：rune 是 int32，可以表示 Unicode 字符
// 2. 虽然这里只使用 ASCII 字符，但使用 rune 更通用
// 3. 与字符串转换兼容：string([]rune) 转换更自然
//
// 为什么这样实现：
// 1. 预定义字符集：包含大小写字母和数字，共 62 个字符
// 2. 使用 rune 切片：预分配长度为 n 的切片，避免动态扩容
// 3. 随机选择字符：使用 RandomInt 从字符集中随机选择
// 4. 转换为字符串：将 rune 切片转为字符串
//
// 设计考虑：
// - 字符集选择：字母+数字，适合大多数场景（密码、令牌等）
// - 预分配切片：避免 append 操作带来的内存重新分配
// - 使用 RandomInt：封装随机数生成逻辑，代码更清晰
//
// 好处：
// - 性能优秀：预分配切片，避免动态扩容
// - 字符集丰富：62 个字符提供足够的随机性
// - 通用性强：可用于生成密码、令牌、ID 等
//
// 使用场景：
// - 生成随机密码
// - 生成 API 密钥
// - 生成临时令牌
// - 生成唯一标识符
func RandomString(n int) string {
	// 定义字符集：大小写字母 + 数字，共 62 个字符
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	// 预分配长度为 n 的 rune 切片，避免动态扩容
	b := make([]rune, n)
	// 随机选择字符填充切片
	for i := range b {
		// 从字符集中随机选择一个字符
		b[i] = letters[RandomInt(0, len(letters))]
	}
	// 将 rune 切片转换为字符串
	return string(b)
}

// RandomInt 生成 [min, max) 范围内的随机整数
//
// 为什么这样实现：
// 1. rand.Intn(n) 生成 [0, n) 范围的随机数
// 2. 通过 min + rand.Intn(max-min) 将范围平移到 [min, max)
// 3. 注意：范围是左闭右开 [min, max)，不包含 max
//
// 设计考虑：
// - 使用标准库 rand：简单可靠，无需引入第三方库
// - 范围计算：max-min 得到区间长度，加上 min 实现平移
// - 左闭右开：符合 Go 语言的习惯（如切片、循环等）
//
// 好处：
// - 简洁高效：一行代码实现范围随机数
// - 标准库：使用标准库，无需额外依赖
// - 通用性强：可用于各种随机数生成场景
//
// 注意：
// - 范围是 [min, max)，不包含 max
// - 如果需要在 [min, max] 范围内，应使用 RandomInt(min, max+1)
//
// 示例：RandomInt(0, 10) 返回 [0, 9] 的随机数
func RandomInt(min, max int) int {
	// rand.Intn(max-min) 生成 [0, max-min) 范围的随机数
	// 加上 min 后，范围变为 [min, max)
	return min + rand.Intn(max-min)
}

// BuildTree 将扁平的节点列表构建为树形结构
//
// 为什么使用泛型：
// 1. 类型安全：泛型确保类型一致性，避免类型断言
// 2. 代码复用：一个函数可以处理所有实现 TreeNode 接口的类型
// 3. 编译时检查：类型不匹配会在编译时报错
//
// 为什么使用接口约束：
// 1. 多态性：通过 TreeNode 接口定义树节点的通用行为
// 2. 解耦：不依赖具体的节点类型，只要实现接口即可
// 3. 扩展性：新增节点类型只需实现接口，无需修改此函数
//
// 算法设计（三遍遍历）：
// 第一遍：建立 ID 到节点的映射，O(1) 查找父节点
// 第二遍：建立父子关系，将子节点添加到父节点的 Children 中
// 第三遍：找出所有根节点（ParentID == 0）
//
// 为什么使用 map：
// 1. 查找效率：O(1) 时间复杂度查找父节点，比 O(n) 遍历快得多
// 2. 空间换时间：使用 map 存储节点，提高查找效率
//
// 设计考虑：
// - 使用 map[int]T：以节点 ID 为 key，节点为 value
// - ParentID == 0 表示根节点：这是常见的约定
// - 三遍遍历：虽然遍历三次，但每遍都是 O(n)，总体 O(n)
//
// 好处：
// - 时间复杂度：O(n)，线性时间复杂度，效率高
// - 空间复杂度：O(n)，使用 map 存储节点
// - 通用性强：适用于所有树形结构（菜单、分类、组织架构等）
// - 类型安全：泛型保证类型一致性
//
// 使用场景：
// - 菜单树构建
// - 分类树构建
// - 组织架构树构建
// - 评论树构建
//
// 示例：
// 输入：[{ID:1, ParentID:0}, {ID:2, ParentID:1}, {ID:3, ParentID:1}]
// 输出：[{ID:1, Children:[{ID:2}, {ID:3}]}]
func BuildTree[T common.TreeNode[T]](nodes []T) []T {
	// ========== 第一遍遍历：建立 ID 到节点的映射 ==========
	// 目的：创建一个哈希表（map），通过节点 ID 快速查找节点
	// 为什么需要这一步？
	//   如果不建立映射，在第二遍遍历时，每次查找父节点都需要遍历整个数组，时间复杂度是 O(n²)
	//   建立映射后，查找父节点的时间复杂度降为 O(1)，总体时间复杂度降为 O(n)
	//
	// 数据结构说明：
	//   - key: 节点的 ID（int 类型）
	//   - value: 节点本身（类型 T，实现了 TreeNode 接口）
	nodeMap := make(map[int]T)

	// 遍历所有节点，将每个节点存入 map
	// 例如：如果节点 ID=1，则 nodeMap[1] = 节点1
	//      如果节点 ID=2，则 nodeMap[2] = 节点2
	for i := range nodes {
		// nodes[i].GetID() 获取当前节点的 ID
		// 将这个节点存入 map，以 ID 作为 key
		// 这样后续就可以通过 ID 快速找到对应的节点了
		nodeMap[nodes[i].GetID()] = nodes[i]
	}

	// ========== 第二遍遍历：建立父子关系 ==========
	// 目的：将每个子节点添加到其父节点的 Children 列表中
	// 这一步是构建树形结构的关键：通过 ParentID 找到父节点，然后将当前节点作为子节点添加进去
	//
	// 工作原理：
	//   1. 遍历每个节点
	//   2. 如果节点有父节点（ParentID != 0），说明它不是根节点
	//   3. 通过 ParentID 从 map 中快速找到父节点
	//   4. 调用父节点的 SetChildren 方法，将当前节点添加到父节点的子节点列表中
	for i := range nodes {
		// 检查当前节点是否有父节点
		// ParentID == 0 表示这是根节点，没有父节点
		// ParentID != 0 表示这是子节点，需要找到父节点并建立关系
		if nodes[i].GetParentID() != 0 {
			// 通过 ParentID 从 map 中查找父节点
			// 例如：如果当前节点的 ParentID = 1，则 parent = nodeMap[1]
			// 这一步的时间复杂度是 O(1)，因为 map 的查找是常数时间
			parent := nodeMap[nodes[i].GetParentID()]

			// 将当前节点添加到父节点的 Children 列表中
			// SetChildren 方法内部会执行类似 parent.Children = append(parent.Children, nodes[i]) 的操作
			// 这样父节点就包含了它的所有子节点，形成了树形结构
			parent.SetChildren(nodes[i])
		}
		// 注意：如果 ParentID == 0，说明是根节点，暂时跳过，在第三遍遍历中处理
	}

	// ========== 第三遍遍历：找出所有根节点 ==========
	// 目的：从所有节点中筛选出根节点（ParentID == 0 的节点），作为树的顶层节点返回
	// 为什么需要这一步？
	//   树形结构通常需要从根节点开始展示，所以需要找出所有根节点
	//   根节点的特征是 ParentID == 0（表示没有父节点）
	//
	// 注意：这里遍历的是 nodeMap 而不是 nodes
	//   因为 nodeMap 包含了所有节点，而且通过 map 遍历可以确保每个节点只处理一次
	var rootNodes []T

	// 遍历 map 中的所有节点
	// i 是 map 的 key（即节点的 ID）
	// nodeMap[i] 是对应的节点
	for i := range nodeMap {
		// 检查节点的 ParentID 是否为 0
		// 如果是 0，说明这是根节点，应该添加到结果列表中
		if nodeMap[i].GetParentID() == 0 {
			// 将根节点添加到结果列表中
			// 最终返回的 rootNodes 就是构建好的树形结构的顶层节点
			// 每个根节点都包含了它的所有子节点（通过 Children 字段），形成了完整的树
			rootNodes = append(rootNodes, nodeMap[i])
		}
	}

	// 返回所有根节点
	// 如果只有一个根节点，返回的列表长度为 1
	// 如果有多个根节点（多棵树），返回的列表长度为多个
	return rootNodes
}
