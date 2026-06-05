/*
Copyright 2025 Hauke Mettendorf.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hauke-cloud/irrigator/internal/valve"
)

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type collectionResponse[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	code, status := mapError(err)
	writeJSON(w, status, errorResponse{Error: err.Error(), Code: code})
}

func mapError(err error) (code string, status int) {
	switch {
	case isNotFound(err):
		return "SCHEDULE_NOT_FOUND", http.StatusNotFound
	case errors.Is(err, valve.ErrNotFound):
		return "VALVE_NOT_FOUND", http.StatusNotFound
	case errors.Is(err, valve.ErrDisabled):
		return "VALVE_DISABLED", http.StatusUnprocessableEntity
	case errors.Is(err, valve.ErrUnreachable):
		return "VALVE_UNREACHABLE", http.StatusServiceUnavailable
	default:
		return "INTERNAL_ERROR", http.StatusInternalServerError
	}
}

func isNotFound(err error) bool {
	return client.IgnoreNotFound(err) == nil && err != nil
}
