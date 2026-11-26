package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestIdentityDocumentRoutesRegister guards the Gin wildcard constraint of the
// contract-scoped identity routes: they must live in the /contracts/:guid
// tree. Registering /contracts/:contract_id/... next to /contracts/:guid
// panics at startup, so this test fails loudly instead of the router.
func TestIdentityDocumentRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &IdentityDocumentHandler{}
	router := gin.New()
	v1 := router.Group("/api/v1")

	inert := func(c *gin.Context) {}
	// The routes the identity documents attach to.
	v1.GET("/contracts/:guid", inert)
	v1.POST("/contracts/:guid/prepayments", inert)
	v1.PATCH("/contracts/:guid/commission/pay", inert)
	v1.GET("/contracts", inert)
	v1.GET("/contracts/stats", inert)
	v1.GET("/users/:user_id", inert)

	v1.GET("/identity_documents/:id/content", handler.Content)
	v1.GET("/identity_documents/:id/preview", handler.Preview)
	v1.GET("/users/:user_id/identity_documents", handler.ListByUser)
	v1.POST("/users/:user_id/identity_documents", handler.UploadForUser)
	v1.GET("/contracts/:guid/identity_documents", handler.ListByContract)
	v1.POST("/contracts/:guid/identity_documents", handler.UploadForContract)
	v1.GET("/contracts/:guid/legacy_documents/:document_id/content", handler.LegacyContent)

	assert.NotNil(t, router)
}

// TestIdentityDocumentRoutesCannotBeRegisteredTwice documents why the person
// listing lives in the sellerAdmin group only: the admin and sellerAdmin
// groups share one tree, so registering the same method and path twice panics
// at startup.
func TestIdentityDocumentRoutesCannotBeRegisteredTwice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	inert := func(c *gin.Context) {}
	v1.GET("/users/:user_id/identity_documents", inert)

	assert.Panics(t, func() {
		v1.GET("/users/:user_id/identity_documents", inert)
	})
}

// TestIdentityDocumentsAreNotServedByStaticRoutes pins the storage layout: the
// documents live outside the two directories that are mounted as static files,
// and no static route exposes the storage root.
func TestIdentityDocumentsAreNotServedByStaticRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := t.TempDir()
	router.Static("/uploads", store+"/uploads")
	router.Static("/receipts", store+"/receipts")

	for _, path := range []string{
		"/identity_documents/2026/01/dni.png",
		"/identity_documents/previews/2026/01/preview.webp",
		"/storage/identity_documents/2026/01/dni.png",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code, "static route must not expose %s", path)
	}
}

// TestCollectContractUploadsKeepsTheTypedFieldsApart covers the parsing of a
// creation request: the DNI and the RTN are identity documents, the
// documents[N] fields are free-form attachments.
func TestCollectContractUploadsKeepsTheTypedFieldsApart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = formRequest(t, http.MethodPost, "/",
		map[string]string{"contract[payment_term]": "12"},
		formFile{"identity_document", "dni.png", "image/png", tinyIdentityPNG(t)},
		formFile{"rtn_document", "rtn.png", "image/png", tinyIdentityPNG(t)},
		formFile{"documents[0]", "comprobante.pdf", "application/pdf", []byte("%PDF-1.4")},
	)

	identityUploads, attachments, err := collectContractUploads(c)
	assert.NoError(t, err)
	if assert.Len(t, identityUploads, 2) {
		kinds := []string{identityUploads[0].Kind, identityUploads[1].Kind}
		assert.ElementsMatch(t, []string{"dni", "rtn"}, kinds)
	}
	if assert.Len(t, attachments, 1) {
		assert.Equal(t, "comprobante.pdf", attachments[0].Filename)
	}
}

func TestCollectContractUploadsRejectsAnUnsupportedAttachmentNamingIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = formRequest(t, http.MethodPost, "/", nil,
		formFile{"documents[0]", "notas.zip", "application/zip", []byte("PK\x03\x04")})

	_, _, err := collectContractUploads(c)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "notas.zip"))
}

func TestCollectContractUploadsRejectsAnUnsupportedIdentityFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = formRequest(t, http.MethodPost, "/", nil,
		formFile{"identity_document", "dni.gif", "image/gif", []byte("GIF89a")})

	_, _, err := collectContractUploads(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dni.gif")
}

func TestCollectContractUploadsAcceptsHEICForTheIdentityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = formRequest(t, http.MethodPost, "/", nil,
		formFile{"identity_document", "dni.heic", "image/heic", []byte("\x00\x00\x00\x18ftypheic")})

	identityUploads, _, err := collectContractUploads(c)
	assert.NoError(t, err, "an iPhone HEIC is stored instead of being rejected")
	assert.Len(t, identityUploads, 1)
}
