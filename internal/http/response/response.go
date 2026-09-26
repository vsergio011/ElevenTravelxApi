package response

import (
	"encoding/json"
	"net/http"
)

type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type ErrorObject struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorObject `json:"error"`
}

type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type ListEnvelope[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

func WriteJSON(writer http.ResponseWriter, statusCode int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)

	if payload == nil {
		return
	}

	_ = json.NewEncoder(writer).Encode(payload)
}

func WriteError(writer http.ResponseWriter, statusCode int, code string, message string, details []ErrorDetail) {
	WriteJSON(writer, statusCode, ErrorEnvelope{
		Error: ErrorObject{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
