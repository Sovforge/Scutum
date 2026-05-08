package plugin

import (
	"context"
	"log"

	"scutum/cmd/internal/store"
)

type PluginLoader interface {
	Load(ctx context.Context, name, path string) error
}

type PluginStore interface {
	ListEnabledPlugins(ctx context.Context) ([]store.PluginRecord, error)
}

func LoadPlugins(ctx context.Context, rt PluginLoader, db PluginStore) error {
	ps, err := db.ListEnabledPlugins(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if err := rt.Load(ctx, p.Name, p.Path); err != nil {
			log.Printf("warn: failed to load plugin %q: %v", p.Name, err)
		} else {
			log.Printf("plugin loaded: %s", p.Name)
		}
	}
	return nil
}
