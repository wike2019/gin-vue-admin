package system

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/ast"
	"github.com/mholt/archives"
	cp "github.com/otiai10/copy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// AutoCodePlugin 插件管理服务的单例实例
// 使用单例模式的好处：
// 1. 确保全局只有一个插件管理实例，避免资源浪费
// 2. 方便在多个地方统一访问插件管理功能
// 3. 符合Go语言的服务层设计模式
var AutoCodePlugin = new(autoCodePlugin)

// autoCodePlugin 插件管理服务结构体
// 使用空结构体的好处：
// 1. 零内存占用，只作为方法接收器使用
// 2. 语义清晰，表明这是一个服务类而非数据类
type autoCodePlugin struct{}

// Install 插件安装方法
// 功能：接收上传的插件zip文件，解压、验证并安装到指定目录
// 参数：file - 上传的插件文件头信息（multipart.FileHeader）
// 返回：web, server - 安装状态标识（1表示成功，-1表示失败），err - 错误信息
//
// 设计思路和好处：
// 1. 使用临时目录处理：先解压到临时目录，处理完成后自动清理，避免污染工作目录
// 2. 延迟清理机制：使用defer确保无论成功失败都会清理临时文件，防止磁盘空间泄漏
// 3. 路径标准化：统一使用filepath.ToSlash转换为正斜杠，保证跨平台兼容性
// 4. 插件结构验证：检查是否符合标准插件结构（server/plugin 或 web/plugin），确保插件质量
// 5. 分离式安装：server和web插件分别安装，支持只安装其中一种，提高灵活性
func (s *autoCodePlugin) Install(file *multipart.FileHeader) (web, server int, err error) {
	// 定义临时解压目录常量
	// 使用相对路径的好处：不依赖系统特定路径，便于部署和迁移
	const GVAPLUGPINATH = "./gva-plug-temp/"

	// 延迟删除临时目录
	// 好处：无论函数正常返回还是异常退出，都会执行清理，防止临时文件堆积
	// 这是Go语言资源管理的最佳实践，类似RAII模式
	defer os.RemoveAll(GVAPLUGPINATH)

	// 检查临时目录是否存在，不存在则创建
	// 使用os.Stat + os.IsNotExist的好处：比直接创建更安全，避免覆盖已存在的目录
	_, err = os.Stat(GVAPLUGPINATH)
	if os.IsNotExist(err) {
		os.Mkdir(GVAPLUGPINATH, os.ModePerm)
	}

	// 打开上传的文件
	// 使用multipart.FileHeader.Open()的好处：可以多次读取文件，不消耗内存
	src, err := file.Open()
	if err != nil {
		return -1, -1, err
	}
	// 延迟关闭文件句柄，防止资源泄漏
	defer src.Close()

	// 在临时目录创建目标文件
	// 使用完整路径拼接的好处：明确文件位置，避免路径混乱
	out, err := os.Create(GVAPLUGPINATH + file.Filename)
	if err != nil {
		return -1, -1, err
	}
	// 延迟关闭文件，确保数据写入磁盘
	defer out.Close()

	// 将上传的文件内容复制到临时文件
	// 使用io.Copy的好处：高效处理大文件，自动管理缓冲区，避免内存溢出
	_, err = io.Copy(out, src)

	// 解压zip文件到临时目录
	// 解压后获取所有文件路径列表，用于后续分析插件结构
	paths, err := utils.Unzip(GVAPLUGPINATH+file.Filename, GVAPLUGPINATH)

	// 过滤掉Mac系统特殊文件（.DS_Store, __MACOSX等）
	// 好处：避免这些系统文件干扰插件安装，保持插件目录干净
	paths = filterFile(paths)

	// 初始化变量
	// 使用-1作为失败标识的好处：与成功值1区分明显，便于判断安装状态
	var webIndex = -1
	var serverIndex = -1
	webPlugin := ""
	serverPlugin := ""

	// 遍历解压后的文件路径，查找插件目录结构
	// 设计思路：通过路径分析确定插件类型和位置
	for i := range paths {
		// 统一转换为正斜杠路径
		// 好处：Windows和Unix路径格式统一，简化后续字符串处理
		paths[i] = filepath.ToSlash(paths[i])

		// 按路径分隔符分割，分析路径结构
		// 标准插件结构：解压根目录/插件名/server/plugin/... 或 web/plugin/...
		pathArr := strings.Split(paths[i], "/")
		ln := len(pathArr)

		// 路径深度小于4说明不是标准插件结构，跳过
		// 好处：快速过滤无效路径，提高处理效率
		if ln < 4 {
			continue
		}

		// 查找server/plugin目录（路径数组索引2和3）
		// 使用len(serverPlugin) == 0的好处：只记录第一个匹配的路径，避免重复处理
		if pathArr[2]+"/"+pathArr[3] == `server/plugin` && len(serverPlugin) == 0 {
			// 使用filepath.Join的好处：自动处理路径分隔符，保证跨平台兼容性
			serverPlugin = filepath.Join(pathArr[0], pathArr[1], pathArr[2], pathArr[3])
		}

		// 查找web/plugin目录
		if pathArr[2]+"/"+pathArr[3] == `web/plugin` && len(webPlugin) == 0 {
			webPlugin = filepath.Join(pathArr[0], pathArr[1], pathArr[2], pathArr[3])
		}
	}

	// 验证插件结构：至少需要server或web其中一种
	// 好处：确保插件符合规范，避免安装无效插件
	if len(serverPlugin) == 0 && len(webPlugin) == 0 {
		zap.L().Error("非标准插件，请按照文档自动迁移使用")
		return webIndex, serverIndex, errors.New("非标准插件，请按照文档自动迁移使用")
	}

	// 如果存在server插件，执行安装
	// 分离处理的好处：server和web可以独立安装，互不影响
	if len(serverPlugin) != 0 {
		// 注意：这里formPath和toPath都传了Server路径，可能是为了从临时目录复制到server目录
		err = installation(serverPlugin, global.GVA_CONFIG.AutoCode.Server, global.GVA_CONFIG.AutoCode.Server)
		if err != nil {
			return webIndex, serverIndex, err
		}
		serverIndex = 1 // 安装成功，更新状态
	}

	// 如果存在web插件，执行安装
	if len(webPlugin) != 0 {
		// formPath是临时目录，toPath是web目录
		err = installation(webPlugin, global.GVA_CONFIG.AutoCode.Server, global.GVA_CONFIG.AutoCode.Web)
		if err != nil {
			return webIndex, serverIndex, err
		}
		webIndex = 1 // 安装成功，更新状态
	}

	return webIndex, serverIndex, err
}

