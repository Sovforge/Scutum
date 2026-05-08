package handlers

import (
	"net/http"
)

// handlerInternalErr logs the detail server-side and returns a generic 500
// to the client so internal implementation details are never exposed.
func handlerInternalErr(w http.ResponseWriter, r *http.Request, msg string, err error) {
	l := GetLogger()
	if l != nil {
		l.Error(msg, "error", err, "path", r.URL.Path)
	}
	http.Error(w, "An internal error occurred. Please check the server logs.", http.StatusInternalServerError)
}

// validatedRecoveryParams clamps n (total shares) and t (threshold) to
// safe values, returning defaults of 5/3 when inputs are out of range.
func validatedRecoveryParams(n, t int) (int, int) {
	if n < 3 || n > 20 {
		n = 5
	}
	if t < 2 || t > n {
		t = 3
	}
	return n, t
}
