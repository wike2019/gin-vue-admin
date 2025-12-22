package utils

import (
	"errors"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Rules 定义验证规则映射类型
// key: 结构体字段名
// value: 该字段需要满足的验证规则列表（字符串数组）
// 设计思路：使用map结构可以灵活地为不同字段配置不同的验证规则
// 好处：支持一个字段配置多个验证规则，例如同时验证非空和长度范围
type Rules map[string][]string

// RulesMap 定义规则方案映射类型
// 用于存储多个不同的验证规则方案，每个方案对应一个key
// 设计思路：允许为不同的业务场景定义不同的验证规则集合
// 好处：可以在不同接口或场景中复用预定义的验证规则，提高代码复用性
type RulesMap map[string]Rules

// CustomizeMap 全局自定义规则注册表
// 用于存储通过RegisterRule注册的验证规则方案
// 设计思路：使用全局变量实现规则方案的集中管理和快速查找
// 好处：可以在应用启动时注册规则，运行时直接通过key获取，避免重复定义
var CustomizeMap = make(map[string]Rules)

//@author: [piexlmax](https://github.com/piexlmax)
//@function: RegisterRule
//@description: 注册自定义规则方案建议在路由初始化层即注册
//@param: key string, rule Rules
//@return: err error

// RegisterRule 注册自定义验证规则方案
// 设计思路：
// 1. 通过key-value方式存储规则方案，便于管理和复用
// 2. 防止重复注册，确保规则方案的唯一性
// 3. 建议在路由初始化时注册，避免运行时注册带来的并发问题
// 好处：
// - 规则方案可复用：一次注册，多处使用
// - 集中管理：所有规则方案统一管理，便于维护
// - 防止冲突：重复注册检查避免规则被意外覆盖
func RegisterRule(key string, rule Rules) (err error) {
	// 检查是否已注册，防止重复注册导致规则被覆盖
	// 这样设计的好处是：如果key已存在，说明可能是配置错误，及时发现问题
	if CustomizeMap[key] != nil {
		return errors.New(key + "已注册,无法重复注册")
	} else {
		// 注册新的规则方案到全局映射表
		CustomizeMap[key] = rule
		return nil
	}
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: NotEmpty
//@description: 非空 不能为其对应类型的0值
//@return: string

// NotEmpty 返回非空验证规则标识符
// 设计思路：使用字符串常量作为规则标识，简单直观
// 好处：
// - 类型安全：通过函数返回，避免直接使用字符串常量可能出现的拼写错误
// - 语义清晰：函数名直接表达验证意图
// - 易于扩展：未来可以在函数内部添加参数或逻辑
func NotEmpty() string {
	return "notEmpty"
}

// @author: [zooqkl](https://github.com/zooqkl)
// @function: RegexpMatch
// @description: 正则校验 校验输入项是否满足正则表达式
// @param:  rule string
// @return: string

// RegexpMatch 返回正则表达式验证规则
// 设计思路：使用"regexp="前缀 + 正则表达式的格式，便于解析
// 好处：
// - 统一格式：所有规则都使用字符串格式，便于统一解析
// - 灵活性强：支持任意正则表达式，满足复杂验证需求
// - 易于识别：通过前缀快速识别规则类型
func RegexpMatch(rule string) string {
	return "regexp=" + rule
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Lt
//@description: 小于入参(<) 如果为string array Slice则为长度比较 如果是 int uint float 则为数值比较
//@param: mark string
//@return: string

// Lt 返回"小于"比较规则
// 设计思路：使用"操作符=值"的字符串格式，便于后续解析
// 好处：
// - 统一格式：所有比较操作使用相同格式，代码一致性好
// - 自动适配：根据字段类型自动判断是长度比较还是数值比较（见compareVerify函数）
// - 使用示例：Lt("10") 表示小于10（数值）或长度小于10（字符串/数组）
func Lt(mark string) string {
	return "lt=" + mark
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Le
//@description: 小于等于入参(<=) 如果为string array Slice则为长度比较 如果是 int uint float 则为数值比较
//@param: mark string
//@return: string

// Le 返回"小于等于"比较规则
// 设计思路：同Lt函数，使用统一的字符串格式
// 好处：语义清晰，支持数值和长度的统一比较逻辑
func Le(mark string) string {
	return "le=" + mark
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Eq
//@description: 等于入参(==) 如果为string array Slice则为长度比较 如果是 int uint float 则为数值比较
//@param: mark string
//@return: string

// Eq 返回"等于"比较规则
// 设计思路：同Lt函数，使用统一的字符串格式
// 好处：支持精确匹配验证，适用于固定长度或固定值的场景
func Eq(mark string) string {
	return "eq=" + mark
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Ne
//@description: 不等于入参(!=)  如果为string array Slice则为长度比较 如果是 int uint float 则为数值比较
//@param: mark string
//@return: string

// Ne 返回"不等于"比较规则
// 设计思路：同Lt函数，使用统一的字符串格式
// 好处：支持排除特定值的验证场景
func Ne(mark string) string {
	return "ne=" + mark
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Ge
//@description: 大于等于入参(>=) 如果为string array Slice则为长度比较 如果是 int uint float 则为数值比较
//@param: mark string
//@return: string

// Ge 返回"大于等于"比较规则
// 设计思路：同Lt函数，使用统一的字符串格式
// 好处：支持最小值验证，常用于确保字符串长度或数值满足最低要求
func Ge(mark string) string {
	return "ge=" + mark
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: Gt
//@description: 大于入参(>) 如果为string array Slice则为长度比较 如果是 int uint float 则为数值比较
//@param: mark string
//@return: string

// Gt 返回"大于"比较规则
// 设计思路：同Lt函数，使用统一的字符串格式
// 好处：支持严格大于的验证场景
func Gt(mark string) string {
	return "gt=" + mark
}

//
//@author: [piexlmax](https://github.com/piexlmax)
//@function: Verify
//@description: 校验方法
//@param: st interface{}, roleMap Rules(入参实例，规则map)
//@return: err error

// Verify 核心验证函数，使用反射机制对结构体进行验证
// 设计思路：
// 1. 使用反射(reflect)实现通用验证，无需为每个结构体编写特定验证代码
// 2. 支持嵌套结构体递归验证，处理复杂数据结构
// 3. 通过规则映射表(roleMap)实现灵活的验证规则配置
// 4. 使用字符串解析识别不同的验证规则类型
// 好处：
// - 通用性强：一个函数可以验证任意结构体，减少重复代码
// - 灵活配置：通过外部传入规则映射，无需修改验证函数即可适配不同需求
// - 支持嵌套：自动处理嵌套结构体，满足复杂业务场景
// - 早期返回：遇到第一个验证失败立即返回，提高性能
func Verify(st interface{}, roleMap Rules) (err error) {
	// 定义比较操作符映射表，用于快速判断是否为比较类规则
	// 使用map查找O(1)时间复杂度，比字符串比较更高效
	// 好处：集中管理所有比较操作符，便于维护和扩展
	compareMap := map[string]bool{
		"lt": true,
		"le": true,
		"eq": true,
		"ne": true,
		"ge": true,
		"gt": true,
	}

	// 获取结构体的类型信息和值信息
	// 使用反射可以在运行时获取结构体的字段信息，实现通用验证
	typ := reflect.TypeOf(st)
	val := reflect.ValueOf(st) // 获取reflect.Value类型，用于访问字段值

	// 检查输入是否为结构体类型
	// 这样设计的好处：类型安全，避免对非结构体类型进行无效验证
	kd := val.Kind() // 获取到st对应的类别
	if kd != reflect.Struct {
		return errors.New("expect struct")
	}

	// 获取结构体字段数量，用于遍历所有字段
	num := val.NumField()

	// 遍历结构体的所有字段，逐一进行验证
	// 设计思路：通过反射遍历，无需硬编码字段名，实现通用验证
	for i := 0; i < num; i++ {
		// 获取字段的类型信息（包含字段名、标签等元数据）
		tagVal := typ.Field(i)
		// 获取字段的实际值
		val := val.Field(i)

		// 递归处理嵌套结构体
		// 设计思路：如果字段本身是结构体，递归调用Verify进行验证
		// 好处：支持多层嵌套的复杂数据结构验证，无需额外处理
		if tagVal.Type.Kind() == reflect.Struct {
			if err = Verify(val.Interface(), roleMap); err != nil {
				return err
			}
		}

		// 检查该字段是否配置了验证规则
		// 如果没有配置规则，跳过验证（允许字段不验证）
		// 好处：灵活性高，只验证需要验证的字段
		if len(roleMap[tagVal.Name]) > 0 {
			// 遍历该字段的所有验证规则（一个字段可以有多个规则）
			// 好处：支持组合验证，例如同时验证非空和长度范围
			for _, v := range roleMap[tagVal.Name] {
				switch {
				// 非空验证：检查字段是否为对应类型的零值
				case v == "notEmpty":
					if isBlank(val) {
						return errors.New(tagVal.Name + "值不能为空")
					}
				// 正则表达式验证：使用正则匹配验证格式
				// 设计思路：通过"regexp="前缀识别，提取正则表达式进行匹配
				case strings.Split(v, "=")[0] == "regexp":
					// 解析规则字符串，提取正则表达式部分
					// 注意：这里假设规则格式为"regexp=pattern"
					if !regexpMatch(strings.Split(v, "=")[1], val.String()) {
						return errors.New(tagVal.Name + "格式校验不通过")
					}
				// 比较类验证：数值或长度比较
				// 设计思路：通过compareMap快速判断是否为比较操作
				// 好处：使用map查找比多次字符串比较更高效
				case compareMap[strings.Split(v, "=")[0]]:
					if !compareVerify(val, v) {
						return errors.New(tagVal.Name + "长度或值不在合法范围," + v)
					}
				}
			}
		}
	}
	return nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: compareVerify
//@description: 长度和数字的校验方法 根据类型自动校验
//@param: value reflect.Value, VerifyStr string
//@return: bool

// compareVerify 根据字段类型自动选择合适的比较方式
// 设计思路：
// 1. 使用类型开关(type switch)根据字段类型选择处理方式
// 2. 字符串和数组使用长度比较，数值类型使用数值比较
// 3. 统一调用compare函数进行实际比较，避免代码重复
// 好处：
// - 自动适配：根据类型自动判断比较方式，无需手动指定
// - 统一接口：所有类型最终都调用compare函数，逻辑集中
// - 类型安全：通过反射获取正确的类型值，避免类型转换错误
// - 支持中文：字符串使用rune计算长度，正确处理多字节字符
func compareVerify(value reflect.Value, VerifyStr string) bool {
	switch value.Kind() {
	// 字符串类型：使用rune长度进行比较
	// 设计思路：使用[]rune而不是len()直接计算，可以正确处理中文字符
	// 好处：一个中文字符算作1个长度单位，符合业务预期
	case reflect.String:
		return compare(len([]rune(value.String())), VerifyStr)
	// 切片和数组类型：使用元素数量进行比较
	// 设计思路：对于集合类型，比较的是元素数量而不是内容
	// 好处：可以验证数组/切片的长度范围
	case reflect.Slice, reflect.Array:
		return compare(value.Len(), VerifyStr)
	// 无符号整数类型：使用数值进行比较
	// 设计思路：统一处理所有无符号整数类型（uint8, uint16, uint32, uint64等）
	// 好处：代码简洁，无需为每种类型单独处理
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return compare(value.Uint(), VerifyStr)
	// 浮点数类型：使用数值进行比较
	// 设计思路：统一处理float32和float64
	// 好处：支持小数验证，满足精度要求较高的场景
	case reflect.Float32, reflect.Float64:
		return compare(value.Float(), VerifyStr)
	// 有符号整数类型：使用数值进行比较
	// 设计思路：统一处理所有有符号整数类型（int8, int16, int32, int64等）
	// 好处：代码简洁，支持负数验证
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return compare(value.Int(), VerifyStr)
	// 其他类型：不支持比较，返回false
	// 设计思路：对于不支持的类型，保守处理，返回验证失败
	// 好处：类型安全，避免对不支持的类型进行错误比较
	default:
		return false
	}
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: isBlank
//@description: 非空校验
//@param: value reflect.Value
//@return: bool

// isBlank 检查值是否为空（零值）
// 设计思路：
// 1. 根据不同类型使用不同的零值判断方式
// 2. 对于基本类型，直接与零值比较
// 3. 对于复杂类型，使用反射的DeepEqual进行深度比较
// 好处：
// - 类型精确：针对不同类型使用最合适的判断方式
// - 全面覆盖：支持所有常见类型的零值判断
// - 语义准确：正确识别各种类型的"空"状态
func isBlank(value reflect.Value) bool {
	switch value.Kind() {
	// 字符串和切片：长度为0即为空
	// 设计思路：使用Len()方法判断，简单高效
	// 好处：直接判断长度，无需转换为字符串比较
	case reflect.String, reflect.Slice:
		return value.Len() == 0
	// 布尔类型：false视为空
	// 设计思路：布尔类型的零值是false，所以!value.Bool()表示为空
	// 好处：符合业务语义，false通常表示"未设置"或"无效"
	case reflect.Bool:
		return !value.Bool()
	// 有符号整数：0视为空
	// 设计思路：整数的零值是0，直接比较
	// 好处：简单直接，性能好
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	// 无符号整数：0视为空
	// 设计思路：同有符号整数，0是零值
	// 好处：统一处理所有整数类型
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	// 浮点数：0.0视为空
	// 设计思路：浮点数的零值是0.0
	// 注意：这里使用==比较，对于浮点数可能存在精度问题，但零值比较是安全的
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	// 接口和指针：nil视为空
	// 设计思路：使用IsNil()方法判断，这是最安全的方式
	// 好处：正确处理nil值，避免空指针异常
	case reflect.Interface, reflect.Ptr:
		return value.IsNil()
	}
	// 其他类型：使用深度比较判断是否为零值
	// 设计思路：对于复杂类型（如结构体、map等），使用DeepEqual与零值比较
	// 好处：通用性强，可以处理任意类型的零值判断
	// 原理：reflect.Zero获取该类型的零值，然后深度比较
	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}

//@author: [piexlmax](https://github.com/piexlmax)
//@function: compare
//@description: 比较函数
//@param: value interface{}, VerifyStr string
//@return: bool

// compare 执行实际的数值或长度比较操作
// 设计思路：
// 1. 解析规则字符串（格式：操作符=值，如"lt=10"）
// 2. 根据值的类型进行类型转换和比较
// 3. 支持6种比较操作：lt(<), le(<=), eq(==), ne(!=), ge(>=), gt(>)
// 好处：
// - 类型安全：根据实际类型进行转换，避免类型错误
// - 统一处理：所有比较操作集中在一个函数，便于维护
// - 错误处理：解析失败时返回false，保证验证的健壮性
func compare(value interface{}, VerifyStr string) bool {
	// 解析规则字符串，格式为"操作符=值"
	// 例如："lt=10" 表示小于10
	// 设计思路：使用"="作为分隔符，简单直观
	VerifyStrArr := strings.Split(VerifyStr, "=")

	// 获取值的反射信息，用于类型判断和值提取
	val := reflect.ValueOf(value)

	switch val.Kind() {
	// 有符号整数类型比较
	// 设计思路：使用ParseInt解析规则中的数值，支持64位整数
	// 好处：可以处理大整数，支持所有有符号整数类型
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// 解析规则中的整数值，使用10进制，64位精度
		VInt, VErr := strconv.ParseInt(VerifyStrArr[1], 10, 64)
		if VErr != nil {
			// 解析失败，返回验证失败
			// 好处：规则格式错误时不会导致panic，而是返回验证失败
			return false
		}
		// 根据操作符执行相应的比较
		// 设计思路：使用switch语句，清晰易读
		switch {
		case VerifyStrArr[0] == "lt":
			return val.Int() < VInt
		case VerifyStrArr[0] == "le":
			return val.Int() <= VInt
		case VerifyStrArr[0] == "eq":
			return val.Int() == VInt
		case VerifyStrArr[0] == "ne":
			return val.Int() != VInt
		case VerifyStrArr[0] == "ge":
			return val.Int() >= VInt
		case VerifyStrArr[0] == "gt":
			return val.Int() > VInt
		default:
			// 未知操作符，返回验证失败
			return false
		}
	// 无符号整数类型比较
	// 设计思路：使用Atoi解析（因为规则中的值通常是正数），然后转换为uint64
	// 注意：这里使用Atoi而不是ParseUint，可能是为了简化代码
	// 潜在问题：如果规则值超过int范围可能会有问题，但实际使用中通常不会
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		VInt, VErr := strconv.Atoi(VerifyStrArr[1])
		if VErr != nil {
			return false
		}
		// 转换为uint64进行比较，确保类型匹配
		switch {
		case VerifyStrArr[0] == "lt":
			return val.Uint() < uint64(VInt)
		case VerifyStrArr[0] == "le":
			return val.Uint() <= uint64(VInt)
		case VerifyStrArr[0] == "eq":
			return val.Uint() == uint64(VInt)
		case VerifyStrArr[0] == "ne":
			return val.Uint() != uint64(VInt)
		case VerifyStrArr[0] == "ge":
			return val.Uint() >= uint64(VInt)
		case VerifyStrArr[0] == "gt":
			return val.Uint() > uint64(VInt)
		default:
			return false
		}
	// 浮点数类型比较
	// 设计思路：使用ParseFloat解析，64位精度
	// 好处：支持小数验证，满足精度要求
	case reflect.Float32, reflect.Float64:
		VFloat, VErr := strconv.ParseFloat(VerifyStrArr[1], 64)
		if VErr != nil {
			return false
		}
		// 浮点数比较，注意精度问题
		// 对于零值比较是安全的，对于其他值可能存在精度误差
		switch {
		case VerifyStrArr[0] == "lt":
			return val.Float() < VFloat
		case VerifyStrArr[0] == "le":
			return val.Float() <= VFloat
		case VerifyStrArr[0] == "eq":
			return val.Float() == VFloat
		case VerifyStrArr[0] == "ne":
			return val.Float() != VFloat
		case VerifyStrArr[0] == "ge":
			return val.Float() >= VFloat
		case VerifyStrArr[0] == "gt":
			return val.Float() > VFloat
		default:
			return false
		}
	// 其他类型不支持比较
	// 设计思路：保守处理，返回验证失败
	default:
		return false
	}
}

// regexpMatch 使用正则表达式匹配字符串
// 设计思路：
// 1. 使用MustCompile编译正则表达式（如果编译失败会panic）
// 2. 使用MatchString进行匹配
// 好处：
// - 简单直接：一行代码完成正则匹配
// - 性能考虑：MustCompile在编译失败时panic，适合在规则注册时发现问题
// 注意：
// - MustCompile在正则表达式无效时会panic，但这通常发生在规则定义阶段
// - 如果需要在运行时处理无效正则，应该使用Compile并检查错误
// - 当前设计假设规则在注册时已经验证过，所以使用MustCompile是合理的
func regexpMatch(rule, matchStr string) bool {
	return regexp.MustCompile(rule).MatchString(matchStr)
}
