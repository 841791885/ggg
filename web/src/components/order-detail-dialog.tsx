"use client";

/**
 * 订单详情弹窗：商品明细 + 地址快照 + 状态流转日志。
 *
 * 状态日志是学习这套后端时最有价值的视图——它能让你看到
 * "谁在什么时候把订单从一个状态推到了另一个状态"，包括 worker 自动关单。
 */
import { useEffect, useState } from "react";
import { orderApi, yuan } from "@/lib/api";
import { ORDER_STATUS_LABEL } from "@/lib/order-flow";
import type { Order, OrderStatusLog } from "@/lib/types";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { OrderStatusBadge } from "./status-badge";

const OPERATOR_LABEL: Record<string, string> = { user: "用户", admin: "运营", system: "系统(worker)" };

export function OrderDetailDialog({ order, onClose }: { order: Order | null; onClose: () => void }) {
  const [logs, setLogs] = useState<OrderStatusLog[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!order) return;
    setLoading(true);
    orderApi
      .detail(order.id)
      .then((res) => setLogs(res.status_logs ?? []))
      .catch(() => setLogs([]))
      .finally(() => setLoading(false));
  }, [order]);

  return (
    <Dialog open={!!order} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[80vh] overflow-y-auto sm:max-w-2xl">
        {order && (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                {order.order_no}
                <OrderStatusBadge status={order.status} />
              </DialogTitle>
            </DialogHeader>

            <div className="grid gap-4 text-sm">
              <div className="grid gap-1 text-xs text-muted-foreground">
                <p>
                  实付 {yuan(order.pay_cent)}
                  {order.discount_cent > 0 && ` · 优惠 ${yuan(order.discount_cent)}`}
                </p>
                <p>
                  收货：{order.address_snapshot.recipient} {order.address_snapshot.phone}
                </p>
                <p>
                  {order.address_snapshot.province}
                  {order.address_snapshot.city}
                  {order.address_snapshot.district}
                  {order.address_snapshot.detail}
                </p>
              </div>

              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>商品</TableHead>
                      <TableHead>单价</TableHead>
                      <TableHead>数量</TableHead>
                      <TableHead className="text-right">小计</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {order.items?.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>
                          {item.product_name}
                          <span className="ml-1 text-xs text-muted-foreground">{item.sku_code}</span>
                        </TableCell>
                        <TableCell>{yuan(item.unit_price_cent)}</TableCell>
                        <TableCell>{item.quantity}</TableCell>
                        <TableCell className="text-right">{yuan(item.subtotal_cent)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div>
                <p className="mb-2 text-xs font-semibold">状态流转</p>
                {loading ? (
                  <Skeleton className="h-20" />
                ) : (
                  <div className="rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>时间</TableHead>
                          <TableHead>流转</TableHead>
                          <TableHead>操作者</TableHead>
                          <TableHead>备注</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {logs.map((log) => (
                          <TableRow key={log.id}>
                            <TableCell className="text-xs">{new Date(log.created_at).toLocaleString("zh-CN")}</TableCell>
                            <TableCell className="text-xs">
                              {log.from_status ? ORDER_STATUS_LABEL[log.from_status as keyof typeof ORDER_STATUS_LABEL] : "创建"} →{" "}
                              {ORDER_STATUS_LABEL[log.to_status]}
                            </TableCell>
                            <TableCell className="text-xs">{OPERATOR_LABEL[log.operator_type] ?? log.operator_type}</TableCell>
                            <TableCell className="text-xs">{log.remark}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}