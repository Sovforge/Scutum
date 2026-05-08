# Scutum Test Suite

This directory contains comprehensive unit and function tests for the Scutum project. The tests are organized by functionality and include edge case testing to ensure code robustness.

## Test Structure

```
tests/
├── wireguard_test.go       # WireGuard key generation, interface config, and peer management
├── s3_test.go              # S3 signing, credentials, and cloud storage operations
├── git_test.go             # Git repository validation, credential injection, and path handling
├── docker_test.go          # Docker deployment, container config, and port mappings
├── kubernetes_test.go      # Kubernetes deployments, pods, scaling, and namespaces
├── handlers_test.go        # HTTP handlers, status codes, headers, and routing
├── clients_test.go         # Docker/Kubernetes client setup and error handling
├── websocket_test.go       # WebSocket handshake, frames, masking, and opcodes
├── integration_test.go     # Integration flows, concurrency, security, and performance
└── README.md               # This file
```

## Test Coverage

### WireGuard (`wireguard_test.go`)
- **Key Generation**: Validates format, length, uniqueness, and randomness
- **Interface Configuration**: Tests all config parameters including MTU and port ranges
- **Peer Management**: Validates peer public keys, endpoints, and allowed IPs
- **Edge Cases**: Port boundary values, IPv6 support, multiple peer scenarios

### S3 Storage (`s3_test.go`)
- **HMAC-SHA256**: Cryptographic hash validation
- **Request Signing**: AWS SigV4 signature generation and format
- **Content Hashing**: SHA256 computation for various payload sizes
- **Header Management**: x-amz-date format, content-sha256, authorization headers
- **Region Support**: Multiple S3-compatible regions (AWS, MinIO, Wasabi, etc.)

### Git Operations (`git_test.go`)
- **Repository URLs**: HTTPS, HTTP, SSH, and file:// protocol validation
- **Credential Injection**: Username/password handling and security
- **Local Paths**: Directory validation, path traversal prevention
- **Authentication**: Token-based and password-based authentication
- **Edge Cases**: Special characters, spaces, nested directories

### Docker Integration (`docker_test.go`)
- **Deployment Validation**: Image names, ports, memory, CPU limits
- **Container Config**: Environment variables, volumes, restart policies
- **Port Mappings**: Valid ranges, duplicate handling, protocol selection
- **Volume Binding**: Read/write modes, named volumes, host paths
- **Restart Policies**: "no", "always", "unless-stopped", "on-failure"

### Kubernetes (`kubernetes_test.go`)
- **Deployment Specs**: Name, namespace, replicas, image validation
- **Pod Configuration**: Container specs, labels, annotations
- **Namespace Handling**: Validation of K8s namespace naming conventions
- **Scaling Operations**: Replica count changes, min/max constraints
- **Resource Management**: CPU, memory requests and limits
- **ConfigMaps and Labels**: Custom resource configuration

### HTTP Handlers (`handlers_test.go`)
- **Health Checks**: Service status endpoint validation
- **HTTP Methods**: GET, POST, PUT, DELETE method handling
- **Status Codes**: Standard response codes and error conditions
- **Path Parameters**: Container IDs, namespace/pod references
- **Query Strings**: Parameter parsing and validation
- **Content Types**: JSON, XML, plain text, binary responses

### Clients (`clients_test.go`)
- **Docker Client**: Unix socket and TCP connection setup
- **Kubernetes Client**: In-cluster and external config handling
- **Error Handling**: HTTP error status codes and messages
- **Streaming**: Event streams, log streams, continuous responses
- **Timeouts**: Connection and request timeout validation
- **Retry Logic**: Maximum retries and backoff strategies

### WebSocket (`websocket_test.go`)
- **Handshake Keys**: Base64 validation and accept header generation
- **Frame Structure**: FIN bit, opcodes, masking bit, payload length
- **Payload Masking**: Client-to-server masking requirements
- **Opcodes**: All valid opcodes (0x0-0x2, 0x8-0xA) and reserved values
- **Control Frames**: Ping/pong, close frames, payload size limits
- **Upgrade Headers**: Required headers and version validation

