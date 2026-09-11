// Package openapi wires the OpenAPI specification and interactive documentation context.
// It resolves API contracts from config or embedded assets and mounts driving HTTP routes.
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/ging/alexandria/api"
	"github.com/ging/alexandria/internal/config"
	"github.com/ging/alexandria/internal/observability"
	"github.com/ging/alexandria/internal/openapi/rest"
	"gopkg.in/yaml.v3"
)

// Name is how this context identifies itself in logs, probes and reports.
const Name = "openapi"

// Deps is everything the context needs from outside itself.
type Deps struct {
	// Config is the whole document.
	Config *config.Config
	// Logger is the process logger, scoped to the module.
	Logger *slog.Logger
}

// Module is the assembled context.
type Module struct {
	router *rest.Router
	logger *slog.Logger
}

// New assembles the context.
func New(deps Deps) (*Module, error) {
	if deps.Config == nil {
		return nil, fmt.Errorf("%s: no configuration given", Name)
	}

	logger := observability.Scoped(deps.Logger, observability.ModuleOpenAPI, "")

	yamlSpec := api.Spec
	if doc, err := deps.Config.Common.API.OpenAPI(); err == nil && len(doc) > 0 {
		yamlSpec = doc
	}

	var parsed any
	if err := yaml.Unmarshal(yamlSpec, &parsed); err != nil {
		return nil, fmt.Errorf("%s: parsing openapi spec: %w", Name, err)
	}

	jsonSpec, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%s: converting openapi spec to json: %w", Name, err)
	}

	return &Module{
		router: rest.NewRouter(jsonSpec, yamlSpec),
		logger: logger,
	}, nil
}

// Name identifies the context.
func (m *Module) Name() string { return Name }

// Register mounts the context's HTTP surface under the versioned API group.
func (m *Module) Register(apiGroup *gin.RouterGroup) { m.router.Register(apiGroup) }

// RegisterRoot mounts the routes pinned to the root of the origin.
func (m *Module) RegisterRoot(engine *gin.Engine) { m.router.RegisterRoot(engine) }

// Checks are the context's contributions to readiness.
func (m *Module) Checks() map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"openapi": func(context.Context) error {
			if !m.router.HasSpec() {
				return fmt.Errorf("no openapi specification available: %w", observability.ErrNotReady)
			}

			return nil
		},
	}
}

// Describe reports the documentation endpoint for the startup report.
func (m *Module) Describe() (string, bool) {
	return "/docs", true
}

// Start readies the context before requests arrive.
func (m *Module) Start(_ context.Context) error {
	return nil
}

// Close releases what the context owns.
func (m *Module) Close() error {
	return nil
}
