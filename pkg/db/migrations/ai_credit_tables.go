package migrations

import (
	"database/sql"
	"scheduler0/pkg/models"
)

// AICreditTables creates the platform AI-credit tables.
//
// These originally shipped inside the already-applied "main_tables" migration
// (32032e7), so any database created before that commit recorded main_tables
// as done and never got them ("no such table: account_ai_credits" on every
// prompt-based schedule). Running them as their own migration fills that gap
// on existing nodes; IF NOT EXISTS keeps it a no-op where the tables already
// exist.
func AICreditTables() models.Migration {
	return models.Migration{
		Name: "ai_credit_tables",
		Up:   aiCreditTablesUp,
		Down: aiCreditTablesDown,
	}
}

func aiCreditTablesUp(db *sql.Tx) error {
	_, err := db.Exec(`
-- Per-account dollar AI-credit balance and auto-top-up preferences. Funds platform-model
-- prompt runs only (token cost + markup); BYOK runs never debit it. Amounts are integer
-- micros ($1 = 1,000,000) to avoid floating-point drift. Defaults: $1 auto-top-up threshold,
-- $10 recharge amount.
CREATE TABLE IF NOT EXISTS account_ai_credits
(
	account_id                  INTEGER PRIMARY KEY,
	balance_micros              INTEGER NOT NULL DEFAULT 0,
	auto_topup_enabled          BOOLEAN NOT NULL DEFAULT 0,
	auto_topup_threshold_micros INTEGER NOT NULL DEFAULT 1000000,
	auto_topup_amount_micros    INTEGER NOT NULL DEFAULT 10000000,
	welcome_granted             BOOLEAN NOT NULL DEFAULT 0,
	date_created                DATETIME NOT NULL,
	date_modified               DATETIME NOT NULL,
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

-- Append-only ledger of every credit movement (welcome grant, top-up, auto-top-up,
-- consumption, refund). balance_after_micros snapshots the resulting balance; the unique
-- idempotency_key makes credit/top-up operations safe to retry (partial index skips the
-- empty keys used by internal consume rows).
CREATE TABLE IF NOT EXISTS account_ai_credit_ledger
(
	id                       INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id               INTEGER NOT NULL,
	amount_micros            INTEGER NOT NULL,
	kind                     TEXT NOT NULL,
	balance_after_micros     INTEGER NOT NULL,
	provider                 TEXT NOT NULL DEFAULT '',
	model                    TEXT NOT NULL DEFAULT '',
	prompt_request_id        INTEGER NOT NULL DEFAULT 0,
	stripe_payment_intent_id TEXT NOT NULL DEFAULT '',
	idempotency_key          TEXT NOT NULL DEFAULT '',
	date_created             DATETIME NOT NULL,
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_ai_credit_ledger_idem ON account_ai_credit_ledger(idempotency_key) WHERE idempotency_key <> '';
CREATE INDEX IF NOT EXISTS idx_account_ai_credit_ledger_account_date ON account_ai_credit_ledger(account_id, date_created);
`)
	return err
}

func aiCreditTablesDown(db *sql.Tx) error {
	_, err := db.Exec(`
	DROP TABLE IF EXISTS account_ai_credit_ledger;
	DROP TABLE IF EXISTS account_ai_credits;
	`)
	return err
}
