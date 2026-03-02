package bootstrap

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"abot/pkg/agent"
	mysqlstore "abot/pkg/storage/mysql"
)

// StoreBundle groups all MySQL-backed stores.
type StoreBundle struct {
	Tenant        *mysqlstore.TenantStore
	Workspace     *mysqlstore.WorkspaceDocStore
	UserWorkspace *mysqlstore.UserWorkspaceDocStore
	SkillRegistry *mysqlstore.SkillRegistryStoreMySQL
	TenantSkill   *mysqlstore.TenantSkillStoreMySQL
	Scheduler     *mysqlstore.SchedulerStoreMySQL
	Proposal      *mysqlstore.SkillProposalStoreMySQL
	MemoryEvent   *mysqlstore.MemoryEventStoreMySQL
	Account       *mysqlstore.AccountStore
	AccountTenant *mysqlstore.AccountTenantStore
	ChatSession   *mysqlstore.ChatSessionStore
}

// NewDatabase connects to MySQL and runs migrations.
func NewDatabase(cfg *agent.Config) (*gorm.DB, error) {
	if cfg.MySQLDSN == "" {
		return nil, fmt.Errorf("mysql_dsn is required")
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := mysqlstore.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	slog.Info("mysql connected, tables migrated")
	return db, nil
}

// NewStores creates all MySQL-backed stores.
func NewStores(db *gorm.DB) *StoreBundle {
	return &StoreBundle{
		Tenant:        mysqlstore.NewTenantStore(db),
		Workspace:     mysqlstore.NewWorkspaceDocStore(db),
		UserWorkspace: mysqlstore.NewUserWorkspaceDocStore(db),
		SkillRegistry: mysqlstore.NewSkillRegistryStore(db),
		TenantSkill:   mysqlstore.NewTenantSkillStore(db),
		Scheduler:     mysqlstore.NewSchedulerStore(db),
		Proposal:      mysqlstore.NewSkillProposalStore(db),
		MemoryEvent:   mysqlstore.NewMemoryEventStore(db),
		Account:       mysqlstore.NewAccountStore(db),
		AccountTenant: mysqlstore.NewAccountTenantStore(db),
		ChatSession:   mysqlstore.NewChatSessionStore(db),
	}
}
