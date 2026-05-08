package plugin

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Load reads a .wasm file, compiles it, and initializes an instance pool.
func (r *Runtime) Load(ctx context.Context, name, path string, rr *RouteRegistrar) error {
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %q already loaded", name)
	}

	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}

	// WASI must be instantiated once in the runtime to support plugins using standard I/O
	wasi_snapshot_preview1.MustInstantiate(ctx, r.rt)

	compiled, err := r.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("compile %s: %w", name, err)
	}

	pluginCtx, cancel := context.WithCancel(ctx)

	pluginCtx = context.WithValue(pluginCtx, RegistrarKey{}, rr)

	p := &Plugin{
		name:     name,
		path:     path,
		compiled: compiled,
		cancel:   cancel,
	}

	// Define how the pool creates new instances when needed
	p.instancePool = sync.Pool{
		New: func() any {
			// Each instance gets the same name/config but unique memory
			mod, err := r.rt.InstantiateModule(pluginCtx, compiled,
				wazero.NewModuleConfig().WithName(name),
			)
			if err != nil {
				return nil
			}
			return mod
		},
	}

	// Warm up: Initialize one instance to call on_load and verify it works
	mod, ok := p.instancePool.Get().(api.Module)
	if !ok {
		cancel()
		return fmt.Errorf("failed to initialize first instance for %s", name)
	}
	defer p.instancePool.Put(mod)

	if fn := mod.ExportedFunction("on_load"); fn != nil {
		if _, err := fn.Call(pluginCtx); err != nil {
			cancel()
			return fmt.Errorf("on_load %s: %w", name, err)
		}
	}

	r.plugins[name] = p
	return nil
}

// Unload cleans up the plugin and closes all instances in the pool.
func (r *Runtime) Unload(ctx context.Context, name string) error {
	p, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %q not found", name)
	}

	if mod, ok := p.instancePool.Get().(api.Module); ok {
		if fn := mod.ExportedFunction("on_unload"); fn != nil {
			_, _ = fn.Call(ctx)
		}
	}

	p.cancel()
	if err := p.compiled.Close(ctx); err != nil {
		return fmt.Errorf("close compiled %s: %w", name, err)
	}

	delete(r.plugins, name)
	return nil
}

// PluginInfo is the public view of a loaded plugin.
type PluginInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// List returns metadata for all loaded plugins.
func (r *Runtime) List() []PluginInfo {
	out := make([]PluginInfo, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, PluginInfo{Name: p.name, Path: p.path})
	}
	return out
}
