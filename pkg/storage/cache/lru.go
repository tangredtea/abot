// Package cache provides in-process LRU caching layers for hot data such as
// tenant configurations, installed skills, and skill content.
package cache

import (
	"abot/pkg/types"
)

// CacheLayer provides in-process LRU caching for hot data.
type CacheLayer struct {
	tenantConfigs *lruCache[string, *types.Tenant]
	tenantSkills  *lruCache[string, []*types.TenantSkill]
	skillContent  *lruCache[string, string]
}

// NewCacheLayer creates a cache with the given capacity per cache type.
func NewCacheLayer(tenantSize, skillSize int) *CacheLayer {
	return &CacheLayer{
		tenantConfigs: NewLRU[string, *types.Tenant](tenantSize),
		tenantSkills:  NewLRU[string, []*types.TenantSkill](tenantSize),
		skillContent:  NewLRU[string, string](skillSize),
	}
}

// --- Tenant Config ---

func (c *CacheLayer) GetTenant(tenantID string) (*types.Tenant, bool) {
	return c.tenantConfigs.Get(tenantID)
}

func (c *CacheLayer) PutTenant(tenant *types.Tenant) {
	c.tenantConfigs.Put(tenant.TenantID, tenant)
}

func (c *CacheLayer) InvalidateTenant(tenantID string) {
	c.tenantConfigs.Delete(tenantID)
}

// --- Tenant Skills ---

func (c *CacheLayer) GetTenantSkills(tenantID string) ([]*types.TenantSkill, bool) {
	return c.tenantSkills.Get(tenantID)
}

func (c *CacheLayer) PutTenantSkills(tenantID string, skills []*types.TenantSkill) {
	c.tenantSkills.Put(tenantID, skills)
}

func (c *CacheLayer) InvalidateTenantSkills(tenantID string) {
	c.tenantSkills.Delete(tenantID)
}

// --- Skill Content ---

func (c *CacheLayer) GetSkillContent(key string) (string, bool) {
	return c.skillContent.Get(key)
}

func (c *CacheLayer) PutSkillContent(key, localPath string) {
	c.skillContent.Put(key, localPath)
}
