package tests

import (
	"context"
	"errors"
	"testing"

	plugins "scutum/cmd/internal/plugins"
	"scutum/cmd/internal/store"
)

type fakePluginStore struct {
	plugins []store.PluginRecord
	err     error
}

func (f fakePluginStore) ListEnabledPlugins(ctx context.Context) ([]store.PluginRecord, error) {
	return f.plugins, f.err
}

type fakePluginLoader struct {
	loaded []store.PluginRecord
	err    error
}

func (f *fakePluginLoader) Load(ctx context.Context, name, path string) error {
	f.loaded = append(f.loaded, store.PluginRecord{Name: name, Path: path})
	return f.err
}

func TestLoadPluginsReturnsListError(t *testing.T) {
	testErr := errors.New("db error")
	store := fakePluginStore{err: testErr}
	loader := &fakePluginLoader{}

	if err := plugins.LoadPlugins(context.Background(), loader, store); !errors.Is(err, testErr) {
		t.Fatalf("expected %v, got %v", testErr, err)
	}
}

func TestLoadPluginsLoadsEnabledPlugins(t *testing.T) {
	store := fakePluginStore{plugins: []store.PluginRecord{{Name: "alpha", Path: "/tmp/alpha.wasm"}, {Name: "beta", Path: "/tmp/beta.wasm"}}}
	loader := &fakePluginLoader{}

	if err := plugins.LoadPlugins(context.Background(), loader, store); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loader.loaded) != len(store.plugins) {
		t.Fatalf("expected %d loads, got %d", len(store.plugins), len(loader.loaded))
	}
	for i, plugin := range store.plugins {
		if loader.loaded[i] != plugin {
			t.Fatalf("unexpected loaded plugin: got %#v, want %#v", loader.loaded[i], plugin)
		}
	}
}

func TestLoadPluginsContinuesWhenPluginLoadFails(t *testing.T) {
	store := fakePluginStore{plugins: []store.PluginRecord{{Name: "alpha", Path: "/tmp/alpha.wasm"}, {Name: "beta", Path: "/tmp/beta.wasm"}}}
	loader := &fakePluginLoader{err: errors.New("load failed")}

	if err := plugins.LoadPlugins(context.Background(), loader, store); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loader.loaded) != len(store.plugins) {
		t.Fatalf("expected %d load attempts, got %d", len(store.plugins), len(loader.loaded))
	}
}