### Integration Tests (`integration_test.go`)
- **Deployment Flows**: Docker, Kubernetes, Git, S3 workflows
- **Error Recovery**: Handling transient failures and permanent errors
- **Concurrency**: Multiple simultaneous operations
- **Security**: Credential handling, path validation, injection prevention
- **Performance**: Operation timeout requirements

## Running Tests

### Run All Tests
```bash
cd /home/andreas/projects/Gitea/Scutum
go test ./tests/... -v
```

### Run Specific Test File
```bash
go test ./tests -run TestWireGuard -v
go test ./tests -run TestS3 -v
go test ./tests -run TestGit -v
```

### Run Specific Test
```bash
go test ./tests -run TestGenerateKey -v
go test ./tests -run TestS3RequestHeaders -v
go test ./tests -run TestDockerDeployment -v
```

### Run with Coverage
```bash
go test ./tests/... -cover -v
go test ./tests/... -coverprofile=coverage.out -v
go tool cover -html=coverage.out
```

### Run with Race Detection
```bash
go test ./tests/... -race -v
```

### Run Tests in Parallel
```bash
go test ./tests/... -parallel 4 -v
```

## Test Organization

Each test file is organized by:
1. **Simple unit tests** - Individual function/component validation
2. **Validation tests** - Input validation and edge cases
3. **Edge case tests** - Boundary values, special conditions
4. **Integration scenarios** - Multi-step workflows
5. **Error handling** - Failure modes and recovery

## Test Coverage Summary

| Module | Test Count | Coverage |
|--------|-----------|----------|
| WireGuard | 6 | Edge cases, validation |
| S3 | 8 | Signing, hashing, regions |
| Git | 7 | URLs, credentials, paths |
| Docker | 7 | Config, volumes, ports |
| Kubernetes | 7 | Deployments, scaling, resources |
| Handlers | 8 | HTTP, headers, status codes |
| Clients | 6 | Connections, errors, timeouts |
| WebSocket | 9 | Handshake, frames, opcodes |
| Integration | 6 | Flows, concurrency, security |

**Total: 64+ test functions** covering core logic and edge cases

## Common Test Patterns

### Validation Testing
```go
tests := []struct {
    name    string
    input   string
    isValid bool
}{
    {name: "valid input", input: "value", isValid: true},
    {name: "empty input", input: "", isValid: false},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // Test validation logic
    })
}
```

### Edge Case Testing
```go
// Test boundary values, special characters, extreme sizes
tests := []struct {
    name   string
    value  int
    valid  bool
}{
    {name: "zero", value: 0, valid: false},
    {name: "max value", value: 65535, valid: true},
    {name: "overflow", value: 65536, valid: false},
}
```

### Error Handling
```go
// Test error conditions and recovery
if err != nil && !tt.expectErr {
    t.Errorf("Unexpected error: %v", err)
}
```

## Debugging Tests

### Enable Verbose Output
```bash
go test ./tests/... -v
```

### Run Single Test with Output
```bash
go test ./tests -run ^TestX$ -v -args -test.v
```

### Write Test Output to File
```bash
go test ./tests/... -v > test_output.txt 2>&1
```

## Adding New Tests

When adding new functionality:
1. Create test cases in the appropriate file
2. Use table-driven testing for multiple scenarios
3. Include edge cases and error conditions
4. Test both happy path and failure paths
5. Run tests to ensure they pass: `go test ./tests/... -v`
6. Check coverage with: `go test ./tests/... -cover`

## Test Dependencies

Tests use only Go standard library:
- `testing` - Test framework
- `crypto/hmac`, `crypto/sha256` - Cryptographic functions
- `encoding/base64` - Base64 encoding
- `net/http` - HTTP types
- `time` - Time functions
- `os`, `filepath` - File system

No external dependencies required.

## Performance Considerations

- Tests run in parallel when possible
- Memory-intensive tests are marked with resource requirements
- Large payload tests use bounded sizes
- Network timeouts are tested without actual network calls
- Each test is independent and can run in any order

## Future Enhancements

Potential areas for extended testing:
- Mock server implementations for Docker/K8s APIs
- Benchmarking tests for performance validation
- E2E tests with real Docker/Kubernetes instances
- Load testing for concurrent operations
- Security testing for injection attacks
- Stress testing for resource limits
