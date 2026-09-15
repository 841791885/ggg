import { Badge } from "@/components/ui/badge";
import { ORDER_STATUS_LABEL } from "@/lib/order-flow";
import { cn } from "@/lib/utils";
import type { OrderStatus, ProductStatus, TaskStatus } from "@/lib/types";

/**
 * 状态徽章：把各种业务状态映射成带配色的标签。
 * 集中在一处，避免各页面各写一套配色导致同一个状态到处长得不一样。
 */
const ORDER_STYLE: Record<OrderStatus, string> = {
  pending_payment: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  paid: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300",
  shipped: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  completed: "bg-muted text-muted-foreground",
  cancelled: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
};

export function OrderStatusBadge({ status }: { status: OrderStatus }) {
  return <Badge variant="outline" className={cn("border-0", ORDER_STYLE[status])}>{ORDER_STATUS_LABEL[status]}</Badge>;
}

const PRODUCT_LABEL: Record<ProductStatus, string> = { draft: "草稿", on_sale: "在售", off_sale: "已下架" };
const PRODUCT_STYLE: Record<ProductStatus, string> = {
  draft: "bg-muted text-muted-foreground",
  on_sale: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  off_sale: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
};

export function ProductStatusBadge({ status }: { status: ProductStatus }) {
  return <Badge variant="outline" className={cn("border-0", PRODUCT_STYLE[status])}>{PRODUCT_LABEL[status]}</Badge>;
}

const TASK_LABEL: Record<TaskStatus, string> = { pending: "排队", running: "执行中", succeeded: "成功", failed: "失败" };
const TASK_STYLE: Record<TaskStatus, string> = {
  pending: "bg-muted text-muted-foreground",
  running: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300",
  succeeded: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  failed: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
};

export function TaskStatusBadge({ status }: { status: TaskStatus }) {
  return <Badge variant="outline" className={cn("border-0", TASK_STYLE[status])}>{TASK_LABEL[status]}</Badge>;
}
