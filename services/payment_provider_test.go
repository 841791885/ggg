package services

import (
	"encoding/hex"
	"testing"
)

func sampleCallback() MockPaymentCallback {
	return MockPaymentCallback{
		PaymentNo: "pay-001", OrderNo: "ord-001", EventNo: "evt-001",
		Result: "success", AmountCent: 9900, Currency: "CNY",
	}
}

// TestSignVerifyRoundTrip 验证签名与验签互为逆操作，且密钥不一致时验签必须失败。
func TestSignVerifyRoundTrip(t *testing.T) {
	cb := sampleCallback()
	cb.Signature = SignMockCallback("secret-a", cb)
	if !VerifyMockCallbackSignature("secret-a", cb) {
		t.Fatal("正确密钥下验签应通过")
	}
	if VerifyMockCallbackSignature("secret-b", cb) {
		t.Fatal("错误密钥下验签必须失败")
	}
}

// TestSignatureCoversEveryField 报文中任一字段被篡改，签名都必须失效（防中间人改金额）。
func TestSignatureCoversEveryField(t *testing.T) {
	base := sampleCallback()
	base.Signature = SignMockCallback("s", base)
	tampered := []MockPaymentCallback{}
	c := base
	c.AmountCent = 1
	tampered = append(tampered, c)
	c = base
	c.EventNo = "evt-evil"
	tampered = append(tampered, c)
	c = base
	c.OrderNo = "ord-other"
	tampered = append(tampered, c)
	c = base
	c.PaymentNo = "pay-other"
	tampered = append(tampered, c)
	c = base
	c.Result = "failed"
	tampered = append(tampered, c)
	for i, cb := range tampered {
		if VerifyMockCallbackSignature("s", cb) {
			t.Fatalf("第%d个篡改报文不应通过验签", i)
		}
	}
}

// TestCanonicalStringStable 规范化串必须确定且不含 signature 自身。
func TestCanonicalStringStable(t *testing.T) {
	got := sampleCallback().CanonicalString()
	want := "pay-001|ord-001|evt-001|success|9900|CNY"
	if got != want {
		t.Fatalf("canonical 不符:\n got=%q\nwant=%q", got, want)
	}
}

// TestSignatureHexFormat 签名必须是 64 位小写 hex（与服务端/前端 WebCrypto 输出对齐）。
func TestSignatureHexFormat(t *testing.T) {
	sig := SignMockCallback("k", sampleCallback())
	if len(sig) != 64 {
		t.Fatalf("sha256 hex 长度应为 64，实际 %d", len(sig))
	}
	if _, err := hex.DecodeString(sig); err != nil {
		t.Fatalf("签名不是合法 hex：%v", err)
	}
}
