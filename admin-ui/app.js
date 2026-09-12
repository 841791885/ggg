const state = { token: localStorage.getItem("gomall_token") || "", page: 1, pageSize: 10, total: 0, products: [], editingID: 0, drawerProduct: null, skuCatalog: [], addresses: [], addressEditingID: 0, orders: [], coupons: [], myCoupons: [], tasks: [], refunds: [], adminRefunds: [], notifications: [] };
const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];
const productImages = [
  "https://images.unsplash.com/photo-1496181133206-80ce9b88a853?auto=format&fit=crop&w=160&q=75",
  "https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=160&q=75",
  "https://images.unsplash.com/photo-1505740420928-5e560c06d30e?auto=format&fit=crop&w=160&q=75",
  "https://images.unsplash.com/photo-1586023492125-27b2c045efd7?auto=format&fit=crop&w=160&q=75"
];

async function api(path, options = {}) {
  const headers = { ...(options.body ? { "Content-Type": "application/json" } : {}), ...(state.token ? { Authorization: `Bearer ${state.token}` } : {}), ...options.headers };
  const response = await fetch(path, { ...options, headers });
  const body = response.status === 204 ? null : await response.json().catch(() => null);
  if (response.status === 401) { logout(); throw new Error(body?.message || "登录已过期"); }
  if (!response.ok) throw new Error(body?.message || `请求失败 (${response.status})`);
  return body?.data;
}

function toast(message) { const el = $("#toast"); el.textContent = message; el.classList.remove("hidden"); setTimeout(() => el.classList.add("hidden"), 2200); }
function formatDate(value) { return value ? new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(new Date(value)) : "-"; }
function statusLabel(status) { return ({ draft: "草稿", on_sale: "在售", off_sale: "已下架", active: "启用", inactive: "停用" })[status] || status; }
function escapeHTML(value = "") { const div = document.createElement("div"); div.textContent = value; return div.innerHTML; }

function showApp() { $("#loginView").classList.add("hidden"); $("#appView").classList.remove("hidden"); loadDashboard(); }
function logout() { state.token = ""; localStorage.removeItem("gomall_token"); $("#appView").classList.add("hidden"); $("#loginView").classList.remove("hidden"); }

$("#loginForm").addEventListener("submit", async (event) => {
  event.preventDefault(); $("#loginError").textContent = "";
  try {
    const result = await api("/api/v1/auth/login", { method: "POST", body: JSON.stringify({ login: $("#loginName").value, password: $("#loginPassword").value }) });
    state.token = result.access_token; localStorage.setItem("gomall_token", state.token); showApp();
  } catch (error) { $("#loginError").textContent = error.message; }
});
$("#logoutButton").addEventListener("click", logout);

const viewMeta = { dashboard: ["业务概览", "商城核心数据与状态"], products: ["商品管理", "维护商品信息与 SKU"], cart: ["购物车", "查看当前登录用户的购物车"], addresses: ["收货地址", "维护当前登录用户的收货地址"], orders: ["我的订单", "下单、支付与生命周期"], coupons: ["优惠券", "领取与管理满减券"], refunds: ["退款售后", "申请退款与运营审核"], notifications: ["消息通知", "交易事件站内信"], tasks: ["后台任务", "Outbox 任务监控与重试"], permissions: ["角色权限", "路由级权限映射"] };
function switchView(name) {
  $$(".view").forEach((el) => el.classList.add("hidden")); $(`#${name}View`).classList.remove("hidden");
  $$(".nav-item").forEach((el) => el.classList.toggle("active", el.dataset.view === name));
  $("#pageTitle").textContent = viewMeta[name][0]; $("#pageSubtitle").textContent = viewMeta[name][1];
  if (name === "products") loadProducts();
  if (name === "cart") loadCartPage();
  if (name === "addresses") loadAddressPage();
  if (name === "orders") loadOrderPage();
  if (name === "coupons") loadCouponPage();
  if (name === "refunds") loadRefundPage();
  if (name === "notifications") loadNotificationPage();
  if (name === "tasks") loadTaskPage();
}
$$(".nav-item").forEach((button) => button.addEventListener("click", () => switchView(button.dataset.view)));
$$("[data-go-products]").forEach((button) => button.addEventListener("click", () => switchView("products")));
$("#refreshButton").addEventListener("click", () => {
  const currentView = $(".nav-item.active").dataset.view;
  if (currentView === "products") loadProducts();
  else if (currentView === "cart") loadCartPage();
  else if (currentView === "addresses") loadAddressPage();
  else if (currentView === "orders") loadOrderPage();
  else if (currentView === "coupons") loadCouponPage();
  else if (currentView === "refunds") loadRefundPage();
  else if (currentView === "notifications") loadNotificationPage();
  else if (currentView === "tasks") loadTaskPage();
  else loadDashboard();
});

