
# Developing a Plugin for the WASM Sandbox  
*A guide for plugin authors targeting the Go wazero host*

The Go host exposes a controlled set of capabilities to WebAssembly modules via the `env` module. A plugin is simply a `.wasm` binary (Rust, TinyGo, AssemblyScript, etc.) that imports these host functions and exports its own entrypoints.

This document explains:

- What the host provides  
- What a plugin must export  
- How to call host functions  
- How memory works  
- How to register routes, use KV storage, and make HTTP requests  

---

## 1. Host Capabilities Available to Plugins

The host registers these functions under the module name `env`:

| Host Function | Purpose |
|---------------|---------|
| `log(ptr, len)` | Write a log message to the host |
| `http_request(methodPtr, methodLen, urlPtr, urlLen)` | Perform an outbound HTTP request |
| `kv_get(keyPtr, keyLen)` | Read from shared KV store |
| `kv_set(keyPtr, keyLen, valPtr, valLen)` | Write to shared KV store |
| `register_route(methodPtr, methodLen, pathPtr, pathLen, handlerPtr, handlerLen)` | Register an HTTP route handled by the plugin |

The host also expects the plugin to export:

- `alloc(size) -> ptr`  
- `free(ptr, size)` (optional but recommended)

These allow the host to write data into the plugin’s memory.

---

## 2. Plugin Requirements

A plugin must:

### ✔ Import host functions

Example (WAT):

```wat
(import "env" "log" (func $log (param i32 i32)))
(import "env" "http_request" (func $http_request (param i32 i32 i32 i32) (result i64)))
(import "env" "kv_get" (func $kv_get (param i32 i32) (result i32)))
(import "env" "kv_set" (func $kv_set (param i32 i32 i32 i32)))
(import "env" "register_route" (func $register_route (param i32 i32 i32 i32 i32 i32)))
```

### ✔ Export memory management

Example (TinyGo):

```go
//export alloc
func alloc(size uint32) uint32 {
    b := make([]byte, size)
    return &b[0]
}
```

---

## 3. Calling Host Functions

### Logging

```go
msg := "hello from plugin"
ptr := alloc(uint32(len(msg)))
copy(memory[ptr:], msg)
log(ptr, uint32(len(msg)))
```

---

### HTTP Request

```go
method := "GET"
url := "https://example.com"

mPtr := writeString(method)
uPtr := writeString(url)

result := http_request(mPtr, uint32(len(method)), uPtr, uint32(len(url)))

// result = (ptr << 32) | len
bodyPtr := uint32(result >> 32)
bodyLen := uint32(result)
```

---

## 4. Using the KV Store

### Set a value

```go
kv_set(keyPtr, keyLen, valPtr, valLen)
```

### Get a value

```go
ptr := kv_get(keyPtr, keyLen)
if ptr != 0 {
    // read from memory
}
```

---

## 5. Registering HTTP Routes

Plugins can dynamically register HTTP handlers:

```go
register_route(
    methodPtr, methodLen,
    pathPtr, pathLen,
    handlerNamePtr, handlerNameLen,
)
```

The host will later call the exported function named by `handlerName`.

Example plugin handler:

```go
//export handle_index
func handle_index(reqPtr uint32, reqLen uint32) uint64 {
    msg := []byte("hello from wasm")
    ptr := alloc(uint32(len(msg)))
    copy(memory[ptr:], msg)
    return uint64(ptr)<<32 | uint64(len(msg))
}
```

---

## 6. Recommended Plugin Structure

```
plugin/
  main.go or lib.rs
  memory.go      # alloc/free helpers
  host.go        # imports
  handlers.go    # HTTP handlers
```

---

## 7. Example Minimal Plugin (TinyGo)

```go
package main

//export alloc
func alloc(size uint32) uint32 {
    b := make([]byte, size)
    return &b[0]
}

//export handle_ping
func handle_ping(reqPtr, reqLen uint32) uint64 {
    msg := []byte("pong")
    ptr := alloc(uint32(len(msg)))
    copy(memory[ptr:], msg)
    return uint64(ptr)<<32 | uint64(len(msg))
}

//export init
func init() {
    registerRoute("GET", "/ping", "handle_ping")
}
```

---
