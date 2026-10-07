package api

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/wyw14/cry-161/internal/model"
	"github.com/wyw14/cry-161/internal/service"
	"net/http"
	"strconv"
)

type layerRequest struct {
	RunID                string  `json:"run_id"`
	Number               int     `json:"number"`
	Material             string  `json:"material"`
	TargetNanometers     float64 `json:"target_nm"`
	RateNanometers       float64 `json:"rate_nm_s"`
	OutlierThresholdNm   float64 `json:"outlier_threshold_nm,omitempty"`
	ConsecutiveAnomalies int     `json:"consecutive_anomalies,omitempty"`
	RecoveryWindow       int     `json:"recovery_window,omitempty"`
}

func runLayer(svc *service.Service, runtime *service.Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input layerRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		runID, ok := model.ParseID(input.RunID)
		if !ok {
			http.Error(w, "invalid run id", http.StatusBadRequest)
			return
		}
		layer := model.Layer{Number: input.Number, Material: input.Material, TargetNanometers: input.TargetNanometers, RateNanometers: input.RateNanometers, OutlierThresholdNm: input.OutlierThresholdNm, ConsecutiveAnomalies: input.ConsecutiveAnomalies, RecoveryWindow: input.RecoveryWindow}
		if err := svc.RunLayer(r.Context(), runtime, runID, layer); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, map[string]any{"status": "completed", "run_id": runID, "layer": layer.Number})
	}
}

func recoverRun(svc *service.Service, runtime *service.Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID, ok := model.ParseID(chi.URLParam(r, "runID"))
		if !ok {
			http.Error(w, "invalid run id", http.StatusBadRequest)
			return
		}
		report := svc.Recover(r.Context(), runtime, runID)
		if !report.Recovered {
			w.WriteHeader(http.StatusConflict)
		}
		writeJSON(w, report)
	}
}

func releaseRun(svc *service.Service, runtime *service.Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID, ok := model.ParseID(chi.URLParam(r, "runID"))
		if !ok {
			http.Error(w, "invalid run id", http.StatusBadRequest)
			return
		}
		operator := r.Header.Get("X-Operator")
		if operator == "" {
			operator = "local-operator"
		}
		record, err := svc.Release(runID, runtime, operator)
		if err != nil {
			w.WriteHeader(http.StatusConflict)
		}
		writeJSON(w, record)
	}
}

func timeline(svc *service.Service, runtime *service.Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = svc
		runID, ok := model.ParseID(chi.URLParam(r, "runID"))
		if !ok {
			http.Error(w, "invalid run id", http.StatusBadRequest)
			return
		}
		limit := 100
		if value := r.URL.Query().Get("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 1000 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsed
		}
		entries := runtime.Timeline.Entries(runID)
		if len(entries) > limit {
			entries = entries[len(entries)-limit:]
		}
		writeJSON(w, entries)
	}
}
