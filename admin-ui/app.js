const state = { token: localStorage.getItem("gomall_token") || "", page: 1, pageSize: 10, total: 0, products: [], editingID: 0, drawerProduct: null, skuCatalog: [] };
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

const viewMeta = { dashboard: ["业务概览", "商城核心数据与状态"], products: ["商品管理", "维护商品信息与 SKU"], cart: ["购物车", "查看当前登录用户的购物车"], permissions: ["角色权限", "路由级权限映射"] };
function switchView(name) {
  $$(".view").forEach((el) => el.classList.add("hidden")); $(`#${name}View`).classList.remove("hidden");
  $$(".nav-item").forEach((el) => el.classList.toggle("active", el.dataset.view === name));
  $("#pageTitle").textContent = viewMeta[name][0]; $("#pageSubtitle").textContent = viewMeta[name][1];
  if (name === "products") loadProducts();
  if (name === "cart") loadCartPage();
}
$$(".nav-item").forEach((button) => button.addEventListener("click", () => switchView(button.dataset.view)));
$$("[data-go-products]").forEach((button) => button.addEventListener("click", () => switchView("products")));
$("#refreshButton").addEventListener("click", () => {
  const currentView = $(".nav-item.active").dataset.view;
  if (currentView === "products") loadProducts();
  else if (currentView === "cart") loadCartPage();
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
  $("#cartID").textContent = cart.id || "未创建";
  $("#cartUserID").textContent = cart.user_id || "-";
  $("#cartItemCount").textContent = items.length;
  $("#cartQuantityTotal").textContent = items.reduce((total, item) => total + item.quantity, 0);
  $("#cartTableBody").innerHTML = items.map((item) => {
    const sku = state.skuCatalog.find((value) => value.id === item.sku_id);
    const skuName = sku ? `${escapeHTML(sku.product_name)} / ${escapeHTML(sku.code)}` : `SKU #${item.sku_id}`;
    const skuMeta = sku ? `¥${(sku.price_cent / 100).toFixed(2)} / 库存 ${sku.stock}` : "SKU 信息未加载";
    return `<tr><td><div class="cart-sku"><b>${skuName}</b><small>sku_id: ${item.sku_id}</small></div></td><td>${skuMeta}</td><td><div class="quantity-control"><input type="number" min="1" value="${item.quantity}" data-cart-quantity="${item.id}"><button class="button secondary" data-save-quantity="${item.id}">保存</button></div></td><td>#${item.id}</td><td class="align-right"><button class="button danger-button" data-remove-cart-item="${item.id}">删除</button></td></tr>`;
  }).join("");
  $("#cartEmpty").classList.toggle("hidden", items.length > 0);
  $$('[data-save-quantity]').forEach((button) => button.onclick = () => updateCartQuantity(Number(button.dataset.saveQuantity)));
  $$('[data-remove-cart-item]').forEach((button) => button.onclick = () => removeCartItem(Number(button.dataset.removeCartItem)));
}

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

const permissions = ["product.create","product.read","product.update","product.delete","sku.create","sku.read","sku.update","sku.delete","cart.read","cart.item.create","cart.item.update","cart.item.delete"];
$("#permissionGrid").innerHTML = permissions.map((p) => `<div class="permission-item"><code>${p}</code><span class="check">✓</span></div>`).join("");
if (state.token) showApp();