async function loadDashboard() {
  try {
    const data = await api("/api/v1/admin/products?page=1&page_size=20");
    const products = data.list || []; $("#metricProducts").textContent = data.total; $("#metricOnSale").textContent = products.filter((p) => p.status === "on_sale").length; $("#metricDraft").textContent = products.filter((p) => p.status === "draft").length;
    let skuTotal = 0; await Promise.all(products.slice(0, 8).map(async (p) => { try { const sku = await api(`/api/v1/admin/products/${p.id}/skus?page=1&page_size=1`); skuTotal += sku.total || 0; } catch {} })); $("#metricSKU").textContent = skuTotal;
    $("#recentProducts").classList.remove("loading"); $("#recentProducts").innerHTML = products.slice(0, 5).map((p) => `<div class="compact-row"><img class="thumb" src="${productImages[p.id % productImages.length]}" alt=""><div><b>${escapeHTML(p.name)}</b><span>${escapeHTML(p.description || "暂无描述")}</span></div><span class="status ${p.status}">${statusLabel(p.status)}</span></div>`).join("") || `<div class="empty">暂无商品</div>`;
  } catch (error) { toast(error.message); }
}

async function loadProducts() {
  const name = $("#productSearch").value.trim();
  try {
    const data = await api(`/api/v1/admin/products?page=${state.page}&page_size=${state.pageSize}${name ? `&name=${encodeURIComponent(name)}` : ""}`); state.products = data.list || []; state.total = data.total || 0; renderProducts();
  } catch (error) { toast(error.message); }
}
function renderProducts() {
  $("#productTableBody").innerHTML = state.products.map((p) => `<tr><td><div class="product-cell"><img class="thumb" src="${productImages[p.id % productImages.length]}" alt=""><div><b>${escapeHTML(p.name)}</b><span>${escapeHTML(p.description || "暂无描述")}</span></div></div></td><td><span class="status ${p.status}">${statusLabel(p.status)}</span></td><td>${formatDate(p.updated_at)}</td><td>#${p.id}</td><td class="align-right"><div class="row-actions"><button data-sku="${p.id}">SKU</button><button data-edit="${p.id}" title="编辑">✎</button><button data-delete="${p.id}" title="删除">⌫</button></div></td></tr>`).join("");
  $("#productEmpty").classList.toggle("hidden", state.products.length > 0); $("#productCount").textContent = `共 ${state.total} 条`; $("#pageNumber").textContent = state.page; $("#prevPage").disabled = state.page <= 1; $("#nextPage").disabled = state.page * state.pageSize >= state.total;
  $$('[data-edit]').forEach((b) => b.onclick = () => openProductModal(Number(b.dataset.edit))); $$('[data-delete]').forEach((b) => b.onclick = () => deleteProduct(Number(b.dataset.delete))); $$('[data-sku]').forEach((b) => b.onclick = () => openSKUDrawer(Number(b.dataset.sku)));
}
let searchTimer; $("#productSearch").addEventListener("input", () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => { state.page = 1; loadProducts(); }, 250); });
$("#prevPage").onclick = () => { if (state.page > 1) { state.page--; loadProducts(); } }; $("#nextPage").onclick = () => { if (state.page * state.pageSize < state.total) { state.page++; loadProducts(); } };

function openProductModal(id = 0) { state.editingID = id; const product = state.products.find((p) => p.id === id); $("#modalTitle").textContent = id ? "编辑商品" : "新建商品"; $("#productName").value = product?.name || ""; $("#productDescription").value = product?.description || ""; $("#formError").textContent = ""; $("#modal").classList.remove("hidden"); }
function closeProductModal() { $("#modal").classList.add("hidden"); }
$("#createProductButton").onclick = () => openProductModal(); $$('[data-close]').forEach((b) => b.onclick = closeProductModal);
$("#productForm").addEventListener("submit", async (event) => { event.preventDefault(); try { const body = JSON.stringify({ name: $("#productName").value, description: $("#productDescription").value }); await api(state.editingID ? `/api/v1/admin/products/${state.editingID}` : "/api/v1/admin/products", { method: state.editingID ? "PATCH" : "POST", body }); closeProductModal(); toast(state.editingID ? "商品已更新" : "商品已创建"); loadProducts(); } catch (error) { $("#formError").textContent = error.message; } });
async function deleteProduct(id) { if (!confirm("确定删除这个商品吗？在售商品不能删除。")) return; try { await api(`/api/v1/admin/products/${id}`, { method: "DELETE" }); toast("商品已删除"); loadProducts(); } catch (error) { toast(error.message); } }

async function openSKUDrawer(productID) { state.drawerProduct = state.products.find((p) => p.id === productID); $("#drawerProductName").textContent = state.drawerProduct?.name || "商品 SKU"; $("#drawerProductMeta").textContent = `商品 ID #${productID}`; $("#skuDrawer").classList.remove("hidden"); $("#skuForm").classList.add("hidden"); await loadSKUs(); }
async function loadSKUs() { try { const data = await api(`/api/v1/admin/products/${state.drawerProduct.id}/skus?page=1&page_size=100`); const list = data.list || []; $("#skuList").innerHTML = list.map((sku) => `<article class="sku-item"><div><b>${escapeHTML(sku.code)}</b><small>${Object.entries(sku.specs || {}).map(([key, value]) => `${escapeHTML(key)}: ${escapeHTML(value)}`).join(" · ")}</small></div><div><div class="price">¥${(sku.price_cent / 100).toFixed(2)}</div><small>库存 ${sku.stock}</small></div><div class="sku-actions"><span class="status ${sku.status}">${statusLabel(sku.status)}</span><button data-status-id="${sku.id}" data-status="${sku.status}">${sku.status === "active" ? "停用" : "启用"}</button></div></article>`).join("") || `<div class="empty">暂无 SKU</div>`; $$('[data-status-id]').forEach((b) => b.onclick = () => toggleSKUStatus(Number(b.dataset.statusId), b.dataset.status)); } catch (error) { $("#skuList").innerHTML = `<div class="empty">${escapeHTML(error.message)}</div>`; } }
async function toggleSKUStatus(id, status) { try { await api(`/api/v1/admin/products/${state.drawerProduct.id}/skus/${id}/status`, { method: "PATCH", body: JSON.stringify({ status: status === "active" ? "inactive" : "active" }) }); toast("SKU 状态已更新"); loadSKUs(); } catch (error) { toast(error.message); } }
$("#createSKUButton").onclick = () => $("#skuForm").classList.remove("hidden"); $("#cancelSKU").onclick = () => $("#skuForm").classList.add("hidden"); $('[data-close-drawer]').onclick = () => $("#skuDrawer").classList.add("hidden");
$("#skuCode").addEventListener("input", (event) => { event.target.value = event.target.value.toUpperCase(); });

