package handlers

import (
	"net/http"

	"scutum/cmd/internal/utils"
)

var (
	handlerLogger *utils.Logger
	proxyHMACKey  []byte
)

func SetLogger(logger *utils.Logger) {
	handlerLogger = logger
}

func GetLogger() *utils.Logger {
	return handlerLogger
}

func SetProxyHMACKey(key []byte) {
	proxyHMACKey = key
}

// audit records a security event using the package-level logger.
// It extracts actor identity from the request context automatically.
func audit(action string, r *http.Request, args ...any) {
	NewBaseHandler(nil).Audit(action, r, args...)
}