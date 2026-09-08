package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	model "ggg/models"
	"ggg/repositories"
)

// maxUserAddresses 是每个用户允许保存的有效地址上限（PRD-004）。
const maxUserAddresses = 20

// phonePattern 匹配大陆 11 位手机号：1 开头，第二位 3-9，后跟 9 位数字。
var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// CreateAddressInput 表示创建收货地址所需的业务参数。
// 不包含 IsDefault：第一个地址自动成为默认，之后设默认只能走 SetDefault。
type CreateAddressInput struct {
	UserID                   uint64
	Recipient, Phone         string
	Province, City, District string
	Detail                   string
}

// UpdateAddressInput 表示修改收货地址所需的业务参数，nil 字段保持原值。
type UpdateAddressInput struct {
	UserID, AddressID                uint64
	Recipient, Phone                 *string
	Province, City, District, Detail *string
}

// AddressService 负责收货地址的业务规则和所有权校验。
type AddressService struct {
	repository repositories.AddressRepository
}

// NewAddressService 创建收货地址业务服务。
func NewAddressService(repository repositories.AddressRepository) *AddressService {
	return &AddressService{repository: repository}
}

// Create 校验字段并创建地址；用户的第一个地址自动设为默认地址。
func (s *AddressService) Create(ctx context.Context, input CreateAddressInput) (model.Address, error) {
	if input.UserID == 0 {
		return model.Address{}, model.ErrInvalidUserID
	}
	recipient := strings.TrimSpace(input.Recipient)
	if n := utf8.RuneCountInString(recipient); n < 2 || n > 30 {
		return model.Address{}, model.ErrInvalidRecipient
	}
	phone := strings.TrimSpace(input.Phone)
	if !phonePattern.MatchString(phone) {
		return model.Address{}, model.ErrInvalidPhone
	}
	province := strings.TrimSpace(input.Province)
	city := strings.TrimSpace(input.City)
	district := strings.TrimSpace(input.District)
	if err := validateRegion(province, city, district); err != nil {
		return model.Address{}, err
	}
	detail := strings.TrimSpace(input.Detail)
	if n := utf8.RuneCountInString(detail); n < 5 || n > 200 {
		return model.Address{}, model.ErrInvalidAddressDetail
	}

	count, err := s.repository.CountAddresses(ctx, input.UserID)
	if err != nil {
		return model.Address{}, fmt.Errorf("统计收货地址数量：%w", err)
	}
	if count >= maxUserAddresses {
		return model.Address{}, model.ErrAddressLimitExceeded
	}

	address := model.Address{
		UserID: input.UserID, Recipient: recipient, Phone: phone,
		Province: province, City: city, District: district, Detail: detail,
		IsDefault: count == 0, // 第一个地址自动成为默认，单条 INSERT 天然无并发问题。
	}
	created, err := s.repository.CreateAddress(ctx, address)
	if err != nil {
		return model.Address{}, fmt.Errorf("创建收货地址：%w", err)
	}
	return created, nil
}

// List 查询当前用户的全部有效地址。
func (s *AddressService) List(ctx context.Context, userID uint64) ([]model.Address, error) {
	if userID == 0 {
		return nil, model.ErrInvalidUserID
	}
	addresses, err := s.repository.ListAddresses(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询收货地址列表：%w", err)
	}
	return addresses, nil
}

// Get 查询当前用户的单个地址。
func (s *AddressService) Get(ctx context.Context, userID, addressID uint64) (model.Address, error) {
	if userID == 0 {
		return model.Address{}, model.ErrInvalidUserID
	}
	if addressID == 0 {
		return model.Address{}, model.ErrAddressNotFound
	}
	address, err := s.repository.GetAddress(ctx, userID, addressID)
	if err != nil {
		return model.Address{}, fmt.Errorf("查询收货地址：%w", err)
	}
	return address, nil
}

// Update 逐个校验提交字段后局部更新地址；is_default 不允许从这里修改。
func (s *AddressService) Update(ctx context.Context, input UpdateAddressInput) (model.Address, error) {
	if input.UserID == 0 {
		return model.Address{}, model.ErrInvalidUserID
	}
	if input.AddressID == 0 {
		return model.Address{}, model.ErrAddressNotFound
	}
	fields := repositories.UpdateAddressFields{}
	if input.Recipient != nil {
		value := strings.TrimSpace(*input.Recipient)
		if n := utf8.RuneCountInString(value); n < 2 || n > 30 {
			return model.Address{}, model.ErrInvalidRecipient
		}
		fields.Recipient = &value
	}
	if input.Phone != nil {
		value := strings.TrimSpace(*input.Phone)
		if !phonePattern.MatchString(value) {
			return model.Address{}, model.ErrInvalidPhone
		}
		fields.Phone = &value
	}
	// 省市区必须整组提交或整组不提交，避免只改一半产生逻辑上不一致的行政区划。
	if input.Province != nil || input.City != nil || input.District != nil {
		if input.Province == nil || input.City == nil || input.District == nil {
			return model.Address{}, model.ErrInvalidAddressRegion
		}
		province := strings.TrimSpace(*input.Province)
		city := strings.TrimSpace(*input.City)
		district := strings.TrimSpace(*input.District)
		if err := validateRegion(province, city, district); err != nil {
			return model.Address{}, err
		}
		fields.Province, fields.City, fields.District = &province, &city, &district
	}
	if input.Detail != nil {
		value := strings.TrimSpace(*input.Detail)
		if n := utf8.RuneCountInString(value); n < 5 || n > 200 {
			return model.Address{}, model.ErrInvalidAddressDetail
		}
		fields.Detail = &value
	}
	if fields.Recipient == nil && fields.Phone == nil && fields.Province == nil && fields.City == nil && fields.District == nil && fields.Detail == nil {
		return model.Address{}, model.ErrEmptyAddressUpdate
	}
	updated, err := s.repository.UpdateAddress(ctx, input.UserID, input.AddressID, fields)
	if err != nil {
		return model.Address{}, fmt.Errorf("更新收货地址：%w", err)
	}
	return updated, nil
}

// SetDefault 通过事务切换默认地址。
func (s *AddressService) SetDefault(ctx context.Context, userID, addressID uint64) (model.Address, error) {
	if userID == 0 {
		return model.Address{}, model.ErrInvalidUserID
	}
	if addressID == 0 {
		return model.Address{}, model.ErrAddressNotFound
	}
	address, err := s.repository.SetDefaultAddress(ctx, userID, addressID)
	if err != nil {
		return model.Address{}, fmt.Errorf("设置默认地址：%w", err)
	}
	return address, nil
}

// Delete 删除当前用户的地址；删除默认地址后不自动补选。
func (s *AddressService) Delete(ctx context.Context, userID, addressID uint64) error {
	if userID == 0 {
		return model.ErrInvalidUserID
	}
	if addressID == 0 {
		return model.ErrAddressNotFound
	}
	if err := s.repository.DeleteAddress(ctx, userID, addressID); err != nil {
		return fmt.Errorf("删除收货地址：%w", err)
	}
	return nil
}

// validateRegion 校验省、市、区名称均不为空且不超过 50 个字符。
func validateRegion(province, city, district string) error {
	for _, region := range []string{province, city, district} {
		if n := utf8.RuneCountInString(region); n < 1 || n > 50 {
			return model.ErrInvalidAddressRegion
		}
	}
	return nil
}