function refreshSKUSpecRows() {
  const rows = $$("#skuSpecRows .sku-spec-row");
  rows.forEach((row) => {
    const removeButton = row.querySelector("[data-remove-spec]");
    removeButton.disabled = rows.length === 1;
    removeButton.onclick = () => { row.remove(); refreshSKUSpecRows(); };
  });
}

function resetSKUSpecRows() {
  $("#skuSpecRows").innerHTML = `<div class="sku-spec-row"><input data-spec-key maxlength="20" placeholder="规格名，如：颜色" required><input data-spec-value maxlength="50" placeholder="规格值，如：黑色" required><button type="button" class="icon-button" data-remove-spec title="删除规格" disabled>×</button></div>`;
  refreshSKUSpecRows();
}

$("#addSKUSpecButton").onclick = () => {
  if ($$("#skuSpecRows .sku-spec-row").length >= 5) {
    toast("一个 SKU 最多设置 5 项规格");
    return;
  }
  const row = document.createElement("div");
  row.className = "sku-spec-row";
  row.innerHTML = `<input data-spec-key maxlength="20" placeholder="规格名，如：容量" required><input data-spec-value maxlength="50" placeholder="规格值，如：128GB" required><button type="button" class="icon-button" data-remove-spec title="删除规格">×</button>`;
  $("#skuSpecRows").appendChild(row);
  refreshSKUSpecRows();
  row.querySelector("[data-spec-key]").focus();
};

refreshSKUSpecRows();
$("#skuForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const specs = {};
  for (const row of $$("#skuSpecRows .sku-spec-row")) {
    const key = row.querySelector("[data-spec-key]").value.trim();
    const value = row.querySelector("[data-spec-value]").value.trim();
    if (Object.hasOwn(specs, key)) {
      $("#skuFormError").textContent = `规格名“${key}”不能重复`;
      return;
    }
    specs[key] = value;
  }
  try {
    await api(`/api/v1/admin/products/${state.drawerProduct.id}/skus`, { method: "POST", body: JSON.stringify({ code: $("#skuCode").value.trim().toUpperCase(), price_cent: Number($("#skuPrice").value), stock: Number($("#skuStock").value), specs }) });
    $("#skuForm").reset();
    resetSKUSpecRows();
    $("#skuForm").classList.add("hidden");
    toast("SKU 已创建");
    loadSKUs();
  } catch (error) {
    $("#skuFormError").textContent = error.message;
  }
});

async function loadSKUCatalog() {
  const productData = await api("/api/v1/admin/products?page=1&page_size=100");
  const products = productData.list || [];
  const skuGroups = await Promise.all(products.map(async (product) => {
    const data = await api(`/api/v1/admin/products/${product.id}/skus?page=1&page_size=100`);
    return (data.list || []).map((sku) => ({ ...sku, product_name: product.name }));
  }));
  state.skuCatalog = skuGroups.flat();
  const activeSKUs = state.skuCatalog.filter((sku) => sku.status === "active");
  $("#cartSKUSelect").innerHTML = `<option value="">请选择 SKU</option>${activeSKUs.map((sku) => `<option value="${sku.id}">${escapeHTML(sku.product_name)} / ${escapeHTML(sku.code)} · 库存 ${sku.stock}</option>`).join("")}`;
}

async function loadCart() {
  const cart = await api("/api/v1/cart");
  const items = cart.items || [];
  state.cartItemsCache = items;
  // 选中状态保存在前端：后端 PUT /cart/selection 是整组替换语义，勾选变化时提交当前集合。
  if (!state.cartSelected) state.cartSelected = new Set(items.filter((item) => item.selected).map((item) => item.id));
  else items.forEach((item) => { if (!item.selected) state.cartSelected.delete(item.id); });
  $("#cartID").textContent = cart.id || "未创建";
  $("#cartUserID").textContent = cart.user_id || "-";
  $("#cartItemCount").textContent = items.length;
  $("#cartQuantityTotal").textContent = items.reduce((total, item) => total + item.quantity, 0);
  $("#cartTableBody").innerHTML = items.map((item) => {
    const sku = state.skuCatalog.find((value) => value.id === item.sku_id);
    const skuName = sku ? `${escapeHTML(sku.product_name)} / ${escapeHTML(sku.code)}` : `SKU #${item.sku_id}`;
    const skuMeta = sku ? `¥${(sku.price_cent / 100).toFixed(2)} / 库存 ${sku.stock}` : "SKU 信息未加载";
    const checked = state.cartSelected.has(item.id);
    return `<tr><td><label class="cart-check"><input type="checkbox" data-cart-select="${item.id}" ${checked ? "checked" : ""}></label></td><td><div class="cart-sku"><b>${skuName}</b><small>sku_id: ${item.sku_id}</small></div></td><td>${skuMeta}</td><td><div class="quantity-control"><input type="number" min="1" value="${item.quantity}" data-cart-quantity="${item.id}"><button class="button secondary" data-save-quantity="${item.id}">保存</button></div></td><td>#${item.id}</td><td class="align-right"><button class="button danger-button" data-remove-cart-item="${item.id}">删除</button></td></tr>`;
  }).join("");
  $("#cartEmpty").classList.toggle("hidden", items.length > 0);
  $$('[data-save-quantity]').forEach((button) => button.onclick = () => updateCartQuantity(Number(button.dataset.saveQuantity)));
  $$('[data-remove-cart-item]').forEach((button) => button.onclick = () => removeCartItem(Number(button.dataset.removeCartItem)));
  $$('[data-cart-select]').forEach((box) => box.onchange = () => toggleCartSelection(Number(box.dataset.cartSelect), box.checked));
  renderCartSummary();
}

