"use client";

/**
 * 侧栏导航。用 next/link 做客户端路由（不刷新页面），
 * 当前项高亮由 usePathname 驱动——不再需要手工切 active 类。
 */
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

interface NavItem {
  href: string;
  label: string;
  icon: string;
  adminOnly?: boolean;
}

const NAV_GROUPS: { title: string; items: NavItem[] }[] = [
  {
    title: "交易流程",
    items: [
      { href: "/shop", label: "逛商城", icon: "️" },
      { href: "/cart", label: "购物车", icon: "🛒" },
      { href: "/orders", label: "我的订单", icon: "" },
    ],
  },
  {
    title: "账户",
    items: [
      { href: "/addresses", label: "收货地址", icon: "📍" },
      { href: "/coupons", label: "优惠券", icon: "🎟️" },
      { href: "/refunds", label: "退款售后", icon: "↩️" },
      { href: "/notifications", label: "消息通知", icon: "🔔" },
    ],
  },
  {
    title: "运营",
    items: [
      { href: "/dashboard", label: "概览待办", icon: "⌂", adminOnly: true },
      { href: "/products", label: "商品管理", icon: "□", adminOnly: true },
      { href: "/tasks", label: "后台任务", icon: "️", adminOnly: true },
    ],
  },
];

export function Sidebar() {
  const pathname = usePathname();
  const { auth, isAdmin, logout } = useAuth();

  return (
    <aside className="sticky top-0 flex h-screen w-56 shrink-0 flex-col border-r bg-card px-3 py-5">
      <div className="mb-6 flex items-center gap-2 px-2">
        <span className="flex size-7 items-center justify-center rounded-md bg-primary text-sm font-bold text-primary-foreground">
          G
        </span>
        <strong className="text-sm">GoMall</strong>
      </div>

      <nav className="flex flex-1 flex-col gap-1 overflow-y-auto">
        {NAV_GROUPS.map((group) => {
          const items = group.items.filter((i) => !i.adminOnly || isAdmin);
          if (items.length === 0) return null;
          return (
            <div key={group.title} className="mb-2">
              <p className="px-2 py-2 text-[10px] font-bold tracking-wide text-muted-foreground uppercase">
                {group.title}
              </p>
              {items.map((item) => {
                const active = pathname === item.href;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={cn(
                      "flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors",
                      active
                        ? "bg-accent font-medium text-accent-foreground"
                        : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                    )}
                  >
                    <span>{item.icon}</span>
                    {item.label}
                  </Link>
                );
              })}
            </div>
          );
        })}
      </nav>

      <div className="flex items-center justify-between border-t pt-4">
        <div className="min-w-0">
          <p className="truncate text-xs font-medium">{auth?.login}</p>
          <p className="text-[10px] text-muted-foreground">{isAdmin ? "运营" : "买家"}</p>
        </div>
        <Button variant="ghost" size="sm" onClick={logout} title="退出登录">
          ↪
        </Button>
      </div>
    </aside>
  );
}