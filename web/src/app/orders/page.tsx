"use client";

/**
 * 我的订单：流程条 + 订单卡片 + 下一步按钮。
 *
 * 关键设计：每张卡片只有一个"主按钮"，显示什么由 nextActionOf(order, isAdmin) 决定。
 * 上一版把按钮散落一地，出现"已关闭订单还显示支付按钮""点了没反应"等问题——
 * 现在"能点的按钮一定可执行"由纯函数保证。
 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { orderApi, yuan } from "@/lib/api";
import { payOrder } from "@/lib/pay";
import { useAuth } from "@/lib/auth-context";
import {
  canCancel,
  canPay,
  formatDuration,
  nextActionOf,
  secondsLeft,
} from "@/lib/order-flow";
import type { Order, OrderStatus } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { OrderStatusBadge } from "@/components/status-badge";
import { OrderFlowBar } from "@/components/order-flow-bar";
import { OrderDetailDialog } from "@/components/order-detail-dialog";

const TABS: { value: string; label: string }[] = [
  { value: "", label: "全部" },
  { value: "pending_payment", label: "待支付" },
  { value: "paid", label: "待发货" },
  { value: "shipped", label: "待收货" },
  { value: "completed", label: "已完成" },
  { value: "cancelled", label: "已关闭" },
];

export default function OrdersPage() {
  const { isAdmin } = useAuth();
  const [orders, setOrders] = useState<Order[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [detailOrder, setDetailOrder] = useState<Order | null>(null);
  const [tick, setTick] = useState(0);

  const load = useCallback(async () => {
    try {
      // 运营看全量订单（含待发货，便于代发货）；买家只看自己的。
      const res = isAdmin ? await orderApi.adminList(filter) : await orderApi.list(filter);
      setOrders(res.list ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载订单失败");
    } finally {
      setLoading(false);
    }
  }, [filter, isAdmin]);

  useEffect(() => {
    void load();
  }, [load]);

  // 每秒重渲染一次，驱动待支付订单的倒计时。
  useEffect(() => {
    const t = setInterval(() => setTick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, []);
  void tick;

  async function handlePay(order: Order) {
    if (!canPay(order)) {
      toast.error("该订单当前状态无法支付");
      return;
    }
    setBusyId(order.id);
    try {
      const outcome = await payOrder(order.id, order.order_no, (s) => toast.info(s, { duration: 1500 }));
      if (outcome.status === "success") toast.success("支付成功，订单已进入待发货");
      else toast.error(`支付未成功：${outcome.step}`);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "支付失败");
    } finally {
      setBusyId(null);
    }
  }

  async function handleCancel(order: Order) {
    setBusyId(order.id);
    try {
      await orderApi.cancel(order.id);
      toast.success("订单已取消，库存已释放");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "取消失败");
    } finally {
      setBusyId(null);
    }
  }

  async function handleShip(order: Order) {
    setBusyId(order.id);
    try {
      await orderApi.ship(order.id);
      toast.success("已发货（运营动作）");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "发货失败");
    } finally {
      setBusyId(null);
    }
  }

  async function handleConfirm(order: Order) {
    setBusyId(order.id);
    try {
      await orderApi.confirmReceipt(order.id);
      toast.success("已确认收货");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "确认收货失败");
    } finally {
      setBusyId(null);
    }
  }

  const focused = orders.find((o) => o.status !== "cancelled") ?? orders[0] ?? null;

  return (
    <div className="flex flex-col gap-4">
      <OrderFlowBar order={focused} />

      <Tabs value={filter} onValueChange={setFilter}>
        <TabsList className="flex-wrap">
          {TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      {loading ? (
        <div className="grid gap-3">{[1, 2].map((i) => <Skeleton key={i} className="h-32" />)}</div>
      ) : orders.length === 0 ? (
        <Card>
          <CardContent className="py-16 text-center text-sm text-muted-foreground">还没有订单</CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {orders.map((order) => {
            const action = nextActionOf(order, isAdmin);
            const left = secondsLeft(order);
            const busy = busyId === order.id;
            return (
              <Card key={order.id}>
                <CardContent className="flex flex-col gap-3 py-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{order.order_no}</p>
                      <p className="text-xs text-muted-foreground">
                        {new Date(order.created_at).toLocaleString("zh-CN")} · {order.items?.length ?? 0} 件 ·{" "}
                        {yuan(order.pay_cent)}
                        {left !== null && (
                          <span className={left < 300 ? "ml-2 text-destructive" : "ml-2"}>
                            {left > 0 ? `剩余 ${formatDuration(left)}` : "等待自动关单…"}
                          </span>
                        )}
                      </p>
                    </div>
                    <OrderStatusBadge status={order.status} />
                  </div>

                  <ul className="grid gap-1 rounded-md bg-muted/50 px-3 py-2 text-xs">
                    {order.items?.map((item) => (
                      <li key={item.id} className="flex justify-between">
                        <span>
                          {item.product_name} {item.sku_code} ×{item.quantity}
                        </span>
                        <span>{yuan(item.subtotal_cent)}</span>
                      </li>
                    ))}
                  </ul>

                  <div className="flex flex-wrap items-center gap-2">
                    {/* 主按钮：由 nextActionOf 决定，保证"显示了就一定可执行" */}
                    {action.kind === "pay" && (
                      <Button size="sm" disabled={busy} onClick={() => handlePay(order)}>
                        {busy ? "支付中…" : "立即支付"}
                      </Button>
                    )}
                    {action.kind === "ship" && (
                      <Button size="sm" disabled={busy} onClick={() => handleShip(order)}>
                        发货
                      </Button>
                    )}
                    {action.kind === "confirm" && (
                      <Button size="sm" disabled={busy} onClick={() => handleConfirm(order)}>
                        确认收货
                      </Button>
                    )}
                    {action.kind === "aftersale" && (
                      <Button size="sm" variant="secondary" onClick={() => setDetailOrder(order)}>
                        查看详情
                      </Button>
                    )}
                    {action.kind === "wait" && <span className="text-xs text-muted-foreground">{action.hint}</span>}

                    {canCancel(order) && (
                      <Button size="sm" variant="outline" disabled={busy} onClick={() => handleCancel(order)}>
                        取消订单
                      </Button>
                    )}
                    <Button size="sm" variant="ghost" onClick={() => setDetailOrder(order)}>
                      详情/日志
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      <OrderDetailDialog order={detailOrder} onClose={() => setDetailOrder(null)} />
    </div>
  );
}

export type { OrderStatus };