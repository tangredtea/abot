package sqlite

import "database/sql"

// InitSchema creates all SQLite tables.
func InitSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS cron_jobs (
	id TEXT PRIMARY KEY,
	tenant_id TEXT,
	user_id TEXT,
	name TEXT,
	enabled INTEGER,
	schedule TEXT,
	message TEXT,
	channel TEXT,
	chat_id TEXT,
	delete_after_run INTEGER,
	state TEXT,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_cron_jobs_tenant ON cron_jobs(tenant_id, enabled);

CREATE TABLE IF NOT EXISTS cron_job_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id TEXT,
	run_at DATETIME,
	duration_ms INTEGER,
	status TEXT,
	error TEXT
);
CREATE INDEX IF NOT EXISTS idx_cron_job_logs_job ON cron_job_logs(job_id);
CREATE INDEX IF NOT EXISTS idx_cron_job_logs_run ON cron_job_logs(run_at);

CREATE TABLE IF NOT EXISTS sender_allowlist (
	tenant_id TEXT,
	chat_id TEXT,
	entry TEXT,
	PRIMARY KEY (tenant_id, chat_id)
);
`
	_, err := db.Exec(schema)
	return err
}
