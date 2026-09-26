package account_ai_credits

import (
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

const (
	TableName                   = "account_ai_credits"
	ColAccountID                = "account_id"
	ColBalanceMicros            = "balance_micros"
	ColAutoTopupEnabled         = "auto_topup_enabled"
	ColAutoTopupThresholdMicros = "auto_topup_threshold_micros"
	ColAutoTopupAmountMicros    = "auto_topup_amount_micros"
	ColWelcomeGranted           = "welcome_granted"
	ColDateCreated              = "date_created"
	ColDateModified             = "date_modified"

	LedgerTableName              = "account_ai_credit_ledger"
	LedgerColID                  = "id"
	LedgerColAccountID           = "account_id"
	LedgerColAmountMicros        = "amount_micros"
	LedgerColKind                = "kind"
	LedgerColBalanceAfterMicros  = "balance_after_micros"
	LedgerColProvider            = "provider"
	LedgerColModel               = "model"
	LedgerColPromptRequestID     = "prompt_request_id"
	LedgerColStripePaymentIntent = "stripe_payment_intent_id"
	LedgerColIdempotencyKey      = "idempotency_key"
	LedgerColDateCreated         = "date_created"
)

const (
	DefaultAutoTopupThresholdMicros int64 = 1_000_000
	DefaultAutoTopupAmountMicros    int64 = 10_000_000
)

type LedgerEntryInput struct {
	AmountMicros          int64
	Kind                  string
	Provider              string
	Model                 string
	PromptRequestID       uint64
	StripePaymentIntentID string
	IdempotencyKey        string
}

type AccountAICreditsRepo interface {
	Ensure(accountId uint64, welcomeMicros int64) (*models.AICredits, *utils.GenericError)
	GetByAccountId(accountId uint64) (*models.AICredits, *utils.GenericError)
	ApplyDelta(accountId uint64, entry LedgerEntryInput) (*models.AICredits, *utils.GenericError)
	UpdateAutoTopup(accountId uint64, enabled bool, thresholdMicros int64, amountMicros int64) (*models.AICredits, *utils.GenericError)
	GetLedger(accountId uint64, limit uint64, offset uint64) ([]models.AICreditLedgerEntry, *utils.GenericError)
	DeleteByAccountId(accountId uint64) *utils.GenericError
}

type accountAICreditsRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	scheduler0RaftActions fsm.Scheduler0RaftActions
	logger                hclog.Logger
}

func NewAccountAICreditsRepo(
	logger hclog.Logger,
	scheduler0RaftActions fsm.Scheduler0RaftActions,
	fsmStore fsm.Scheduler0RaftStore,
) AccountAICreditsRepo {
	return &accountAICreditsRepo{
		fsmStore:              fsmStore,
		scheduler0RaftActions: scheduler0RaftActions,
		logger:                logger.Named("account-ai-credits-repo"),
	}
}

func (repo *accountAICreditsRepo) GetByAccountId(accountId uint64) (*models.AICredits, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	if accountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(
		ColAccountID, ColBalanceMicros, ColAutoTopupEnabled,
		ColAutoTopupThresholdMicros, ColAutoTopupAmountMicros,
		ColWelcomeGranted, ColDateCreated, ColDateModified,
	).
		From(TableName).
		Where(fmt.Sprintf("%s = ?", ColAccountID), accountId).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("ai credits not found for account id: %d", accountId))
	}

	var record models.AICredits
	if scanErr := rows.Scan(
		&record.AccountID, &record.BalanceMicros, &record.AutoTopupEnabled,
		&record.AutoTopupThresholdMicros, &record.AutoTopupAmountMicros,
		&record.WelcomeGranted, &record.DateCreated, &record.DateModified,
	); scanErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
	}
	return &record, nil
}