// toggleCartSelection 切换单行选中并整组提交到后端，保持前后端选中状态一致。
async function toggleCartSelection(itemID, checked) {
  if (checked) state.cartSelected.add(itemID); else state.cartSelected.delete(itemID);
  try {
    await api("/api/v1/cart/selection", { method: "PUT", body: JSON.stringify({ item_ids: [...state.cartSelected] }) });
    renderCartSummary();
  } catch (error) { toast(error.message); }
}

// renderCartSummary 用本地选中集合实时计算合计；下单前再调服务端 preview 做权威校验。
function renderCartSummary() {
  const selectedCount = state.cartSelected.size;
  $("#cartSelectedCount").textContent = selectedCount;
  const totalCent = (state.cartItemsCache || []).filter((item) => state.cartSelected.has(item.id))
    .reduce((sum, item) => sum + item.quantity * (state.skuCatalog.find((s) => s.id === item.sku_id)?.price_cent || 0), 0);
  $("#cartSelectedTotal").textContent = `¥${(totalCent / 100).toFixed(2)}`;
  $("#checkoutButton").disabled = selectedCount === 0;
}

// checkoutOrder 结算：先服务端预览确认可购买项和权威金额，再用默认地址创建订单。
async function checkoutOrder() {
  try {
    const preview = await api("/api/v1/cart/preview", { method: "POST" });
    if (preview.purchasable_count === 0) { toast("没有可购买的商品：" + (preview.items[0]?.reason || "购物车为空")); return; }
    const lines = preview.items.map((i) => `${i.product_name}/${i.sku_code} ×${i.quantity} ${i.purchasable ? "✓ ¥" + (i.subtotal_cent / 100).toFixed(2) : "✕ " + i.reason}`).join("\n");
    if (!confirm(`确认下单？\n${lines}\n合计 ¥${(preview.total_cent / 100).toFixed(2)}`)) return;
    const addresses = (await api("/api/v1/addresses")).items || [];
    const address = addresses.find((a) => a.is_default) || addresses[0];
    if (!address) { toast("请先在收货地址页创建地址"); switchView("addresses"); return; }
    const idempotencyKey = `ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    // 下单成功后清除已购明细：后端暂无"按选中项清空购物车"接口，逐条删除等价实现。
    const purchased = new Set(preview.items.filter((i) => i.purchasable).map((i) => i.item_id));
    for (const itemID of purchased) {
      try { await api(`/api/v1/cart/items/${itemID}`, { method: "DELETE" }); state.cartSelected.delete(itemID); } catch {}
    }
    toast("订单已创建，去订单页支付");
    switchView("orders");
  } catch (error) { toast(error.message); }
}
$("#checkoutButton").onclick = checkoutOrder;

async function loadCartPage() {
  try {
    await loadSKUCatalog();
    await loadCart();
  } catch (error) {
    toast(error.message);
  }
}

async function updateCartQuantity(itemID) {
  const input = $(`[data-cart-quantity="${itemID}"]`);
  try {
    await api(`/api/v1/cart/items/${itemID}`, { method: "PATCH", body: JSON.stringify({ quantity: Number(input.value) }) });
    toast("购物车数量已更新");
    await loadCart();
  } catch (error) {
    toast(error.message);
  }
}

async function removeCartItem(itemID) {
  if (!confirm("确定从购物车移除这条商品吗？")) return;
  try {
    await api(`/api/v1/cart/items/${itemID}`, { method: "DELETE" });
    state.cartSelected?.delete(itemID); // 同步清理本地选中集合，避免结算时提交已删除的明细。
    toast("购物车商品已删除");
    await loadCart();
  } catch (error) {
    toast(error.message);
  }
}

$("#refreshCartButton").onclick = loadCartPage;
$("#addCartItemForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  $("#cartFormError").textContent = "";
  try {
    await api("/api/v1/cart/items", { method: "POST", body: JSON.stringify({ sku_id: Number($("#cartSKUSelect").value), quantity: Number($("#cartQuantity").value) }) });
    toast("SKU 已加入购物车");
    await loadCart();
  } catch (error) {
    $("#cartFormError").textContent = error.message;
  }
});

async function loadAddressPage() {
  try { state.addresses = (await api("/api/v1/addresses")).items || []; renderAddresses(); }
  catch (error) { toast(error.message); }
}

function renderAddresses() {
  $("#addressTotal").textContent = state.addresses.length;
  const current = state.addresses.find((a) => a.is_default);
  $("#addressDefaultName").textContent = current ? current.recipient : "-";
  $("#addressTableBody").innerHTML = state.addresses.map((a) => `<tr><td><div class="cart-sku"><b>${escapeHTML(a.recipient)}</b><small>${escapeHTML(a.phone)}</small></div></td><td>${escapeHTML(`${a.province} ${a.city} ${a.district}`)}</td><td class="address-detail" title="${escapeHTML(a.detail)}">${escapeHTML(a.detail)}</td><td>${a.is_default ? '<span class="status on_sale">默认</span>' : '<span class="status draft">普通</span>'}</td><td class="align-right"><div class="row-actions">${a.is_default ? "" : `<button data-default="${a.id}" title="设为默认">★</button>`}<button data-edit-address="${a.id}" title="编辑">✎</button><button data-delete-address="${a.id}" title="删除">⌫</button></div></td></tr>`).join("");
  $("#addressEmpty").classList.toggle("hidden", state.addresses.length > 0);
  $$("[data-default]").forEach((b) => b.onclick = () => setDefaultAddress(Number(b.dataset.default)));
  $$("[data-edit-address]").forEach((b) => b.onclick = () => openAddressForm(Number(b.dataset.editAddress)));
  $$("[data-delete-address]").forEach((b) => b.onclick = () => deleteAddress(Number(b.dataset.deleteAddress)));
}

function openAddressForm(id = 0) {
  state.addressEditingID = id;
  const address = state.addresses.find((a) => a.id === id);
  $("#addressFormTitle").textContent = id ? `编辑地址 #${id}` : "新增地址";
  $("#addressSubmitButton").textContent = id ? "保存修改" : "保存地址";
  $("#addressCancelButton").classList.toggle("hidden", !id);
  $("#addressRecipient").value = address?.recipient || "";
  $("#addressPhone").value = address?.phone || "";
  $("#addressProvince").value = address?.province || "";
  $("#addressCity").value = address?.city || "";
  $("#addressDistrict").value = address?.district || "";
  $("#addressDetail").value = address?.detail || "";
  $("#addressFormError").textContent = "";
}

$("#refreshAddressButton").onclick = loadAddressPage;
$("#addressCancelButton").onclick = () => openAddressForm();

$("#addressForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  $("#addressFormError").textContent = "";
  const editing = state.addressEditingID !== 0;
  const payload = { recipient: $("#addressRecipient").value, phone: $("#addressPhone").value, province: $("#addressProvince").value, city: $("#addressCity").value, district: $("#addressDistrict").value, detail: $("#addressDetail").value };
  try {
    await api(editing ? `/api/v1/addresses/${state.addressEditingID}` : "/api/v1/addresses", { method: editing ? "PATCH" : "POST", body: JSON.stringify(payload) });
    toast(editing ? "地址已更新" : "地址已创建");
    openAddressForm();
    await loadAddressPage();
  } catch (error) { $("#addressFormError").textContent = error.message; }
});

