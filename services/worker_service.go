package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "ggg/models"
	"ggg/repositories"

	"go.uber.org/zap"
)

// WorkerService 是后台任务消费者（PRD-008 进阶 A3）：goroutine 循环领取到期任务并执行。
//
// 与 JS 的根本差异在这里体现：Node 的"后台任务"仍跑在同一个事件循环线程上，await 让出即可；
// Go 的 worker 是真·并行操作系统线程的 goroutine，拥有独立生命周期——
// 因此必须显式约定"怎么停"（ctx 取消）和"停之前等谁干完"（WaitGroup），这就是本文件的全部主题。
type WorkerService struct {
	repository repositories.Repository
	interval   time.Duration // 扫描间隔
	handlers   map[string]func(context.Context, repositories.TaskContext) error
}

// NewWorkerService 创建 worker 并注册任务处理器表。
// handler 按 task_type 分发——新增异步业务（如退款到账通知）只需往表里加一项，主循环零改动。
func NewWorkerService(repository repositories.Repository, interval time.Duration) *WorkerService {
	w := &WorkerService{
		repository: repository,
		interval:   interval,
		handlers:   map[string]func(context.Context, repositories.TaskContext) error{},
	}
	w.handlers["order_timeout_close"] = w.handleOrderTimeoutClose
	w.handlers["notification_send"] = w.handleNotificationSend
	return w
}

// Start 是 worker 的主循环，等价于前端心智里的 setInterval(tick, 5000)——
// 区别在于它跑在一个独立 goroutine（系统线程级的并行执行单元）里，且必须自己处理"怎么停"。
// 这个函数【不会自己结束】：调用方通过取消 ctx 来通知它收工（见 main.go 的 stopWorker），
// 并负责 go 它 + WaitGroup 等它退出。
//
// 三层停止机制各司其职（A3 的核心设计）：
//
//	① ticker.C           心跳：每 interval 尝试领一批
//	② ctx.Done()         关机信号：select 多路等待让它能随时中断等待
//	③ RevertRunningTasks 崩溃保险：进程被 kill -9 时连①②的机会都没有，靠下次启动回收兜底
func (w *WorkerService) Start(ctx context.Context) {
	// ── 开机第一件事：收拾上辈子的烂摊子 ──
	// 如果上次进程是被 kill -9 / 断电杀掉的，它领走但没做完的任务会永远卡在 running 状态
	// （没人替它改回来）。所以每次启动先把所有 running 重置回 pending，让本轮能重新领取它们。
	// attempts 不清零：崩溃也算消耗了一次重试额度，防止"必崩任务"无限复活。
	if reverted, err := w.repository.RevertRunningTasks(ctx); err != nil {
		zap.L().Error("回收遗留 running 任务失败", zap.Error(err))
	} else if reverted > 0 {
		zap.L().Info("已回收上次进程遗留的任务", zap.Int64("count", reverted))
	}

	// ── 心跳引擎 ──
	// NewTicker(5s) ≈ setInterval：每 5 秒往 ticker.C 这个"信箱"投一封定时信。
	// C 是只读通道（channel），下面 select 就是在两个信箱之间"听哪个先响"。
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop() // 不 Stop 会泄漏底层定时器资源——ticker 也必须"善后"，同 defer Close 家族
	zap.L().Info("worker 启动", zap.Duration("interval", w.interval))

	// ── 主循环：for + select = "同时等两件事，谁先来处理谁" ──
	// select 阻塞在这里时整个 goroutine 处于休眠，不占 CPU（对比 JS 事件循环的空转等待）。
	for {
		select {
		case <-ctx.Done():
			// 信箱①："关机"信号。<- 是从通道取信的语法；ctx 被取消的瞬间此路必通。
			// 走到这里意味着 main 调了 stopWorker()，本函数 return 后 goroutine 自然消亡。
			// 注意：这只是"不再接新活"。此刻若有任务干到一半，runOnce 是同步调用、必然跑完才回到这里；
			// main 那边的 workerDone.Wait() 负责等我们彻底退出后才放行进程结束。
			zap.L().Info("worker 收到停机信号，优雅退出")
			return
		case <-ticker.C:
			// 信箱②：定时信到了 → 干一轮活（去数据库领任务并执行）。
			w.runOnce(ctx)
		}
	}
}

