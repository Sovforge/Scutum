package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
)

type Runtime struct {
	rt      wazero.Runtime
	plugins map[string]*Plugin
}

type Plugin struct {
	name         string
	path         string
	compiled     wazero.CompiledModule // Keep the compiled code
	instancePool sync.Pool             // Pool of api.Module
	cancel       context.CancelFunc
}

func NewRuntime(ctx context.Context) (*Runtime, error) {
	cfg := wazero.NewRuntimeConfig().WithMemoryLimitPages(32)
	rt := wazero.NewRuntimeWithConfig(ctx, cfg)

	r := &Runtime{
		rt:      rt,
		plugins: make(map[string]*Plugin),
	}

	if err := r.registerHostFunctions(ctx); err != nil {
		return nil, fmt.Errorf("register host functions: %w", err)
	}

	return r, nil
}

func (r *Runtime) Close(ctx context.Context) error {
	for _, p := range r.plugins {
		p.cancel()
		if err := p.compiled.Close(ctx); err != nil {
			return fmt.Errorf("close plugin %s: %w", p.name, err)
		}
	}
	return r.rt.Close(ctx)
}