// installation 执行插件安装的核心函数
// 功能：将插件从源路径复制到目标路径
// 参数：path - 插件相对路径，formPath - 源路径配置，toPath - 目标路径配置
//
// 设计思路和好处：
// 1. 路径解析：从相对路径中提取插件名称，用于冲突检测
// 2. 冲突检测：安装前检查目标目录是否已存在同名插件，避免覆盖
// 3. 安全复制：使用第三方库的Copy函数，支持跳过特定文件，保证复制过程可控
// 4. 路径构建：使用filepath.Join构建完整路径，自动处理不同操作系统的路径差异
func installation(path string, formPath string, toPath string) error {
	// 将路径统一转换为正斜杠并分割，便于提取插件名称
	arr := strings.Split(filepath.ToSlash(path), "/")
	ln := len(arr)

	// 路径深度检查：至少需要3层（如：解压目录/插件名/plugin）
	// 好处：提前验证路径有效性，避免后续处理出错
	if ln < 3 {
		return errors.New("arr")
	}

	// 从路径中提取插件名称
	// 路径结构：解压目录/插件名/server或web/plugin/...
	// arr[ln-3] 就是插件名称（倒数第三层）
	// 好处：不依赖固定路径格式，通过相对位置提取，更灵活
	name := arr[ln-3]

	// 构建源路径：根目录 + 源路径配置 + 插件相对路径
	// 使用filepath.Join的好处：自动处理路径分隔符，跨平台兼容
	var form = filepath.Join(global.GVA_CONFIG.AutoCode.Root, formPath, path)

	// 构建目标路径：根目录 + 目标路径配置 + plugin目录
	var to = filepath.Join(global.GVA_CONFIG.AutoCode.Root, toPath, "plugin")

	// 检查目标目录是否已存在同名插件
	// 使用os.Stat的好处：不创建文件，只检查存在性，性能好
	_, err := os.Stat(to + name)
	if err == nil {
		// 如果已存在，记录错误日志并返回
		// 好处：防止覆盖已有插件，保护用户数据
		zap.L().Error("autoPath 已存在同名插件，请自行手动安装", zap.String("to", to))
		return errors.New(toPath + "已存在同名插件，请自行手动安装")
	}

	// 执行文件复制，使用Skip选项跳过Mac特殊文件
	// 使用cp.Copy的好处：
	// 1. 支持目录递归复制
	// 2. 可以自定义跳过规则（如Mac系统文件）
	// 3. 比手动实现更可靠，处理了各种边界情况
	return cp.Copy(form, to, cp.Options{Skip: skipMacSpecialDocument})
}

