package classify_request

import (
	"fmt"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/fsm"
	"scheduler0-private/pkg/models"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

const (
	TableName      = "account_classify_requests"
	ColID          = "id"
	ColAccountID   = "account_id"
	ColKind        = "kind"
	ColPrompt      = "prompt"
	ColDecision    = "decision"
	ColStatus      = "status"
	ColError       = "error"
	ColDateCreated = "date_created"
)

// ClassifyRequestFilter narrows and counts the classify-request log for an account.
type ClassifyRequestFilter struct {
	AccountID uint64
	StartDate *time.Time
	EndDate   *time.Time
	Status    string
}

// ClassifyRequestRepo persists and counts the account classify-request log. Usage is derived
// by counting successful rows within the account's current period.
type ClassifyRequestRepo interface {
	Record(req models.AccountClassifyRequest) error
	CountClassifyRequests(filter ClassifyRequestFilter) (uint64, error)
}

type classifyRequestRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	scheduler0RaftActions fsm.Scheduler0RaftActions
	logger                hclog.Logger
}

func NewClassifyRequestRepo(
	logger hclog.Logger,
	scheduler0RaftActions fsm.Scheduler0RaftActions,
	fsmStore fsm.Scheduler0RaftStore,
) ClassifyRequestRepo {
	return &classifyRequestRepo{
		fsmStore:              fsmStore,
		scheduler0RaftActions: scheduler0RaftActions,
		logger:                logger.Named("classify-request-repo"),
	}
}

// Record inserts a classify-request row through the Raft log so it replicates across the cluster.
func (r *classifyRequestRepo) Record(req models.AccountClassifyRequest) error {
	if req.DateCreated.IsZero() {
		req.DateCreated = time.Now().UTC()
	}

	query, params, buildErr := sq.Insert(TableName).
		Columns(ColAccountID, ColKind, ColPrompt, ColDecision, ColStatus, ColError, ColDateCreated).
		Values(req.AccountID, req.Kind, req.Prompt, req.Decision, req.Status, req.Error, req.DateCreated).
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
		return fmt.Errorf("failed to record classify request: %s", applyErr.Message)
	}
	return nil
}

func (r *classifyRequestRepo) applyFilters(b sq.SelectBuilder, f ClassifyRequestFilter) sq.SelectBuilder {
	b = b.Where(fmt.Sprintf("%s = ?", ColAccountID), f.AccountID)
	if f.StartDate != nil {
		b = b.Where(fmt.Sprintf("datetime(%s) >= datetime(?)", ColDateCreated), f.StartDate.Format(time.RFC3339))
	}
	if f.EndDate != nil {
		b = b.Where(fmt.Sprintf("datetime(%s) <= datetime(?)", ColDateCreated), f.EndDate.Format(time.RFC3339))
	}
	if f.Status != "" {
		b = b.Where(fmt.Sprintf("%s = ?", ColStatus), f.Status)
	}
	return b
}

// CountClassifyRequests returns the number of rows matching the filter.
func (r *classifyRequestRepo) CountClassifyRequests(f ClassifyRequestFilter) (uint64, error) {
	r.fsmStore.GetDataStore().ConnectionLock()
	defer r.fsmStore.GetDataStore().ConnectionUnlock()

	builder := sq.Select("COUNT(*)").From(TableName).
		RunWith(r.fsmStore.GetDataStore().GetOpenConnection())
	builder = r.applyFilters(builder, f)

	var count uint64
	if err := builder.QueryRow().Scan(&count); err != nil {
		r.logger.Error("CountClassifyRequests: query failed", "error", err, "accountID", f.AccountID)
		return 0, err
	}
	return count, nil
}
