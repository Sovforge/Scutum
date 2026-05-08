package handlers

import "net/http"

// Deployer defines what a deployment controller must do
type Deployer interface {
	PostDeploy(w http.ResponseWriter, r *http.Request)
}