async function setDefaultAddress(id) {
  try { await api(`/api/v1/addresses/${id}/default`, { method: "PUT" }); toast("默认地址已切换"); await loadAddressPage(); }
  catch (error) { toast(error.message); }
}

async function deleteAddress(id) {
  if (!confirm("确定删除这个收货地址吗？删除默认地址后不会自动补选新默认。")) return;
  try { await api(`/api/v1/addresses/${id}`, { method: "DELETE" }); toast("地址已删除"); await loadAddressPage(); }
  catch (error) { toast(error.message); }
}

// 订单状态中文映射，与后端状态机枚举保持一致。
const orderStatusLabels = { pending_payment: "待支付", paid: "已支付", shipped: "已发货", completed: "已完成", cancelled: "已取消" };
const taskStatusLabels = { pending: "待执行", running: "执行中", succeeded: "成功", failed: "失败" };

async function loadOrderPage() {
  try { state.orders = (await api("/api/v1/orders?page=1&page_size=20")).list || []; renderOrders(); }
  catch (error) { toast(error.message); }
}

function renderOrders() {
  $("#orderTotal").textContent = state.orders.length;
  $("#orderPending").textContent = state.orders.filter((o) => o.status === "pending_payment").length;
  $("#orderCompleted").textContent = state.orders.filter((o) => o.status === "completed").length;
  // 操作列按状态机渲染文字按钮；每个动作标注归属角色（运营）避免消费者误以为是自己该做的操作。
  const actionButton = (attr, id, label, cls = "") => `<button class="link-button ${cls}" data-${attr}="${id}">${label}</button>`;
  $("#orderTableBody").innerHTML = state.orders.map((o) => {
    const actions = [];
    if (o.status === "pending_payment") { actions.push(actionButton("pay", o.id, "创建支付单"), actionButton("cancel", o.id, "取消", "danger")); }
    if (o.status === "paid") actions.push(actionButton("ship", o.id, "发货（运营）"));
    if (o.status === "shipped") actions.push(actionButton("receipt", o.id, "确认收货"));
    if (o.status === "completed" && (o.items || []).some((i) => i.refund_status === "none")) actions.push(actionButton("review-order", o.id, "评价/退款"));
    actions.push(actionButton("detail", o.id, "状态历史"));
    return `<tr><td><div class="cart-sku"><b>#${o.id}</b><small>${escapeHTML(o.order_no)}</small></div></td><td>${(o.items || []).map((i) => `${escapeHTML(i.product_name)} ×${i.quantity} <small>项${i.id}${i.refund_status !== "none" ? " · " + ({ pending: "退款中", refunded: "已退款" })[i.refund_status] : ""}</small>`).join("<br>")}</td><td>¥${(o.pay_cent / 100).toFixed(2)}</td><td><span class="status ${orderStatusClass(o.status)}">${orderStatusLabels[o.status] || o.status}</span></td><td class="align-right"><div class="row-actions">${actions.join(" ")}</div></td></tr>`;
  }).join("");
  $("#orderEmpty").classList.toggle("hidden", state.orders.length > 0);
  $$("[data-pay]").forEach((b) => b.onclick = () => createPayment(Number(b.dataset.pay)));
  $$("[data-cancel]").forEach((b) => b.onclick = () => cancelOrder(Number(b.dataset.cancel)));
  $$("[data-ship]").forEach((b) => b.onclick = () => shipOrder(Number(b.dataset.ship)));
  $$("[data-receipt]").forEach((b) => b.onclick = () => confirmReceipt(Number(b.dataset.receipt)));
  $$("[data-detail]").forEach((b) => b.onclick = () => showOrderDetail(Number(b.dataset.detail)));
  $$("[data-review-order]").forEach((b) => b.onclick = () => openAfterSaleDialog(Number(b.dataset.reviewOrder)));
}

