/**
 * 支付编排：创建支付单 → 代演渠道回调 → 轮询真实状态。
 *
 * 为什么前端要"演渠道"：真实系统里回调由支付渠道的服务器发起，
 * 学习项目没有那个服务器，所以由页面用共享密钥签一次 HMAC 并投递，
 * 完整走一遍"异步通知 → 验签 → 幂等消费"的真实链路。
 */
import { paymentApi, sendMockCallback } from "./api";

export interface PayOutcome {
  status: "success" | "failed" | "timeout";
  step: string;
}

/** 执行一次完整支付。onStep 用于向用户播报进度（体现异步过程）。 */
export async function payOrder(
  orderId: number,
  orderNo: string,
  onStep?: (text: string) => void,
): Promise<PayOutcome> {
  onStep?.("① 创建支付单…");
  const payment = await paymentApi.create(orderId);

  onStep?.("② 模拟渠道扣款并发送回调…");
  await sendMockCallback({
    paymentNo: payment.payment_no,
    orderNo,
    amountCent: payment.amount_cent,
    currency: payment.currency,
  });

  // 回调是异步的：轮询直到支付单离开 pending（最多约 3 秒）。
  for (let i = 0; i < 10; i++) {
    const latest = await paymentApi.getByNo(payment.payment_no);
    if (latest.status !== "pending") {
      onStep?.(latest.status === "success" ? "③ 支付成功" : `③ 支付结束：${latest.status}`);
      return { status: latest.status === "success" ? "success" : "failed", step: latest.status };
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  return { status: "timeout", step: "等待回调超时" };
}
