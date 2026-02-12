package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/docs"
)

// ServeDocs serves the Scalar API Reference page.
func (h *Handler) ServeDocs(c *gin.Context) {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>Wheel API Reference</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/docs/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// ServeOpenAPISpec serves the generated OpenAPI spec JSON.
func (h *Handler) ServeOpenAPISpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
}
