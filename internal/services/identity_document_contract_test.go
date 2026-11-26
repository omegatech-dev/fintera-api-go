package services

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// identityCreationFixture wires a ContractService with the identity document
// service and the mocks a creation touches.
type identityCreationFixture struct {
	service     *ContractService
	identity    *fakeIdentityRepo
	contracts   *mockContractRepo
	lots        *mockLotRepo
	users       *mockUserRepoForContract
	lot         *models.Lot
	applicantID uint
}

func newIdentityCreationFixture(t *testing.T, required bool) *identityCreationFixture {
	t.Helper()
	logger.Setup("test")
	ctx := context.Background()

	contractRepo := new(mockContractRepo)
	lotRepo := new(mockLotRepo)
	userRepo := new(mockUserRepoForContract)
	paymentRepo := new(mockPaymentRepo)
	ledgerRepo := new(mockLedgerRepo)

	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1),
	)

	auditSvc := NewAuditService(gdb)
	worker := jobs.NewWorker(1)
	t.Cleanup(func() { _ = worker.Shutdown(context.Background()) })

	service := NewContractService(
		contractRepo,
		lotRepo,
		userRepo,
		paymentRepo,
		ledgerRepo,
		NewNotificationService(nil, nil),
		NewEmailService(&config.Config{ResendAPIKey: "mock-key"}),
		auditSvc,
		worker,
	)

	identityRepo := newFakeIdentityRepo()
	service.SetIdentityDocumentService(NewIdentityDocumentService(
		identityRepo, &fakeContractRepo{}, newIdentityTestStorage(t), nil, required,
	))

	lot := &models.Lot{
		ID:     1,
		Status: models.LotStatusAvailable,
		Price:  10000,
		Project: models.Project{
			Name: "Test Project",
		},
	}
	lotRepo.On("FindByID", ctx, uint(1)).Return(lot, nil)
	lotRepo.On("ReserveIfAvailable", ctx, uint(1)).Return(true, nil)
	userRepo.On("FindByID", ctx, uint(2)).Return(&models.User{ID: 2, FullName: "Test User"}, nil)

	return &identityCreationFixture{
		service:     service,
		identity:    identityRepo,
		contracts:   contractRepo,
		lots:        lotRepo,
		users:       userRepo,
		lot:         lot,
		applicantID: 2,
	}
}

func (f *identityCreationFixture) contract() *models.Contract {
	return &models.Contract{LotID: 1, ApplicantUserID: f.applicantID}
}

func TestCreateWithIdentityDocumentsRejectsAMissingDocumentBeforeWritingAnything(t *testing.T) {
	fixture := newIdentityCreationFixture(t, true)
	fixture.contracts.On("Create", mock.Anything, mock.AnythingOfType("*models.Contract")).Return(nil)

	_, err := fixture.service.CreateWithIdentityDocuments(
		context.Background(), fixture.contract(), ContractAssignmentInput{}, nil, nil,
	)
	require.Error(t, err)

	var required ErrIdentityDocumentsRequired
	require.True(t, errors.As(err, &required))
	assert.Equal(t, []string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN}, required.Missing)
	assert.Empty(t, fixture.identity.docs, "no document is stored by a rejected request")
	fixture.contracts.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateWithIdentityDocumentsRejectsAnUnsupportedFileNamingIt(t *testing.T) {
	fixture := newIdentityCreationFixture(t, false)
	fixture.contracts.On("Create", mock.Anything, mock.AnythingOfType("*models.Contract")).Return(nil)

	_, err := fixture.service.CreateWithIdentityDocuments(
		context.Background(), fixture.contract(), ContractAssignmentInput{},
		[]IdentityDocumentUpload{
			{Kind: models.IdentityDocumentKindDNI, Header: newUploadFile(t, "dni.zip", "application/zip", []byte("PK\x03\x04"))},
		}, nil,
	)
	require.Error(t, err)

	var typeErr *ErrIdentityDocumentType
	require.True(t, errors.As(err, &typeErr))
	assert.Equal(t, "dni.zip", typeErr.Filename)
	assert.Empty(t, fixture.identity.docs)
	fixture.contracts.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateWithIdentityDocumentsStoresBothDocumentsAndLinksThem(t *testing.T) {
	fixture := newIdentityCreationFixture(t, true)
	fixture.contracts.On("Create", mock.Anything, mock.AnythingOfType("*models.Contract")).Return(nil)

	result, err := fixture.service.CreateWithIdentityDocuments(
		context.Background(), fixture.contract(), ContractAssignmentInput{},
		[]IdentityDocumentUpload{
			{Kind: models.IdentityDocumentKindDNI, Header: newUploadFile(t, "dni.png", "image/png", tinyPNG(t))},
			{Kind: models.IdentityDocumentKindRTN, Header: newUploadFile(t, "rtn.png", "image/png", tinyPNG(t))},
		}, nil,
	)
	require.NoError(t, err)

	assert.ElementsMatch(t,
		[]string{models.IdentityDocumentKindDNI, models.IdentityDocumentKindRTN},
		result.Filled,
	)
	assert.Empty(t, result.AlreadyPresent)

	require.Len(t, fixture.contracts.identityLinks, 2, "both documents are linked to the contract")
	for _, link := range fixture.contracts.identityLinks {
		require.NotNil(t, link.IdentityDocumentID)
		require.NotNil(t, link.Kind)
		assert.Equal(t, models.IdentityDocumentSourceUpload, link.Source)
		assert.Equal(t, uint(1), link.ContractID, "the link carries the contract id")
	}
}

func TestCreateWithIdentityDocumentsKeepsTheStoredDocumentAndReportsIt(t *testing.T) {
	fixture := newIdentityCreationFixture(t, true)
	fixture.identity.docs[docKey(2, models.IdentityDocumentKindDNI)] = &models.IdentityDocument{
		ID: 21, UserID: 2, Kind: models.IdentityDocumentKindDNI,
		FilePath: "identity_documents/2026/01/viejo.png", OriginalFilename: "viejo.png",
	}
	fixture.contracts.On("Create", mock.Anything, mock.AnythingOfType("*models.Contract")).Return(nil)

	result, err := fixture.service.CreateWithIdentityDocuments(
		context.Background(), fixture.contract(), ContractAssignmentInput{},
		[]IdentityDocumentUpload{
			{Kind: models.IdentityDocumentKindDNI, Header: newUploadFile(t, "nuevo.png", "image/png", tinyPNG(t))},
			{Kind: models.IdentityDocumentKindRTN, Header: newUploadFile(t, "rtn.png", "image/png", tinyPNG(t))},
		}, nil,
	)
	require.NoError(t, err)

	assert.Equal(t, []string{models.IdentityDocumentKindRTN}, result.Filled)
	assert.Equal(t, []string{models.IdentityDocumentKindDNI}, result.AlreadyPresent)
	assert.Equal(t, "viejo.png", fixture.identity.docs[docKey(2, models.IdentityDocumentKindDNI)].OriginalFilename)
}

func TestCreateWithIdentityDocumentsWithoutTheServiceStillCreatesTheContract(t *testing.T) {
	fixture := newIdentityCreationFixture(t, false)
	fixture.service.SetIdentityDocumentService(nil)
	fixture.contracts.On("Create", mock.Anything, mock.AnythingOfType("*models.Contract")).Return(nil)

	_, err := fixture.service.CreateWithIdentityDocuments(
		context.Background(), fixture.contract(), ContractAssignmentInput{}, nil, nil,
	)
	require.NoError(t, err)
	fixture.contracts.AssertExpectations(t)
}
