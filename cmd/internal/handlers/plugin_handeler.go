package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	plugin "scutum/cmd/internal/plugins"
)

type PluginHandler struct {
	runtime   *plugin.Runtime
	registrar *plugin.RouteRegistrar
}

func NewPluginHandler(rt *plugin.Runtime, rr *plugin.RouteRegistrar) *PluginHandler {
	return &PluginHandler{
		runtime:   rt,
		registrar: rr,
	}
}

type loadRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (h *PluginHandler) HandleLoad(w http.ResponseWriter, r *http.Request) {
	var req loadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Path == "" {
		http.Error(w, "name and path are required", http.StatusBadRequest)
		return
	}
	if err := h.runtime.Load(r.Context(), req.Name, req.Path, h.registrar); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("PLUGIN_LOADED", r, "plugin_name", req.Name, "plugin_path", req.Path)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("plugin loaded"))
}

func (h *PluginHandler) HandleUnload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if err := h.runtime.Unload(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("PLUGIN_UNLOADED", r, "plugin_name", name)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("plugin unloaded"))
}

func (h *PluginHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	plugins := h.runtime.List()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(mustJSON(plugins))
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

type pluginUploadStore interface {
	RegisterPlugin(ctx context.Context, id, name, path string) error
}

// HandleUpload accepts a multipart .wasm upload, saves it, and loads it.
func (h *PluginHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if filepath.Ext(header.Filename) != ".wasm" {
		http.Error(w, "only .wasm files are accepted", http.StatusBadRequest)
		return
	}

	pluginsDir := os.Getenv("PLUGINS_DIR")
	if pluginsDir == "" {
		pluginsDir = "/app/plugins"
	}
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		http.Error(w, "failed to create plugins directory", http.StatusInternalServerError)
		return
	}

	destPath := filepath.Join(pluginsDir, filepath.Base(header.Filename))
	f, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "failed to save plugin", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	if _, err := io.Copy(f, file); err != nil {
		http.Error(w, "failed to write plugin", http.StatusInternalServerError)
		return
	}

	if err := h.runtime.Load(r.Context(), name, destPath, h.registrar); err != nil {
		os.Remove(destPath)
		http.Error(w, "failed to load plugin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	audit("PLUGIN_UPLOADED", r, "plugin_name", name, "plugin_path", destPath, "filename", header.Filename)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"name": name,
		"path": destPath,
	})
}