// runOnce 是"一次心跳"里干的活：循环领任务→执行→记账，直到没得领或本轮配额用完。
func (w *WorkerService) runOnce(ctx context.Context) {
	// 上限 20 条：假设积压了 1 万条到期任务，一口气全干完会让下一次 select 等很久——
	// 期间就算收到关机信号也无法被检查（select 只在两个 case 之间才被评估）。
	// 分批让心跳正常跳动，停机检查点也保持密集。类比：长循环里定期 await 让出事件循环。
	for range 20 { // range 整数（Go 1.22+）= 循环 20 次，不需要下标就不声明变量
		if ctx.Err() != nil {
			// ctx.Err() != nil 表示已被取消（用户 Ctrl+C 了）。主动在每轮开头检查，
			// 比等 select 发现更快放下手头队列——"停机时少领活"的自觉行为。
			return
		}
		// 去数据库抢一条到期任务。三个返回值：任务本体 / 是否抢到 / 错误。
		// Go 多返回值没有 JS 解构报错文化兜底，编译器强制你正视每个值——这就是到处 if err != nil 的原因。
		task, claimed, err := w.repository.ClaimDueTask(ctx, time.Now().UTC())
		if err != nil {
			zap.L().Error("领取任务失败", zap.Error(err))
			return // 数据库都读不动了，继续跑只会刷错误日志；等下次心跳再试
		}
		if !claimed {
			return // 队列空 or 被别人抢先——都是正常情况，把控制权还给主循环等下次心跳
		}

		// 按类型分发处理器。map[string] 函数 的取值语法和 JS 对象一致，
		// 但多了 ", ok" 惯例：取不到不返回 undefined，而是零值+false，必须显式判断。
		handler, ok := w.handlers[task.TaskType]
		if !ok {
			// 未知类型直接定格失败（backoff=0）：代码里没有这个 handler，重试一万次也不会突然有了
			// ——毒任务不许无限循环。
			_ = w.repository.MarkTaskFailedOrRetry(ctx, task.ID, fmt.Sprintf("未知任务类型 %q", task.TaskType), 0)
			continue
		}
		// 执行业务。传 TaskContext（任务 + 仓库句柄），handler 借此能跨模块读写订单、通知。
		err = handler(ctx, repositories.TaskContext{Task: task, Repository: w.repository})
		if err == nil {
			if markErr := w.repository.MarkTaskSucceeded(ctx, task.ID); markErr != nil {
				// 业务成功了但状态没记上——只告警不回滚（业务效果已落库，回滚反而制造不一致）。
				// 后果是该任务会被重复领取重跑一次，靠 handler 自身的幂等设计消化（见各 handler 注释）。
				zap.L().Error("标记任务成功失败", zap.Uint64("task_id", task.ID), zap.Error(markErr))
			}
			continue
		}
		// ── 失败路径：指数退避重试 ──
		// 1<<min(attempts,6) 起步翻倍：第 1 次失败等约 2s、第 2 次 4s、第 3 次 8s……封顶约 2 分钟。
		// min 防的是 attempts 很大时 1<<attempts 溢出成天文数字。
		// 意义：故障通常是暂时的（DB 抖动/下游超时），越挫越勇地猛撞只会加重故障。
		backoff := time.Duration(1<<min(task.Attempts, 6)) * time.Second
		zap.L().Warn("任务执行失败", zap.Uint64("task_id", task.ID), zap.String("type", task.TaskType),
			zap.Int("attempts", task.Attempts), zap.Error(err))
		if markErr := w.repository.MarkTaskFailedOrRetry(ctx, task.ID, err.Error(), backoff); markErr != nil {
			zap.L().Error("回写任务失败状态出错", zap.Uint64("task_id", task.ID), zap.Error(markErr))
		}
	}
}