// filterFile 过滤文件路径列表，移除Mac系统特殊文件
// 功能：从路径列表中移除.DS_Store和__MACOSX等Mac系统文件
// 参数：paths - 原始文件路径列表
// 返回：过滤后的文件路径列表
//
// 设计思路和好处：
//  1. 预分配容量：使用make([]string, 0, len(paths))预分配切片容量
//     好处：减少内存重新分配次数，提高性能，特别是在处理大量文件时
//  2. 复用过滤函数：调用skipMacSpecialDocument统一判断逻辑
//     好处：代码复用，维护方便，逻辑集中
//  3. 函数式风格：通过遍历和条件判断过滤，代码清晰易读
func filterFile(paths []string) []string {
	// 预分配切片，容量为原列表长度
	// 好处：大多数情况下不需要扩容，预分配可以避免多次内存分配
	np := make([]string, 0, len(paths))

	// 遍历所有路径，过滤掉Mac特殊文件
	for _, path := range paths {
		// 使用skipMacSpecialDocument判断是否需要跳过
		// 忽略FileInfo参数（传nil）的好处：这里只需要路径字符串，不需要文件信息
		if ok, _ := skipMacSpecialDocument(nil, path, ""); ok {
			continue // 跳过Mac特殊文件
		}
		np = append(np, path)
	}
	return np
}

// skipMacSpecialDocument 判断是否为Mac系统特殊文件，用于文件复制时跳过
// 功能：检测路径中是否包含Mac系统自动生成的特殊文件
// 参数：_ - FileInfo（未使用，用下划线忽略），src - 源路径，_ - 目标路径（未使用）
// 返回：bool - true表示需要跳过，false表示保留；error - 错误信息
//
// 设计思路和好处：
//  1. 匹配Mac系统文件：.DS_Store（目录元数据）和__MACOSX（压缩包元数据目录）
//     好处：这些文件对插件功能无意义，过滤后保持插件目录干净
//  2. 使用strings.Contains：简单高效的字符串匹配
//     好处：不需要正则表达式，性能好，代码简洁
//  3. 函数签名兼容cp.Copy的Skip选项：符合第三方库的接口要求
//     好处：可以直接作为回调函数使用，无需适配层
func skipMacSpecialDocument(_ os.FileInfo, src, _ string) (bool, error) {
	// 检查路径中是否包含Mac系统特殊文件标识
	// .DS_Store：Mac Finder创建的隐藏文件，存储文件夹显示属性
	// __MACOSX：Mac压缩工具在zip文件中创建的元数据目录
	if strings.Contains(src, ".DS_Store") || strings.Contains(src, "__MACOSX") {
		return true, nil // 返回true表示需要跳过此文件
	}
	return false, nil // 返回false表示保留此文件
}

