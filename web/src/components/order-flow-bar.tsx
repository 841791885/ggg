"use client";

/**
 * 交易流程条：显示某订单在"提交→支付→发货→收货→完成"中的位置。
 *
 * 步骤高亮/完成的判断全部来自 lib/order-flow.ts 的纯函数，
 * 这里只负责渲染——上一版把判断和渲染混在一起，是 bug 高发区。
 */
import { FLOW_STEPS, flowStepOf } from "@/lib/order-flow";
import { cn } from "@/lib/utils";
import type { Order } from "@/lib/types";

export function OrderFlowBar({ order }: { order: Order | null }) {
  if (!order) {
    return (
      <div className="rounded-lg border bg-card px-4 py-5 text-sm text-muted-foreground">
        选择下方任一订单，这里会显示它在交易流程中的位置
      </div>
    );
  }

  const current = flowStepOf(order.status);
  const cancelled = order.status === "cancelled";

  return (
    <div className="flex flex-wrap items-center gap-1.5 rounded-lg border bg-card px-4 py-5">
      {FLOW_STEPS.map((step, i) => (
        <div key={step} className="flex items-center gap-1.5">
          {i > 0 && <span className="text-xs text-muted-foreground">→</span>}
          <span
            className={cn(
              "rounded-full px-2.5 py-1 text-xs whitespace-nowrap",
              cancelled
                ? "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300"
                : i < current
                  ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                  : i === current
                    ? "bg-primary font-semibold text-primary-foreground"
                    : "bg-muted text-muted-foreground",
            )}
          >
            {i < current && !cancelled ? "✓ " : ""}
            {step}
          </span>
        </div>
      ))}
      {cancelled && <span className="ml-2 rounded-full bg-red-100 px-2.5 py-1 text-xs text-red-700 dark:bg-red-950 dark:text-red-300">已关闭</span>}
    </div>
  );
}