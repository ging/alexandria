// Package rest_test verifies the OpenAPI router HTTP endpoints and content negotiations.
// It exercises JSON/YAML delivery and Swagger UI rendering across root and versioned routes.
package rest_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ging/alexandria/internal/openapi/rest"
)

// stubJSON returns minimal valid JSON bytes for specification tests.
func stubJSON() []byte {
	return []byte(`{"openapi":"3.0.3","info":{"title":"Test API"}}`)
}

// stubYAML returns minimal valid YAML bytes for specification tests.
func stubYAML() []byte {
	return []byte("openapi: 3.0.3\ninfo:\n  title: Test API\n")
}

func TestRouterHasSpec(t *testing.T) {
	t.Parallel()

	rEmpty := rest.NewRouter(nil, nil)
	if rEmpty.HasSpec() {
		t.Error("HasSpec() on empty router = true, want false")
	}

	rPopulated := rest.NewRouter(stubJSON(), stubYAML())
	if !rPopulated.HasSpec() {
		t.Error("HasSpec() on populated router = false, want true")
	}
}

func TestRouterRegisterRoot(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := rest.NewRouter(stubJSON(), stubYAML())
	router.RegisterRoot(engine)

	t.Run("serves json spec", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /openapi.json status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
			t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("serves yaml spec", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.yaml", nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /openapi.yaml status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/yaml") {
			t.Errorf("Content-Type = %q, want application/yaml", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("serves docs html", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /docs status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("Content-Type = %q, want text/html", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("redirects swagger to docs", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/swagger", nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusMovedPermanently {
			t.Fatalf("GET /swagger status = %d, want 301", rec.Code)
		}
		if rec.Header().Get("Location") != "/docs" {
			t.Errorf("Location = %q, want /docs", rec.Header().Get("Location"))
		}
	})
}
