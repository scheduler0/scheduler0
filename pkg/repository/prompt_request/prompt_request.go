package prompt_request

import (
	"fmt"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

const (
	TableName        = "account_prompt_requests"
	ColID            = "id"
	ColAccountID     = "account_id"
	ColPrompt        = "prompt"
	ColProvider      = "provider"
	ColModel         = "model"
	ColOutput        = "output"
	ColInputTokens   = "input_tokens"
	ColOutputTokens  = "output_tokens"
	ColTotalTokens   = "total_tokens"
	ColDurationMs    = "duration_ms"
	ColEstimatedCost = "estimated_cost_usd"
	ColStatus        = "status"
	ColError         = "error"
	ColDateCreated   = "date_created"
)

// PromptRequestFilter narrows and paginates the prompt-request log for an account.
type PromptRequestFilter struct {
	AccountID      uint64
	StartDate      *time.Time
	EndDate        *time.Time
	Provider       string
	Model          string
	Status         string
	Search         string // substring match on prompt or output
	OrderDirection string // ASC or DESC (by date_created); defaults to DESC
	Limit          uint64
	Offset         uint64
}

// PromptRequestRepo persists and queries the account prompt-request log.
type PromptRequestRepo interface {
	Record(req models.AccountPromptRequest) error
	GetPromptRequestsFiltered(filter PromptRequestFilter) ([]models.AccountPromptRequest, error)
	CountPromptRequests(filter PromptRequestFilter) (uint64, error)
	SumEstimatedCostUSD(filter PromptRequestFilter) (float64, error)
}

type promptRequestRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	scheduler0RaftActions fsm.Scheduler0RaftActions
	logger                hclog.Logger
}

func NewPromptRequestRepo(
	logger hclog.Logger,
	scheduler0RaftActions fsm.Scheduler0RaftActions,
	fsmStore fsm.Scheduler0RaftStore,
) PromptRequestRepo {
	return &promptRequestRepo{
		fsmStore:              fsmStore,
		scheduler0RaftActions: scheduler0RaftActions,
		logger:                logger.Named("prompt-request-repo"),
	}
}

// Record inserts a prompt-request row through the Raft log so it replicates across the cluster.
func (r *promptRequestRepo) Record(req models.AccountPromptRequest) error {
	if req.DateCreated.IsZero() {
		req.DateCreated = time.Now().UTC()
	}

	query, params, buildErr := sq.Insert(TableName).
		Columns(
			ColAccountID, ColPrompt, ColProvider, ColModel, ColOutput,
			ColInputTokens, ColOutputTokens, ColTotalTokens,
			ColDurationMs, ColEstimatedCost, ColStatus, ColError, ColDateCreated,
		).
		Values(
			req.AccountID, req.Prompt, req.Provider, req.Model, req.Output,
			req.InputTokens, req.OutputTokens, req.TotalTokens,
			req.DurationMs, req.EstimatedCostUSD, req.Status, req.Error, req.DateCreated,
		).
		ToSql()
	if buildErr != nil {
		r.logger.Error("Record: failed to build insert", "error", buildErr, "accountID", req.AccountID)
		return buildErr
	}

	_, applyErr := r.scheduler0RaftActions.WriteCommandToRaftLog(
		r.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		query,
		params,
		[]uint64{},
		0,
	)
	if applyErr != nil {
		r.logger.Error("Record: raft write failed", "error", applyErr, "accountID", req.AccountID)
		return fmt.Errorf("failed to record prompt request: %s", applyErr.Message)
	}
	return nil
}

func (r *promptRequestRepo) applyFilters(b sq.SelectBuilder, f PromptRequestFilter) sq.SelectBuilder {
	b = b.Where(fmt.Sprintf("%s = ?", ColAccountID), f.AccountID)
	if f.StartDate != nil {
		b = b.Where(fmt.Sprintf("datetime(%s) >= datetime(?)", ColDateCreated), f.StartDate.Format(time.RFC3339))
	}
	if f.EndDate != nil {
		b = b.Where(fmt.Sprintf("datetime(%s) <= datetime(?)", ColDateCreated), f.EndDate.Format(time.RFC3339))
	}
	if f.Provider != "" {
		b = b.Where(fmt.Sprintf("%s = ?", ColProvider), f.Provider)
	}
	if f.Model != "" {
		b = b.Where(fmt.Sprintf("%s = ?", ColModel), f.Model)
	}
	if f.Status != "" {
		b = b.Where(fmt.Sprintf("%s = ?", ColStatus), f.Status)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		b = b.Where(fmt.Sprintf("(%s LIKE ? OR %s LIKE ?)", ColPrompt, ColOutput), like, like)
	}
	return b
}

