# GoMall 建设记录（2026-09-02 ~ 09-03）

> 这份文档回答"这一步都干了什么"：按时间线记录已完成的功能、关键设计决策、
> 过程中修掉的 bug 和踩过的坑。做进阶任务时遇到分层/权限/事务问题，先来这里找同类先例。

## 一、交付总账

| 领域 | 接口数 | 新文件 | 代表文件 |
|---|---|---|---|
| 收货地址（PRD-004） | 5 | 迁移 + 模型 + 仓库 + 服务 + 控制器×3+http×6 | services/address_service.go |
| 购物车选择/预览（PRD-004 收尾） | 2 | — | services/cart_service.go SetSelection |
| 订单（PRD-005 骨架） | 8 | models/order.go 等 7 文件 | services/order_service.go |
| 支付（PRD-007 骨架） | 2 | models/payment.go | services/payment_service.go |
| 退款（PRD-009 骨架） | 5 | models/refund.go | services/refund_service.go |
| 优惠券（PRD-009 骨架） | 6 | models/coupon.go | services/coupon_service.go |
| 评价（PRD-009 骨架） | 4 | models/review.go | services/review_service.go |
| 通知（PRD-008 骨架） | 4 | models/notification.go | services/notification_service.go |
| 后台任务（PRD-008 骨架） | 2 | models/task.go | services/task_service.go |
| admin-ui 新视图 | — | address.css + 7 个视图区块 | admin-ui/app.js |

合计：**约 38 个接口、12 张新表、10 个导航视图**，全部端到端实测通过。

## 二、时间线

### 09-02 晚：收货地址模块（第一次完整走通"AI 实现 + 人验收"流程）
1. 分析 PRD-004 → 规划模式出方案（Plan agent 评审）→ 批准后实现
2. 迁移 `005_addresses` → model → repository（首次用 GORM Transaction）→ service → controller → router → .http → UI
3. Chrome DevTools 浏览器实测全流程；并发 curl 切换默认地址验证行锁串行化

### 09-03 凌晨：交易链路 CRUD 一次性铺开（应用户要求）
1. 读 PRD-005~009 的 API 增量与数据定义
2. 一个迁移 `006_trade` 建 12 张表 + cart_items.selected
3. 七个领域依次：model → errors → repository（接口并入 Repository 聚合）→ service → permission → handler → router → main 组装
4. 边写边冒烟：curl 走通 下单→幂等重放→支付单→发货→非法迁移拦截→收货→退款→审核→券领取去重→通知已读→任务重试
5. 中途发现并修复 3 个真 bug（见第四节）
6. 补 UI：订单/优惠券/任务/退款/通知 五个视图 + 购物车结算区

### 09-03 下午：闭环修补 + 文档体系
1. 用户指出购物车无下单按钮 → 补勾选列 + 实时合计 + 「去结算」（预览→确认→下单→清车→跳订单页）
2. 订单操作从 emoji 图标改为文字链，运营动作标注归属角色
3. favicon 404 噪声 → 全局 NoRoute 处理
4. 产出三份文档：ADVANCED-TASKS.md（下一步入口）、本文件、docs/README.md 状态同步

## 三、关键设计决策（为什么这么做）

| 决策 | 理由 |
|---|---|
| 金额全用"分"int64 | 浮点数有精度陷阱；PRD 明确要求最小货币单位整数 |
| 越权访问统一返回 404 而非 403 | 不向探测者泄露"资源存在但你不配"这一信息 |
| 所有私有资源 SQL 必带 user_id 条件 | 所有权过滤下沉到 repository，service/controller 无法遗漏 |
| 默认地址切换 = 事务内先全清再置位 | MySQL 不支持部分唯一索引；UPDATE...WHERE user_id=? 的行锁天然串行化并发切换 |
| addresses.default_user_id 生成列 + 唯一键 | 数据库层兜底，把软删除条件编码进表达式，防绕过应用的脏写入 |
| PATCH 地址忽略 is_default | 设默认只走专用事务接口，堵住绕过清零逻辑产生多默认的口子 |
| 幂等键存 SHA-256 前 32 位 | 客户端可传任意字符串，定长哈希避免长度/字符集问题；request_hash 区分"同键不同内容" |
| 订单快照存 JSON | 商品改名改价不影响历史订单；address_snapshot 同理 |
| 状态变更 UPDATE 带 from 条件 | RowsAffected=0 即并发抢先，天然乐观校验，不用额外版本字段 |
| 券"每人每模板一张"用唯一键 | 先把业务规则固化进 schema（uk_user_coupons_user_template），放开限额时再演进 |
| 商品公开评价列表不挂 JWT | 游客看口碑是电商常态，与商品目录公开语义一致 |
| admin 同时持有消费者权限 | 管理员登录也是自己数据的主人；私有性靠所有权过滤而非角色隔离 |
| 静态资源不做认证 | `<link>`/`<script>` 不发 Authorization 头，门控会导致白屏；安全边界在 API 层 |
| 测试数据用 .http 文件而非 Go test | 项目现状无集成测试基建；A6 任务会把这类验证沉淀为自动化测试 |

## 四、过程中修掉的 Bug 与踩的坑（经验库）

1. **admin 缺 order.create 权限 → 403**：RBAC 最初按"角色=身份"设计，但订单类接口本质是"用户的私有数据"。教训：权限点命名要区分"资源归属"和"管理动作"。
2. **isModelNotFound 漏登记 ErrPaymentNotFound → 支付单误报 404**：payment service 里"查不到待支付单"是正常的创建路径，却被当成故障短路。教训：自写的错误分类函数新增 sentinel 时必须同步登记——已在注释里写明。
3. **幂等冲突判断写反**：拿 RequestHash 与幂等键哈希比较，永远不等 → 所有重试误报 409。修正为先算请求内容指纹再比对。教训：涉及"相同 key 不同内容"的判断，两个哈希的来源必须都是请求体派生值。
4. **`router.Static` 不接受中间件**：想给 admin-ui 加角色门控编译失败；改 Group+StaticFS 后又被真实浏览器打脸（资源 401 白屏），最终回退。教训见上表第三行静态资源决策。
5. **prompt()/confirm() 阻塞 MCP 点击**：UI 自动化测试时弹窗导致元素超时。处理：handle_dialog 接受后继续。
6. **`go run` 后台进程随 shell 退出而死**：多次重启后旧进程残留导致行为与代码不一致，排查半天。教训：验证前先确认端口上的进程是不是你以为的那个（ps lstart + strings 二进制双重确认）。
7. **gofmt 结构体对齐**：手写 tag 对齐经常不合规，写完一批就跑 `gofmt -l`，别攒着。

## 五、当前系统边界（诚实清单）

- 下单**不扣库存**——A1 解决前，同一 SKU 可以被无限下单
- 支付单创建后**停在 pending**，订单不会变 paid——A2 解决前，"支付成功"只能手动改库模拟
- 没有任何东西**消费 background_tasks**——表和运维 API 就绪，worker 是 A3
- 优惠券**不参与金额计算**——discount_cent 恒为 0，A8 才接上
- 预览查询是 N+1——功能正确，量大才需要 A5
- OpenAPI 契约未建——PRD-004 遗留项，建议做完 A2 后顺手补（接口稳定了再写契约省返工）

## 六、质量门槛执行记录

每次交付前固定跑：`go build ./... && go vet ./... && go test -race ./... && gofmt -l . && node --check admin-ui/app.js` —— 截至本文档完成时全部通过。

## 七、下一步

看 [ADVANCED-TASKS.md](./ADVANCED-TASKS.md)：**A1 下单事务与库存预占**。