// PubPlug 发布插件，将插件打包成zip文件
// 功能：将已安装的插件（web和server两部分）打包成标准格式的zip文件，便于分发
// 参数：plugName - 插件名称
// 返回：zipPath - 生成的zip文件完整路径，err - 错误信息
//
// 设计思路和好处：
// 1. 安全验证：使用filepath.Clean防止路径穿越攻击
// 2. 完整性检查：确保web和server两部分都存在才打包
// 3. 标准格式：打包成符合安装要求的目录结构（插件名/web/plugin/...）
// 4. 统一存储：生成的zip文件存放在server目录，便于管理和下载
func (s *autoCodePlugin) PubPlug(plugName string) (zipPath string, err error) {
	// 参数验证：插件名称不能为空
	// 好处：提前发现无效输入，避免后续处理出错
	if plugName == "" {
		return "", errors.New("插件名称不能为空")
	}

	// 防止路径穿越攻击
	// filepath.Clean的作用：
	// 1. 移除路径中的".."和"."，防止访问上级目录
	// 2. 规范化路径格式，统一处理路径分隔符
	// 好处：安全防护，防止恶意插件名称导致文件系统被破坏
	plugName = filepath.Clean(plugName)

	// 构建web和server插件的完整路径
	// 使用filepath.Join的好处：自动处理不同操作系统的路径分隔符
	webPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Web, "plugin", plugName)
	serverPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", plugName)

	// 验证web插件目录是否存在
	// 好处：确保插件完整性，避免打包不完整的插件
	_, err = os.Stat(webPath)
	if err != nil {
		return "", errors.New("web路径不存在")
	}

	// 验证server插件目录是否存在
	_, err = os.Stat(serverPath)
	if err != nil {
		return "", errors.New("server路径不存在")
	}

	// 生成zip文件名
	fileName := plugName + ".zip"

	// 准备要打包的文件映射
	// map的key是源路径（实际文件位置），value是zip内的路径（打包后的目录结构）
	// 设计思路：将分散的web和server目录重新组织成标准插件结构
	// 好处：打包后的zip文件可以直接用于Install方法安装，形成闭环
	files, err := archives.FilesFromDisk(context.Background(), nil, map[string]string{
		// web插件：从实际路径映射到 zip内路径 插件名/web/plugin/插件名
		webPath: plugName + "/web/plugin/" + plugName,
		// server插件：从实际路径映射到 zip内路径 插件名/server/plugin/插件名
		serverPath: plugName + "/server/plugin/" + plugName,
	})

	// 创建zip输出文件
	// 使用相对路径的好处：文件创建在当前工作目录，便于后续移动或删除
	out, err := os.Create(fileName)
	if err != nil {
		return
	}
	// 延迟关闭文件，确保数据写入磁盘
	defer out.Close()

	// 配置压缩格式为Zip
	// 使用CompressedArchive的好处：
	// 1. 统一的归档接口，可以轻松切换压缩格式（如Gzip、Tar等）
	// 2. 支持压缩选项（当前注释掉了Gz压缩，只使用Zip归档）
	// 好处：代码灵活，未来可以轻松添加压缩功能
	format := archives.CompressedArchive{
		//Compression: archives.Gz{}, // 可以启用Gzip压缩以减小文件大小
		Archival: archives.Zip{}, // 使用Zip格式，兼容性好
	}

	// 执行归档操作，将文件打包成zip
	// 使用context.Background()的好处：支持超时控制（虽然这里没设置，但接口支持）
	err = format.Archive(context.Background(), out, files)
	if err != nil {
		return
	}

	// 返回zip文件的完整路径
	// 好处：调用者可以直接使用该路径进行文件下载或传输
	return filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, fileName), nil
}

