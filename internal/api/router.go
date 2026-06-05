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
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func newRouter(h *handler) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	r.Get("/api/v1/healthz", h.healthz)
	r.Get("/api/v1/readyz", h.readyz)

	r.Group(func(r chi.Router) {
		r.Use(requireClientCert)
		r.Get("/api/v1/schedules", h.listSchedules)
		r.Get("/api/v1/schedules/{name}", h.getSchedule)
		r.Put("/api/v1/schedules/{name}/execute", h.executeSchedule)
	})

	return r
}
