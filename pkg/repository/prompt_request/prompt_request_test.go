package prompt_request

import (
	"strings"
	"testing"

	sq "github.com/Masterminds/squirrel"
)

// applyFilters does not touch the Raft store, so a zero-value repo is sufficient to exercise
// the query-building logic that backs GetPromptRequestsFiltered.
func TestApplyFilters_BuildsExpectedSQL(t *testing.T) {
	repo := &promptRequestRepo{}

	base := sq.Select("*").From(TableName)
	built := repo.applyFilters(base, PromptRequestFilter{
		AccountID: 42,
		Provider:  "openai",
		Model:     "gpt-4.1-mini",
		Status:    "success",
		Search:    "digest",
	})

	sqlStr, args, err := built.ToSql()
	if err != nil {
		t.Fatalf("ToSql failed: %v", err)
	}

	for _, want := range []string{"account_id = ?", "provider = ?", "model = ?", "status = ?", "prompt LIKE ?", "output LIKE ?"} {
		if !strings.Contains(sqlStr, want) {
			t.Errorf("expected SQL to contain %q, got: %s", want, sqlStr)
		}
	}

	// account_id + provider + model + status + (prompt LIKE, output LIKE) = 6 bound args.
	if len(args) != 6 {
		t.Errorf("expected 6 args, got %d: %v", len(args), args)
	}

	// The search term must be wrapped for a substring match.
	foundLike := false
	for _, a := range args {
		if a == "%digest%" {
			foundLike = true
		}
	}
	if !foundLike {
		t.Errorf("expected a %%digest%% LIKE argument, got: %v", args)
	}
}

func TestApplyFilters_AccountOnly(t *testing.T) {
	repo := &promptRequestRepo{}
	sqlStr, args, err := repo.applyFilters(sq.Select("*").From(TableName), PromptRequestFilter{AccountID: 7}).ToSql()
	if err != nil {
		t.Fatalf("ToSql failed: %v", err)
	}
	if strings.Contains(sqlStr, "provider = ?") || strings.Contains(sqlStr, "LIKE") {
		t.Errorf("account-only filter should not add optional predicates: %s", sqlStr)
	}
	if len(args) != 1 || args[0] != uint64(7) {
		t.Errorf("expected single account_id arg, got: %v", args)
	}
}
