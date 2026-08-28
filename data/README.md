# 本地运行数据

`mysql/` 保存 Docker 中 MySQL 的实际数据文件，对应容器内的
`/var/lib/mysql`。

这些是 MySQL 管理的二进制文件，不要手工修改，也不要提交到 Git。
需要查看或修改业务数据时，应使用 MySQL 客户端或 GORM。
