package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeHistoricalSaleRepository struct {
	repository.HistoricalSaleRepository
	sales  []models.HistoricalSale
	nextID uint
}

func newFakeHistoricalSaleRepository() *fakeHistoricalSaleRepository {
	return &fakeHistoricalSaleRepository{nextID: 1}
}

func (r *fakeHistoricalSaleRepository) Create(_ context.Context, sale *models.HistoricalSale) error {
	for _, existing := range r.sales {
		if existing.CreatedBy == sale.CreatedBy &&
			stringValue(existing.ClientIdentity) == stringValue(sale.ClientIdentity) &&
			existing.SaleDate.Equal(sale.SaleDate) {
			return fmt.Errorf("duplicate key value violates unique constraint")
		}
	}
	sale.ID = r.nextID
	r.nextID++
	if sale.CreatedAt.IsZero() {
		sale.CreatedAt = time.Now()
	}
	r.sales = append(r.sales, *sale)
	return nil
}

func (r *fakeHistoricalSaleRepository) Update(_ context.Context, sale *models.HistoricalSale) error {
	for i := range r.sales {
		if r.sales[i].ID == sale.ID {
			r.sales[i] = *sale
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeHistoricalSaleRepository) FindByID(_ context.Context, id uint) (*models.HistoricalSale, error) {
	for i := range r.sales {
		if r.sales[i].ID == id {
			copy := r.sales[i]
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeHistoricalSaleRepository) FindByIDWithDetails(ctx context.Context, id uint) (*models.HistoricalSale, error) {
	return r.FindByID(ctx, id)
}

func (r *fakeHistoricalSaleRepository) List(_ context.Context, query *repository.HistoricalSaleQuery) ([]models.HistoricalSale, int64, error) {
	result := make([]models.HistoricalSale, 0, len(r.sales))
	for _, sale := range r.sales {
		if query.CreatorID > 0 && sale.CreatedBy != query.CreatorID {
			continue
		}
		if query.ValidationStatus != "" && query.ValidationStatus != "all" && sale.ValidationStatus != query.ValidationStatus {
			continue
		}
		result = append(result, sale)
	}
	return result, int64(len(result)), nil
}

func (r *fakeHistoricalSaleRepository) ListForReport(_ context.Context, creatorID *uint, startDate, endDate *time.Time) ([]models.HistoricalSale, error) {
	result := make([]models.HistoricalSale, 0, len(r.sales))
	for _, sale := range r.sales {
		if sale.ValidationStatus == models.HistoricalSaleStatusRejected {
			continue
		}
		if creatorID != nil && sale.CreatedBy != *creatorID {
			continue
		}
		if startDate != nil && sale.SaleDate.Before(*startDate) {
			continue
		}
		if endDate != nil && sale.SaleDate.After(*endDate) {
			continue
		}
		result = append(result, sale)
	}
	return result, nil
}

func (r *fakeHistoricalSaleRepository) ExistsByIdentity(_ context.Context, creatorID uint, identity string, saleDate time.Time) (bool, error) {
	for _, sale := range r.sales {
		if sale.CreatedBy == creatorID && stringValue(sale.ClientIdentity) == identity && sale.SaleDate.Equal(saleDate) {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeHistoricalSaleRepository) UpdateValidation(_ context.Context, id uint, status string, validatedBy uint, at time.Time, reason *string) error {
	for i := range r.sales {
		if r.sales[i].ID != id {
			continue
		}
		r.sales[i].ValidationStatus = status
		r.sales[i].ValidatedBy = &validatedBy
		r.sales[i].ValidatedAt = &at
		r.sales[i].RejectionReason = reason
		return nil
	}
	return gorm.ErrRecordNotFound
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestHistoricalSaleCreateIsPendingAndUsesSaleDate(t *testing.T) {
	repo := newFakeHistoricalSaleRepository()
	service := NewHistoricalSaleService(repo, nil, nil)
	ctx := context.Background()

	input := HistoricalSaleInput{
		FullName:    stringPtrForTest("María López"),
		Identity:    stringPtrForTest("0801-1990-12345"),
		SalePrice:   floatPtrForTest(100000),
		SaleDate:    stringPtrForTest("2023-05-15"),
		ProjectName: stringPtrForTest("Residencial Los Pinos"),
	}
	sale, err := service.Create(ctx, 4, input, "", "")
	require.NoError(t, err)
	assert.Equal(t, models.HistoricalSaleStatusPending, sale.ValidationStatus)
	assert.Equal(t, uint(4), sale.CreatedBy)
	assert.Equal(t, 5.0, *sale.Rate)
	assert.InDelta(t, 5000.0, sale.CommissionAmount, 0.01)

	response := sale.ToResponse()
	assert.Equal(t, models.CommissionOriginHistorical, response.CommissionOrigin)
	assert.Equal(t, "2023-05-15", response.CommissionAccrualDate)
	assert.Equal(t, "María López", response.FullName)

	_, err = service.Create(ctx, 4, input, "", "")
	assert.ErrorIs(t, err, ErrHistoricalSaleDuplicate, "identity and sale date identify a duplicate")
}

func TestHistoricalSaleAdminValidation(t *testing.T) {
	repo := newFakeHistoricalSaleRepository()
	service := NewHistoricalSaleService(repo, nil, nil)
	ctx := context.Background()

	first, err := service.Create(ctx, 4, sampleHistoricalInput("111", "2023-01-10"), "", "")
	require.NoError(t, err)
	validated, err := service.Validate(ctx, first.ID, 1, "", "")
	require.NoError(t, err)
	assert.Equal(t, models.HistoricalSaleStatusValidated, validated.ValidationStatus)
	require.NotNil(t, validated.ValidatedBy)
	assert.Equal(t, uint(1), *validated.ValidatedBy)
	assert.NotNil(t, validated.ValidatedAt)

	second, err := service.Create(ctx, 4, sampleHistoricalInput("222", "2023-02-10"), "", "")
	require.NoError(t, err)
	reason := "sin respaldo"
	rejected, err := service.Reject(ctx, second.ID, 1, reason, "", "")
	require.NoError(t, err)
	assert.Equal(t, models.HistoricalSaleStatusRejected, rejected.ValidationStatus)
	require.NotNil(t, rejected.RejectionReason)
	assert.Equal(t, reason, *rejected.RejectionReason)
}

func TestHistoricalSaleImportCSVSummary(t *testing.T) {
	repo := newFakeHistoricalSaleRepository()
	existing := models.HistoricalSale{
		ClientName:       "Existente",
		ClientIdentity:   stringPtrForTest("111"),
		SalePrice:        1000,
		SaleDate:         mustDate(t, "2023-01-01"),
		ValidationStatus: models.HistoricalSaleStatusPending,
		CreatedBy:        4,
	}
	require.NoError(t, repo.Create(context.Background(), &existing))

	service := NewHistoricalSaleService(repo, nil, nil)
	csv := strings.NewReader(`nombre,identidad,precio,fecha_venta,proyecto
Juan Pérez,222,100000,2023-02-01,Proyecto A
Duplicado,111,50000,2023-01-01,Proyecto B
Sin Precio,333,abc,2023-03-01,Proyecto C
Sin Fecha,444,1000,not-a-date,Proyecto D
`)

	result, err := service.ImportCSV(context.Background(), 4, csv, HistoricalImportOptions{
		MarkHistorical:         true,
		SkipDuplicates:         true,
		CommissionWithSaleDate: true,
	}, "", "")
	require.NoError(t, err)
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 1, result.Valid)
	assert.Equal(t, 1, result.Imported)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Reason, "precio")
	assert.Contains(t, result.Errors[1].Reason, "fecha")

	imported := repo.sales[len(repo.sales)-1]
	assert.Equal(t, "Juan Pérez", imported.ClientName)
	assert.Equal(t, models.HistoricalSaleStatusPending, imported.ValidationStatus)
	assert.InDelta(t, 5000.0, imported.CommissionAmount, 0.01, "the commission uses the default rate and sale date")
}

func TestHistoricalSaleImportWithoutSkipDuplicatesReportsError(t *testing.T) {
	repo := newFakeHistoricalSaleRepository()
	existing := models.HistoricalSale{
		ClientName:       "Existente",
		ClientIdentity:   stringPtrForTest("111"),
		SalePrice:        1000,
		SaleDate:         mustDate(t, "2023-01-01"),
		ValidationStatus: models.HistoricalSaleStatusPending,
		CreatedBy:        4,
	}
	require.NoError(t, repo.Create(context.Background(), &existing))

	service := NewHistoricalSaleService(repo, nil, nil)
	csv := strings.NewReader("nombre,identidad,precio,fecha_venta\nDuplicado,111,50000,2023-01-01\n")
	result, err := service.ImportCSV(context.Background(), 4, csv, HistoricalImportOptions{
		SkipDuplicates:         false,
		CommissionWithSaleDate: true,
	}, "", "")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 0, result.Valid)
	assert.Equal(t, 0, result.Skipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Reason, "ya registradas")
}

func TestHistoricalSaleImportAcceptsColumnMapping(t *testing.T) {
	repo := newFakeHistoricalSaleRepository()
	service := NewHistoricalSaleService(repo, nil, nil)
	csv := strings.NewReader("Cliente,Documento,Monto,Fecha\nAna,999,250000,2024-04-01\n")

	result, err := service.ImportCSV(context.Background(), 4, csv, HistoricalImportOptions{
		SkipDuplicates:         true,
		CommissionWithSaleDate: true,
		Mapping: map[string]string{
			"full_name":  "0",
			"identity":   "1",
			"sale_price": "2",
			"sale_date":  "3",
		},
	}, "", "")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Valid)
	require.Len(t, repo.sales, 1)
	assert.Equal(t, "Ana", repo.sales[0].ClientName)
	assert.Equal(t, "2024-04-01", repo.sales[0].SaleDate.Format(models.HistoricalSaleDateLayout))
}

func TestHistoricalSaleImportWithoutCommissionLeavesZero(t *testing.T) {
	repo := newFakeHistoricalSaleRepository()
	service := NewHistoricalSaleService(repo, nil, nil)
	csv := strings.NewReader("nombre,identidad,precio,fecha_venta\nLuis,777,100000,2023-06-01\n")
	result, err := service.ImportCSV(context.Background(), 4, csv, HistoricalImportOptions{
		SkipDuplicates:         true,
		CommissionWithSaleDate: false,
	}, "", "")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Valid)
	assert.Equal(t, 0.0, repo.sales[0].CommissionAmount)
}

func sampleHistoricalInput(identity, saleDate string) HistoricalSaleInput {
	return HistoricalSaleInput{
		FullName:  stringPtrForTest("Venta " + identity),
		Identity:  stringPtrForTest(identity),
		SalePrice: floatPtrForTest(100000),
		SaleDate:  stringPtrForTest(saleDate),
	}
}

func stringPtrForTest(value string) *string  { return &value }
func floatPtrForTest(value float64) *float64 { return &value }

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation(models.HistoricalSaleDateLayout, value, time.UTC)
	require.NoError(t, err)
	return parsed
}
