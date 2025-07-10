package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// Import docs to register swagger
	_ "github.com/voidrunnerhq/voidrunner/docs"
)

// DocsHandler handles API documentation endpoints
type DocsHandler struct{}

// NewDocsHandler creates a new documentation handler
func NewDocsHandler() *DocsHandler {
	return &DocsHandler{}
}

// GetSwaggerJSON serves the swagger.json file
// @Summary Get OpenAPI JSON specification
// @Description Returns the OpenAPI specification in JSON format
// @Tags Documentation
// @Produce json
// @Success 200 {object} map[string]interface{} "OpenAPI specification"
// @Router /docs/swagger.json [get]
func (h *DocsHandler) GetSwaggerJSON(c *gin.Context) {
	// Serve the swagger.json file
	c.File("./docs/swagger.json")
}

// GetSwaggerYAML serves the swagger.yaml file
// @Summary Get OpenAPI YAML specification
// @Description Returns the OpenAPI specification in YAML format
// @Tags Documentation
// @Produce text/yaml
// @Success 200 {string} string "OpenAPI specification in YAML"
// @Router /docs/swagger.yaml [get]
func (h *DocsHandler) GetSwaggerYAML(c *gin.Context) {
	// Serve the swagger.yaml file
	c.File("./docs/swagger.yaml")
}

// RedirectToSwaggerUI redirects to Swagger UI
// @Summary Redirect to Swagger UI
// @Description Redirects to the Swagger UI interface
// @Tags Documentation
// @Success 302 {string} string "Redirect to Swagger UI"
// @Router /docs [get]
func (h *DocsHandler) RedirectToSwaggerUI(c *gin.Context) {
	c.Redirect(http.StatusFound, "/docs/")
}

// GetSwaggerUI returns the Swagger UI handler
func (h *DocsHandler) GetSwaggerUI() gin.HandlerFunc {
	return ginSwagger.WrapHandler(swaggerFiles.Handler)
}

// GetAPIIndex serves a simple API documentation index
// @Summary API Documentation Index
// @Description Returns an HTML page with links to various API documentation formats
// @Tags Documentation
// @Produce text/html
// @Success 200 {string} string "HTML documentation index"
// @Router /api [get]
func (h *DocsHandler) GetAPIIndex(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>VoidRunner API Documentation</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 2rem;
            line-height: 1.6;
            color: #333;
        }
        .header {
            text-align: center;
            margin-bottom: 3rem;
            padding-bottom: 2rem;
            border-bottom: 2px solid #e1e5e9;
        }
        .header h1 {
            color: #2c3e50;
            margin-bottom: 0.5rem;
        }
        .header p {
            color: #7f8c8d;
            font-size: 1.1rem;
        }
        .links {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
            margin-bottom: 3rem;
        }
        .link-card {
            background: #f8f9fa;
            border: 1px solid #e1e5e9;
            border-radius: 8px;
            padding: 1.5rem;
            text-decoration: none;
            color: inherit;
            transition: all 0.2s ease;
        }
        .link-card:hover {
            background: #e9ecef;
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
        }
        .link-card h3 {
            margin: 0 0 0.5rem 0;
            color: #2c3e50;
        }
        .link-card p {
            margin: 0;
            color: #7f8c8d;
            font-size: 0.9rem;
        }
        .endpoints {
            background: #f8f9fa;
            border-radius: 8px;
            padding: 1.5rem;
        }
        .endpoints h3 {
            margin-top: 0;
            color: #2c3e50;
        }
        .endpoint-list {
            list-style: none;
            padding: 0;
        }
        .endpoint-list li {
            margin-bottom: 0.5rem;
            padding: 0.5rem;
            background: white;
            border-radius: 4px;
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.9rem;
        }
        .method {
            display: inline-block;
            padding: 0.2rem 0.5rem;
            border-radius: 3px;
            color: white;
            font-weight: bold;
            margin-right: 0.5rem;
            min-width: 60px;
            text-align: center;
        }
        .get { background: #28a745; }
        .post { background: #007bff; }
        .put { background: #ffc107; color: #333; }
        .delete { background: #dc3545; }
    </style>
</head>
<body>
    <div class="header">
        <h1>VoidRunner API Documentation</h1>
        <p>Distributed task execution platform API</p>
    </div>
    
    <div class="links">
        <a href="/docs/" class="link-card">
            <h3>📖 Interactive Documentation</h3>
            <p>Swagger UI - Try the API endpoints directly in your browser</p>
        </a>
        
        <a href="/docs/swagger.json" class="link-card">
            <h3>📄 OpenAPI JSON</h3>
            <p>Raw OpenAPI specification in JSON format</p>
        </a>
        
        <a href="/docs/swagger.yaml" class="link-card">
            <h3>📋 OpenAPI YAML</h3>
            <p>OpenAPI specification in YAML format</p>
        </a>
        
        <a href="/health" class="link-card">
            <h3>💓 Health Check</h3>
            <p>Check API service health and status</p>
        </a>
    </div>
    
    <div class="endpoints">
        <h3>🛠 Quick Reference</h3>
        <ul class="endpoint-list">
            <li><span class="method get">GET</span> /health - Health check</li>
            <li><span class="method get">GET</span> /ready - Readiness check</li>
            <li><span class="method post">POST</span> /api/v1/auth/register - Register user</li>
            <li><span class="method post">POST</span> /api/v1/auth/login - Login user</li>
            <li><span class="method get">GET</span> /api/v1/auth/me - Get current user</li>
            <li><span class="method get">GET</span> /api/v1/tasks - List tasks</li>
            <li><span class="method post">POST</span> /api/v1/tasks - Create task</li>
            <li><span class="method get">GET</span> /api/v1/tasks/{id} - Get task</li>
            <li><span class="method post">POST</span> /api/v1/tasks/{id}/executions - Execute task</li>
        </ul>
    </div>
</body>
</html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
