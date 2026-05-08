package handlers

import "scutum/cmd/internal/utils"

var (
	handlerLogger  *utils.Logger
	proxyHMACKey   []byte
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