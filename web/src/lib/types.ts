/**
 * 后端接口契约的 TypeScript 镜像（对应 Go 的 controllers/*_response.go）。
 *
 * 为什么要有这个文件：上一版原生 JS 前端反复出现 "undefined" 显示问题
 * （购物车没有 product_name、券没有 seq），根因是"接口返回什么"只存在于
 * 后端代码和口头约定里，前端写到哪算哪。这里把每个响应的形状写死，
 * 组件里写 item.product_name 时编译器就能保证这个字段真的存在。
 *
 * 维护约定：后端 DTO 改了字段 → 这里同步改 → 所有用到的地方立即编译报错。
 * （进阶做法是 Go 生成 TS，学习阶段先手工同步，成本更低也更直观。）
 */

/* ───────────────────────── 通用信封 ───────────────────────── */
// 后端统一响应：{code, message, data}（见 controllers/response.go）
export interface ApiEnvelope<T> {
  code: number;
  message?: string;
  data: T;
}

export interface PageResult<T> {
  total: number;
  list: T[];
}

/* ───────────────────────── 用户与认证 ───────────────────────── */
export type UserRole = "admin" | "customer";

export interface LoginResponse {
  access_token: string;
  // 后端目前只回 token；用户身份由前端从 JWT 解出或登录时按角色判断。
  // 若后端后续补充 user 对象，这里直接加字段即可。
}

/** 前端保存的登录态（localStorage）。 */
export interface AuthState {
  token: string;
  login: string;
  role: UserRole;
}

/* ──────────────────────── 商品与 SKU ───────────────────────── */
export type ProductStatus = "draft" | "on_sale" | "off_sale";
export type SKUStatus = "active" | "inactive";

export interface Product {
  id: number;
  name: string;
  description: string;
  status: ProductStatus;
  created_at: string;
  updated_at: string;
}

export interface SKU {
  id: number;
  product_id: number;
  code: string;
  specs: Record<string, string>;
  price_cent: number;
  stock: number;
  status: SKUStatus;
  created_at: string;
  updated_at: string;
}

/* ───────────────────────── 购物车 ───────────────────────── */
/**
 * 购物车明细。展示字段（product_name 等）由后端 CartService 聚合填充——
 * 曾经这里只有 sku_id，前端无从显示商品名，是上一版 undefined 的元凶之一。
 */
export interface CartItem {
  id: number;
  cart_id: number;
  sku_id: number;
  quantity: number;
  selected: boolean;
  // ↓ 服务端聚合的展示信息
  product_id: number;
  product_name: string;
  sku_code: string;
  price_cent: number;
  stock: number;
  purchasable: boolean;
  reason?: string;
}

export interface Cart {
  id: number;
  user_id: number;
  items: CartItem[];
}

/* ───────────────────────── 下单预览 ───────────────────────── */
export interface PreviewItem {
  item_id: number;
  sku_id: number;
  product_name: string;
  sku_code: string;
  unit_price_cent: number;
  quantity: number;
  subtotal_cent: number;
  stock: number;
  purchasable: boolean;
  reason?: string;
}

export interface CartPreview {
  items: PreviewItem[];
  total_cent: number;
  purchasable_count: number;
}

/* ───────────────────────── 订单 ───────────────────────── */
export type OrderStatus = "pending_payment" | "paid" | "shipped" | "completed" | "cancelled";
export type OperatorType = "user" | "admin" | "system";

export interface AddressSnapshot {
  recipient: string;
  phone: string;
  province: string;
  city: string;
  district: string;
  detail: string;
}

export interface OrderItem {
  id: number;
  order_id: number;
  sku_id: number;
  product_name: string;
  sku_code: string;
  unit_price_cent: number;
  quantity: number;
  subtotal_cent: number;
  refund_status: "none" | "pending" | "refunded";
}

export interface Order {
  id: number;
  order_no: string;
  user_id: number;
  status: OrderStatus;
  total_cent: number;
  pay_cent: number;
  discount_cent: number;
  address_snapshot: AddressSnapshot;
  items?: OrderItem[];
  expires_at: string | null;
  paid_at: string | null;
  shipped_at: string | null;
  completed_at: string | null;
  cancelled_at: string | null;
  created_at: string;
}

export interface OrderStatusLog {
  id: number;
  order_id: number;
  from_status: OrderStatus | "";
  to_status: OrderStatus;
  operator_type: OperatorType;
  operator_id: number;
  remark: string;
  created_at: string;
}

/* ───────────────────────── 支付 ───────────────────────── */
export type PaymentStatus = "pending" | "success" | "failed" | "closed";

export interface Payment {
  id: number;
  payment_no: string;
  order_id: number;
  user_id: number;
  amount_cent: number;
  currency: string;
  channel: string;
  status: PaymentStatus;
  event_no: string | null;
  paid_at: string | null;
  created_at: string;
}

/* ───────────────────────── 收货地址 ──────────────────────── */
export interface Address {
  id: number;
  user_id: number;
  recipient: string;
  phone: string;
  province: string;
  city: string;
  district: string;
  detail: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

/* ───────────────────────── 优惠券 ───────────────────────── */
export type CouponTemplateStatus = "active" | "inactive";
export type UserCouponStatus = "unused" | "used" | "expired";

export interface CouponTemplate {
  id: number;
  name: string;
  type: string;
  threshold_cent: number;
  discount_cent: number;
  total_count: number;
  remaining: number;
  per_user_limit: number;
  starts_at: string;
  ends_at: string;
  status: CouponTemplateStatus;
}

/**
 * 用户持有的券。template_name / threshold_cent / discount_cent 是后端聚合的展示字段
 * （券包页要显示"满X减Y"，光有 template_id 前端无从渲染）；seq 是 A4 引入的"第几张"。
 */
export interface UserCoupon {
  id: number;
  template_id: number;
  seq: number;
  status: UserCouponStatus;
  template_name: string;
  threshold_cent: number;
  discount_cent: number;
  claimed_at: string;
  used_at: string | null;
}

/* ───────────────────────── 退款 ───────────────────────── */
export type RefundStatus = "pending" | "approved" | "rejected" | "refunded";

export interface Refund {
  id: number;
  refund_no: string;
  order_id: number;
  order_item_id: number;
  amount_cent: number;
  reason: string;
  status: RefundStatus;
  reviewed_by: number | null;
  reviewed_at: string | null;
  created_at: string;
}

/* ───────────────────────── 通知 ───────────────────────── */
export interface Notification {
  id: number;
  type: string;
  title: string;
  content: string;
  read: boolean;
  read_at: string | null;
  created_at: string;
}

/* ──────────────────────── 后台任务 ───────────────────────── */
export type TaskStatus = "pending" | "running" | "succeeded" | "failed";

export interface BackgroundTask {
  id: number;
  task_type: string;
  payload: Record<string, unknown>;
  status: TaskStatus;
  attempts: number;
  max_attempts: number;
  next_run_at: string;
  last_error: string;
  created_at: string;
}

/* ───────────────────────── 模拟支付渠道路由 ───────────────────────── */
/**
 * 前端代演"渠道回调"时提交的报文（对应后端 services.MockPaymentCallback）。
 * signature 必须按 支付单号|订单号|事件号|结果|金额|币种 拼接后用
 * HMAC-SHA256(密钥, 串) 计算——与后端 CanonicalString 完全一致，改动需同步。
 */
export interface MockCallbackPayload {
  payment_no: string;
  order_no: string;
  event_no: string;
  result: "success" | "failed";
  amount_cent: number;
  currency: string;
  signature: string;
}