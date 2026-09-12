package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// MockPaymentCallback 是模拟支付渠道回调的报文结构。
// 真实渠道（微信/支付宝）字段更多，但"业务参数 + 签名"的骨架一致；Provider 接口把它隔离在这里，
// service 主流程不感知渠道细节。
type MockPaymentCallback struct {
	PaymentNo  string `json:"payment_no"`  // 支付单号（我们的系统生成的）
	OrderNo    string `json:"order_no"`    // 订单号：与支付单双重交叉校验
	EventNo    string `json:"event_no"`    // 渠道事件号：幂等消费的唯一键
	Result     string `json:"result"`      // success / failed
	AmountCent int64  `json:"amount_cent"` // 渠道实收金额，必须与支付单一致
	Currency   string `json:"currency"`    // 币种一致性校验
	Signature  string `json:"signature"`   // 下方 canonical 串的 HMAC-SHA256
}

// CanonicalString 返回参与签名的规范化报文串。
// 目的：签名对象必须是【双方按同一规则拼出的定长格式】，而不是对整个 JSON body 签名——
// JSON 序列化不稳定（字段顺序、空格、转义都可能不同），任何差异都会导致验签失败。
// 约定格式：字段以 | 分隔、固定顺序、不含 signature 自身。
func (c MockPaymentCallback) CanonicalString() string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%s", c.PaymentNo, c.OrderNo, c.EventNo, c.Result, c.AmountCent, c.Currency)
}

// SignMockCallback 用共享密钥对报文计算 HMAC-SHA256 签名（hex 小写）。
// 模拟渠道侧用它生成 signature；我们收到回调后用 VerifyMockCallbackSignature 重算比对。
func SignMockCallback(secret string, callback MockPaymentCallback) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(callback.CanonicalString()))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyMockCallbackSignature 恒定时间比较签名，防时序侧信道。
// 目的：普通字符串比较在第一个不匹配字符就返回，攻击者可通过响应耗时逐字节猜签名；
// hmac.Equal 内部比较所有字节、耗时只与长度相关。验签场景必须用它。
func VerifyMockCallbackSignature(secret string, callback MockPaymentCallback) bool {
	expected := SignMockCallback(secret, callback)
	return hmac.Equal([]byte(expected), []byte(callback.Signature))
}
