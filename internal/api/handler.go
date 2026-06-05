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
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	irrigatorv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
)

// Executor is the subset of controller.Executor used by handlers.
type Executor interface {
	Execute(ctx context.Context, key types.NamespacedName) error
}

type handler struct {
	k8s      client.Client
	executor Executor
	log      *slog.Logger
}

// listSchedules returns all IrrigationSchedule objects the controller can see,
// optionally filtered by the ?namespace= query parameter.
func (h *handler) listSchedules(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFrom(r)
	limit, offset := parsePagination(r)

	var list irrigatorv1alpha1.IrrigationScheduleList
	opts := []client.ListOption{}
	if ns != "" {
		opts = append(opts, client.InNamespace(ns))
	}
	if err := h.k8s.List(r.Context(), &list, opts...); err != nil {
		h.log.ErrorContext(r.Context(), "list schedules", "error", err)
		writeError(w, err)
		return
	}

	total := len(list.Items)
	end := offset + limit
	if end > total {
		end = total
	}
	page := list.Items
	if offset < total {
		page = list.Items[offset:end]
	} else {
		page = nil
	}

	writeJSON(w, http.StatusOK, collectionResponse[irrigatorv1alpha1.IrrigationSchedule]{
		Items:  page,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// getSchedule returns a single IrrigationSchedule by name (and optional namespace).
func (h *handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ns := namespaceFrom(r)

	var schedule irrigatorv1alpha1.IrrigationSchedule
	if err := h.k8s.Get(r.Context(), types.NamespacedName{Name: name, Namespace: ns}, &schedule); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}

// executeSchedule triggers an immediate execution of the named schedule,
// bypassing the cron timer.
func (h *handler) executeSchedule(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ns := namespaceFrom(r)

	key := types.NamespacedName{Name: name, Namespace: ns}

	// Verify the schedule exists before executing.
	var schedule irrigatorv1alpha1.IrrigationSchedule
	if err := h.k8s.Get(r.Context(), key, &schedule); err != nil {
		writeError(w, err)
		return
	}

	h.log.InfoContext(r.Context(), "force-executing schedule", "schedule", key,
		"client_cn", clientCN(r))

	if err := h.executor.Execute(r.Context(), key); err != nil {
		h.log.ErrorContext(r.Context(), "force execution failed", "schedule", key, "error", err)
		writeError(w, err)
		return
	}

	// Refresh status after execution.
	if err := h.k8s.Get(r.Context(), key, &schedule); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, schedule)
}

// healthz is the liveness probe — always returns 200.
func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz is the readiness probe — always returns 200 once the server is up.
func (h *handler) readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func namespaceFrom(r *http.Request) string {
	if ns := r.URL.Query().Get("namespace"); ns != "" {
		return ns
	}
	return r.Header.Get("X-Namespace")
}

func clientCN(r *http.Request) string {
	if r.TLS != nil && len(r.TLS.VerifiedChains) > 0 && len(r.TLS.VerifiedChains[0]) > 0 {
		return r.TLS.VerifiedChains[0][0].Subject.CommonName
	}
	return ""
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}
