package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"scutum/cmd/internal/store"
)

// generateString creates a string of specified length
func generateString(length int) string {
	return strings.Repeat("a", length)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// split splits a string by a delimiter
func split(s, sep string) []string {
	return strings.Split(s, sep)
}

// toLower converts a string to lowercase
func toLower(s string) string {
	return strings.ToLower(s)
}

// isValidJSON validates JSON and checks for duplicate keys
func isValidJSON(s string) bool {
	if s == "" {
		return false
	}

	// First check if it's valid JSON
	var js interface{}
	if json.Unmarshal([]byte(s), &js) != nil {
		return false
	}

	// Check for duplicate keys by parsing manually
	return !hasDuplicateKeys(s)
}

// hasDuplicateKeys checks if JSON string has duplicate object keys
func hasDuplicateKeys(s string) bool {
	keys := make(map[string]bool)
	inString := false
	escaped := false
	keyStart := -1
	inObject := 0

	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			if !inString {
				inString = true
				if inObject > 0 && keyStart == -1 {
					keyStart = i + 1
				}
			} else {
				inString = false
				if inObject > 0 && keyStart != -1 {
					key := s[keyStart:i]
					if keys[key] {
						return true // Duplicate key found
					}
					keys[key] = true
					keyStart = -1
				}
			}
			continue
		}

		if !inString {
			switch ch {
			case '{':
				inObject++
			case '}':
				inObject--
				keys = make(map[string]bool) // Reset for nested objects
			case ':':
				if inObject > 0 && keyStart != -1 {
					// Key ended, value starting
				}
			}
		}
	}

	return false
}

// setUnexportedField sets an unexported field on a struct using reflection
func setUnexportedField(target interface{}, fieldName string, value interface{}) {
	v := reflect.ValueOf(target).Elem()
	f := v.FieldByName(fieldName)
	if !f.IsValid() {
		panic("field not found: " + fieldName)
	}
	if f.CanSet() {
		f.Set(reflect.ValueOf(value))
		return
	}
	// For unexported fields, use unsafe
	reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

type mockWG struct {
	addPeerErr error
	status     string
	statusErr  error
	dump       string
	dumpErr    error
}

func (m *mockWG) AddPeer(iface, pub, endpoint, ips string, keepalive int) error {
	return m.addPeerErr
}

func (m *mockWG) GetStatus(iface string) (string, error) {
	return m.status, m.statusErr
}

func (m *mockWG) GetDump(iface string) (string, error) {
	return m.dump, m.dumpErr
}

func (m *mockWG) UpdatePeerEndpoint(iface, publicKey, endpoint string) error {
	return nil
}

type mockFailWGService struct{}

func (m *mockFailWGService) AddPeer(ifaceName, publicKey, endpoint, allowedIPs string, keepalive int) error {
	return fmt.Errorf("wg failure")
}

func (m *mockFailWGService) GetStatus(ifaceName string) (string, error) {
	return "", fmt.Errorf("wg failure")
}

func (m *mockFailWGService) GetDump(ifaceName string) (string, error) {
	return "", fmt.Errorf("wg failure")
}

func (m *mockFailWGService) UpdatePeerEndpoint(ifaceName, publicKey, endpoint string) error {
	return fmt.Errorf("wg failure")
}

type mockNodeProxyStore struct{}

func (m *mockNodeProxyStore) GetNode(_ context.Context, _ string) (store.NodeRecord, error) {
	return store.NodeRecord{}, fmt.Errorf("not found")
}
