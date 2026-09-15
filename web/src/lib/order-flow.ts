/**
 * 订单流程引擎 —— 全项目判断"这单走到哪了、下一步能做什么"的唯一出处。
 *
 * 上一版把这个逻辑散落在渲染函数里，导致按钮该显示的不显示、该禁用的不禁用。
 * 抽成纯函数的好处：可以单独测试，组件只负责渲染结果。
 */
import type { Order, OrderStatus } from "./types";

/** 交易主线的五个阶段（进度条按此渲染）。 */
export const FLOW_STEPS = ["提交订单", "支付成功", "商家发货", "确认收货", "完成"] as const;

/** 状态 → 中文标签。 */
export const ORDER_STATUS_LABEL: Record<OrderStatus, string> = {
  pending_payment: "待支付",
  paid: "待发货",
  shipped: "待收货",
  completed: "已完成",
  cancelled: "已关闭",
};

/** 状态 → 在流程条中的位置（-1 表示旁路终态，不参与进度展示）。 */
export function flowStepOf(status: OrderStatus): number {
  switch (status) {
    case "pending_payment":
      return 0;
    case "paid":
      return 1;
    case "shipped":
      return 2;
    case "completed":
      return 4;
    default:
      return -1; // cancelled
  }
}

/** 下一步动作的类型；由组件决定如何渲染与执行。 */
export type OrderAction =
  | { kind: "pay" }
  | { kind: "ship" } // 运营发货
  | { kind: "confirm" } // 确认收货
  | { kind: "aftersale" }
  | { kind: "wait"; hint: string } // 等待对方（如等商家发货）
  | { kind: "none" };

/**
 * 判断某订单"现在能做什么"。
 * @param isAdmin 运营身份可代为发货；买家只能等待。
 */
export function nextActionOf(order: Order, isAdmin: boolean): OrderAction {
  switch (order.status) {
    case "pending_payment":
      return { kind: "pay" };
    case "paid":
      return isAdmin ? { kind: "ship" } : { kind: "wait", hint: "等待商家发货" };
    case "shipped":
      return { kind: "confirm" };
    case "completed":
      return { kind: "aftersale" };
    default:
      return { kind: "none" }; // 已取消
  }
}

/** 待支付订单的剩余时间（秒）；非待支付返回 null。 */
export function secondsLeft(order: Order): number | null {
  if (order.status !== "pending_payment" || !order.expires_at) return null;
  return Math.max(0, Math.round((new Date(order.expires_at).getTime() - Date.now()) / 1000));
}

/** 秒 → "12分34秒" / "45秒"。 */
export function formatDuration(seconds: number): string {
  if (seconds >= 60) return `${Math.floor(seconds / 60)}分${seconds % 60}秒`;
  return `${seconds}秒`;
}

/** 订单能否取消：仅待支付。 */
export const canCancel = (order: Order) => order.status === "pending_payment";

/** 订单能否支付：仅待支付（前端守卫，后端状态机仍会兜底校验）。 */
export const canPay = (order: Order) => order.status === "pending_payment";