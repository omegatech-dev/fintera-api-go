package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCapitalRepaymentRequest_resolvedAmount(t *testing.T) {
	tests := []struct {
		name     string
		request  CapitalRepaymentRequest
		expected float64
	}{
		{
			name:     "documented field",
			request:  CapitalRepaymentRequest{CapitalRepaymentAmount: 1500},
			expected: 1500,
		},
		{
			name:     "legacy amount field",
			request:  CapitalRepaymentRequest{Amount: 900},
			expected: 900,
		},
		{
			name:     "documented field wins over legacy alias",
			request:  CapitalRepaymentRequest{CapitalRepaymentAmount: 1200, Amount: 800},
			expected: 1200,
		},
		{
			name:     "missing amount",
			request:  CapitalRepaymentRequest{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.request.resolvedAmount())
		})
	}
}

func TestCapitalRepaymentRejectsNonPositiveAmounts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// The service is intentionally nil: validation happens before it is used.
	handler := &ContractHandler{}

	bodies := map[string]string{
		"missing amount":      `{}`,
		"zero documented":     `{"capital_repayment_amount":0}`,
		"negative documented": `{"capital_repayment_amount":-50}`,
		"negative legacy":     `{"amount":-10}`,
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "/capital_repayment", bytes.NewBufferString(body))
			context.Request.Header.Set("Content-Type", "application/json")
			context.Params = gin.Params{{Key: "contract_id", Value: "7"}}

			handler.CapitalRepayment(context)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "mayor a 0")
		})
	}
}
