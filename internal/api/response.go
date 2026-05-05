package api

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	Error validationError `json:"error"`
}

type validationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorResponse{
		Error: validationError{
			Code:    code,
			Message: message,
		},
	})
}
