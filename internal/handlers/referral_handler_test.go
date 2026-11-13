package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
)

type referralVisitRepoStub struct {
	repository.ReferralRepository
	visits []models.ReferralVisit
}

func (r *referralVisitRepoStub) RecordVisit(_ context.Context, visit *models.ReferralVisit) error {
	r.visits = append(r.visits, *visit)
	return nil
}

type referralSellerRepoStub struct {
	repository.SellerRepository
	seller *models.User
}

func (r *referralSellerRepoStub) FindActiveByReferralGUID(_ context.Context, _ string) (*models.User, error) {
	if r.seller == nil {
		return nil, errors.New("seller not found")
	}
	return r.seller, nil
}

func newReferralHandlerForTest(seller *models.User) (*ReferralHandler, *referralVisitRepoStub) {
	visits := &referralVisitRepoStub{}
	service := services.NewReferralService(visits, &referralSellerRepoStub{seller: seller}, &config.Config{})
	return NewReferralHandler(service), visits
}

func TestReferralHandlerVisit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	guid := "8f14e45f-ceea-4a67-9c0f-1f2d3e4a5b6c"
	seller := &models.User{ID: 3, Role: models.RoleSeller, Status: models.StatusActive, ReferralGUID: &guid}
	handler, visits := newReferralHandlerForTest(seller)

	router := gin.New()
	router.POST("/api/v1/public/referrals/:guid/visit", handler.Visit)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/public/referrals/"+guid+"/visit", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if len(visits.visits) != 1 {
		t.Fatalf("visits = %d", len(visits.visits))
	}
	if visits.visits[0].SellerID != seller.ID || visits.visits[0].ReferralGUID != guid {
		t.Fatalf("visit = %+v", visits.visits[0])
	}
}

func TestReferralHandlerVisitUnknownGUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, visits := newReferralHandlerForTest(nil)

	router := gin.New()
	router.POST("/api/v1/public/referrals/:guid/visit", handler.Visit)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/public/referrals/missing/visit", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if len(visits.visits) != 0 {
		t.Fatal("unknown GUID must not record a visit")
	}
}

func TestReferralHandlerRedirectUsesGUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	guid := "8f14e45f-ceea-4a67-9c0f-1f2d3e4a5b6c"
	seller := &models.User{ID: 3, Role: models.RoleSeller, Status: models.StatusActive, ReferralGUID: &guid}
	handler, visits := newReferralHandlerForTest(seller)

	router := gin.New()
	router.GET("/r/:guid", handler.Redirect)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/r/"+guid, nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", recorder.Code)
	}
	location := recorder.Header().Get("Location")
	if !strings.Contains(location, "ref="+guid) {
		t.Fatalf("location = %q", location)
	}
	if len(visits.visits) != 1 {
		t.Fatalf("visits = %d", len(visits.visits))
	}
	if len(recorder.Result().Cookies()) == 0 {
		t.Fatal("expected an attribution cookie")
	}
}
