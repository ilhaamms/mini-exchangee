package response

import (
	"encoding/json"
	"net/http"
)

// APIResponse is the standard response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSON sends a JSON response with the given status code
func JSON(w http.ResponseWriter, status int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// Success sends a success response
func Success(w http.ResponseWriter, message string, data interface{}) {
	JSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Created sends a 201 created response
func Created(w http.ResponseWriter, message string, data interface{}) {
	JSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// BadRequest sends a 400 error response
func BadRequest(w http.ResponseWriter, message string) {
	JSON(w, http.StatusBadRequest, APIResponse{
		Success: false,
		Error:   message,
	})
}

// NotFound sends a 404 error response
func NotFound(w http.ResponseWriter, message string) {
	JSON(w, http.StatusNotFound, APIResponse{
		Success: false,
		Error:   message,
	})
}

// InternalError sends a 500 error response
func InternalError(w http.ResponseWriter, message string) {
	JSON(w, http.StatusInternalServerError, APIResponse{
		Success: false,
		Error:   message,
	})
}
