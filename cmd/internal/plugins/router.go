package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/tetratelabs/wazero/api"
)

// RouteRegistrar allows plugins to register HTTP routes at runtime.
type RouteRegistrar struct {
	mux     *http.ServeMux
	runtime *Runtime
}

func NewRouteRegistrar(mux *http.ServeMux, rt *Runtime) *RouteRegistrar {
	return &RouteRegistrar{mux: mux, runtime: rt}
}

// RegisterRoutes calls each loaded plugin's register_routes export,
// which tells the host what paths the plugin wants to handle.
func (rr *RouteRegistrar) RegisterRoutes(ctx context.Context) error {
	for name, p := range rr.runtime.plugins {
		mod := p.instancePool.Get().(api.Module)

		fn := mod.ExportedFunction("register_routes")
		if fn != nil {
			if _, err := fn.Call(ctx); err != nil {
				p.instancePool.Put(mod)
				return fmt.Errorf("register_routes %s: %w", name, err)
			}
		}

		p.instancePool.Put(mod)
	}
	return nil
}

// AddRoute is called by host functions when a plugin registers a route.
// The plugin provides method, path, and its handler function name.
func (rr *RouteRegistrar) AddRoute(pluginName, method, path, handlerFn string) {
	pattern := fmt.Sprintf("%s /plugin/%s%s", method, pluginName, path)
	rr.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		p, ok := rr.runtime.plugins[pluginName]
		if !ok {
			http.Error(w, "plugin not loaded", http.StatusServiceUnavailable)
			return
		}

		// 1. Get an execution instance from the pool
		mod, ok := p.instancePool.Get().(api.Module)
		if !ok {
			http.Error(w, "no available plugin instances", http.StatusServiceUnavailable)
			return
		}
		defer p.instancePool.Put(mod)

		fn := mod.ExportedFunction(handlerFn)
		if fn == nil {
			http.Error(w, "handler not found in plugin", http.StatusNotFound)
			return
		}

		// 3. Process the body
		body, _ := io.ReadAll(r.Body)
		var ptr uint32
		if len(body) > 0 {
			ptr = writeToPluginMemory(mod, body)
			if ptr != 0 {
				defer freePluginMemory(r.Context(), mod, ptr, uint32(len(body)))
			}
		}

		// 4. Call the handler on the specific pooled instance
		if _, err := fn.Call(r.Context(), uint64(ptr), uint64(len(body))); err != nil {
			http.Error(w, fmt.Sprintf("plugin error: %v", err), http.StatusInternalServerError)
		}
	})
}
