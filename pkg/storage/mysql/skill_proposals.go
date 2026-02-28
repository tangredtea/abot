package mysql

import (
	"context"

	"gorm.io/gorm"

	"abot/pkg/types"
)

// SkillProposalStoreMySQL implements types.SkillProposalStore.
type SkillProposalStoreMySQL struct {
	db *gorm.DB
}

func NewSkillProposalStore(db *gorm.DB) *SkillProposalStoreMySQL {
	return &SkillProposalStoreMySQL{db: db}
}

func (s *SkillProposalStoreMySQL) Create(ctx context.Context, p *types.SkillProposal) error {
	m := proposalToModel(p)
	return s.db.WithContext(ctx).Create(&m).Error
}

func (s *SkillProposalStoreMySQL) Get(ctx context.Context, id int64) (*types.SkillProposal, error) {
	var m SkillProposalModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return proposalFromModel(&m), nil
}

func (s *SkillProposalStoreMySQL) List(ctx context.Context, status string) ([]*types.SkillProposal, error) {
	q := s.db.WithContext(ctx)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var models []SkillProposalModel
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*types.SkillProposal, len(models))
	for i := range models {
		out[i] = proposalFromModel(&models[i])
	}
	return out, nil
}

func (s *SkillProposalStoreMySQL) UpdateStatus(ctx context.Context, id int64, status, reviewedBy string) error {
	return s.db.WithContext(ctx).
		Model(&SkillProposalModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      status,
			"reviewed_by": reviewedBy,
		}).Error
}

// --- converters ---

func proposalToModel(p *types.SkillProposal) *SkillProposalModel {
	return &SkillProposalModel{
		ID:         p.ID,
		SkillName:  p.SkillName,
		ProposedBy: p.ProposedBy,
		ObjectPath: p.ObjectPath,
		Status:     p.Status,
		ReviewedBy: p.ReviewedBy,
		CreatedAt:  p.CreatedAt,
	}
}

func proposalFromModel(m *SkillProposalModel) *types.SkillProposal {
	return &types.SkillProposal{
		ID:         m.ID,
		SkillName:  m.SkillName,
		ProposedBy: m.ProposedBy,
		ObjectPath: m.ObjectPath,
		Status:     m.Status,
		ReviewedBy: m.ReviewedBy,
		CreatedAt:  m.CreatedAt,
	}
}

var _ types.SkillProposalStore = (*SkillProposalStoreMySQL)(nil)