// InitMenu 初始化插件菜单，将数据库中的菜单信息写入插件的初始化文件
// 功能：读取插件的menu.go模板文件，通过AST解析找到菜单数组，用实际菜单数据替换，然后写回文件
// 参数：menuInfo - 包含插件名称、父菜单名称和菜单ID列表的请求信息
// 返回：err - 错误信息
//
// 设计思路和好处：
//  1. AST操作：使用Go的AST（抽象语法树）解析和修改代码，而不是字符串替换
//     好处：更安全可靠，不会破坏代码结构，支持复杂语法
//  2. 模板化：插件提供初始化文件模板，运行时填充实际数据
//     好处：插件开发者只需关注模板，系统自动生成初始化代码
//  3. 关联数据预加载：使用Preload一次性加载菜单的关联数据（参数、按钮）
//     好处：减少数据库查询次数，提高性能
//  4. 父菜单自动创建：自动为插件创建顶级父菜单
//     好处：统一插件菜单结构，便于管理和展示
func (s *autoCodePlugin) InitMenu(menuInfo request.InitMenu) (err error) {
	// 构建插件菜单初始化文件的路径
	// 标准路径：根目录/server/plugin/插件名/initialize/menu.go
	menuPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", menuInfo.PlugName, "initialize", "menu.go")

	// 读取菜单初始化文件内容
	// 这个文件是模板，包含一个空的SysBaseMenu数组
	src, err := os.ReadFile(menuPath)
	if err != nil {
		fmt.Println(err)
	}

	// 创建文件集（FileSet），用于AST解析
	// FileSet用于跟踪源代码位置信息，便于错误报告和代码生成
	fileSet := token.NewFileSet()

	// 解析Go源代码为AST
	// 参数说明：fileSet - 文件集，"" - 文件名（空字符串表示从内存读取），src - 源代码，0 - 解析模式
	// 好处：将源代码转换为可操作的树形结构，而不是字符串
	astFile, err := parser.ParseFile(fileSet, "", src, 0)

	// 在AST中查找SysBaseMenu类型的数组声明
	// 这个数组就是我们要填充的菜单数据容器
	arrayAst := ast.FindArray(astFile, "model", "SysBaseMenu")

	var menus []system.SysBaseMenu

	// 创建插件的父菜单（顶级菜单）
	// 设计思路：每个插件都有一个统一的入口菜单
	// 好处：
	// 1. 统一插件菜单结构，便于用户识别
	// 2. 所有插件子菜单都挂在这个父菜单下，层次清晰
	parentMenu := []system.SysBaseMenu{
		{
			ParentId:  0,                          // 0表示顶级菜单
			Path:      menuInfo.PlugName + "Menu", // 路径使用插件名+Menu
			Name:      menuInfo.PlugName + "Menu", // 名称与路径一致
			Hidden:    false,                      // 默认显示
			Component: "view/routerHolder.vue",    // 使用路由占位组件
			Sort:      0,                          // 排序为0，可后续调整
			Meta: system.Meta{
				Title: menuInfo.ParentMenu, // 使用用户指定的父菜单标题
				Icon:  "school",            // 默认图标
			},
		},
	}

	// 从数据库查询指定的菜单及其关联数据
	// Preload("Parameters")：预加载菜单参数，避免N+1查询问题
	// Preload("MenuBtn")：预加载菜单按钮，一次性获取所有关联数据
	// Find(&menus, "id in (?)", menuInfo.Menus)：查询ID在指定列表中的菜单
	// 好处：一次查询获取所有需要的数据，包括关联数据，性能好
	err = global.GVA_DB.Preload("Parameters").Preload("MenuBtn").Find(&menus, "id in (?)", menuInfo.Menus).Error
	if err != nil {
		return err
	}

	// 将父菜单和查询到的菜单合并
	// 使用append的好处：父菜单在前，子菜单在后，符合菜单树的结构
	menus = append(parentMenu, menus...)

	// 将菜单结构体数组转换为AST表达式
	// 好处：可以直接替换AST节点，保持代码格式和结构
	menuExpr := ast.CreateMenuStructAst(menus)

	// 替换AST中的数组元素
	// 直接修改AST节点，而不是字符串替换
	// 好处：保证生成的代码语法正确，格式规范
	arrayAst.Elts = *menuExpr

	// 将修改后的AST转换回Go源代码
	var out []byte
	bf := bytes.NewBuffer(out)
	// 使用printer.Fprint将AST打印为源代码
	// 好处：自动格式化代码，保持Go代码风格一致
	printer.Fprint(bf, fileSet, astFile)

	// 将生成的源代码写回文件
	// 0666权限：所有用户可读写
	// 好处：确保文件可被后续处理，同时保持合理的权限控制
	os.WriteFile(menuPath, bf.Bytes(), 0666)
	return nil
}

