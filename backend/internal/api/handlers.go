package api

import (
	"backend/internal/queue"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/guregu/dynamo/v2"
)

var validate = validator.New()

type handler struct {
	q  *queue.DataQueue
	db *dynamo.Table
}

type shortReq struct {
	URL string `json:"longurl" validate:"required,url"`
}

type urlMap struct {
	ID  string `dynamo:"id"`
	URL string `dynamo:"original_url"`
}

func (h *handler) shorten(w http.ResponseWriter, r *http.Request) {
	var req shortReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(req); err != nil {
		http.Error(w, "bad url", http.StatusBadRequest)
		return
	}

	select {
	case id := <-h.q.Pipe:
		mapping := urlMap{ID: id, URL: req.URL}

		if err := h.db.Put(mapping).Run(r.Context()); err != nil {
			slog.Error("dynamo put failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"redirect_id": id})

	default:
		// Queue empty - no IDs available to assign
		slog.Warn("worker pipe empty")
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}
}

func (h *handler) redirect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || len(id) > 20 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var res urlMap
	err := h.db.Get("id", id).One(r.Context(), &res)
	if err != nil {
		if errors.Is(err, dynamo.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		slog.Error("dynamo get failed", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, res.URL, http.StatusMovedPermanently)
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
