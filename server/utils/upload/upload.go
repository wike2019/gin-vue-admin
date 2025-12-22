package upload

import (
	"mime/multipart"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// OSS 对象存储接口
// 通过定义统一的接口，屏蔽不同云厂商/本地存储之间的差异，
// 上层业务只依赖 `OSS` 接口，不直接依赖具体实现，达到“面向接口编程”的目的，
// 这样切换或新增存储实现时，只需要增加结构体并实现该接口即可，业务代码无需改动。
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [ccfish86](https://github.com/ccfish86)
type OSS interface {
	// UploadFile 统一的上传入口
	// 参数：*multipart.FileHeader —— 直接复用 Gin/HTTP 上传得到的文件头，调用方便
	// 返回：完整访问 URL、存储中的 key（或相对路径）、以及错误信息
	// 好处：
	//   1. 业务方拿到 URL 就能直接给前端使用；
	//   2. key 可用于后续删除或做权限控制；
	//   3. 统一约定返回值，便于在不同 OSS 之间平滑迁移。
	UploadFile(file *multipart.FileHeader) (string, string, error)

	// DeleteFile 统一的删除入口
	// 只暴露按 key 删除的能力，避免业务层关心各个云厂商 SDK 的细节，
	// 同时可以在具体实现里做权限、日志等扩展。
	DeleteFile(key string) error
}

// NewOss OSS的实例化方法
// 该工厂方法会根据全局配置 `global.GVA_CONFIG.System.OssType` 返回对应的 OSS 实现：
//   - 调用方只需要 `oss := NewOss()`，完全不用关心当前到底是本地、七牛、COS 还是其它云；
//   - 当运维或配置文件切换 OssType 时，无需改业务代码，就能换一套存储方案；
//   - 新增厂商时，只需增加实现结构体并在此处补充 case 分支即可，扩展性好。
//
// Author [SliverHorn](https://github.com/SliverHorn)
// Author [ccfish86](https://github.com/ccfish86)
func NewOss() OSS {
	switch global.GVA_CONFIG.System.OssType {
	case "local":
		// 本地存储实现：
		//   适合开发/测试环境或简单部署场景，无外网依赖，部署成本低。
		return &Local{}
	case "qiniu":
		// 七牛云对象存储实现：
		//   对接成熟的云存储服务，利用其 CDN、图片处理等能力。
		return &Qiniu{}
	case "tencent-cos":
		// 腾讯云 COS 实现
		return &TencentCOS{}
	case "aliyun-oss":
		// 阿里云 OSS 实现
		return &AliyunOSS{}
	case "huawei-obs":
		// 华为云 OBS 实现
		return HuaWeiObs
	case "aws-s3":
		// 亚马逊 S3 实现
		return &AwsS3{}
	case "cloudflare-r2":
		// Cloudflare R2 实现：
		//   适合与 Cloudflare 生态深度结合的场景。
		return &CloudflareR2{}
	case "minio":
		// MinIO 自建对象存储实现：
		//   - 通过统一的 S3 协议兼容多种部署环境；
		//   - 适合内网或私有化部署场景；
		//   - 这里使用单独的 GetMinio 函数进行初始化，便于封装连接参数和复用逻辑。
		minioClient, err := GetMinio(global.GVA_CONFIG.Minio.Endpoint, global.GVA_CONFIG.Minio.AccessKeyId, global.GVA_CONFIG.Minio.AccessKeySecret, global.GVA_CONFIG.Minio.BucketName, global.GVA_CONFIG.Minio.UseSSL)
		if err != nil {
			// 这里提前在启动阶段检查 MinIO 是否可用：
			//   - 使用 Warn 日志提示具体错误，方便排查配置/网络问题；
			//   - 直接 panic 终止服务启动，避免“存储其实不可用但服务仍然对外提供上传接口”的危险情况，
			//     可以让运维在部署阶段就发现问题，而不是等到线上用户上传失败才暴露。
			global.GVA_LOG.Warn("你配置了使用minio，但是初始化失败，请检查minio可用性或安全配置: " + err.Error())
			panic("minio初始化失败") // 保守策略：配置了 MinIO 却初始化失败时直接阻止服务启动，保证数据安全与行为一致性
		}
		return minioClient
	default:
		// 默认回退到本地存储：
		//   当配置缺失或写错时，系统仍能工作（特别是在开发环境）；
		//   同时也避免因为配置轻微错误导致整个上传功能不可用。
		return &Local{}
	}
}
