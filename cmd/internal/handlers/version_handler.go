package handlers

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
)

type versionResponse struct {
	Version string `json:"version"`
	Build   string `json:"build"`
	Commit  string `json:"commit"`
}

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	resp := versionResponse{
		Version: "v0.9.1",
		Build:   "2026.04",
		Commit:  "unknown",
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				resp.Commit = s.Value[:7]
			}
			if s.Key == "vcs.time" && len(s.Value) >= 7 {
				resp.Build = s.Value[:7]
			}
		}
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			resp.Version = info.Main.Version
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
