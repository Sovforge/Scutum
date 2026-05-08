package plugin

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/tetratelabs/wazero/api"
)

var httpClient = http.DefaultClient

func SetHTTPClient(c *http.Client) {
	httpClient = c
}

// RegistrarKey is the context key for passing the RouteRegistrar to host functions.
type RegistrarKey struct{}

// registerHostFunctions defines every capability a plugin is allowed to use.
// Anything not listed here is unreachable from WASM.
func (r *Runtime) registerHostFunctions(ctx context.Context) error {
	_, err := r.rt.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithFunc(hostLog).
		Export("log").
		NewFunctionBuilder().
		WithFunc(hostHTTPRequest).
		Export("http_request").
		NewFunctionBuilder().
		WithFunc(hostKVGet).
		Export("kv_get").
		NewFunctionBuilder().
		WithFunc(hostKVSet).
		Export("kv_set").
		NewFunctionBuilder().
		WithFunc(hostRegisterRoute).
		Export("register_route").
		Instantiate(ctx)
	return err
}

func hostLog(_ context.Context, mod api.Module, ptr, length uint32) {
	msg, ok := readString(mod, ptr, length)
	if !ok {
		return
	}
	log.Printf("[plugin:%s] %s", mod.Name(), msg)
}

func hostHTTPRequest(ctx context.Context, mod api.Module,
	methodPtr, methodLen,
	urlPtr, urlLen uint32,
) uint64 {
	method, ok := readString(mod, methodPtr, methodLen)
	if !ok {
		return 0
	}
	rawURL, ok := readString(mod, urlPtr, urlLen)
	if !ok {
		return 0
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, nil)
	if err != nil {
		log.Printf("[plugin:%s] bad request: %v", mod.Name(), err)
		return 0
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[plugin:%s] request failed: %v", mod.Name(), err)
		return 0
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[plugin:%s] read body: %v", mod.Name(), err)
		return 0
	}

	ptr := writeToPluginMemory(mod, body)
	if ptr == 0 && len(body) > 0 {
		log.Printf("[plugin:%s] failed to allocate memory for response body", mod.Name())
		return 0
	}

	return (uint64(ptr) << 32) | uint64(len(body))
}

func hostKVGet(_ context.Context, mod api.Module, keyPtr, keyLen uint32) uint32 {
	key, ok := readString(mod, keyPtr, keyLen)
	if !ok {
		return 0
	}
	KVStore.mu.RLock()
	val, exists := KVStore.store[key]
	KVStore.mu.RUnlock()
	if !exists {
		return 0
	}
	return writeToPluginMemory(mod, []byte(val))
}

func hostKVSet(_ context.Context, mod api.Module, keyPtr, keyLen, valPtr, valLen uint32) {
	key, ok := readString(mod, keyPtr, keyLen)
	if !ok {
		return
	}
	val, ok := readString(mod, valPtr, valLen)
	if !ok {
		return
	}
	KVStore.mu.Lock()
	KVStore.store[key] = val
	KVStore.mu.Unlock()
}

// hostRegisterRoute is called by a plugin to register an HTTP route with the host.
func hostRegisterRoute(ctx context.Context, mod api.Module,
	methodPtr, methodLen,
	pathPtr, pathLen,
	handlerPtr, handlerLen uint32,
) {
	method, ok := readString(mod, methodPtr, methodLen)
	if !ok {
		return
	}
	path, ok := readString(mod, pathPtr, pathLen)
	if !ok {
		return
	}
	handler, ok := readString(mod, handlerPtr, handlerLen)
	if !ok {
		return
	}
	rr, ok := ctx.Value(RegistrarKey{}).(*RouteRegistrar)
	if !ok {
		log.Printf("[plugin:%s] no route registrar in context", mod.Name())
		return
	}
	rr.AddRoute(mod.Name(), method, path, handler)
}

// KVStore is the host-side persistent store visible to all plugins.
// Exported for testing.
var KVStore = struct {
	mu    sync.RWMutex
	store map[string]string
}{
	store: make(map[string]string),
}

// KVGet retrieves a value from the plugin KV store.
// Exported for testing.
func KVGet(key string) (string, bool) {
	KVStore.mu.RLock()
	val, exists := KVStore.store[key]
	KVStore.mu.RUnlock()
	return val, exists
}

// KVSet stores a value in the plugin KV store.
// Exported for testing.
func KVSet(key, value string) {
	KVStore.mu.Lock()
	KVStore.store[key] = value
	KVStore.mu.Unlock()
}

// ClearKVStore clears the plugin KV store.
// Exported for testing.
func ClearKVStore() {
	KVStore.mu.Lock()
	KVStore.store = make(map[string]string)
	KVStore.mu.Unlock()
}

func readString(mod api.Module, ptr, length uint32) (string, bool) {
	buf, ok := mod.Memory().Read(ptr, length)
	if !ok {
		log.Printf("[plugin:%s] memory read out of bounds: ptr=%d len=%d", mod.Name(), ptr, length)
		return "", false
	}
	return string(buf), true
}

func writeToPluginMemory(mod api.Module, data []byte) uint32 {
	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		log.Printf("[plugin:%s] missing exported alloc function", mod.Name())
		return 0
	}
	results, err := alloc.Call(context.Background(), uint64(len(data)))
	if err != nil || len(results) == 0 {
		log.Printf("[plugin:%s] alloc failed: %v", mod.Name(), err)
		return 0
	}
	ptr := uint32(results[0])
	if !mod.Memory().Write(ptr, data) {
		log.Printf("[plugin:%s] memory write failed at ptr=%d", mod.Name(), ptr)
		return 0
	}
	return ptr
}

func freePluginMemory(ctx context.Context, mod api.Module, ptr uint32, length uint32) {
	free := mod.ExportedFunction("free")
	if free != nil {
		_, err := free.Call(ctx, uint64(ptr), uint64(length))
		if err != nil {
			log.Printf("[plugin:%s] failed to free memory: %v", mod.Name(), err)
		}
	}
}
