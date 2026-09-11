// Package rest provides the driving HTTP adapter for OpenAPI specification endpoints.
// It serves raw JSON/YAML specification schemas and an interactive Swagger UI.
package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Router exposes the OpenAPI documents and interactive documentation over HTTP.
type Router struct {
	specJSON []byte
	specYAML []byte
}

// NewRouter constructs a documentation router with provided specification payloads.
func NewRouter(specJSON, specYAML []byte) *Router {
	return &Router{
		specJSON: specJSON,
		specYAML: specYAML,
	}
}

// HasSpec reports whether specification payloads are populated.
func (r *Router) HasSpec() bool {
	return len(r.specJSON) > 0 || len(r.specYAML) > 0
}

// Register mounts documentation routes under the versioned API group.
func (r *Router) Register(api *gin.RouterGroup) {
	docs := api.Group("/openapi")

	docs.GET("/openapi.json", r.serveJSON)
	docs.GET("/openapi.yaml", r.serveYAML)
	docs.GET("/docs", r.serveUI)
}

// RegisterRoot mounts documentation routes at the origin root outside version prefixes.
func (r *Router) RegisterRoot(engine *gin.Engine) {
	engine.GET("/openapi.json", r.serveJSON)
	engine.GET("/openapi.yaml", r.serveYAML)
	engine.GET("/docs", r.serveUI)
	engine.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs")
	})
}

// serveJSON writes the OpenAPI specification in JSON format.
func (r *Router) serveJSON(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", r.specJSON)
}

// serveYAML writes the OpenAPI specification in YAML format.
func (r *Router) serveYAML(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", r.specYAML)
}

// serveUI renders the interactive Swagger UI interface.
func (r *Router) serveUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerUIHTML))
}

// swaggerUIHTML is the static single-page Swagger UI bundle HTML template.
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Alexandria API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5/favicon-32x32.png" />
  <style>
    body { margin: 0; padding: 0; background: #fafafa; font-family: sans-serif; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`
