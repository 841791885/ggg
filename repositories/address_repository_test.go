package repositories

// 默认地址的并发正确性测试（PRD-004 进阶 A6）。
//
// ─ 这个文件在验证什么 ──
// "默认地址至多一个"这条规则有两道防线，本文件两个测试各验证一道：
//   防线①【应用层】SetDefaultAddress 在事务内"先清零该用户所有默认，再置位目标" —— 测试一
//   防线②【数据库层】生成列 default_user_id + 唯一键 —— 测试二
//
// ── 为什么两道都要 ──
// 只靠应用层：将来有人写了个新接口、或运维手工 UPDATE，绕过了这段逻辑就会破功；
// 只靠数据库层：用户每次设默认都会撞唯一键报 500，体验糟糕。
// 应用层负责"正常路径顺畅"，数据库层负责"异常路径兜底"——这是本项目反复出现的双保险模式
// （幂等键、券的 seq、支付 event_no 都是它）。
//
// ── 怎么跑 ──
//   go test ./repositories/ -run TestDefault -v          # 跑本文件相关用例
//   go test ./repositories/ -race                        # 开数据竞争检测
//   go test ./repositories/ -run TestSetDefaultAddressConcurrent -count=5   # 重复跑抓偶发
//
// ️ 这是【集成测试】：需要本机 MySQL 已启动（docker start gomall-mysql）。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	appConfig "ggg/config"
	"ggg/database"
	model "ggg/models"
)

// testRepo 是整包测试共用的仓库实例，由 TestMain 初始化。
var testRepo *MySQLRepository

// TestMain 是整个测试文件的入口：跑任何 Test 之前先执行一次，适合做"建连接"这类一次性准备。
//
// 关键点：必须调用 m.Run()，否则本包所有测试函数都不会执行（这是新手最常踩的坑）。
// 返回值作为进程退出码交给 go test —— 0 表示通过。
func TestMain(m *testing.M) {
	// go test 的工作目录是【包目录】(repositories/)，而 config.yaml 在项目根，
	// 所以这里要显式跳到上一级。已在根目录跑的话（GOMALL_CONFIG 指定）就用指定的。
	configPath := appConfig.Path()
	if _, err := os.Stat(configPath); err != nil {
		configPath = filepath.Join("..", "config.yaml")
	}
	cfg, err := appConfig.Load(configPath)
	if err != nil {
		panic("加载配置失败（测试需要能读到 config.yaml）：" + err.Error())
	}
	ctx := context.Background()
	gormDB, sqlDB, err := database.OpenMySQL(ctx, cfg.Database)
	if err != nil {
		panic("连接数据库失败，请先启动 MySQL（docker start gomall-mysql）：" + err.Error())
	}
	defer sqlDB.Close()

	// 测试里把 GORM 的 SQL 日志静音，否则每条 SQL 都打印、测试输出被淹没。
	// ⚠️ 顺序要求：必须先静音再交给仓库，否则仓库拿着的是"会打印"的那个连接。
	// （之前写反了：先建仓库、后改 gormDB，等于没静音——新手常犯）
	silentDB := gormDB.Session(&gorm.Session{Logger: gormDB.Logger.LogMode(logger.Silent)})
	testRepo = NewMySQLRepository(silentDB)

	m.Run()
}

// TestSetDefaultAddressConcurrent 验证防线①：并发切换默认地址后，最终恰好只剩一个默认。
//
// 这是"并发测试"的完整套路，值得记住的四步：
//
//	① 准备互不干扰的数据（每个用例独立的 userID）
//	② 用栅栏把 N 个 goroutine 卡在同一时刻同时起跑
//	③ 忽略单个请求的成功/失败，只等全部跑完
//	④ 断言【不变量】——"恰好一个默认"，而不是"谁赢了"
func TestSetDefaultAddressConcurrent(t *testing.T) {
	const (
		rounds  = 20 // 跑 20 轮：并发 bug 往往不是每次必现，靠重复提高撞上的概率
		workers = 20 // 每轮 20 个 goroutine 同时抢
	)

	for round := range rounds {
		// ─ 第①步：准备数据 ─
		// 每轮用独立用户：避免轮次之间互相干扰，也让断言范围干净（只数这个用户的地址）。
		userID := createTestUser(t, round)
		addressIDs := make([]uint64, 3)
		for i := range addressIDs {
			addressIDs[i] = createTestAddress(t, userID)
		}

		// ── 第②步：栅栏（barrier）──
		// start 是个无缓冲 channel，所有 goroutine 执行到 <-start 会阻塞住。
		// 等全部就位后 close(start)，阻塞的接收方【同一时刻】被唤醒 —— 这就是"同时起跑"。
		//
		// 为什么需要它？goroutine 是异步创建的，第 20 个可能比第 1 个晚启动几毫秒；
		// 没有栅栏的话，先启动的那些早就跑完了，根本不构成竞争 —— 测试会"假通过"。
		start := make(chan struct{})
		var wg sync.WaitGroup

		for i := range workers {
			wg.Add(1) // 开工前登记：WaitGroup 是计数器，Add 必须在 goroutine 外调用
			go func(targetID uint64) {
				defer wg.Done() // 收工销假：用 defer 保证即使 panic 也会执行
				<-start         // 堵在这里等发令枪

				ctx := context.Background()
				// 故意丢弃 error：并发下必然有请求失败（互相清掉了对方的默认标记），
				// 那是【正确行为】不是 bug。测试只关心最终状态，不关心每个请求是否都成功。
				_, _ = testRepo.SetDefaultAddress(ctx, userID, targetID)
			}(addressIDs[i%len(addressIDs)]) // 轮流指向不同地址，制造真实的多方竞争
		}

		close(start) //  发令枪响：20 个 goroutine 同时冲进临界区
		wg.Wait()    // 等所有 goroutine 跑完（不带超时，卡死就是真卡死了）

		// ─ 第④步：断言不变量 ─
		// 直接查库数数，而不是看某个请求的返回值 —— 返回值只代表单个请求的视角，
		// 数据库才算"事实来源"（这条原则贯穿本项目：缓存可以撒谎，DB 不会）。
		var defaultCount int64
		if err := testRepo.db.Model(&model.Address{}).
			Where("user_id = ? AND is_default = 1", userID).
			Count(&defaultCount).Error; err != nil {
			t.Fatalf("第 %d 轮统计默认地址失败：%v", round, err)
		}
		if defaultCount != 1 {
			t.Fatalf("第 %d 轮：默认地址数量 = %d，期望恰好 1 —— 并发破坏了不变量", round, defaultCount)
		}

		cleanupTestUser(t, userID) // 清理本轮数据，避免污染开发库和下一轮
	}
}

