package token

import (
	"errors"
	"testing"
	"time"
)

func TestTknClaimsValidate(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		claims TknClaims
		want   error
	}{
		{
			name: "valid claims",
			claims: TknClaims{
				Sub:    "spiffe://agentauth.local/agent/orch/task/inst",
				Scope:  []string{"read:Customers:12345"},
				TaskId: "task-1",
				Exp:    now.Add(5 * time.Minute).Unix(),
			},
			want: nil,
		},
		{
			name: "missing subject",
			claims: TknClaims{
				Scope:  []string{"read:Customers:12345"},
				TaskId: "task-1",
				Exp:    now.Add(5 * time.Minute).Unix(),
			},
			want: ErrClaimsSubjectRequired,
		},
		{
			name: "missing scope",
			claims: TknClaims{
				Sub:    "spiffe://agentauth.local/agent/orch/task/inst",
				TaskId: "task-1",
				Exp:    now.Add(5 * time.Minute).Unix(),
			},
			want: ErrClaimsScopeRequired,
		},
		{
			name: "missing task id",
			claims: TknClaims{
				Sub:   "spiffe://agentauth.local/agent/orch/task/inst",
				Scope: []string{"read:Customers:12345"},
				Exp:   now.Add(5 * time.Minute).Unix(),
			},
			want: ErrClaimsTaskIDRequired,
		},
		{
			name: "expired",
			claims: TknClaims{
				Sub:    "spiffe://agentauth.local/agent/orch/task/inst",
				Scope:  []string{"read:Customers:12345"},
				TaskId: "task-1",
				Exp:    now.Add(-1 * time.Minute).Unix(),
			},
			want: ErrClaimsExpired,
		},
		{
			name: "exp missing",
			claims: TknClaims{
				Sub:    "spiffe://agentauth.local/agent/orch/task/inst",
				Scope:  []string{"read:Customers:12345"},
				TaskId: "task-1",
			},
			want: ErrClaimsExpired,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.claims.Validate(now)
			if !errors.Is(err, tc.want) {
				t.Fatalf("want error %v, got %v", tc.want, err)
			}
		})
	}
}

