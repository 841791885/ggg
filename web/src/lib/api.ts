/**
 * 统一的 API 客户端。
 *
 * 职责边界：这里只管"怎么跟后端说话"（鉴权头、错误解包、类型标注），
 * 不管业务规则，也不碰 React 状态——组件通过 hooks 调用它。
 *
 * 与上一版原生 JS 的差别：所有方法的返回值都有明确类型，
 * 组件里读到 item.product_name 时编译器保证该字段存在。
 */
import type {
  Address,
  ApiEnvelope,
  AuthState,
  BackgroundTask,
  Cart,
  CartItem,
  CartPreview,
  CouponTemplate,
  LoginResponse,
  Notification,
  Order,
  OrderStatusLog,
  PageResult,
  Payment,
  Product,
  Refund,
  SKU,
  UserCoupon,
} from "./types";

const TOKEN_KEY = "gomall_token";
const AUTH_KEY = "gomall_auth";

/** 读取登录态（客户端组件用；SSR 时返回 null）。 */
export function getAuth(): AuthState | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(AUTH_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as AuthState;
  } catch {
    return null;
  }
}

export function setAuth(state: AuthState | null) {
  if (typeof window === "undefined") return;
  if (state) {
    window.localStorage.setItem(AUTH_KEY, JSON.stringify(state));
    window.localStorage.setItem(TOKEN_KEY, state.token);
  } else {
    window.localStorage.removeItem(AUTH_KEY);
    window.localStorage.removeItem(TOKEN_KEY);
  }
}

/** 业务错误：携带后端返回的 code/message，供 UI 区分展示。 */
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  /** 额外请求头（如下单的 Idempotency-Key）。 */
  headers?: Record<string, string>;
  /** 携带 token（默认 true；登录等公开接口置 false）。 */
  auth?: boolean;
}

/**
 * 核心请求函数。所有 API 调用都经由它，保证：
 *  1. 自动附带 Authorization（除非显式关闭）
 *  2. 统一解包 {code, message, data} 信封，失败抛 ApiError
 *  3. 网络异常也抛 ApiError，调用方只需 catch 一处
 */
async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, headers = {}, auth = true } = options;
  const finalHeaders: Record<string, string> = { "Content-Type": "application/json", ...headers };
  if (auth) {
    const token = typeof window !== "undefined" ? window.localStorage.getItem(TOKEN_KEY) : null;
    if (token) finalHeaders.Authorization = `Bearer ${token}`;
  }

  let response: Response;
  try {
    response = await fetch(path, {
      method,
      headers: finalHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "网络异常：请确认后端服务（go run .）已启动");
  }

  let payload: ApiEnvelope<T> | null = null;
  try {
    payload = (await response.json()) as ApiEnvelope<T>;
  } catch {
    throw new ApiError(response.status, `响应解析失败（HTTP ${response.status}）`);
  }

  if (!response.ok || (payload.code && payload.code >= 400)) {
    throw new ApiError(payload.code || response.status, payload.message || "请求失败");
  }
  return payload.data;
}

const qs = (params: Record<string, string | number | undefined>) => {
  const sp = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== "") sp.set(k, String(v));
  });
  const s = sp.toString();
  return s ? `?${s}` : "";
};

/* ═══════════════════════ 认证 ═══════════════════════ */
export const authApi = {
  login: (login: string, password: string) =>
    request<LoginResponse>("/api/v1/auth/login", { method: "POST", body: { login, password }, auth: false }),
};

