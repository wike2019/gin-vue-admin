package upload

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"

	"github.com/tencentyun/cos-go-sdk-v5"
	"go.uber.org/zap"
)

// TencentCOS 是腾讯云 COS 实现类
// 通过实现统一的上传/删除接口，可以在不改动业务代码的前提下，
// 在多种对象存储（本地、七牛、阿里云、腾讯云等）之间自由切换。
// 这里本身不保存状态，因此设计成空 struct，更轻量。
type TencentCOS struct{}

// UploadFile 上传文件到腾讯云 COS
//  1. 统一接收 gin 等框架中上传的 *multipart.FileHeader，方便在控制器层直接调用。
//  2. 使用当前时间戳 + 原始文件名 作为 fileKey，既能保证较高的唯一性，又保留原文件名信息，
//     方便排查和对接前端展示。
//  3. 具体的 Bucket、Region、PathPrefix、Secret 等统一从 global.GVA_CONFIG 中读取，
//     可以通过配置文件切换环境，而无需改动代码，利于配置中心和多环境部署。
func (*TencentCOS) UploadFile(file *multipart.FileHeader) (string, string, error) {
	client := NewClient()
	f, openError := file.Open()
	if openError != nil {
		// 统一使用全局 logger 打日志，方便在系统日志中定位问题。
		global.GVA_LOG.Error("function file.Open() failed", zap.Any("err", openError.Error()))
		return "", "", errors.New("function file.Open() failed, err:" + openError.Error())
	}
	// 使用 defer 关闭文件句柄，避免因为中途 return 导致资源泄漏。
	defer f.Close()

	// 使用 Unix 时间戳拼接原文件名，简单易懂，同时大幅降低覆盖风险。
	fileKey := fmt.Sprintf("%d%s", time.Now().Unix(), file.Filename)

	_, err := client.Object.Put(context.Background(), global.GVA_CONFIG.TencentCOS.PathPrefix+"/"+fileKey, f, nil)
	if err != nil {
		// 这里直接 panic，通常配合全局恢复中间件使用，可以在接口层统一返回错误，
		// 同时中断后续逻辑，避免产生“上传失败但仍继续使用”的错误状态。
		panic(err)
	}
	// 返回值拆分为：完整访问 URL + 文件 key
	// - 完整 URL 方便前端直接使用；
	// - 文件 key 则在需要删除/替换等操作时更灵活，不依赖完整 URL。
	return global.GVA_CONFIG.TencentCOS.BaseURL + "/" + global.GVA_CONFIG.TencentCOS.PathPrefix + "/" + fileKey, fileKey, nil
}

// DeleteFile 从腾讯云 COS 删除文件
// 仅依赖文件 key，而不是完整 URL，使得存储、删除逻辑更清晰，也便于以后更换域名。
func (*TencentCOS) DeleteFile(key string) error {
	client := NewClient()
	name := global.GVA_CONFIG.TencentCOS.PathPrefix + "/" + key
	_, err := client.Object.Delete(context.Background(), name)
	if err != nil {
		// 同样使用统一日志组件记录错误，便于排查线上问题。
		global.GVA_LOG.Error("function bucketManager.Delete() failed", zap.Any("err", err.Error()))
		return errors.New("function bucketManager.Delete() failed, err:" + err.Error())
	}
	return nil
}

// NewClient 初始化腾讯云 COS 客户端
//  1. 通过配置拼接出 Bucket 级别的访问域名，避免硬编码域名，便于不同环境复用。
//  2. 使用 cos-go-sdk-v5 提供的 AuthorizationTransport 自动完成请求签名，
//     开发者只关心业务逻辑，不需要手写签名算法，减少出错可能。
//  3. 将 client 抽成独立函数，Upload/Delete 等方法都统一通过该函数创建，
//     代码更集中，也方便以后做连接池、超时设置等统一扩展。
func NewClient() *cos.Client {
	urlStr, _ := url.Parse("https://" + global.GVA_CONFIG.TencentCOS.Bucket + ".cos." + global.GVA_CONFIG.TencentCOS.Region + ".myqcloud.com")
	baseURL := &cos.BaseURL{BucketURL: urlStr}
	client := cos.NewClient(baseURL, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  global.GVA_CONFIG.TencentCOS.SecretID,
			SecretKey: global.GVA_CONFIG.TencentCOS.SecretKey,
		},
	})
	return client
}
