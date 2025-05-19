package response

import (
	"encoding/json"
	"net/http"
)

type JsonResponse struct {
	Error   bool   `json:"error"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, status int, data any, isErr bool, msg string) error {
	return JSONWithHeaders(w, status, data, isErr, msg, nil)
}

func JSONWithHeaders(w http.ResponseWriter, status int, data any, isErr bool, msg string, headers http.Header) error {
	for key, value := range headers {
		w.Header()[key] = value
	}

	response := &JsonResponse{
		Error:   isErr,
		Message: msg,
		Data:    nil,
	}
	if !isErr {
		response.Data = data
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return err
	}

	return nil
}
