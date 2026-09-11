// Package openapi_test exercises assembly, lifecycle, and route exposure of the openapi module.
// It verifies HTTP responses and readiness probes for both root and versioned endpoints.
package openapi_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ging/alexandria/internal/config"
	"github.com/ging/alexandria/internal/openapi"
)

// stubConfig produces a minimal configuration instance for unit tests.
func stubConfig() *config.Config {
	return &config.Config{
		Common: config.Common{
			API: config.API{
				Version: "v1",
			},
		},
	}
}

// stubLogger produces a discarded logger to suppress test noise.
func stubLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewRejectsNilConfig(t *testing.T) {
	t.Parallel()

	_, err := openapi.New(openapi.Deps{
		Config: nil,
		Logger: stubLogger(),
	})
	if err == nil {
		t.Fatal("New(nil config) succeeded, want error")
	}
}

func TestModuleMetadataAndLifecycle(t *testing.T) {
	t.Parallel()

	mod, err := openapi.New(openapi.Deps{
		Config: stubConfig(),
		Logger: stubLogger(),
	})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	if mod.Name() != "openapi" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "openapi")
	}

	if path, ok := mod.Describe(); !ok || path != "/docs" {
		t.Errorf("Describe() = (%q, %v), want (%q, true)", path, ok, "/docs")
	}

	checks := mod.Checks()
	if check, exists := checks["openapi"]; !exists || check(context.Background()) != nil {
		t.Error("Checks()[openapi] failed or missing")
	}

	if err := mod.Start(context.Background()); err != nil {
		t.Errorf("Start() = %v", err)
	}

	if err := mod.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}

func TestRegisterRootEndpoints(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	engine := gin.New()

	mod, err := openapi.New(openapi.Deps{
		Config: stubConfig(),
		Logger: stubLogger(),
	})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	mod.RegisterRoot(engine)

	tests := []struct {
		name        string
		path        string
		wantStatus  int
		wantContent string
		wantType    string
	}{
		{
			name:        "openapi json",
			path:        "/openapi.json",
			wantStatus:  http.StatusOK,
			wantContent: `"openapi": "3.0.3"`,
			wantType:    "application/json",
		},
		{
			name:        "openapi yaml",
			path:        "/openapi.yaml",
			wantStatus:  http.StatusOK,
			wantContent: "openapi: 3.0.3",
			wantType:    "application/yaml",
		},
		{
			name:        "docs ui",
			path:        "/docs",
			wantStatus:  http.StatusOK,
			wantContent: "swagger-ui",
			wantType:    "text/html",
		},
		{
			name:        "swagger redirect",
			path:        "/swagger",
			wantStatus:  http.StatusMovedPermanently,
			wantContent: "",
			wantType:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			engine.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("GET %s = %d, want %d", tc.path, rec.Code, tc.wantStatus)
			}

			if tc.wantType != "" && !strings.Contains(rec.Header().Get("Content-Type"), tc.wantType) {
				t.Errorf("GET %s Content-Type = %q, want %q", tc.path, rec.Header().Get("Content-Type"), tc.wantType)
			}

			if tc.wantContent != "" && !strings.Contains(rec.Body.String(), tc.wantContent) {
				t.Errorf("GET %s body does not contain %q", tc.path, tc.wantContent)
			}
		})
	}
}

func TestRegisterVersionedEndpoints(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api/v1")

	mod, err := openapi.New(openapi.Deps{
		Config: stubConfig(),
		Logger: stubLogger(),
	})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	mod.Register(api)

	paths := []string{
		"/api/v1/openapi/openapi.json",
		"/api/v1/openapi/openapi.yaml",
		"/api/v1/openapi/docs",
	}

	for _, p := range paths {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, p, nil)
		rec := httptest.NewRecorder()

		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}
}