// 售后入口：对已完成订单的某个可退订单项发起评价或退款申请。
async function openAfterSaleDialog(orderID) {
  const order = state.orders.find((o) => o.id === orderID);
  const item = (order?.items || []).find((i) => i.refund_status === "none");
  if (!item) { toast("该订单没有可操作的订单项"); return; }
  const action = prompt(`订单项 #${item.id}「${item.product_name}」\n输入 1 = 提交评价（默认5星）\n输入 2 = 申请退款 ¥${(item.subtotal_cent / 100).toFixed(2)}\n输入其他取消`);
  try {
    if (action === "1") { await api(`/api/v1/order-items/${item.id}/reviews`, { method: "POST", body: JSON.stringify({ rating: 5, content: "UI 快捷好评" }) }); toast("评价已提交（商品详情页评价列表接口 GET /products/:id/reviews 可查看，暂未单独做展示页）"); }
    else if (action === "2") { await api(`/api/v1/order-items/${item.id}/refunds`, { method: "POST", body: JSON.stringify({ amount_cent: item.subtotal_cent, reason: "UI 申请全额退款" }) }); toast("退款申请已提交，请到退款售后页审核"); }
    else return;
    await loadOrderPage();
  } catch (error) { toast(error.message); }
}

// 状态 → 现有 status 徽章配色：复用商品状态的三色风格。
function orderStatusClass(status) { return ({ pending_payment: "draft", paid: "on_sale", shipped: "on_sale", completed: "active", cancelled: "off_sale" })[status] || "draft"; }

async function previewOrder() {
  try {
    const preview = await api("/api/v1/cart/preview", { method: "POST" });
    alert(`可购买 ${preview.purchasable_count} 项，合计 ¥${(preview.total_cent / 100).toFixed(2)}\n` + preview.items.map((i) => `${i.product_name}/${i.sku_code} ×${i.quantity} ${i.purchasable ? "✓" : "✕ " + i.reason}`).join("\n"));
  } catch (error) { toast(error.message); }
}

async function createOrderFromCart() {
  try {
    const addresses = (await api("/api/v1/addresses")).items || [];
    const address = addresses.find((a) => a.is_default) || addresses[0];
    if (!address) { toast("请先创建收货地址"); return; }
    // 目的：每次用新幂等键代表一次新的下单意图；网络重试时应复用同一 key。
    const idempotencyKey = `ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    await api("/api/v1/orders", { method: "POST", body: JSON.stringify({ address_id: address.id }), headers: { "Idempotency-Key": idempotencyKey } });
    toast("订单已创建");
    await loadOrderPage();
  } catch (error) { toast(error.message); }
}

// createPayment 创建支付单 → 模拟渠道异步回调（HMAC 签名）→ 轮询真实支付状态直至终态。
// 说明：浏览器端不持有验签密钥，这里演示"渠道视角"的完整链路；真实项目中回调由渠道服务器发起，
// 前端只负责轮询结果。学习用途下由页面代演渠道角色。
async function createPayment(orderID) {
  try {
    const payment = await api(`/api/v1/orders/${orderID}/payments`, { method: "POST" });
    toast(`支付单 ${payment.payment_no} 已创建，等待支付结果…`);
    // 模拟渠道行为：带签名地通知我们的回调接口（event_no 唯一，重复投递安全）。
    const order = state.orders.find((o) => o.id === orderID);
    const eventNo = `evt-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const canonical = `${payment.payment_no}|${order ? order.order_no : ""}|${eventNo}|success|${payment.amount_cent}|${payment.currency}`;
    const signature = await hmacSha256Hex(MOCK_CHANNEL_SECRET, canonical);
    await fetch("/api/v1/payment-callbacks/mock", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ payment_no: payment.payment_no, order_no: order ? order.order_no : "", event_no: eventNo, result: "success", amount_cent: payment.amount_cent, currency: payment.currency, signature }),
    });
    // 轮询支付单直到非 pending（回调是异步的，UI 展示真实等待过程）。
    for (let i = 0; i < 10; i++) {
      const latest = await api(`/api/v1/payments/${payment.payment_no}`);
      if (latest.status !== "pending") {
        toast(latest.status === "success" ? "支付成功，订单已进入待发货" : "支付结束：" + latest.status);
        break;
      }
      await new Promise((r) => setTimeout(r, 300));
    }
    await loadOrderPage();
  } catch (error) { toast(error.message); }
}

