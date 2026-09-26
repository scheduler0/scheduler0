package migrations

import (
	"database/sql"
	"scheduler0-private/pkg/models"
)

func SeedSystemAccount() models.Migration {
	return models.Migration{
		Name: "seed_system_account",
		Up:   seedSystemAccountUp,
		Down: seedSystemAccountDown,
	}
}

func seedSystemAccountUp(db *sql.Tx) error {
	_, err := db.Exec(`
		INSERT INTO accounts (name, date_created) VALUES ('System', datetime('now'));

		INSERT INTO account_features (account_id, feature_id, date_created)
			SELECT 1, id, datetime('now') FROM features WHERE name != 'team_members';

		INSERT INTO account_job_executions_count (account_id, execution_count, date_created, date_modified, next_reset_date)
			VALUES (1, 9999999999, datetime('now'), datetime('now'), datetime('now'));

		INSERT INTO account_ai_quota_period (account_id, period_start, next_reset_date, date_created, date_modified)
			VALUES (1, datetime('now'), datetime('now', '+1 month'), datetime('now'), datetime('now'));
	`)
	return err
}

func seedSystemAccountDown(db *sql.Tx) error {
	_, err := db.Exec(`
		DELETE FROM account_ai_quota_period WHERE account_id = 1;
		DELETE FROM account_job_executions_count WHERE account_id = 1;
		DELETE FROM account_features WHERE account_id = 1;
		DELETE FROM accounts WHERE name = 'System';
	`)
	return err
}
