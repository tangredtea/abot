package storage_test

import (
	"testing"

	"abot/pkg/storage/cache"
	"abot/pkg/types"
)

// --- CacheLayer Tenant tests ---

func TestCacheLayer_TenantMiss(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	_, ok := c.GetTenant("missing")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheLayer_TenantPutGet(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	tenant := &types.Tenant{TenantID: "t1", Name: "Test"}
	c.PutTenant(tenant)

	got, ok := c.GetTenant("t1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Name != "Test" {
		t.Errorf("name: %q", got.Name)
	}
}

func TestCacheLayer_TenantInvalidate(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	c.PutTenant(&types.Tenant{TenantID: "t1"})
	c.InvalidateTenant("t1")

	_, ok := c.GetTenant("t1")
	if ok {
		t.Error("expected miss after invalidate")
	}
}

// --- CacheLayer TenantSkills tests ---

func TestCacheLayer_TenantSkillsMiss(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	_, ok := c.GetTenantSkills("missing")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheLayer_TenantSkillsPutGet(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	skills := []*types.TenantSkill{
		{TenantID: "t1", SkillID: 1},
		{TenantID: "t1", SkillID: 2},
	}
	c.PutTenantSkills("t1", skills)

	got, ok := c.GetTenantSkills("t1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 2 {
		t.Errorf("expected 2 skills, got %d", len(got))
	}
}

func TestCacheLayer_TenantSkillsInvalidate(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	c.PutTenantSkills("t1", []*types.TenantSkill{{TenantID: "t1", SkillID: 1}})
	c.InvalidateTenantSkills("t1")

	_, ok := c.GetTenantSkills("t1")
	if ok {
		t.Error("expected miss after invalidate")
	}
}

// --- CacheLayer SkillContent tests ---

func TestCacheLayer_SkillContentMiss(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	_, ok := c.GetSkillContent("missing")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheLayer_SkillContentPutGet(t *testing.T) {
	c := cache.NewCacheLayer(10, 10)
	c.PutSkillContent("skill:v1", "/tmp/skill-v1.py")

	got, ok := c.GetSkillContent("skill:v1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "/tmp/skill-v1.py" {
		t.Errorf("path: %q", got)
	}
}

// --- LRU eviction ---

func TestCacheLayer_LRUEviction(t *testing.T) {
	c := cache.NewCacheLayer(2, 2) // capacity=2

	c.PutTenant(&types.Tenant{TenantID: "t1"})
	c.PutTenant(&types.Tenant{TenantID: "t2"})
	c.PutTenant(&types.Tenant{TenantID: "t3"}) // should evict t1

	_, ok := c.GetTenant("t1")
	if ok {
		t.Error("expected t1 to be evicted")
	}
	_, ok = c.GetTenant("t3")
	if !ok {
		t.Error("expected t3 to be present")
	}
}

// --- LRU generic tests (migrated from pkg/storage/cache/lru_test.go) ---

func TestLRU_PutGet(t *testing.T) {
	c := cache.NewLRU[string, int](3)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Errorf("Get(a): got %d, %v", v, ok)
	}
	_, ok = c.Get("missing")
	if ok {
		t.Error("expected miss for unknown key")
	}
}

func TestLRU_Eviction(t *testing.T) {
	c := cache.NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // should evict "a"

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c): got %d, %v", v, ok)
	}
}

func TestLRU_AccessRefreshesOrder(t *testing.T) {
	c := cache.NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a") // refresh "a", now "b" is oldest
	c.Put("c", 3) // should evict "b"

	if _, ok := c.Get("b"); ok {
		t.Error("expected 'b' to be evicted after 'a' was accessed")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("expected 'a' to survive")
	}
}

func TestLRU_UpdateExisting(t *testing.T) {
	c := cache.NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("a", 99)

	v, ok := c.Get("a")
	if !ok || v != 99 {
		t.Errorf("expected updated value 99, got %d", v)
	}
}

func TestLRU_Delete(t *testing.T) {
	c := cache.NewLRU[string, int](3)
	c.Put("a", 1)
	c.Delete("a")

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be deleted")
	}
	// Delete non-existent key should not panic
	c.Delete("nonexistent")
}
