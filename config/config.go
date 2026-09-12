package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "config.yaml"

// Duration 表示 YAML 中使用 Go 格式书写的时间长度，例如 5s 或 30m。
type Duration time.Duration

// UnmarshalYAML 将 YAML 字符串解析为时间长度。
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	value, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("解析时间长度 %q: %w", node.Value, err)
	}
	*d = Duration(value)
	return nil
}

// Value 返回标准库的 time.Duration。
func (d Duration) Value() time.Duration {
	return time.Duration(d)
}

// Config 表示应用全部配置。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Payment  PaymentConfig  `yaml:"payment"`
	Worker   WorkerConfig   `yaml:"worker"`
	Log      LogConfig      `yaml:"log"`
}

// LogConfig 表示应用日志环境和最低输出级别。
type LogConfig struct {
	Environment string `yaml:"environment"`
	Level       string `yaml:"level"`
}

// AuthConfig 表示 JWT 认证配置。
type AuthConfig struct {
	JWTSecret string   `yaml:"jwt_secret"`
	TokenTTL  Duration `yaml:"token_ttl"`
}

// WorkerConfig 是后台任务消费者配置（PRD-008 进阶 A3）。
// Enabled 提供开关：学习期调试 HTTP 接口时可能不希望 worker 自动改数据，关掉即可。
type WorkerConfig struct {
	Enabled      bool     `yaml:"enabled"`
	ScanInterval Duration `yaml:"scan_interval"`
}

// PaymentConfig 表示模拟支付渠道配置（PRD-007 进阶 A2）。
type PaymentConfig struct {
	// CallbackSecret 是 HMAC 回调验签密钥，与模拟渠道共享同一值；生产部署应通过环境变量注入而非提交仓库。
	CallbackSecret string `yaml:"callback_secret"`
}

// ServerConfig 表示 HTTP 服务配置。
type ServerConfig struct {
	Address           string   `yaml:"address"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
}

// DatabaseConfig 表示 MySQL 和连接池配置。
type DatabaseConfig struct {
	Host                  string   `yaml:"host"`
	Port                  int      `yaml:"port"`
	Username              string   `yaml:"username"`
	Password              string   `yaml:"password"`
	Name                  string   `yaml:"name"`
	Charset               string   `yaml:"charset"`
	Collation             string   `yaml:"collation"`
	MaxOpenConnections    int      `yaml:"max_open_connections"`
	MaxIdleConnections    int      `yaml:"max_idle_connections"`
	ConnectionMaxLifetime Duration `yaml:"connection_max_lifetime"`
	ConnectionMaxIdleTime Duration `yaml:"connection_max_idle_time"`
	ConnectTimeout        Duration `yaml:"connect_timeout"`
}

// DSN 生成 go-sql-driver/mysql 使用的连接串。
func (c DatabaseConfig) DSN() string {
	// 使用驱动提供的 Config 生成连接串，可以正确转义密码等特殊字符，
	// 比 fmt.Sprintf 手工拼接 DSN 更可靠。
	mysqlConfig := mysqldriver.Config{
		User:                 c.Username,
		Passwd:               c.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		DBName:               c.Name,
		Collation:            c.Collation,
		ParseTime:            true,
		Loc:                  time.UTC,
		AllowNativePasswords: true,
		ClientFoundRows:      true,
		Params: map[string]string{
			"charset": c.Charset,
		},
	}
	return mysqlConfig.FormatDSN()
}

// Path 返回配置文件路径，可通过 GOMALL_CONFIG 覆盖。
func Path() string {
	if path := os.Getenv("GOMALL_CONFIG"); path != "" {
		return path
	}
	return defaultConfigPath
}

// Load 严格读取并校验 YAML 配置。
func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	// 严格模式会拒绝结构体中不存在的 YAML 字段，避免配置名拼错后被静默忽略。
	decoder.KnownFields(true)

	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("解析配置文件 %s: %w", path, err)
	}

	// 配置文件只允许一个 YAML 文档，防止第二个 --- 后面的内容意外被忽略。
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, fmt.Errorf("配置文件 %s 只能包含一个 YAML 文档", path)
		}
		return Config{}, fmt.Errorf("解析配置文件 %s: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("校验配置文件 %s: %w", path, err)
	}
	return config, nil
}

// Validate 校验启动所需的配置值。
func (c Config) Validate() error {
	if c.Server.Address == "" {
		return errors.New("server.address 不能为空")
	}
	if c.Server.ReadHeaderTimeout.Value() <= 0 || c.Server.ShutdownTimeout.Value() <= 0 {
		return errors.New("HTTP 超时必须大于 0")
	}
	if c.Database.Host == "" || c.Database.Username == "" || c.Database.Name == "" {
		return errors.New("数据库 host、username 和 name 不能为空")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return errors.New("database.port 必须在 1 到 65535 之间")
	}
	if c.Database.Charset == "" || c.Database.Collation == "" {
		return errors.New("数据库 charset 和 collation 不能为空")
	}
	if c.Database.MaxOpenConnections < 1 {
		return errors.New("database.max_open_connections 必须大于 0")
	}
	if c.Database.MaxIdleConnections < 0 || c.Database.MaxIdleConnections > c.Database.MaxOpenConnections {
		return errors.New("database.max_idle_connections 必须在 0 和最大连接数之间")
	}
	if c.Database.ConnectionMaxLifetime.Value() <= 0 ||
		c.Database.ConnectionMaxIdleTime.Value() <= 0 ||
		c.Database.ConnectTimeout.Value() <= 0 {
		return errors.New("数据库连接超时配置必须大于 0")
	}
	if c.Auth.JWTSecret == "" || c.Auth.TokenTTL.Value() <= 0 {
		return errors.New("auth.jwt_secret 不能为空且 auth.token_ttl 必须大于 0")
	}
	if c.Payment.CallbackSecret == "" {
		return errors.New("payment.callback_secret 不能为空（回调验签密钥）")
	}
	if c.Worker.Enabled && c.Worker.ScanInterval.Value() <= 0 {
		return errors.New("worker.enabled 为 true 时 worker.scan_interval 必须大于 0")
	}
	if c.Log.Environment != "development" && c.Log.Environment != "production" {
		return errors.New("log.environment 只能是 development 或 production")
	}
	if c.Log.Level == "" {
		return errors.New("log.level 不能为空")
	}
	return nil
}
