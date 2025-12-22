package upload

import (
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"go.uber.org/zap"
)

// CloudflareR2 实现了 gin-vue-admin 中统一的上传接口（统一上传适配层）。
// 使用一个“空结构体 + 方法”的方式有这些好处：
// 1. 通过接口统一管理本地、七牛、COS、MinIO、R2 等不同后端，调用方只依赖接口，不关心具体实现；
// 2. 不在结构体中保存状态，所有配置都从全局配置读取，简化生命周期和依赖注入；
// 3. 后续要替换存储实现或调整 R2 接入方式，只要改这里，不影响上层业务逻辑。
type CloudflareR2 struct{}

// UploadFile 把前端上传的文件保存到 Cloudflare R2。
// 返回 fileUrl 和 fileName 的原因：
// - fileUrl：完整可访问地址，给前端或其他服务直接访问使用；
// - fileName：对象存储内部真正的 Key（带路径前缀），便于后续删除或做其他操作。
func (c *CloudflareR2) UploadFile(file *multipart.FileHeader) (fileUrl string, fileName string, err error) {
	// 每次上传时都创建一个新的 Session，基于 AWS S3 兼容协议访问 Cloudflare R2：
	// - 复用成熟、稳定的 AWS SDK 生态；
	// - Session 是并发安全的，适合在高并发场景下使用。
	session := c.newSession()
	// s3manager.Uploader 内部封装了分片上传、重试等细节，调用简单、可靠性更高。
	client := s3manager.NewUploader(session)

	// 使用“时间戳 + 原文件名”作为 key 的一部分：
	// 1. 保证同名文件不会互相覆盖；
	// 2. 同时保留了原始文件名信息，排查问题时更直观。
	fileKey := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)
	// fileName 是写入到 R2 中的完整对象 Key（通常会加上 Path 作为业务前缀）：
	// - 控制台中浏览对象更有层次感；
	// - 可以按模块（如 avatar/、file/）划分存储；
	// - 日后迁移或清理时，可以按前缀快速过滤。
	fileName = fmt.Sprintf("%s/%s", global.GVA_CONFIG.CloudflareR2.Path, fileKey)

	// FileHeader 只是文件信息，这里真正打开底层文件流用于上传。
	f, openError := file.Open()
	if openError != nil {
		// 日志统一使用 zap，结构化输出错误信息，方便线上问题排查。
		global.GVA_LOG.Error("function file.Open() failed", zap.Any("err", openError.Error()))
		return "", "", errors.New("function file.Open() failed, err:" + openError.Error())
	}
	// 使用 defer 确保文件句柄一定被关闭，避免文件描述符泄漏。
	defer f.Close() // 创建文件 defer 关闭

	// 组装 S3 兼容的上传参数：
	// - Bucket：对应 Cloudflare R2 的 Bucket 名；
	// - Key：对象在 Bucket 中的路径（含 Path 前缀）；
	// - Body：文件流。
	input := &s3manager.UploadInput{
		Bucket: aws.String(global.GVA_CONFIG.CloudflareR2.Bucket),
		Key:    aws.String(fileName),
		Body:   f,
	}

	// 实际执行上传，如果发生网络、权限或配置错误，会返回 err。
	_, err = client.Upload(input)
	if err != nil {
		global.GVA_LOG.Error("function uploader.Upload() failed", zap.Any("err", err.Error()))
		return "", "", err
	}

	// 返回值说明：
	// - fileUrl：BaseURL（往往是绑定了 R2 的自定义域名或 CDN 域名） + 对象 Key 拼接，前端直接用；
	// - fileKey：只保存“时间戳_原文件名”这段，用作数据库中的标识字段，避免泄露内部路径结构。
	return fmt.Sprintf("%s/%s", global.GVA_CONFIG.CloudflareR2.BaseURL,
			fileName),
		fileKey,
		nil
}

// DeleteFile 根据业务层保存的 key（fileKey，即“时间戳_原文件名”）删除云端文件。
// 对业务层来说只关心一个简单的 key，内部再负责拼接 Path，保证接口统一、实现可替换。
func (c *CloudflareR2) DeleteFile(key string) error {
	// 删除时同样通过 S3 兼容客户端访问 R2，与上传配置保持一致。
	session := newSession()
	svc := s3.New(session)
	// 还原出实际在 R2 中的完整 Key：Path + "/" + key。
	filename := global.GVA_CONFIG.CloudflareR2.Path + "/" + key
	bucket := global.GVA_CONFIG.CloudflareR2.Bucket

	// 发起删除请求，如果配置、权限等有问题会返回错误。
	_, err := svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
	})
	if err != nil {
		global.GVA_LOG.Error("function svc.DeleteObject() failed", zap.Any("err", err.Error()))
		return errors.New("function svc.DeleteObject() failed, err:" + err.Error())
	}

	// DeleteObject 在服务端是异步执行的，这里通过 WaitUntilObjectNotExists 等待对象真正被删干净：
	// - 调用方在函数返回后可以更确信对象已经不存在；
	// - 避免刚删完立刻访问时出现短暂不一致。
	_ = svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
	})
	return nil
}

// newSession 根据全局配置创建一个访问 Cloudflare R2 的会话。
// Cloudflare R2 提供 S3 兼容协议，因此可以直接使用 AWS 官方 SDK：
// - 降低心智负担，无需再学习一套新的 SDK；
// - 对项目中已经用 S3 协议的其他地方可以复用经验。
func (*CloudflareR2) newSession() *session.Session {
	// Cloudflare R2 的 endpoint 形式为：
	//   <account-id>.r2.cloudflarestorage.com
	// 通过配置 AccountID，就能在不同账号之间复用同一套代码。
	endpoint := fmt.Sprintf("%s.r2.cloudflarestorage.com", global.GVA_CONFIG.CloudflareR2.AccountID)

	return session.Must(session.NewSession(&aws.Config{
		// R2 要求 Region 写成 "auto"，并不会像传统 AWS 区域那样影响物理位置。
		Region:   aws.String("auto"),
		Endpoint: aws.String(endpoint),
		// 使用静态 AccessKeyID / SecretAccessKey 鉴权：
		// - 开发环境可以直接写在配置文件中；
		// - 生产环境可通过环境变量或配置中心注入，方便统一管理和轮换密钥。
		Credentials: credentials.NewStaticCredentials(
			global.GVA_CONFIG.CloudflareR2.AccessKeyID,
			global.GVA_CONFIG.CloudflareR2.SecretAccessKey,
			"",
		),
	}))
}
