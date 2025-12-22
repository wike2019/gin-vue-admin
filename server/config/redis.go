package config

// Redis Redis缓存配置结构体
// Redis是高性能的内存数据库，常用于缓存、会话存储、消息队列等场景
// 设计优势：
// 1. 高性能：内存存储，读写速度极快，适合高并发场景
// 2. 灵活部署：支持单实例和集群两种模式，可根据规模选择
// 3. 多实例支持：通过Name字段区分多个Redis实例，实现功能隔离
// 4. 数据隔离：通过DB字段实现逻辑数据库隔离，同一Redis实例可服务多个应用
type Redis struct {
	// Name 实例名称：标识当前Redis实例的名字，用于区分多个Redis连接
	// 使用场景：
	// 1. 多实例管理：应用可能连接多个Redis（如缓存Redis、会话Redis）
	// 2. 日志标识：在日志中标识使用的是哪个Redis实例
	// 3. 配置管理：通过名称查找对应的配置，便于管理
	Name string `mapstructure:"name" json:"name" yaml:"name"`
	// Addr 服务器地址：Redis服务器的地址和端口
	// 格式：host:port，例如 "127.0.0.1:6379"
	// 设计好处：通过配置指定地址，便于在不同环境（开发/测试/生产）切换
	Addr string `mapstructure:"addr" json:"addr" yaml:"addr"`
	// Password 密码：Redis服务器的认证密码
	// 安全要求：
	// 1. 生产环境必须设置密码，防止未授权访问
	// 2. 密码应足够复杂，定期更换
	// 3. 不要将密码提交到代码仓库，使用环境变量或配置中心
	Password string `mapstructure:"password" json:"password" yaml:"password"`
	// DB 数据库编号：单实例模式下选择Redis的哪个逻辑数据库（0-15）
	// 设计目的：
	// 1. 数据隔离：不同应用或模块使用不同的DB，避免数据冲突
	// 2. 逻辑分离：同一Redis实例可以服务多个应用，提高资源利用率
	// 注意：集群模式下不支持DB选择，所有数据都在DB 0
	DB int `mapstructure:"db" json:"db" yaml:"db"`
	// UseCluster 是否使用集群模式：标识当前配置是单实例还是集群模式
	// true：集群模式，使用ClusterAddrs中的节点列表
	// false：单实例模式，使用Addr指定的单个节点
	// 设计好处：
	// 1. 高可用：集群模式提供高可用和故障转移
	// 2. 扩展性：集群模式可以水平扩展，支持更大数据量
	// 3. 灵活切换：通过配置切换模式，无需修改代码
	UseCluster bool `mapstructure:"useCluster" json:"useCluster" yaml:"useCluster"`
	// ClusterAddrs 集群节点地址列表：集群模式下所有Redis节点的地址
	// 格式：["host1:port1", "host2:port2", ...]
	// 设计考虑：
	// 1. 高可用：配置多个节点，即使部分节点故障也能正常工作
	// 2. 负载均衡：客户端可以连接到任意节点，Redis自动路由请求
	// 3. 最少节点：建议至少配置3个节点，保证集群的稳定性
	ClusterAddrs []string `mapstructure:"clusterAddrs" json:"clusterAddrs" yaml:"clusterAddrs"`
}
