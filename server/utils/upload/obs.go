package upload

import (
	"mime/multipart"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	"github.com/pkg/errors"
)

// HuaWeiObs 是一个全局的华为 OBS 上传工具实例
// 使用全局单例的意义：
// 1. 方便在整个项目中直接通过 upload.HuaWeiObs 调用，无需到处 new
// 2. 该结构体本身是无状态的（不保存连接、配置等），全局复用是安全的
// 3. 遵循 gin-vue-admin 其他上传模块（七牛、腾讯云等）的统一写法，便于扩展和维护
var HuaWeiObs = new(Obs)

// Obs 是对华为云 OBS SDK 的一层封装
// 不在这里保存具体配置和客户端对象，而是按需创建客户端：
// 1. 避免长期持有连接导致的资源占用问题
// 2. 读取配置统一从 global.GVA_CONFIG 中获取，便于热更新 / 统一管理
// 3. 让该类型保持“轻量、无状态”，方便在整个项目中复用
type Obs struct{}

// NewHuaWeiObsClient 创建一个新的华为 OBS 客户端
// 这么写的好处：
// 1. 将 SDK 初始化逻辑集中在一个函数里，调用方不需要关心 AccessKey/SecretKey/Endpoint 等细节
// 2. 如果以后需要为 OBS 客户端增加统一的日志、代理、超时等配置，只需修改这一处即可
// 3. 使用 global.GVA_CONFIG 统一读取配置，符合项目整体配置管理风格
func NewHuaWeiObsClient() (client *obs.ObsClient, err error) {
	return obs.New(global.GVA_CONFIG.HuaWeiObs.AccessKey, global.GVA_CONFIG.HuaWeiObs.SecretKey, global.GVA_CONFIG.HuaWeiObs.Endpoint)
}

// UploadFile 将一个表单上传的文件（*multipart.FileHeader）上传到华为 OBS
// 返回值依次为：完整访问路径、文件名、错误信息
// 设计上的考虑与好处：
// 1. 对上层调用者屏蔽底层 obs SDK 的复杂结构，只暴露简单的 filepath/filename
// 2. 统一在这里设置 Bucket、Key、ContentType 等信息，调用方便、减少重复代码
// 3. 使用 errors.Wrap 包装错误，保留底层错误信息的同时加上业务语义，便于日志排查
func (o *Obs) UploadFile(file *multipart.FileHeader) (string, string, error) {
	// 通过 *multipart.FileHeader 打开实际文件流
	// 这样可以直接用于 obs.PutObject 的 Body 字段，避免中间再做一次磁盘读写，提高性能
	open, err := file.Open()
	if err != nil {
		// 这里直接返回底层错误：
		// 调用方可以知道是“打开文件失败”，而不是 OBS 操作失败，方便定位问题
		return "", "", err
	}
	// 使用 defer 确保文件句柄在函数结束时被关闭：
	// 1. 避免遗漏 Close 导致的句柄泄露
	// 2. 让资源管理逻辑简洁且不易出错
	defer open.Close()

	// 直接使用上传时的原始文件名作为对象存储的 Key
	// 优点：
	// 1. 调试时更容易在 OBS 控制台找到对应文件
	// 2. 与用户上传文件名保持一致，用户体验更好
	// 注意：如果业务需要防重名或加目录前缀，可在此处做统一处理
	filename := file.Filename

	// 构造 PutObjectInput：
	// - Bucket: 使用配置中的默认桶，避免硬编码
	// - Key: 使用 filename 作为对象键
	// - ContentType: 从请求头中透传 content-type，保持原始 MIME 信息
	input := &obs.PutObjectInput{
		PutObjectBasicInput: obs.PutObjectBasicInput{
			ObjectOperationInput: obs.ObjectOperationInput{
				Bucket: global.GVA_CONFIG.HuaWeiObs.Bucket,
				Key:    filename,
			},
			HttpHeader: obs.HttpHeader{
				ContentType: file.Header.Get("content-type"),
			},
		},
		Body: open,
	}

	// 每次上传时按需创建客户端：
	// 1. 避免全局长连接在配置变更后不能及时生效的问题
	// 2. 结合 SDK 的内部连接池策略，由 SDK 自身决定连接管理
	var client *obs.ObsClient
	client, err = NewHuaWeiObsClient()
	if err != nil {
		// 使用 errors.Wrap 增加“获取华为对象存储对象失败”这样的业务语义
		// 日志中能看到更清晰的错误链路
		return "", "", errors.Wrap(err, "获取华为对象存储对象失败!")
	}

	// 调用 obs SDK 执行真正的上传
	_, err = client.PutObject(input)
	if err != nil {
		// 同样使用 Wrap，为“文件上传失败”增加上下文，方便排障
		return "", "", errors.Wrap(err, "文件上传失败!")
	}

	// 拼接最终对外暴露的访问路径：
	// Path 一般为配置好的前缀（如 CDN 域名/统一目录），这里统一拼接有利于后续替换或升级路径结构
	filepath := global.GVA_CONFIG.HuaWeiObs.Path + "/" + filename
	return filepath, filename, err
}

// DeleteFile 根据对象 Key 删除华为 OBS 中的文件
// 这么抽象一个方法的意义：
// 1. 上层业务只需要关心“传入资源 Key”，不需要了解 obs.DeleteObject 的细节
// 2. 如果后续要增加“逻辑删除”“回收站”等功能，也可以在此方法中扩展，而不影响上层调用
func (o *Obs) DeleteFile(key string) error {
	// 与上传相同，这里也是按需创建客户端，保证配置更新可即时生效
	client, err := NewHuaWeiObsClient()
	if err != nil {
		return errors.Wrap(err, "获取华为对象存储对象失败!")
	}

	// 构造删除对象的输入参数：
	// - Bucket: 从全局配置读取，保持统一
	// - Key: 由调用方传入，支持精确删除某一个对象
	input := &obs.DeleteObjectInput{
		Bucket: global.GVA_CONFIG.HuaWeiObs.Bucket,
		Key:    key,
	}

	// 这里将 output 单独声明出来，以便在错误信息中打印出来辅助排查
	var output *obs.DeleteObjectOutput
	output, err = client.DeleteObject(input)
	if err != nil {
		// 使用 Wrapf，把 key 和 output 一并输出，有助于快速发现是“对象不存在”还是“权限不足”等问题
		return errors.Wrapf(err, "删除对象(%s)失败!, output: %v", key, output)
	}
	return nil
}