/* ═══════════════════════ 商品与 SKU ═══════════════════════ */
export const productApi = {
  /** 公开目录：后端强制只返回在售商品。 */
  listOnSale: (page = 1, pageSize = 50, name = "") =>
    request<PageResult<Product>>(`/api/v1/products${qs({ page, page_size: pageSize, name })}`, { auth: false }),
  /** 公开规格：只返回启用中的 SKU。 */
  listPublicSKUs: (productId: number) =>
    request<PageResult<SKU>>(`/api/v1/products/${productId}/skus`, { auth: false }),
  // ── 运营接口 ──
  adminList: (page = 1, pageSize = 10, name = "") =>
    request<PageResult<Product>>(`/api/v1/admin/products${qs({ page, page_size: pageSize, name })}`),
  create: (name: string, description: string) =>
    request<Product>("/api/v1/admin/products", { method: "POST", body: { name, description } }),
  update: (id: number, name?: string, description?: string) =>
    request<Product>(`/api/v1/admin/products/${id}`, {
      method: "PATCH",
      body: { ...(name !== undefined && { name }), ...(description !== undefined && { description }) },
    }),
  /** 上架/下架/转草稿：状态机在服务端校验，非法迁移返回 409。 */
  updateStatus: (id: number, status: string) =>
    request<Product>(`/api/v1/admin/products/${id}/status`, { method: "PATCH", body: { status } }),
  remove: (id: number) => request<void>(`/api/v1/admin/products/${id}`, { method: "DELETE" }),
  listSKUs: (productId: number) =>
    request<PageResult<SKU>>(`/api/v1/admin/products/${productId}/skus${qs({ page: 1, page_size: 100 })}`),
  createSKU: (productId: number, body: { code: string; specs: Record<string, string>; price_cent: number; stock: number }) =>
    request<SKU>(`/api/v1/admin/products/${productId}/skus`, { method: "POST", body }),
  updateSKUStatus: (productId: number, skuId: number, status: string) =>
    request<SKU>(`/api/v1/admin/products/${productId}/skus/${skuId}/status`, { method: "PATCH", body: { status } }),
};

/* ══════════════════════ 购物车 ══════════════════════ */
export const cartApi = {
  get: () => request<Cart>("/api/v1/cart"),
  add: (skuId: number, quantity: number) =>
    request<CartItem>("/api/v1/cart/items", { method: "POST", body: { sku_id: skuId, quantity } }),
  updateQuantity: (itemId: number, quantity: number) =>
    request<CartItem>(`/api/v1/cart/items/${itemId}`, { method: "PATCH", body: { quantity } }),
  setSelected: (itemId: number, selected: boolean) =>
    request<CartItem>(`/api/v1/cart/items/${itemId}`, { method: "PATCH", body: { selected } }),
  remove: (itemId: number) => request<void>(`/api/v1/cart/items/${itemId}`, { method: "DELETE" }),
  preview: () => request<CartPreview>("/api/v1/cart/preview", { method: "POST" }),
};

/* ═══════════════════════ 订单 ══════════════════════ */
export const orderApi = {
  /**
   * 创建订单。幂等键由调用方生成并保证"一次购买意图一个值"——
   * 重试必须复用同一个键，否则会重复下单（详见后端 order_service.go 注释）。
   */
  create: (addressId: number, idempotencyKey: string) =>
    request<Order>("/api/v1/orders", {
      method: "POST",
      body: { address_id: addressId },
      headers: { "Idempotency-Key": idempotencyKey },
    }),
  list: (status = "", page = 1, pageSize = 30) =>
    request<PageResult<Order>>(`/api/v1/orders${qs({ status, page, page_size: pageSize })}`),
  detail: (id: number) => request<{ order: Order; status_logs: OrderStatusLog[] }>(`/api/v1/orders/${id}`),
  cancel: (id: number) => request<Order>(`/api/v1/orders/${id}/cancel`, { method: "POST" }),
  confirmReceipt: (id: number) => request<Order>(`/api/v1/orders/${id}/confirm-receipt`, { method: "POST" }),
  // 运营
  adminList: (status = "", page = 1, pageSize = 30) =>
    request<PageResult<Order>>(`/api/v1/admin/orders${qs({ status, page, page_size: pageSize })}`),
  ship: (id: number) => request<Order>(`/api/v1/admin/orders/${id}/ship`, { method: "POST" }),
};

/* ══════════════════════ 支付 ══════════════════════ */
export const paymentApi = {
  create: (orderId: number) => request<Payment>(`/api/v1/orders/${orderId}/payments`, { method: "POST" }),
  getByNo: (paymentNo: string) => request<Payment>(`/api/v1/payments/${paymentNo}`),
};

/* ═══════════════════════ 地址 ═══════════════════════ */
export const addressApi = {
  list: () => request<{ items: Address[] }>("/api/v1/addresses"),
  create: (body: Omit<Address, "id" | "user_id" | "is_default" | "created_at" | "updated_at">) =>
    request<Address>("/api/v1/addresses", { method: "POST", body }),
  update: (id: number, body: Partial<Address>) => request<Address>(`/api/v1/addresses/${id}`, { method: "PUT", body }),
  setDefault: (id: number) => request<Address>(`/api/v1/addresses/${id}/default`, { method: "PUT" }),
  remove: (id: number) => request<void>(`/api/v1/addresses/${id}`, { method: "DELETE" }),
};