func (repo *accountAICreditsRepo) Ensure(accountId uint64, welcomeMicros int64) (*models.AICredits, *utils.GenericError) {
	record, getErr := repo.GetByAccountId(accountId)
	if getErr == nil {
		return record, nil
	}
	if getErr.Type != http.StatusNotFound {
		return nil, getErr
	}

	now := scheduler0time.GetSchedulerTime().GetTime(time.Now())
	welcomeGranted := welcomeMicros > 0
	balance := int64(0)
	if welcomeGranted {
		balance = welcomeMicros
	}

	insertQuery, insertParams, buildErr := sq.Insert(TableName).
		Columns(
			ColAccountID, ColBalanceMicros, ColAutoTopupEnabled,
			ColAutoTopupThresholdMicros, ColAutoTopupAmountMicros,
			ColWelcomeGranted, ColDateCreated, ColDateModified,
		).
		Values(
			accountId, balance, false,
			DefaultAutoTopupThresholdMicros, DefaultAutoTopupAmountMicros,
			welcomeGranted, now, now,
		).
		ToSql()
	if buildErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
	}

	if _, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, insertQuery, insertParams, []uint64{}, 0,
	); applyErr != nil {
		repo.logger.Error("Ensure: failed to write credits insert to raft log", "error", applyErr, "accountId", accountId)
		return nil, applyErr
	}

	if welcomeGranted {
		if ledgerErr := repo.insertLedger(accountId, LedgerEntryInput{
			AmountMicros:   welcomeMicros,
			Kind:           models.AICreditKindWelcome,
			IdempotencyKey: fmt.Sprintf("welcome:%d", accountId),
		}, balance, now); ledgerErr != nil {
			repo.logger.Error("Ensure: failed to write welcome ledger entry", "error", ledgerErr, "accountId", accountId)
		}
	}

	return &models.AICredits{
		AccountID:                accountId,
		BalanceMicros:            balance,
		AutoTopupEnabled:         false,
		AutoTopupThresholdMicros: DefaultAutoTopupThresholdMicros,
		AutoTopupAmountMicros:    DefaultAutoTopupAmountMicros,
		WelcomeGranted:           welcomeGranted,
		DateCreated:              now,
		DateModified:             now,
	}, nil
}

func (repo *accountAICreditsRepo) ApplyDelta(accountId uint64, entry LedgerEntryInput) (*models.AICredits, *utils.GenericError) {
	record, getErr := repo.Ensure(accountId, 0)
	if getErr != nil {
		return nil, getErr
	}

	if entry.IdempotencyKey != "" {
		exists, existErr := repo.ledgerKeyExists(entry.IdempotencyKey)
		if existErr != nil {
			return nil, existErr
		}
		if exists {
			repo.logger.Info("ApplyDelta: idempotent no-op, ledger key already applied", "accountId", accountId, "key", entry.IdempotencyKey)
			return record, nil
		}
	}

	newBalance := record.BalanceMicros + entry.AmountMicros
	if newBalance < 0 {
		newBalance = 0
	}

	now := scheduler0time.GetSchedulerTime().GetTime(time.Now())
	updateQuery, updateParams, buildErr := sq.Update(TableName).
		Set(ColBalanceMicros, newBalance).
		Set(ColDateModified, now).
		Where(fmt.Sprintf("%s = ?", ColAccountID), accountId).
		ToSql()
	if buildErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
	}
	if _, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, updateQuery, updateParams, []uint64{}, 0,
	); applyErr != nil {
		repo.logger.Error("ApplyDelta: failed to write balance update to raft log", "error", applyErr, "accountId", accountId)
		return nil, applyErr
	}

	if ledgerErr := repo.insertLedger(accountId, entry, newBalance, now); ledgerErr != nil {
		repo.logger.Error("ApplyDelta: failed to write ledger entry", "error", ledgerErr, "accountId", accountId)
	}

	record.BalanceMicros = newBalance
	record.DateModified = now
	return record, nil
}