// TestDefaultAddressUniqueConstraint 验证防线②：绕过应用层直插第二条默认，必须被数据库拦下。
//
// 这是"数据库约束是最后防线"的实证：模拟运维手工 UPDATE、或未来某个忘了校验的新接口。
func TestDefaultAddressUniqueConstraint(t *testing.T) {
	userID := createTestUser(t, 9999)
	defer cleanupTestUser(t, userID)
	ctx := context.Background()

	// ── ① 直插第一条默认地址：应该成功 ──
	// 注意用的是 testRepo.db（裸连接）而不是 SetDefaultAddress：
	// 就是要绕开业务逻辑，看看数据库自己扛不扛得住。
	first := &model.Address{
		UserID: userID, Recipient: "张三", Phone: "13800138000",
		Province: "江苏省", City: "南京市", District: "玄武区",
		Detail: "测试路 1 号", IsDefault: true,
	}
	if err := testRepo.db.WithContext(ctx).Create(first).Error; err != nil {
		t.Fatalf("第一条默认地址应该能插入，却失败了：%v", err)
	}

	// ── ② 再插第二条默认：必须失败 ──
	second := &model.Address{
		UserID: userID, Recipient: "李四", Phone: "13900139000",
		Province: "江苏省", City: "南京市", District: "玄武区",
		Detail: "测试路 2 号", IsDefault: true,
	}
	err := testRepo.db.WithContext(ctx).Create(second).Error

	// ── ③ 断言 ──
	if err == nil {
		t.Fatal("数据库放行了第二条默认地址 —— 生成列唯一键兜底失效！检查迁移 005 是否执行")
	}
	// 错误类型判断：database/mysql.go 开了 TranslateError:true，
	// 驱动层的 MySQL 1062 会被 GORM 转成统一的 gorm.ErrDuplicatedKey —— 不用自己解析错误号。
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("期望唯一键冲突（ErrDuplicatedKey），实际是：%v", err)
	}
	t.Logf("✅ 数据库正确拦下第二条默认地址：%v", err)

	// ── ④ 补充断言：软删除后名额应释放 ──
	// 唯一键包含 deleted_at，所以删掉旧默认后应该能再设一个默认（NULL 不参与唯一比较）。
	if err := testRepo.db.Delete(&model.Address{}, first.ID).Error; err != nil {
		t.Fatalf("软删除第一条默认地址失败：%v", err)
	}
	third := &model.Address{
		UserID: userID, Recipient: "王五", Phone: "13700137000",
		Province: "江苏省", City: "南京市", District: "玄武区",
		Detail: "测试路 3 号", IsDefault: true,
	}
	if err := testRepo.db.WithContext(ctx).Create(third).Error; err != nil {
		t.Fatalf("软删除旧默认后应能设置新默认，却失败了：%v", err)
	}
}

/* ═══════════════ 测试辅助函数 ══════════════
 * 命名不带 Test 前缀，否则会被 go test 当成用例执行。
 * 每个都以 t.Helper() 开头：报错时堆栈指向调用方，而不是辅助函数内部——
 * 不然错误信息总是"在这一行"，看不出是哪个用例失败的。
 */

func createTestUser(t *testing.T, seed int) uint64 {
	t.Helper()
	// 用户名/邮箱带时间戳后缀：这两个字段有唯一索引，固定值第二次跑就撞键了。
	suffix := time.Now().UnixNano() % 1_000_000
	user := &model.User{
		Username:     fmt.Sprintf("test_cn_%d_%d", seed, suffix),
		Email:        fmt.Sprintf("test_cn_%d_%d@test.local", seed, suffix),
		PasswordHash: "not-a-real-hash", // 测试不校验密码，占位即可
		Role:         "customer",
		Status:       "active",
	}
	if err := testRepo.db.Create(user).Error; err != nil {
		t.Fatalf("造测试用户失败：%v", err)
	}
	return user.ID
}

func createTestAddress(t *testing.T, userID uint64) uint64 {
	t.Helper()
	addr := &model.Address{
		UserID: userID, Recipient: "测试", Phone: "13800000000",
		Province: "江苏省", City: "南京市", District: "玄武区",
		Detail: "测试地址", IsDefault: false, // 初始都不是默认，让并发去竞争
	}
	if err := testRepo.db.Create(addr).Error; err != nil {
		t.Fatalf("造测试地址失败：%v", err)
	}
	return addr.ID
}

func cleanupTestUser(t *testing.T, userID uint64) {
	t.Helper()
	// 顺序不能反：addresses 有外键指向 users，先删地址再删用户，否则违反约束。
	// 这里用软删除（GORM 默认行为）：留痕不碍事，且不会和其他用例的历史数据冲突。
	testRepo.db.Where("user_id = ?", userID).Delete(&model.Address{})
	testRepo.db.Delete(&model.User{}, userID)
}