/* ═══════════════════════ 优惠券 ═══════════════════════ */
export const couponApi = {
  /** 可领取的券模板（后端强制 active）。 */
  listTemplates: () => request<PageResult<CouponTemplate>>("/api/v1/coupons/templates?page=1&page_size=50"),
  listMine: () => request<{ items: UserCoupon[] }>("/api/v1/coupons"),
  claim: (templateId: number) => request<UserCoupon>(`/api/v1/coupons/${templateId}/claim`, { method: "POST" }),
  // 运营
  adminList: () => request<PageResult<CouponTemplate>>("/api/v1/admin/coupon-templates?page=1&page_size=50"),
};

/* ═══════════════════════ 退款 ══════════════════════ */
export const refundApi = {
  listMine: () => request<{ items: Refund[] }>("/api/v1/refunds"),
  apply: (orderItemId: number, amountCent: number, reason: string) =>
    request<Refund>(`/api/v1/order-items/${orderItemId}/refunds`, { method: "POST", body: { amount_cent: amountCent, reason } }),
  adminList: (status = "") => request<PageResult<Refund>>(`/api/v1/admin/refunds${qs({ page: 1, page_size: 30, status })}`),
  approve: (id: number) => request<Refund>(`/api/v1/admin/refunds/${id}/approve`, { method: "POST" }),
  reject: (id: number) => request<Refund>(`/api/v1/admin/refunds/${id}/reject`, { method: "POST" }),
};

/* ═══════════════════════ 通知与任务 ═══════════════════════ */
export const notificationApi = {
  list: () => request<PageResult<Notification>>("/api/v1/notifications?page=1&page_size=50"),
  markAllRead: () => request<void>("/api/v1/notifications/read-all", { method: "PUT" }),
};

export const taskApi = {
  list: (status = "") => request<PageResult<BackgroundTask>>(`/api/v1/admin/tasks${qs({ page: 1, page_size: 50, status })}`),
  retry: (id: number) => request<BackgroundTask>(`/api/v1/admin/tasks/${id}/retry`, { method: "POST" }),
};

/* ══════════════ 模拟支付渠道（前端代演渠道角色） ═══════════════ */
/** 与后端 config.yaml 的 payment.callback_secret 一致；仅开发环境演示用。 */
export const MOCK_CHANNEL_SECRET = "mock-channel-secret-dev-only";

/**
 * 计算回调签名（HMAC-SHA256）。
 * 拼接规则必须与后端 services.MockPaymentCallback.CanonicalString 完全一致，
 * 否则验签失败——这是"前后端共享同一份协议"的典型例子。
 */
export async function signCallback(params: {
  paymentNo: string;
  orderNo: string;
  eventNo: string;
  result: "success" | "failed";
  amountCent: number;
  currency: string;
}): Promise<string> {
  const canonical = `${params.paymentNo}|${params.orderNo}|${params.eventNo}|${params.result}|${params.amountCent}|${params.currency}`;
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(MOCK_CHANNEL_SECRET),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(canonical));
  return Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

/** 向"渠道回调"端点投递支付结果（模拟真实渠道服务器的异步通知）。 */
export async function sendMockCallback(payload: {
  paymentNo: string;
  orderNo: string;
  amountCent: number;
  currency: string;
  result?: "success" | "failed";
}) {
  const eventNo = `evt-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const result = payload.result ?? "success";
  const signature = await signCallback({
    paymentNo: payload.paymentNo,
    orderNo: payload.orderNo,
    eventNo,
    result,
    amountCent: payload.amountCent,
    currency: payload.currency,
  });
  return request<{ received: boolean }>("/api/v1/payment-callbacks/mock", {
    method: "POST",
    auth: false, // 回调是渠道服务器发起的，无登录态；安全靠验签
    body: {
      payment_no: payload.paymentNo,
      order_no: payload.orderNo,
      event_no: eventNo,
      result,
      amount_cent: payload.amountCent,
      currency: payload.currency,
      signature,
    },
  });
}

/** 生成幂等键（一次购买意图一个值）。 */
export const newIdempotencyKey = () =>
  `web-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;

/** 格式化分 → 元。 */
export const yuan = (cent: number) => `¥${(cent / 100).toFixed(2)}`;