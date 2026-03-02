package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"abot/pkg/types"
)

// AccountStore implements types.AccountStore using GORM.
type AccountStore struct {
	db *gorm.DB
}

func NewAccountStore(db *gorm.DB) *AccountStore {
	return &AccountStore{db: db}
}

func (s *AccountStore) Create(ctx context.Context, account *types.Account) error {
	m := accountToModel(account)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("AccountStore.Create(%s): %w", account.ID, err)
	}
	return nil
}

func (s *AccountStore) GetByID(ctx context.Context, id string) (*types.Account, error) {
	var m AccountModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, fmt.Errorf("AccountStore.GetByID(%s): %w", id, err)
	}
	return accountFromModel(&m), nil
}

func (s *AccountStore) GetByEmail(ctx context.Context, email string) (*types.Account, error) {
	var m AccountModel
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&m).Error; err != nil {
		return nil, fmt.Errorf("AccountStore.GetByEmail(%s): %w", email, err)
	}
	return accountFromModel(&m), nil
}

func (s *AccountStore) Update(ctx context.Context, account *types.Account) error {
	m := accountToModel(account)
	if err := s.db.WithContext(ctx).Save(&m).Error; err != nil {
		return fmt.Errorf("AccountStore.Update(%s): %w", account.ID, err)
	}
	return nil
}

func accountToModel(a *types.Account) *AccountModel {
	return &AccountModel{
		ID:           a.ID,
		Email:        a.Email,
		PasswordHash: a.PasswordHash,
		DisplayName:  a.DisplayName,
		Status:       a.Status,
		Role:         a.Role,
		CreatedAt:    a.CreatedAt,
	}
}

func accountFromModel(m *AccountModel) *types.Account {
	return &types.Account{
		ID:           m.ID,
		Email:        m.Email,
		PasswordHash: m.PasswordHash,
		DisplayName:  m.DisplayName,
		Status:       m.Status,
		Role:         m.Role,
		CreatedAt:    m.CreatedAt,
	}
}

var _ types.AccountStore = (*AccountStore)(nil)

// AccountTenantStore implements types.AccountTenantStore using GORM.
type AccountTenantStore struct {
	db *gorm.DB
}

func NewAccountTenantStore(db *gorm.DB) *AccountTenantStore {
	return &AccountTenantStore{db: db}
}

func (s *AccountTenantStore) Create(ctx context.Context, at *types.AccountTenant) error {
	m := accountTenantToModel(at)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("AccountTenantStore.Create(%s,%s): %w", at.AccountID, at.TenantID, err)
	}
	return nil
}

func (s *AccountTenantStore) ListByAccount(ctx context.Context, accountID string) ([]*types.AccountTenant, error) {
	var models []AccountTenantModel
	if err := s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("AccountTenantStore.ListByAccount(%s): %w", accountID, err)
	}
	out := make([]*types.AccountTenant, len(models))
	for i := range models {
		out[i] = accountTenantFromModel(&models[i])
	}
	return out, nil
}

func (s *AccountTenantStore) HasAccess(ctx context.Context, accountID, tenantID string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&AccountTenantModel{}).
		Where("account_id = ? AND tenant_id = ?", accountID, tenantID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("AccountTenantStore.HasAccess(%s,%s): %w", accountID, tenantID, err)
	}
	return count > 0, nil
}

func accountTenantToModel(at *types.AccountTenant) *AccountTenantModel {
	return &AccountTenantModel{
		AccountID: at.AccountID,
		TenantID:  at.TenantID,
		Role:      at.Role,
		CreatedAt: at.CreatedAt,
	}
}

func accountTenantFromModel(m *AccountTenantModel) *types.AccountTenant {
	return &types.AccountTenant{
		AccountID: m.AccountID,
		TenantID:  m.TenantID,
		Role:      m.Role,
		CreatedAt: m.CreatedAt,
	}
}

var _ types.AccountTenantStore = (*AccountTenantStore)(nil)
