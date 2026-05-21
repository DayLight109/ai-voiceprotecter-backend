# sqlc 生成代码占位目录

sqlc 在 P1 实施时执行 `make gateway-sqlc` 会向此目录写入：
- `db.go` — Queries / DBTX 接口
- `models.go` — 表结构 struct
- `<table>.sql.go` — 每个 query 对应的强类型方法

骨架阶段保留该目录，避免编译期路径错误。
