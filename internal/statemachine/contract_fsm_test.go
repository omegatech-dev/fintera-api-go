package statemachine

import (
	"context"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
)

// contractStatuses is the full set of contract states. Tests use it to prove
// that transitions are rejected for every state outside the documented sources.
var contractStatuses = []string{
	models.ContractStatusPending,
	models.ContractStatusSubmitted,
	models.ContractStatusApproved,
	models.ContractStatusSigned,
	models.ContractStatusRejected,
	models.ContractStatusCancelled,
	models.ContractStatusClosed,
}

func contractStatusSet(statuses ...string) map[string]bool {
	set := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		set[status] = true
	}
	return set
}

func TestContractFSM_Approve(t *testing.T) {
	ctx := context.Background()
	allowed := contractStatusSet(
		models.ContractStatusPending,
		models.ContractStatusSubmitted,
		models.ContractStatusRejected,
	)

	for _, status := range contractStatuses {
		t.Run("from "+status, func(t *testing.T) {
			contract := &models.Contract{Status: status}

			err := NewContractFSM(contract).Approve(ctx)

			if allowed[status] {
				assert.NoError(t, err)
				assert.Equal(t, models.ContractStatusApproved, contract.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, contract.Status, "status must not change on a rejected transition")
		})
	}
}

func TestContractFSM_Reject(t *testing.T) {
	ctx := context.Background()
	allowed := contractStatusSet(
		models.ContractStatusPending,
		models.ContractStatusSubmitted,
	)

	for _, status := range contractStatuses {
		t.Run("from "+status, func(t *testing.T) {
			contract := &models.Contract{Status: status}

			err := NewContractFSM(contract).Reject(ctx)

			if allowed[status] {
				assert.NoError(t, err)
				assert.Equal(t, models.ContractStatusRejected, contract.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, contract.Status, "status must not change on a rejected transition")
		})
	}
}

func TestContractFSM_Cancel(t *testing.T) {
	ctx := context.Background()
	allowed := contractStatusSet(
		models.ContractStatusPending,
		models.ContractStatusSubmitted,
		models.ContractStatusRejected,
	)

	for _, status := range contractStatuses {
		t.Run("from "+status, func(t *testing.T) {
			contract := &models.Contract{Status: status}

			err := NewContractFSM(contract).Cancel(ctx)

			if allowed[status] {
				assert.NoError(t, err)
				assert.Equal(t, models.ContractStatusCancelled, contract.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, contract.Status, "status must not change on a rejected transition")
		})
	}
}

func TestContractFSM_Submit(t *testing.T) {
	ctx := context.Background()
	allowed := contractStatusSet(
		models.ContractStatusPending,
		models.ContractStatusRejected,
	)

	for _, status := range contractStatuses {
		t.Run("from "+status, func(t *testing.T) {
			contract := &models.Contract{Status: status}

			err := NewContractFSM(contract).Submit(ctx)

			if allowed[status] {
				assert.NoError(t, err)
				assert.Equal(t, models.ContractStatusSubmitted, contract.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, contract.Status, "status must not change on a rejected transition")
		})
	}
}

func TestContractFSM_Reopen(t *testing.T) {
	ctx := context.Background()

	for _, status := range contractStatuses {
		t.Run("from "+status, func(t *testing.T) {
			contract := &models.Contract{Status: status}

			err := NewContractFSM(contract).Reopen(ctx)

			if status == models.ContractStatusClosed {
				assert.NoError(t, err)
				assert.Equal(t, models.ContractStatusApproved, contract.Status)
				return
			}
			assert.Error(t, err)
			assert.Equal(t, status, contract.Status, "status must not change on a rejected transition")
		})
	}
}

func TestContractFSM_Close(t *testing.T) {
	ctx := context.Background()
	zero := 0.0
	negative := -500.0
	positive := 500.0

	tests := []struct {
		name    string
		status  string
		balance *float64
		wantErr bool
	}{
		{name: "approved settled", status: models.ContractStatusApproved, balance: &zero},
		{name: "approved overpaid", status: models.ContractStatusApproved, balance: &negative},
		{name: "approved with outstanding balance", status: models.ContractStatusApproved, balance: &positive, wantErr: true},
		{name: "approved without cached balance", status: models.ContractStatusApproved, balance: nil, wantErr: true},
		{name: "pending", status: models.ContractStatusPending, balance: &zero, wantErr: true},
		{name: "submitted", status: models.ContractStatusSubmitted, balance: &zero, wantErr: true},
		{name: "rejected", status: models.ContractStatusRejected, balance: &zero, wantErr: true},
		{name: "cancelled", status: models.ContractStatusCancelled, balance: &zero, wantErr: true},
		{name: "already closed", status: models.ContractStatusClosed, balance: &zero, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			contract := &models.Contract{Status: tc.status, Balance: tc.balance}

			err := NewContractFSM(contract).Close(ctx)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.status, contract.Status, "status must not change on a rejected transition")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, models.ContractStatusClosed, contract.Status)
		})
	}
}
