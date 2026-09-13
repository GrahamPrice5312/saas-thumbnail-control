package thumbnail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProcessRetriesWithFreshMultipartBody(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization header missing")
		}
		if r.Header.Get("Idempotency-Key") != "upload-42:compact" {
			t.Errorf("idempotency key missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 4 {
			t.Errorf("body fields = %v", body)
		}
		for _, field := range []string{"image", "ops", "format", "store"} {
			if _, ok := body[field]; !ok {
				t.Errorf("missing field %q", field)
			}
		}
		ops, ok := body["ops"].([]any)
		if !ok || len(ops) != 1 {
			t.Fatalf("ops = %#v", body["ops"])
		}
		op := ops[0].(map[string]any)
		params := op["params"].(map[string]any)
		if op["op"] != "resize" || params["width"] != float64(320) || params["height"] != float64(180) || params["fit"] != "cover" {
			t.Errorf("resize op = %#v", op)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]string{"message": "retry later"}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"id": "img_42"}, "metadata": map[string]string{}})
	}))
	defer server.Close()

	client := NewImageClient("test-key")
	client.Endpoint = server.URL
	client.Sleep = func(context.Context, time.Duration) error { return nil }
	env, err := client.Process(context.Background(), []byte("image-bytes"), "product.png", Variant{Name: "compact", Width: 320, Height: 180}, "upload-42:compact")
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK || calls != 2 {
		t.Fatalf("ok = %v, calls = %d", env.OK, calls)
	}
}
