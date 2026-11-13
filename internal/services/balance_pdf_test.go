package services

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBalancePDFGeneratesValidPDF(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	now := time.Date(2026, 11, 11, 15, 30, 0, 0, time.UTC)
	data := service.prepareBalancePDFData(balanceTestUser(), []*models.Contract{balanceTestContract()}, now)

	buf, err := buildBalancePDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)

	content := buf.Bytes()
	assert.True(t, bytes.HasPrefix(content, []byte("%PDF-")), "output should be a PDF")
	assert.Greater(t, len(content), 1024, "PDF should not be tiny")
	assert.GreaterOrEqual(t, reportPDFPageCount(content), 1, "PDF should contain at least one page")
}

func TestBuildBalancePDFEmptyStatement(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	now := time.Date(2026, 11, 11, 15, 30, 0, 0, time.UTC)
	data := service.prepareBalancePDFData(balanceTestUser(), nil, now)
	require.False(t, data.HasContracts)

	buf, err := buildBalancePDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)
	assert.True(t, bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")))
	assert.Greater(t, buf.Len(), 1024)
}

func TestBuildBalancePDFLongStatementSpansPages(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	now := time.Date(2026, 11, 11, 15, 30, 0, 0, time.UTC)

	contracts := make([]*models.Contract, 0, 12)
	for index := 0; index < 12; index++ {
		contract := balanceTestContract()
		contract.ID = uint(100 + index)
		contract.GUID = fmt.Sprintf("balance-%04d-0000-0000-000000000000", index)
		contracts = append(contracts, contract)
	}

	data := service.prepareBalancePDFData(balanceTestUser(), contracts, now)
	require.Len(t, data.Contracts, 12)

	buf, err := buildBalancePDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)

	content := buf.Bytes()
	assert.True(t, bytes.HasPrefix(content, []byte("%PDF-")))
	assert.Greater(t, reportPDFPageCount(content), 1, "multi-contract statements should span pages")
}
