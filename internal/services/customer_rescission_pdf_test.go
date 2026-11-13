package services

import (
	"bytes"
	"context"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareCustomerRecordPDFData(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	contract := contractPDFTestContract()
	basePrice := 1000000.0
	overridePrice := 950000.0
	contract.Lot.Price = basePrice
	contract.Lot.OverridePrice = &overridePrice

	data := service.prepareCustomerRecordPDFData(context.Background(), contract)

	assert.Equal(t, "#101", data.Reference)
	assert.Equal(t, "José Ramón Núñez", data.Client.Name)
	assert.Equal(t, "Residencial Los Álamos", data.Property.ProjectName)
	assert.Equal(t, "Directo", data.FinancingType)
	assert.True(t, data.HasOverride)
	assert.Equal(t, "L 950,000.00", data.Price)
	assert.Equal(t, "L 1,000,000.00", data.BasePrice)
	assert.Equal(t, "L 950,000.00", data.OverridePrice)
	assert.Equal(t, "L 5,000.00", data.Reserve)
	assert.Equal(t, "L 120,000.00", data.DownPayment)
	assert.Equal(t, "L 11,358.00", data.Installment)
	assert.Equal(t, "84 meses", data.Term)
}

func TestBuildCustomerRecordPDFGeneratesValidPDF(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	data := service.prepareCustomerRecordPDFData(context.Background(), contractPDFTestContract())

	buf, err := buildCustomerRecordPDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)

	content := buf.Bytes()
	assert.True(t, bytes.HasPrefix(content, []byte("%PDF-")))
	assert.Greater(t, len(content), 1024)
	assert.GreaterOrEqual(t, reportPDFPageCount(content), 1)
}

func TestPrepareRescissionPDFData(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	contract := contractPDFTestContract()

	data := service.prepareRescissionPDFData(context.Background(), contract, 5000, 1000)

	assert.Equal(t, "#101", data.Reference)
	assert.Equal(t, "José Ramón Núñez", data.Client.Name)
	assert.Equal(t, "Residencial Los Álamos", data.Property.ProjectName)
	assert.Equal(t, "Lote 5", data.Property.LotName)
	assert.Equal(t, "L 5,000.00", data.RefundAmount)
	assert.Equal(t, "L 1,000.00", data.PenaltyAmount)
	assert.NotEmpty(t, data.Month)
	assert.NotEmpty(t, data.Year)
}

func TestBuildRescissionPDFGeneratesValidPDF(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	data := service.prepareRescissionPDFData(context.Background(), contractPDFTestContract(), 5000, 1000)

	buf, err := buildRescissionPDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)

	content := buf.Bytes()
	assert.True(t, bytes.HasPrefix(content, []byte("%PDF-")))
	assert.Greater(t, len(content), 1024)
	assert.GreaterOrEqual(t, reportPDFPageCount(content), 1)
}

func TestBuildCustomerRecordPDFHandlesEmptyContract(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)
	data := service.prepareCustomerRecordPDFData(context.Background(), &models.Contract{})

	buf, err := buildCustomerRecordPDF(data)
	require.NoError(t, err)
	require.NotNil(t, buf)
	assert.True(t, bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")))
	assert.Greater(t, buf.Len(), 1024)
}
