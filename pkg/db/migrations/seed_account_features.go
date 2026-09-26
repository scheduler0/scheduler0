package migrations

import (
	"database/sql"
	"scheduler0/pkg/models"
)

func SeedAccountFeatures() models.Migration {
	return models.Migration{
		Name: "seed_account_features",
		Up:   seedAccountFeaturesUp,
		Down: seedAccountFeaturesDown,
	}
}

var paidFeatures = []string{
	"increased_retry_max_by_five",
	"increased_job_payload_size_to_1mb",
	"increased_number_of_job_executions_100_k_per_month",
	"increased_execution_logs_90_days_retention",
	"team_members",
	"increased_number_of_classify_requests_100_k_per_month",
	"increased_number_of_prompt_requests_100_k_per_month",
}

func seedAccountFeaturesUp(db *sql.Tx) error {
	for _, name := range paidFeatures {
		if _, err := db.Exec(`INSERT INTO features (name, date_created) VALUES (?, datetime('now'));`, name); err != nil {
			return err
		}
	}
	return nil
}

func seedAccountFeaturesDown(db *sql.Tx) error {
	for _, name := range paidFeatures {
		if _, err := db.Exec(`DELETE FROM features WHERE name = ?;`, name); err != nil {
			return err
		}
	}
	return nil
}
