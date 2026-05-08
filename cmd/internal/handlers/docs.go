package handlers

import "net/http"

// DocsHandler serves the OpenAPI spec and the Scalar API reference UI.
type DocsHandler struct {
	spec []byte
}

func NewDocsHandler(spec []byte) *DocsHandler {
	return &DocsHandler{spec: spec}
}

func (h *DocsHandler) HandleSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(h.spec)
}

func (h *DocsHandler) HandleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(scalarHTML))
}

const scalarHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Scutum API</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/openapi.yaml"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`
