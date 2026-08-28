# GoMall

GoMall 是一个使用 Go、Gin、GORM 和 MySQL 逐步实现的单商户商城项目。当前阶段已经完成配置读取、MySQL 连接、健康检查，以及商品和 SKU 的 Repository。

## 1. 当前技术栈

| 技术 | 当前版本 | 在项目中的作用 | 官方资料 |
| --- | --- | --- | --- |
| Go | 1.25.0 | 编写和运行后端服务 | [Go 官网](https://go.dev/) |
| Gin | v1.12.0 | HTTP 路由、请求上下文和 JSON 响应 | [Gin 官网](https://gin-gonic.com/)、[Gin GitHub](https://github.com/gin-gonic/gin) |
| GORM | v1.31.2 | ORM，负责使用 Go 结构体操作数据库 | [GORM 官网](https://gorm.io/)、[GORM 中文文档](https://gorm.io/zh_CN/docs/) |
| GORM MySQL Driver | v1.6.0 | 让 GORM 能够连接 MySQL | [GORM MySQL 文档](https://gorm.io/docs/connecting_to_the_database.html#MySQL)、[GitHub](https://github.com/go-gorm/mysql) |
| Go MySQL Driver | v1.10.0 | 底层 MySQL 驱动，并负责安全生成 DSN、识别 MySQL 错误 | [GitHub](https://github.com/go-sql-driver/mysql)、[pkg.go.dev](https://pkg.go.dev/github.com/go-sql-driver/mysql) |
| YAML v3 | v3.0.1 | 把 `config.yaml` 解析成 Go 结构体 | [pkg.go.dev](https://pkg.go.dev/gopkg.in/yaml.v3)、[GitHub](https://github.com/go-yaml/yaml/tree/v3) |
| MySQL | 8.4 | 保存商品、SKU 等业务数据 | [MySQL 8.4 文档](https://dev.mysql.com/doc/refman/8.4/en/) |
| Docker | 本机版本 | 运行隔离的 MySQL 容器 | [Docker 安装文档](https://docs.docker.com/get-started/get-docker/) |

这里的 `config` 有两个不同概念：

- [`config/`](./config/) 是我们自己编写的 Go 包，负责定义、读取和校验应用配置。
- `gopkg.in/yaml.v3` 是第三方库，`config` 包使用它解析 YAML。

## 2. 项目目录结构

项目采用根级传统 MVC 分层，`main.go` 保留在根目录。目录会随着阶段逐步增加，不提前创建没有代码的空目录。

```text
.
├── main.go                    # 程序入口和依赖组装
├── config.yaml                # 本地运行配置
├── config/                    # 配置结构、读取与校验
├── controllers/               # HTTP 请求解析和 JSON 响应
├── database/                  # MySQL、连接池等基础连接
├── models/                    # 领域模型和领域错误
├── repositories/              # 存储接口与 GORM/MySQL 实现
├── services/                  # 业务校验、状态规则和流程编排
├── routes/                    # Gin 路由和中间件注册
├── migrations/                # 数据库建表和结构变更 SQL
├── data/
│   ├── README.md              # 本地数据目录说明
│   └── mysql/                 # MySQL 实际数据，已被 Git 忽略
├── docs/                      # 总 PRD、路线图和阶段 PRD
├── go.mod                     # Go 模块名和依赖版本
└── go.sum                     # 依赖内容校验哈希
```

各目录的意义：

| 位置 | 负责什么 | 不应该负责什么 |
| --- | --- | --- |
| `main.go` | 读取配置、创建数据库连接、组装 Controller 和 Router、启动与关闭服务 | 商品校验、SQL 查询等业务逻辑 |
| `config.yaml` | 保存 HTTP 地址、MySQL 地址和连接池参数 | Go 代码和数据库数据 |
| `config/` | 将 YAML 解析成强类型配置并校验 | 创建业务对象或处理 HTTP |
| `controllers/` | 解析路径、查询参数和 JSON 请求，调用 Service，生成 HTTP/JSON 响应 | 直接编写 SQL 或实现复杂业务规则 |
| `database/` | 创建 GORM 连接、取得 `database/sql` 连接池并检查可用性 | 商品增删改查 |
| `models/` | 表达 Product、SKU、状态和稳定的领域错误 | 依赖 Gin 或处理 HTTP 请求 |
| `repositories/` | 定义持久化接口，并用 GORM 实现当前阶段的数据操作 | 决定 HTTP 状态码或消费者展示规则 |
| `services/` | 校验业务输入、执行状态迁移并调用 Repository | 解析 HTTP JSON 或直接编写 SQL |
| `routes/` | 创建 Gin Engine、注册 URL 和中间件 | 实现业务逻辑 |
| `migrations/` | 保存可追踪的表结构 SQL | 保存 MySQL 运行时数据 |
| `data/mysql/` | 持久化 MySQL 的实际二进制文件 | 手工编辑或提交到 Git |
| `docs/` | 保存最终目标、每阶段步骤、规则和验收标准 | 参与程序运行 |

当前业务层从创建商品这一条最小业务开始：

```text
services/
└── product_service.go         # 商品校验、状态迁移和业务流程
```

主要依赖方向是：

```text
HTTP 请求
    -> routes
    -> controllers
    -> services
    -> repositories
    -> models / MySQL
```

`main.go` 负责把这些对象组装起来。下层不能反过来依赖上层，例如 Repository 不能依赖 Controller。

### 2.1 请求和响应类型放在哪里

当前还没有商品 API，所以商品 Request/Response 类型尚未创建。实现商品 Controller 时计划增加：

```text
controllers/
├── product_controller.go      # 商品接口处理函数
├── product_request.go         # 创建、修改、分页等请求结构体
└── product_response.go        # 商品详情、列表和错误响应结构体
```

Request/Response 属于 HTTP 接口契约，放在 Controller 边界；`models.Product` 和 `models.SKU` 属于领域及数据库模型。二者分开有三个原因：

1. 防止客户端修改 `id`、`created_at`、`deleted_at` 等系统字段。
2. 数据库字段变化时，不必同时破坏前端接口。
3. 列表、详情和管理端接口可以返回不同的数据结构。

这是 REST API 项目，因此不创建 HTML `views/` 目录，JSON Response 类型承担接口展示模型的职责。

## 3. MySQL 是怎样安装到 Docker 里的

我们没有进入容器手动安装 MySQL。使用的是 Docker Hub 上的 [MySQL 官方镜像](https://hub.docker.com/_/mysql)：

```text
mysql:8.4 镜像 -> gomall-mysql 容器 -> 容器内运行 MySQL 8.4
                                      |
                                      +-> 项目 data/mysql/ 保存数据库文件
                                      +-> 本机 3307 端口映射到容器 3306 端口
```

`docker run` 会先检查本机有没有 `mysql:8.4` 镜像。没有时会自动从 Docker Hub 下载，然后用它创建并启动容器。因此不需要额外执行 `docker pull`。

### 3.1 前置检查

先安装并启动 Docker Desktop，然后检查 Docker 是否可用：

```bash
docker --version
docker info
```

Docker Desktop 下载地址见 [Docker 官方安装文档](https://docs.docker.com/get-started/get-docker/)。

### 3.2 首次创建 MySQL 容器

在项目根目录执行：

```bash
docker run --name gomall-mysql \
  -p 3307:3306 \
  -e MYSQL_ROOT_PASSWORD=gomall_root_dev \
  -e MYSQL_DATABASE=gomall \
  -e MYSQL_USER=gomall \
  -e MYSQL_PASSWORD=gomall_dev \
  -e TZ=UTC \
  -v "$PWD/data/mysql:/var/lib/mysql" \
  -v "$PWD/migrations:/docker-entrypoint-initdb.d:ro" \
  -d mysql:8.4 \
  --character-set-server=utf8mb4 \
  --collation-server=utf8mb4_0900_ai_ci \
  --default-time-zone=+00:00 \
  --sql-mode=STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION
```

命令参数的意义：

| 参数 | 意义 |
| --- | --- |
| `docker run` | 根据镜像创建并启动一个新容器 |
| `--name gomall-mysql` | 把容器命名为 `gomall-mysql`，后续可以直接用名称操作 |
| `-p 3307:3306` | 将本机 `3307` 映射到容器的 MySQL `3306`，Go 应用连接 `127.0.0.1:3307` |
| `-e MYSQL_ROOT_PASSWORD=...` | 设置 MySQL `root` 用户的密码 |
| `-e MYSQL_DATABASE=gomall` | 第一次初始化数据目录时创建 `gomall` 数据库 |
| `-e MYSQL_USER=gomall` | 第一次初始化时创建应用用户 `gomall` |
| `-e MYSQL_PASSWORD=...` | 设置应用用户 `gomall` 的密码 |
| `-e TZ=UTC` | 将容器时区设为 UTC，减少不同时区导致的时间混乱 |
| `-v "$PWD/data/mysql:/var/lib/mysql"` | 将项目的 `data/mysql/` 绑定到 MySQL 数据目录，容器停止或删除后数据仍保留在项目中 |
| `-v "$PWD/migrations:/docker-entrypoint-initdb.d:ro"` | 把本项目迁移目录只读挂载到 MySQL 初始化目录 |
| `-d` | 在后台运行容器 |
| `mysql:8.4` | 使用 MySQL 官方镜像的 `8.4` 标签 |
| `--character-set-server=utf8mb4` | 使用完整 UTF-8 字符集，可保存中文和 emoji |
| `--collation-server=utf8mb4_0900_ai_ci` | 设置 MySQL 8 的默认排序和比较规则 |
| `--default-time-zone=+00:00` | 将数据库默认时区设为 UTC |
| `--sql-mode=...` | 启用较严格的数据校验，尽早暴露非法日期、除零等问题 |

Docker 命令和数据卷的完整说明：

- [docker container run 官方文档](https://docs.docker.com/reference/cli/docker/container/run/)
- [Docker bind mounts 官方文档](https://docs.docker.com/engine/storage/bind-mounts/)
- [MySQL 官方镜像环境变量说明](https://hub.docker.com/_/mysql#environment-variables)

### 3.3 初始化 SQL 是怎样执行的

首次使用空的 `data/mysql/` 目录时，MySQL 官方镜像会自动执行 `/docker-entrypoint-initdb.d` 中的 SQL 文件。本项目将 [`migrations/001_catalog.sql`](./migrations/001_catalog.sql) 挂载到了这个目录，它会创建：

- `products` 商品表
- `skus` SKU 表
- `gomall_test` 集成测试数据库

注意：初始化脚本只在 `data/mysql/` 为空时执行一次。以后执行 `docker restart` 或 `docker start` 不会重新执行。修改表结构时应新增迁移脚本并主动执行，不能依赖重启容器。

当前持久化关系如下：

```text
项目 data/mysql/ <-> 容器 /var/lib/mysql
```

`data/mysql/` 中是 MySQL 管理的二进制数据文件，可以观察有哪些文件，但不要手工编辑。`data/mysql/` 已加入 `.gitignore`，不会被提交到 Git。需要迁移真实数据时应使用 `mysqldump` 导出和导入，不要直接复制不同操作系统上的 `.ibd` 等底层文件。

### 3.4 日常操作和验证

```bash
# 查看容器状态
docker ps --filter name=gomall-mysql

# 查看启动日志
docker logs gomall-mysql

# 停止容器，数据仍保留在项目的 data/mysql/ 中
docker stop gomall-mysql

# 再次启动已有容器，不要重复执行 docker run
docker start gomall-mysql

# 进入 MySQL 命令行，出现提示后输入 gomall_dev
docker exec -it gomall-mysql mysql -ugomall -p
```

进入 MySQL 后可以执行：

```sql
SHOW DATABASES;
USE gomall;
SHOW TABLES;
SELECT * FROM products;
```

## 4. Go 第三方库是怎样安装的

Go 项目的第三方库不是全局复制到项目目录里。`go get` 会下载模块、更新 [`go.mod`](./go.mod) 中的版本，并更新 [`go.sum`](./go.sum) 中用于校验下载内容的哈希。

本项目使用下面的命令安装直接依赖。版本号写出来是为了能够复现当前环境：

```bash
go get github.com/gin-gonic/gin@v1.12.0
go get gorm.io/gorm@v1.31.2
go get gorm.io/driver/mysql@v1.6.0
go get github.com/go-sql-driver/mysql@v1.10.0
go get gopkg.in/yaml.v3@v3.0.1
go mod tidy
```

也可以不写 `@版本号`，此时 Go 会按模块规则选择版本。学习项目建议保留 `go.mod`，不要每次都追最新版本。

常用依赖命令：

```bash
# 下载 go.mod 中已经声明的依赖
go mod download

# 根据代码中的 import 增删依赖，并整理 go.mod/go.sum
go mod tidy

# 查看当前模块依赖
go list -m all

# 查看某个包的本地文档
go doc gorm.io/gorm
```

`go get` 用来给当前项目添加依赖；`go install` 一般用来安装带 `main` 包的命令行工具。安装 GORM、Gin 这类代码库应该使用 `go get`。

## 5. 每个第三方库在项目里怎样使用

### 5.1 Gin：HTTP 服务

安装：

```bash
go get github.com/gin-gonic/gin@v1.12.0
```

本项目的使用位置：

- [`routes/router.go`](./routes/router.go)：创建 Gin Engine 并注册路由。
- [`controllers/health_controller.go`](./controllers/health_controller.go)：读取请求上下文并返回 JSON。

最小示例：

```go
router := gin.Default()
router.GET("/ping", func(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
})
router.Run(":8080")
```

当前项目没有直接调用 `router.Run`，而是把 Gin Engine 交给标准库 `http.Server`，这样可以配置超时并实现优雅停机。

### 5.2 GORM：使用结构体操作数据库

安装 GORM 核心和 MySQL 适配器：

```bash
go get gorm.io/gorm@v1.31.2
go get gorm.io/driver/mysql@v1.6.0
```

这两个包分工不同：

- `gorm.io/gorm` 是 ORM 核心，提供 `Create`、`First`、`Find`、`Updates`、`Delete` 和事务等能力。
- `gorm.io/driver/mysql` 是 MySQL 方言适配器，告诉 GORM 怎样连接和生成适合 MySQL 的 SQL。

本项目的使用位置：

- [`database/mysql.go`](./database/mysql.go)：通过 `gorm.Open` 创建连接，并设置连接池。
- [`models/product.go`](./models/product.go)：使用 GORM tag 描述主键、索引、字段类型和软删除字段。
- [`repositories/mysql_repository.go`](./repositories/mysql_repository.go)：当前使用 GORM 实现创建商品；其他数据操作会按学习进度逐步添加。

连接 MySQL 的核心代码：

```go
db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
if err != nil {
	return err
}

sqlDB, err := db.DB()
if err != nil {
	return err
}
defer sqlDB.Close()

if err := sqlDB.Ping(); err != nil {
	return err
}
```

基础 CRUD 示例：

```go
product := models.Product{Name: "Go 入门课", Status: models.ProductStatusDraft}

// INSERT
err := db.Create(&product).Error

// SELECT ... WHERE id = ?
err = db.First(&product, product.ID).Error

// UPDATE
err = db.Model(&product).Update("name", "Go 商城实战").Error

// 软删除：模型存在 gorm.DeletedAt 时，不会直接删除物理数据
err = db.Delete(&product).Error
```

更多资料：

- [GORM 声明模型](https://gorm.io/zh_CN/docs/models.html)
- [GORM CRUD](https://gorm.io/zh_CN/docs/create.html)
- [GORM 事务](https://gorm.io/zh_CN/docs/transactions.html)
- [GORM 连接池](https://gorm.io/zh_CN/docs/generic_interface.html#Connection-Pool)

### 5.3 Go MySQL Driver：底层驱动和 DSN

安装：

```bash
go get github.com/go-sql-driver/mysql@v1.10.0
```

GORM MySQL Driver 底层也依赖这个驱动。本项目还直接使用它，所以把它列为直接依赖：

- [`config/config.go`](./config/config.go)：用 `mysql.Config` 生成 DSN，避免手工拼接用户名、密码和查询参数。

DSN 是数据库连接字符串，例如：

```text
gomall:gomall_dev@tcp(127.0.0.1:3307)/gomall?charset=utf8mb4&parseTime=true&loc=UTC
```

推荐使用结构体生成 DSN：

```go
mysqlConfig := mysqldriver.Config{
	User:      "gomall",
	Passwd:    "gomall_dev",
	Net:       "tcp",
	Addr:      "127.0.0.1:3307",
	DBName:    "gomall",
	ParseTime: true,
	Loc:       time.UTC,
}
dsn := mysqlConfig.FormatDSN()
```

### 5.4 YAML v3：读取 config.yaml

安装：

```bash
go get gopkg.in/yaml.v3@v3.0.1
```

本项目在 [`config/config.go`](./config/config.go) 中使用它，把根目录 [`config.yaml`](./config.yaml) 映射到 Go 结构体。

最小示例：

```go
type ServerConfig struct {
	Address string `yaml:"address"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
}

content, err := os.ReadFile("config.yaml")
if err != nil {
	return err
}

decoder := yaml.NewDecoder(bytes.NewReader(content))
decoder.KnownFields(true)

var config Config
if err := decoder.Decode(&config); err != nil {
	return err
}
```

`yaml:"address"` 叫结构体标签，它建立 YAML 字段与 Go 字段的对应关系。`KnownFields(true)` 会在 YAML 写了未知字段时报错，能够尽早发现配置名拼错的问题。

项目还为 `5s`、`30m` 这类配置实现了 `UnmarshalYAML`，将字符串转换为标准库的 `time.Duration`。

## 6. 配置文件

应用默认读取根目录 [`config.yaml`](./config.yaml)：

```yaml
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
```

配置的读取路径是：

```text
config.yaml
    -> config.Load 解析和校验
    -> DatabaseConfig.DSN 生成连接串
    -> database.OpenMySQL 创建 GORM 和 database/sql 连接池
    -> main.go 启动 HTTP 服务
```

可以通过环境变量指定另一份配置文件：

```bash
GOMALL_CONFIG=config.test.yaml go run .
```

当前账号密码只用于本机学习环境。生产项目不应该把真实密码提交到 Git，应改为从环境变量或密钥管理服务读取。

## 7. 启动项目

确认 `gomall-mysql` 正在运行：

```bash
docker start gomall-mysql
docker ps --filter name=gomall-mysql
```

下载依赖并启动 Go 服务：

```bash
go mod download
go run .
```

`go run .` 表示编译并运行当前目录中的 `main` 包。因为 [`main.go`](./main.go) 在项目根目录，所以不需要写 `go run api/main.go`。

服务启动后，在另一个终端验证：

```bash
curl http://127.0.0.1:8080/ping
curl http://127.0.0.1:8080/health/ready
```

预期响应：

```json
{"code":200,"data":{"message":"pong"}}
{"code":200,"data":{"status":"ok"}}
```

### 7.1 创建第一个商品

当前第一条完整业务调用链是：

```text
POST /api/v1/admin/products
    -> routes
    -> ProductController
    -> ProductService
    -> MySQLRepository
    -> GORM
    -> MySQL products 表
```

发送请求：

```bash
curl -i http://127.0.0.1:8080/api/v1/admin/products \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "GoMall Logo T 恤",
    "description": "用于学习 Go 商城业务分层"
  }'
```

成功时返回 `201 Created`。新商品一定是 `draft`，并且还没有 SKU：

```json
{
	"code": 201,
  "data": {
    "id": 1,
    "name": "GoMall Logo T 恤",
    "description": "用于学习 Go 商城业务分层",
    "status": "draft",
    "created_at": "2026-08-28T00:00:00Z",
    "updated_at": "2026-08-28T00:00:00Z",
    "skus": []
  }
}
```

## 8. 测试

普通测试不要求 MySQL：

```bash
go test ./...
```

MySQL 集成测试使用独立的 `gomall_test` 数据库，需要先启动容器并显式启用：

```bash
MYSQL_INTEGRATION=1 go test ./...
```

进一步检查竞态问题和静态错误：

```bash
MYSQL_INTEGRATION=1 go test -race ./...
go vet ./...
```

## 9. 为什么 go.mod 里还有很多 indirect 依赖

[`go.mod`](./go.mod) 第一组 `require` 是本项目代码直接 `import` 的库。第二组带 `// indirect` 的库是 Gin、GORM 等库继续依赖的库。

例如，项目直接依赖 Gin，Gin 为了完成参数校验、JSON 编解码等功能又依赖其他模块。它们由 Go Modules 自动管理，一般不需要逐个安装或直接学习。执行 `go mod tidy` 会根据代码和依赖关系维护这份列表。

## 10. 项目文档

- [初学者术语表](./docs/GLOSSARY.md)
- [完整产品规划](./docs/PRODUCT-PRD.md)
- [分阶段开发路线](./docs/ROADMAP.md)
- [阶段文档索引](./docs/README.md)
- [第一阶段 PRD](./docs/PRD-001.md)