// MOCK_CHANNEL_SECRET 与后端 config.yaml 的 payment.callback_secret 一致（仅开发环境演示用）。
const MOCK_CHANNEL_SECRET = "mock-channel-secret-dev-only";

// hmacSha256Hex 用 Web Crypto 计算 HMAC-SHA256 并转 hex，与服务端 SignMockCallback 规则对齐。
async function hmacSha256Hex(secret, message) {
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const sig = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(message));
  return [...new Uint8Array(sig)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function cancelOrder(id) {
  if (!confirm("确定取消这个订单吗？")) return;
  try { await api(`/api/v1/orders/${id}/cancel`, { method: "POST" }); toast("订单已取消"); await loadOrderPage(); }
  catch (error) { toast(error.message); }
}

async function shipOrder(id) {
  try { await api(`/api/v1/admin/orders/${id}/ship`, { method: "POST" }); toast("已发货（运营接口）"); await loadOrderPage(); }
  catch (error) { toast(error.message); }
}

async function confirmReceipt(id) {
  try { await api(`/api/v1/orders/${id}/confirm-receipt`, { method: "POST" }); toast("已确认收货"); await loadOrderPage(); }
  catch (error) { toast(error.message); }
}

async function showOrderDetail(id) {
  try {
    const detail = await api(`/api/v1/orders/${id}`);
    alert(detail.status_logs.map((l) => `${l.from_status || "∅"} → ${l.to_status}（${l.operator_type} ${formatDate(l.created_at)}）`).join("\n") || "无状态历史");
  } catch (error) { toast(error.message); }
}

$("#refreshOrderButton").onclick = loadOrderPage;
$("#previewOrderButton").onclick = previewOrder;
$("#createOrderButton").onclick = createOrderFromCart;

async function loadCouponPage() {
  try {
    const [templates, mine] = await Promise.all([api("/api/v1/admin/coupon-templates?page=1&page_size=50"), api("/api/v1/coupons")]);
    state.coupons = templates.list || []; state.myCoupons = mine.items || []; renderCoupons();
  } catch (error) { toast(error.message); }
}

function renderCoupons() {
  const now = Date.now();
  $("#couponTableBody").innerHTML = state.coupons.map((t) => {
    const claimable = t.status === "active" && new Date(t.starts_at) < now && new Date(t.ends_at) > now && t.remaining > 0;
    return `<tr><td><b>${escapeHTML(t.name)}</b></td><td>满 ¥${(t.threshold_cent / 100).toFixed(0)} 减 ¥${(t.discount_cent / 100).toFixed(0)}</td><td>${t.remaining}/${t.total_count}</td><td><small>${formatDate(t.starts_at)} ~ ${formatDate(t.ends_at)}</small></td><td class="align-right">${claimable ? `<button class="button secondary" data-claim="${t.id}">领取</button>` : '<span class="status draft">不可领</span>'}</td></tr>`;
  }).join("");
  $("#couponEmpty").classList.toggle("hidden", state.coupons.length > 0);
  $$("[data-claim]").forEach((b) => b.onclick = () => claimCoupon(Number(b.dataset.claim)));
  $("#myCouponList").innerHTML = state.myCoupons.map((c) => {
    const template = state.coupons.find((t) => t.id === c.template_id);
    return `<article class="sku-item"><div><b>${escapeHTML(template?.name || "券 #" + c.template_id)}</b><small>满 ¥${((template?.threshold_cent || 0) / 100).toFixed(0)} 减 ¥${((template?.discount_cent || 0) / 100).toFixed(0)}</small></div><span class="status ${c.status === "unused" ? "active" : "draft"}">${({ unused: "未使用", used: "已使用", expired: "已过期" })[c.status] || c.status}</span></article>`;
  }).join("") || '<div class="empty">还没有领取优惠券</div>';
}

async function claimCoupon(templateID) {
  try { await api(`/api/v1/coupons/${templateID}/claim`, { method: "POST" }); toast("优惠券已领取"); await loadCouponPage(); }
  catch (error) { toast(error.message); }
}
$("#refreshCouponButton").onclick = loadCouponPage;

async function loadTaskPage() {
  try { state.tasks = (await api("/api/v1/admin/tasks?page=1&page_size=50")).list || []; renderTasks(); }
  catch (error) { toast(error.message); }
}

function renderTasks() {
  $("#taskTableBody").innerHTML = state.tasks.map((t) => `<tr><td><div class="cart-sku"><b>#${t.id}</b><small>${escapeHTML(t.task_type)}</small></div></td><td><span class="status ${t.status === "failed" ? "off_sale" : t.status === "succeeded" ? "active" : "draft"}">${taskStatusLabels[t.status] || t.status}</span></td><td>${t.attempts}/${t.max_attempts}</td><td><small>${escapeHTML(t.last_error || "-")}</small></td><td class="align-right">${t.status === "failed" ? `<button class="button secondary" data-retry-task="${t.id}">重试</button>` : ""}</td></tr>`).join("");
  $("#taskEmpty").classList.toggle("hidden", state.tasks.length > 0);
  $$("[data-retry-task]").forEach((b) => b.onclick = () => retryTask(Number(b.dataset.retryTask)));
}

async function retryTask(id) {
  try { await api(`/api/v1/admin/tasks/${id}/retry`, { method: "POST" }); toast("任务已重新排队"); await loadTaskPage(); }
  catch (error) { toast(error.message); }
}
$("#refreshTaskButton").onclick = loadTaskPage;

// ---- 退款售后：消费者申请 + 运营审核双栏视图 ----
const refundStatusLabels = { pending: "待审核", approved: "已通过", rejected: "已驳回", refunded: "已退款" };

async function loadRefundPage() {
  try {
    const [mine, admin] = await Promise.all([api("/api/v1/refunds"), api("/api/v1/admin/refunds?page=1&page_size=20")]);
    state.refunds = mine.items || []; state.adminRefunds = admin.list || []; renderRefunds();
  } catch (error) { toast(error.message); }
}

function renderRefunds() {
  $("#refundTableBody").innerHTML = state.refunds.map((r) => `<tr><td><div class="cart-sku"><b>#${r.id}</b><small>${escapeHTML(r.refund_no)}</small></div></td><td>¥${(r.amount_cent / 100).toFixed(2)}</td><td><small>${escapeHTML(r.reason || "-")}</small></td><td><span class="status ${r.status === "refunded" ? "active" : r.status === "rejected" ? "off_sale" : "draft"}">${refundStatusLabels[r.status] || r.status}</span></td><td><small>${formatDate(r.created_at)}</small></td></tr>`).join("");
  $("#refundEmpty").classList.toggle("hidden", state.refunds.length > 0);
  // 运营审核台只列待处理项，通过/驳回后自动消失，减少误操作。
  const pending = state.adminRefunds.filter((r) => r.status === "pending");
  $("#adminRefundList").innerHTML = pending.map((r) => `<article class="sku-item"><div><b>#${r.id} ¥${(r.amount_cent / 100).toFixed(2)}</b><small>${escapeHTML(r.reason || "无原因")} · 订单项 ${r.order_item_id}</small></div><div class="sku-actions"><button data-approve="${r.id}" class="button primary">通过</button><button data-reject="${r.id}" class="button secondary">驳回</button></div></article>`).join("") || '<div class="empty">没有待审核的退款</div>';
  $$("[data-approve]").forEach((b) => b.onclick = () => reviewRefund(Number(b.dataset.approve), true));
  $$("[data-reject]").forEach((b) => b.onclick = () => reviewRefund(Number(b.dataset.reject), false));
}

async function reviewRefund(id, approve) {
  if (!approve && !confirm("确定驳回这个退款申请吗？")) return;
  try { await api(`/api/v1/admin/refunds/${id}/${approve ? "approve" : "reject"}`, { method: "POST" }); toast(approve ? "退款已通过并模拟到账" : "退款已驳回"); await loadRefundPage(); }
  catch (error) { toast(error.message); }
}
$("#refreshRefundButton").onclick = loadRefundPage;

// ---- 消息通知 ----
async function loadNotificationPage() {
  try { state.notifications = (await api("/api/v1/notifications?page=1&page_size=50")).list || []; renderNotifications(); }
  catch (error) { toast(error.message); }
}

function renderNotifications() {
  $("#notificationList").innerHTML = state.notifications.map((n) => `<article class="notification-item ${n.read ? "read" : ""}" data-notification="${n.id}"><div class="dot"></div><div><b>${escapeHTML(n.title)}</b><p>${escapeHTML(n.content)}</p><small>${n.type} · ${formatDate(n.created_at)}</small></div></article>`).join("");
  $("#notificationEmpty").classList.toggle("hidden", state.notifications.length > 0);
  // 点击未读条目即标记已读；已读条目不再发请求。
  $$("[data-notification]").forEach((el) => el.onclick = async () => {
    const id = Number(el.dataset.notification);
    if (state.notifications.find((n) => n.id === id)?.read) return;
    try { await api(`/api/v1/notifications/${id}/read`, { method: "PUT" }); await loadNotificationPage(); }
    catch (error) { toast(error.message); }
  });
}

$("#refreshNotificationButton").onclick = loadNotificationPage;
$("#readAllNotificationButton").onclick = async () => {
  try { await api("/api/v1/notifications/read-all", { method: "PUT" }); toast("已全部标记为已读"); await loadNotificationPage(); }
  catch (error) { toast(error.message); }
};

const permissions = ["product.create","product.read","product.update","product.delete","sku.create","sku.read","sku.update","sku.delete","cart.read","cart.item.create","cart.item.update","cart.item.delete","address.create","address.read","address.update","address.delete","order.create","order.read","order.cancel","order.ship","payment.create","payment.read","refund.apply","refund.read","refund.review","coupon.claim","coupon.manage","review.create","review.read","review.visibility","notification.read","task.read","task.retry"];
$("#permissionGrid").innerHTML = permissions.map((p) => `<div class="permission-item"><code>${p}</code><span class="check">✓</span></div>`).join("");
if (state.token) showApp();
