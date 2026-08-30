package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ggg/config"

	mysqldriver "github.com/go-sql-driver/mysql"
)

const validConfig = `
server:
  address: ":8080"
  read_header_timeout: 5s
  shutdown_timeout: 10s
database:
  host: "127.0.0.1"
  port: 3307
  username: "gomall"
  password: "gomall_dev"
  name: "gomall"
  charset: "utf8mb4"
  collation: "utf8mb4_0900_ai_ci"
  max_open_connections: 20
  max_idle_connections: 10
  connection_max_lifetime: 30m
  connection_max_idle_time: 5m
  connect_timeout: 5s
auth:
  jwt_secret: "test-secret"
  token_ttl: 2h
log:
  environment: "development"
  level: "debug"
`

func TestLoadValidConfig(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, validConfig)
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("读取配置失败: %v", err)
	}

	if loaded.Server.Address != ":8080" {
		t.Fatalf("server.address = %q, want :8080", loaded.Server.Address)
	}
	if loaded.Database.ConnectTimeout.Value() != 5*time.Second {
		t.Fatalf("database.connect_timeout = %v, want 5s", loaded.Database.ConnectTimeout.Value())
	}

	parsedDSN, err := mysqldriver.ParseDSN(loaded.Database.DSN())
	if err != nil {
		t.Fatalf("生成的 DSN 无法解析: %v", err)
	}
	if parsedDSN.DBName != "gomall" || parsedDSN.Addr != "127.0.0.1:3307" || !parsedDSN.ParseTime {
		t.Fatalf("DSN 配置错误: %+v", parsedDSN)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	t.Parallel()

	content := validConfig + "unknown_field: true\n"
	if _, err := config.Load(writeConfig(t, content)); err == nil {
		t.Fatal("未知配置字段应返回错误")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	content := `
server:
  address: ":8080"
  read_header_timeout: 5s
  shutdown_timeout: 10s
database:
  host: "127.0.0.1"
  port: 70000
  username: "gomall"
  name: "gomall"
  charset: "utf8mb4"
  collation: "utf8mb4_0900_ai_ci"
  max_open_connections: 20
  max_idle_connections: 10
  connection_max_lifetime: 30m
  connection_max_idle_time: 5m
  connect_timeout: 5s
`
	if _, err := config.Load(writeConfig(t, content)); err == nil {
		t.Fatal("非法端口应返回错误")
	}
}

func TestPathCanBeOverridden(t *testing.T) {
	t.Setenv("GOMALL_CONFIG", "custom-config.yaml")
	if got := config.Path(); got != "custom-config.yaml" {
		t.Fatalf("config.Path() = %q, want custom-config.yaml", got)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}
	return path
}