// handleOrderTimeoutClose 关闭到期未付订单并释放预占库存。
// 复用 CancelOrder 事务的前提：它只接受 pending_payment 状态，天然幂等——
// 重复投递时第二次进来 RowsAffected=0 → ErrInvalidOrderTransition → 此处识别为"已完成过"直接成功返回。
// 执行体只有三行：解参数 → 调事务 → 分类错误。复杂度全在"什么算成功"的判断上。
func (w *WorkerService) handleOrderTimeoutClose(ctx context.Context, tc repositories.TaskContext) error {
	orderID, err := uint64FromPayload(tc.Task.Payload, "order_id")
	if err != nil {
		return err // 数据本身坏了，重试也不会变好——但会消耗 attempts 直到定格 failed，让运营看见
	}
	// 系统关单没有"当前用户"身份，CancelOrder 的 user_id 过滤不适用——用系统版专用入口。
	_, err = tc.Repository.CancelOrderBySystem(ctx, orderID)
	if err != nil {
		// ★ worker 世界的通用幂等哲学："要达成的状态已经达成了"就是成功，不是失败。
		// 订单已被用户取消 / 已被支付推进 / 甚至不存在——无论哪种，"它不该再占着库存等待支付"
		// 这个目标都已实现或失去意义，返回 nil 让任务定格 succeeded，不再空转重试。
		if errors.Is(err, model.ErrInvalidOrderTransition) || errors.Is(err, model.ErrOrderNotFound) {
			return nil
		}
		// 其余错误（连接池耗尽、死锁重试用尽等瞬态故障）原样上抛 → runOnce 走退避重试。
		return fmt.Errorf("超时关单（订单 %d）：%w", orderID, err)
	}
	return nil
}

// handleNotificationSend 把 Outbox 事件转成站内信。
// 幂等策略：写入前按 user+type+title 判重，任务重试不会轰炸用户。
func (w *WorkerService) handleNotificationSend(ctx context.Context, tc repositories.TaskContext) error {
	// 先解出全部业务参数。注意 string 字段用 `x, _ :=` 忽略 ok——通知标题缺失不值得让任务失败，
	// 空串发出去也比卡死队列好（这是"尽力而为"型任务的取舍；金额类就绝不能这么写）。
	userID, err := uint64FromPayload(tc.Task.Payload, "user_id")
	if err != nil {
		return err
	}
	title, _ := tc.Task.Payload["title"].(string)
	content, _ := tc.Task.Payload["content"].(string)
	notifType, _ := tc.Task.Payload["type"].(string)
	// ★ 幂等判重：本任务可能在"通知已插入、但 MarkSucceeded 前进程崩了"的窗口后被重跑，
	// 不判重就会给用户重复轰炸。查一次再插，把"恰好一次"降级为"至少一次 + 去重"。
	exists, err := tc.Repository.HasNotificationLike(ctx, userID, notifType, title)
	if err != nil {
		return err
	}
	if exists {
		return nil // 上次其实已经发成功了，静默确认
	}
	_, err = tc.Repository.CreateNotification(ctx, model.Notification{
		UserID: userID, Type: notifType, Title: title, Content: content,
	})
	return err
}

// uint64FromPayload 从 JSON payload 提取数值字段。
// 坑点教学：map[string]any 经 encoding/json 反序列化后，所有数字都是 float64（JS 同款行为！），
// 直接 .(uint64) 断言永远失败；这里兼容 float64/int64 两种来源（内存构造 vs 数据库读回）。
func uint64FromPayload(payload map[string]any, key string) (uint64, error) {
	switch value := payload[key].(type) {
	case float64:
		if value < 0 {
			return 0, fmt.Errorf("payload 字段 %s 不能为负", key)
		}
		return uint64(value), nil
	case int64:
		return uint64(value), nil
	case uint64:
		return value, nil
	default:
		return 0, fmt.Errorf("payload 缺少数值字段 %s（实际类型 %T）", key, payload[key])
	}
}
