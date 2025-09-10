package common

import (
	"encoding/json"
	"log"
	"net/http"
)

func JSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func BadRequest(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusBadRequest, map[string]any{"error": msg})
}

func Internal(w http.ResponseWriter, msg string, err error) {
	log.Println(msg, ":", err)
	JSON(w, http.StatusInternalServerError, map[string]any{"error": msg})
}
