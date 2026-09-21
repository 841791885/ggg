#!/usr/bin/env bash
# 开发环境数据库一键初始化：起 MySQL 容器 → 建库授权 → 按序执行全部迁移 → 记账。
#
# 用法:  ./scripts/init-db.sh
# 幂等性：重复执行安全（容器已存在则跳过创建；建库用 IF NOT EXISTS；
#         goose 依据 schema_migrations 跳过已应用版本）。
# 依赖：Docker Desktop 已启动；goose 可选（没有时用 mysql 客户端逐个文件执行）。
set -euo pipefail
cd "$(dirname "$0")/.."

DB_NAME="${GOMALL_DB_NAME:-gomall}"
DB_USER="${GOMALL_DB_USER:-gomall}"
DB_PASS="${GOMALL_DB_PASS:-gomall_dev}"
DB_PORT="${GOMALL_DB_PORT:-3307}"
CONTAINER=gomall-mysql

# ── ① 确保 MySQL 容器在跑 ──
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
  if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    echo "▶ 启动已存在的 ${CONTAINER} 容器"
    docker start "$CONTAINER"
  else
    # 卷名自动发现：若历史上已存在同名卷（注意 Docker 会把首字符的非字母数字转为下划线，
    # gomall-mysql-data 与 gomall_mysql_data 可能并存），优先复用带数据的旧卷，避免"新空卷假初始化"。
    VOL=""
    for cand in gomall_mysql_data gomall-mysql-data; do
      if docker volume inspect "$cand" >/dev/null 2>&1; then VOL="$cand"; break; fi
    done
    [ -z "$VOL" ] && VOL=gomall_mysql_data
    echo "▶ 首次创建 MySQL 容器（数据卷 $VOL 持久化）"
    docker run -d --name "$CONTAINER" \
      -e MYSQL_ROOT_PASSWORD=root_dev \
      -e MYSQL_DATABASE="$DB_NAME" \
      -e MYSQL_USER="$DB_USER" \
      -e MYSQL_PASSWORD="$DB_PASS" \
      -e TZ=UTC \
      -p "${DB_PORT}:3306" \
      -v "$VOL:/var/lib/mysql" \
      --character-set-server=utf8mb4 --collation-server=utf8mb4_0900_ai_ci \
      mysql:8
  fi
fi

# ── ② 等待 MySQL 就绪（首启要跑初始化脚本，可能要几十秒）──
echo "▶ 等待 MySQL 可用…"
for i in $(seq 1 60); do
  if docker exec "$CONTAINER" mysqladmin ping -h127.0.0.1 -uroot -proot_dev --silent 2>/dev/null; then
    break
  fi
  [ "$i" = 60 ] && { echo "❌ 等待超时，看日志：docker logs $CONTAINER"; exit 1; }
  sleep 2
done
echo "✓ MySQL 就绪"

# ── ③ 授权（best-effort，两种历史路径都要兼容）──
# 路径A：本脚本首建的容器 → root 密码是 root_dev，且 gomall 用户已被镜像自动创建
# 路径B：历史上手工建的容器 → root 密码未知，但 gomall 账号通常已有全库权限，跳过即可
if docker exec "$CONTAINER" mysqladmin ping -h127.0.0.1 -uroot -proot_dev --silent 2>/dev/null; then
  docker exec "$CONTAINER" mysql -uroot -proot_dev -e "
    GRANT ALL PRIVILEGES ON ${DB_NAME}.* TO '${DB_USER}'@'%';
    FLUSH PRIVILEGES;" 2>/dev/null || echo "ℹ️ root 授权跳过（权限可能已具备）"
else
  echo "ℹ️ root 密码非默认值（历史手工创建的容器），跳过授权步骤"
fi

# ── ③.5 迁移前置检查：gomall 账号必须能建库 ──
# 迁移 001 会 CREATE DATABASE gomall_test（集成测试用的第二数据库），gomall 账号默认只有
# 自己库的权限，全新容器下该语句报 1044。root 可用时补授；不可用则明确提示方案。
if docker exec "$CONTAINER" mysqladmin ping -h127.0.0.1 -uroot -proot_dev --silent 2>/dev/null; then
  docker exec "$CONTAINER" mysql -uroot -proot_dev -e "
    GRANT CREATE, DROP ON *.* TO '${DB_USER}'@'%'; FLUSH PRIVILEGES;" 2>/dev/null \
    || echo "ℹ️ 建库授权跳过"
fi

# ── ④ 按序执行未应用的迁移 ──
# 优先用 goose（自动记账+跳过已应用）；没装就退化为顺序执行所有 up.sql 并手工记账。
if command -v goose >/dev/null 2>&1; then
  DSN="mysql://${DB_USER}:${DB_PASS}@tcp(127.0.0.1:${DB_PORT})/${DB_NAME}?tls=false"
  goose -dir migrations -table schema_migrations mysql "$DSN" up
else
  # 无 goose 的退化路径：以账本为准逐版本补执行。
  # ⚠️ 账本有记录但表实际缺失（比如挂载了错误数据卷）会被跳过——那种情况请重建空库再跑。
  echo "ℹ️ 未检测到 goose，用 mysql 客户端按账本补执行未应用的迁移"
  # 先确保账本表存在（goose 会自动建；这里手工建同构表兜底）。
  docker exec "$CONTAINER" mysql -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" -e "
    CREATE TABLE IF NOT EXISTS schema_migrations (
      version BIGINT NOT NULL PRIMARY KEY,
      dirty TINYINT(1) NOT NULL DEFAULT 0
    );" 2>/dev/null
  for f in migrations/*.up.sql; do
    ver=$(basename "$f" | sed -E 's/^0*([0-9]+)_.*/\1/')
    applied=$(docker exec "$CONTAINER" mysql -N -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" \
      -e "SELECT COUNT(*) FROM schema_migrations WHERE version=$ver" 2>/dev/null || echo 0)
    if [ "$applied" = "0" ]; then
      echo "  ▶ 应用 $f"
      if ! docker exec -i "$CONTAINER" mysql -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" < "$f"; then
        echo "❌ $f 执行失败，中止（半途结构不可信，修复后重跑）"; exit 1
      fi
      docker exec "$CONTAINER" mysql -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" \
        -e "INSERT INTO schema_migrations (version, dirty) VALUES ($ver, 0)"
    fi
  done
fi

echo "✓ 迁移完成。下一步：./scripts/seed-admin.sh 创建管理员账号"