func (repo *accountAICreditsRepo) UpdateAutoTopup(accountId uint64, enabled bool, thresholdMicros int64, amountMicros int64) (*models.AICredits, *utils.GenericError) {
	if _, ensureErr := repo.Ensure(accountId, 0); ensureErr != nil {
		return nil, ensureErr
	}
	if thresholdMicros <= 0 {
		thresholdMicros = DefaultAutoTopupThresholdMicros
	}
	if amountMicros <= 0 {
		amountMicros = DefaultAutoTopupAmountMicros
	}

	now := scheduler0time.GetSchedulerTime().GetTime(time.Now())
	updateQuery, updateParams, buildErr := sq.Update(TableName).
		Set(ColAutoTopupEnabled, enabled).
		Set(ColAutoTopupThresholdMicros, thresholdMicros).
		Set(ColAutoTopupAmountMicros, amountMicros).
		Set(ColDateModified, now).
		Where(fmt.Sprintf("%s = ?", ColAccountID), accountId).
		ToSql()
	if buildErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
	}
	if _, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, updateQuery, updateParams, []uint64{}, 0,
	); applyErr != nil {
		repo.logger.Error("UpdateAutoTopup: failed to write to raft log", "error", applyErr, "accountId", accountId)
		return nil, applyErr
	}
	return repo.GetByAccountId(accountId)
}

func (repo *accountAICreditsRepo) GetLedger(accountId uint64, limit uint64, offset uint64) ([]models.AICreditLedgerEntry, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	if accountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}
	if limit == 0 {
		limit = 50
	}

	builder := sq.Select(
		LedgerColID, LedgerColAccountID, LedgerColAmountMicros, LedgerColKind,
		LedgerColBalanceAfterMicros, LedgerColProvider, LedgerColModel,
		LedgerColPromptRequestID, LedgerColStripePaymentIntent, LedgerColIdempotencyKey,
		LedgerColDateCreated,
	).
		From(LedgerTableName).
		Where(fmt.Sprintf("%s = ?", LedgerColAccountID), accountId).
		OrderBy(fmt.Sprintf("%s DESC", LedgerColDateCreated)).
		Limit(limit).
		Offset(offset).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := builder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	entries := []models.AICreditLedgerEntry{}
	for rows.Next() {
		var e models.AICreditLedgerEntry
		if scanErr := rows.Scan(
			&e.ID, &e.AccountID, &e.AmountMicros, &e.Kind,
			&e.BalanceAfterMicros, &e.Provider, &e.Model,
			&e.PromptRequestID, &e.StripePaymentIntentID, &e.IdempotencyKey,
			&e.DateCreated,
		); scanErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		entries = append(entries, e)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	return entries, nil
}

func (repo *accountAICreditsRepo) DeleteByAccountId(accountId uint64) *utils.GenericError {
	if accountId == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}
	for _, table := range []string{LedgerTableName, TableName} {
		query, params, err := sq.Delete(table).
			Where(fmt.Sprintf("%s = ?", ColAccountID), accountId).
			ToSql()
		if err != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		if _, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
			repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0,
		); applyErr != nil {
			return applyErr
		}
	}
	return nil
}

func (repo *accountAICreditsRepo) ledgerKeyExists(key string) (bool, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	builder := sq.Select("COUNT(*)").
		From(LedgerTableName).
		Where(fmt.Sprintf("%s = ?", LedgerColIdempotencyKey), key).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	var count uint64
	if err := builder.QueryRow().Scan(&count); err != nil {
		return false, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	return count > 0, nil
}

func (repo *accountAICreditsRepo) insertLedger(accountId uint64, entry LedgerEntryInput, balanceAfter int64, now time.Time) *utils.GenericError {
	query, params, buildErr := sq.Insert(LedgerTableName).
		Columns(
			LedgerColAccountID, LedgerColAmountMicros, LedgerColKind,
			LedgerColBalanceAfterMicros, LedgerColProvider, LedgerColModel,
			LedgerColPromptRequestID, LedgerColStripePaymentIntent, LedgerColIdempotencyKey,
			LedgerColDateCreated,
		).
		Values(
			accountId, entry.AmountMicros, entry.Kind,
			balanceAfter, entry.Provider, entry.Model,
			entry.PromptRequestID, entry.StripePaymentIntentID, entry.IdempotencyKey,
			now,
		).
		ToSql()
	if buildErr != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
	}
	if _, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0,
	); applyErr != nil {
		return applyErr
	}
	return nil
}
