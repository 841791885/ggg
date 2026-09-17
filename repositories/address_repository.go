package repositories

import (
	"context"
	"errors"
	"fmt"

	model "ggg/models"
	"gorm.io/gorm"
)

// UpdateAddressFields 表示本次需要更新的地址字段，nil 表示不更新该字段。
// 不包含 IsDefault：设置默认地址必须走 SetDefaultAddress 的事务路径，防止绕过清零逻辑产生多默认。
type UpdateAddressFields struct {
	Recipient *string
	Phone     *string
	Province  *string
	City      *string
	District  *string
	Detail    *string
}

// CreateAddress 新增收货地址并回填自增 ID。
func (r *MySQLRepository) CreateAddress(ctx context.Context, address model.Address) (model.Address, error) {
	if err := r.db.WithContext(ctx).Create(&address).Error; err != nil {
		return model.Address{}, fmt.Errorf("创建收货地址：%w", err)
	}
	return address, nil
}

// ListAddresses 查询用户全部有效地址，默认地址排最前，其次按创建时间倒序。
func (r *MySQLRepository) ListAddresses(ctx context.Context, userID uint64) ([]model.Address, error) {
	var addresses []model.Address
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_default DESC, created_at DESC").
		Find(&addresses).Error; err != nil {
		return nil, fmt.Errorf("查询收货地址列表：%w", err)
	}
	return addresses, nil
}

// GetAddress 按 ID 查询当前用户自己的地址；他人地址统一按不存在处理。
func (r *MySQLRepository) GetAddress(ctx context.Context, userID, addressID uint64) (model.Address, error) {
	var address model.Address
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Address{}, model.ErrAddressNotFound
	}
	if err != nil {
		return model.Address{}, fmt.Errorf("查询收货地址：%w", err)
	}
	return address, nil
}

// CountAddresses 统计用户有效地址数量，GORM 自动排除软删除记录。
func (r *MySQLRepository) CountAddresses(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Address{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计收货地址数量：%w", err)
	}
	return count, nil
}

// UpdateAddress 局部更新当前用户的地址字段，字段条件与归属条件在同一条 SQL 中，防止越权修改。
func (r *MySQLRepository) UpdateAddress(ctx context.Context, userID, addressID uint64, fields UpdateAddressFields) (model.Address, error) {
	updates := map[string]any{}
	if fields.Recipient != nil {
		updates["recipient"] = *fields.Recipient
	}
	if fields.Phone != nil {
		updates["phone"] = *fields.Phone
	}
	if fields.Province != nil {
		updates["province"] = *fields.Province
	}
	if fields.City != nil {
		updates["city"] = *fields.City
	}
	if fields.District != nil {
		updates["district"] = *fields.District
	}
	if fields.Detail != nil {
		updates["detail"] = *fields.Detail
	}
	if len(updates) == 0 {
		return model.Address{}, model.ErrEmptyAddressUpdate
	}
	result := r.db.WithContext(ctx).Model(&model.Address{}).
		Where("id = ? AND user_id = ?", addressID, userID).
		Updates(updates)
	if result.Error != nil {
		return model.Address{}, fmt.Errorf("更新收货地址：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Address{}, model.ErrAddressNotFound
	}
	return r.GetAddress(ctx, userID, addressID)
}

// DeleteAddress 软删除当前用户的地址。
// 删除默认地址后不自动补选新默认，这是 PRD-004 明确的产品决定。
func (r *MySQLRepository) DeleteAddress(ctx context.Context, userID, addressID uint64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", addressID, userID).
		Delete(&model.Address{})
	if result.Error != nil {
		return fmt.Errorf("删除收货地址：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrAddressNotFound
	}
	return nil
}

// SetDefaultAddress 在事务内切换默认地址。
// 第一步 UPDATE 清除该用户所有有效地址的默认标记，InnoDB 会对命中的行加排他锁；
// 并发的第二个切换事务必须等待前一个提交，因此两步之间不会交叉，最终最多只有一个默认地址。
// 目标地址不存在或不属于当前用户时返回错误，事务整体回滚，不会出现"全被清零但没有默认"的中间态。
func (r *MySQLRepository) SetDefaultAddress(ctx context.Context, userID, addressID uint64) (model.Address, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Address{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return fmt.Errorf("清除默认地址标记：%w", err)
		}
		result := tx.Model(&model.Address{}).
			Where("id = ? AND user_id = ?", addressID, userID).
			Update("is_default", true)
		if result.Error != nil {
			return fmt.Errorf("设置默认地址：%w", result.Error)
		}
		if result.RowsAffected == 0 {
			return model.ErrAddressNotFound
		}
		return nil
	})
	if err != nil {
		return model.Address{}, err
	}
	return r.GetAddress(ctx, userID, addressID)
}

// 编译期检查：本文件负责的领域接口是否都实现了。
// 少写方法时错误直接指向这里，而不是 mysql_repository.go 里的全量断言。
var _ AddressRepository = (*MySQLRepository)(nil)
