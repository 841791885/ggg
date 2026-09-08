package controllers

import (
	"time"

	model "ggg/models"
)

// AddressResponse 表示收货地址的对外响应结构，隐藏软删除等内部字段。
type AddressResponse struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	Recipient string    `json:"recipient"`
	Phone     string    `json:"phone"`
	Province  string    `json:"province"`
	City      string    `json:"city"`
	District  string    `json:"district"`
	Detail    string    `json:"detail"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// newAddressResponse 将收货地址模型转换为对外响应结构。
func newAddressResponse(address *model.Address) AddressResponse {
	return AddressResponse{
		ID: address.ID, UserID: address.UserID, Recipient: address.Recipient, Phone: address.Phone,
		Province: address.Province, City: address.City, District: address.District, Detail: address.Detail,
		IsDefault: address.IsDefault, CreatedAt: address.CreatedAt, UpdatedAt: address.UpdatedAt,
	}
}
