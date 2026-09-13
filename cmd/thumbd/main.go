package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	thumbnail "example.com/saas-thumbnail-control"
)

type server struct {
	registry *thumbnail.Registry
	images   *thumbnail.ImageClient
}

type tenantRequest struct {
	ID string `json:"id"`
}

type stateRequest struct {
	State thumbnail.AccountState `json:"state"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &server{registry: thumbnail.NewRegistry(), images: thumbnail.NewImageClient(key)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/tenants", s.onboard)
	mux.HandleFunc("POST /admin/tenants/{id}/state", s.changeState)
	mux.HandleFunc("POST /tenants/{id}/thumbnails", s.createThumbnails)
	log.Printf("thumbd listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) onboard(w http.ResponseWriter, r *http.Request) {
	var input tenantRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.registry.Onboard(input.ID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": input.ID, "state": thumbnail.StateOnboarding})
}

func (s *server) changeState(w http.ResponseWriter, r *http.Request) {
	var input stateRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	id := r.PathValue("id")
	if err := s.registry.Transition(id, input.State); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "state": input.State})
}

func (s *server) createThumbnails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	plan, err := s.registry.ThumbnailPlan(id)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image is required")
		return
	}
	defer file.Close()
	image, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read image")
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key is required")
		return
	}

	results := make(map[string]json.RawMessage, len(plan))
	for _, variant := range plan {
		env, err := s.images.Process(r.Context(), image, header.Filename, variant, requestID+":"+variant.Name)
		if err != nil {
			var apiErr *thumbnail.APIError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
				writeJSON(w, apiErr.Status, map[string]any{"error": apiErr.Problem})
				return
			}
			writeError(w, http.StatusBadGateway, "image processing request failed")
			return
		}
		results[variant.Name] = env.Data
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tenant_id": id, "thumbnails": results})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