// GetPromptRequestsFiltered returns the account's prompt requests matching the filter,
// ordered by date_created and paginated.
func (r *promptRequestRepo) GetPromptRequestsFiltered(f PromptRequestFilter) ([]models.AccountPromptRequest, error) {
	r.fsmStore.GetDataStore().ConnectionLock()
	defer r.fsmStore.GetDataStore().ConnectionUnlock()

	direction := "DESC"
	if f.OrderDirection == constants.OrderDirectionAsc {
		direction = "ASC"
	}

	builder := sq.Select(
		ColID, ColAccountID, ColPrompt, ColProvider, ColModel, ColOutput,
		ColInputTokens, ColOutputTokens, ColTotalTokens,
		ColDurationMs, ColEstimatedCost, ColStatus, ColError, ColDateCreated,
	).From(TableName).RunWith(r.fsmStore.GetDataStore().GetOpenConnection())

	builder = r.applyFilters(builder, f)
	builder = builder.OrderBy(fmt.Sprintf("%s %s", ColDateCreated, direction))
	if f.Limit > 0 {
		builder = builder.Limit(f.Limit)
	}
	if f.Offset > 0 {
		builder = builder.Offset(f.Offset)
	}

	rows, err := builder.Query()
	if err != nil {
		r.logger.Error("GetPromptRequestsFiltered: query failed", "error", err, "accountID", f.AccountID)
		return nil, err
	}
	defer rows.Close()

	results := []models.AccountPromptRequest{}
	for rows.Next() {
		var pr models.AccountPromptRequest
		if scanErr := rows.Scan(
			&pr.ID, &pr.AccountID, &pr.Prompt, &pr.Provider, &pr.Model, &pr.Output,
			&pr.InputTokens, &pr.OutputTokens, &pr.TotalTokens,
			&pr.DurationMs, &pr.EstimatedCostUSD, &pr.Status, &pr.Error, &pr.DateCreated,
		); scanErr != nil {
			r.logger.Error("GetPromptRequestsFiltered: scan failed", "error", scanErr, "accountID", f.AccountID)
			return nil, scanErr
		}
		results = append(results, pr)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return results, nil
}

// CountPromptRequests returns the total number of rows matching the filter (ignoring pagination),
// for building pager controls.
func (r *promptRequestRepo) CountPromptRequests(f PromptRequestFilter) (uint64, error) {
	r.fsmStore.GetDataStore().ConnectionLock()
	defer r.fsmStore.GetDataStore().ConnectionUnlock()

	builder := sq.Select("COUNT(*)").From(TableName).
		RunWith(r.fsmStore.GetDataStore().GetOpenConnection())
	builder = r.applyFilters(builder, f)

	var count uint64
	if err := builder.QueryRow().Scan(&count); err != nil {
		r.logger.Error("CountPromptRequests: query failed", "error", err, "accountID", f.AccountID)
		return 0, err
	}
	return count, nil
}

// SumEstimatedCostUSD returns the sum of estimated_cost_usd for rows matching the filter
// (ignoring pagination). Empty matches yield 0.
func (r *promptRequestRepo) SumEstimatedCostUSD(f PromptRequestFilter) (float64, error) {
	r.fsmStore.GetDataStore().ConnectionLock()
	defer r.fsmStore.GetDataStore().ConnectionUnlock()

	builder := sq.Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", ColEstimatedCost)).From(TableName).
		RunWith(r.fsmStore.GetDataStore().GetOpenConnection())
	builder = r.applyFilters(builder, f)

	var sum float64
	if err := builder.QueryRow().Scan(&sum); err != nil {
		r.logger.Error("SumEstimatedCostUSD: query failed", "error", err, "accountID", f.AccountID)
		return 0, err
	}
	return sum, nil
}