// InitAPI 初始化插件API，将数据库中的API信息写入插件的初始化文件
// 功能：读取插件的api.go模板文件，通过AST解析找到API数组，用实际API数据替换，然后写回文件
// 参数：apiInfo - 包含插件名称和API ID列表的请求信息
// 返回：err - 错误信息
//
// 设计思路和好处：
//  1. AST操作：与InitMenu类似，使用AST解析和修改代码，保证代码质量
//     好处：生成的代码语法正确，格式规范，不会破坏原有代码结构
//  2. 模板化设计：插件提供API初始化模板，系统自动填充数据
//     好处：插件开发者只需定义模板，运行时自动生成初始化代码
//  3. 批量查询：使用IN查询一次性获取所有API数据
//     好处：减少数据库查询次数，提高性能
//  4. 代码生成：将数据库数据转换为可执行的Go代码
//     好处：插件安装时自动注册API，无需手动配置
func (s *autoCodePlugin) InitAPI(apiInfo request.InitApi) (err error) {
	// 构建插件API初始化文件的路径
	// 标准路径：根目录/server/plugin/插件名/initialize/api.go
	apiPath := filepath.Join(global.GVA_CONFIG.AutoCode.Root, global.GVA_CONFIG.AutoCode.Server, "plugin", apiInfo.PlugName, "initialize", "api.go")

	// 读取API初始化文件内容
	// 这个文件是模板，包含一个空的SysApi数组
	src, err := os.ReadFile(apiPath)
	if err != nil {
		fmt.Println(err)
	}

	// 创建文件集，用于AST解析和代码生成
	fileSet := token.NewFileSet()

	// 解析Go源代码为AST
	// 好处：将源代码转换为可操作的树形结构，便于精确修改
	astFile, err := parser.ParseFile(fileSet, "", src, 0)

	// 在AST中查找SysApi类型的数组声明
	// 这个数组就是我们要填充的API数据容器
	arrayAst := ast.FindArray(astFile, "model", "SysApi")

	var apis []system.SysApi

	// 从数据库查询指定的API列表
	// Find(&apis, "id in (?)", apiInfo.APIs)：使用IN查询批量获取API
	// 好处：一次查询获取所有API数据，比循环查询效率高得多
	err = global.GVA_DB.Find(&apis, "id in (?)", apiInfo.APIs).Error
	if err != nil {
		return err
	}

	// 将API结构体数组转换为AST表达式
	// 好处：可以直接替换AST节点，保持代码格式和结构
	apisExpr := ast.CreateApiStructAst(apis)

	// 替换AST中的数组元素
	// 直接修改AST节点，而不是字符串替换
	// 好处：保证生成的代码语法正确，格式规范
	arrayAst.Elts = *apisExpr

	// 将修改后的AST转换回Go源代码
	var out []byte
	bf := bytes.NewBuffer(out)
	// 使用printer.Fprint将AST打印为源代码
	// 好处：自动格式化代码，保持Go代码风格一致，符合gofmt规范
	printer.Fprint(bf, fileSet, astFile)

	// 将生成的源代码写回文件
	// 0666权限：所有用户可读写
	// 好处：确保文件可被后续处理，同时保持合理的权限控制
	os.WriteFile(apiPath, bf.Bytes(), 0666)
	return nil
}
