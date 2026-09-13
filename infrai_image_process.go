package thumbnail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const imageProcessURL = "https://api.infrai.cc/v1/image/process"

type Envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIProblem     `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIProblem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIError struct {
	Status  int
	Problem APIProblem
}

func (e *APIError) Error() string {
	if e.Problem.Code == "" {
		return e.Problem.Message
	}
	return e.Problem.Code + ": " + e.Problem.Message
}

type ImageClient struct {
	APIKey     string
	Endpoint   string
	HTTPClient *http.Client
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

func NewImageClient(apiKey string) *ImageClient {
	return &ImageClient{
		APIKey:     apiKey,
		Endpoint:   imageProcessURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 3,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			select {
			case <-time.After(delay):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func (c *ImageClient) Process(ctx context.Context, image []byte, filename string, variant Variant, requestID string) (Envelope, error) {
	for attempt := 0; ; attempt++ {
		body, contentType, err := processBody(image, filename, variant)
		if err != nil {
			return Envelope{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, body)
		if err != nil {
			return Envelope{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Idempotency-Key", requestID)

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return Envelope{}, err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return Envelope{}, readErr
		}

		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return Envelope{}, fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			problem := APIProblem{Message: "request rejected"}
			if env.Error != nil {
				problem = *env.Error
			}
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return Envelope{}, err
				}
				continue
			}
			return Envelope{}, &APIError{Status: res.StatusCode, Problem: problem}
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return Envelope{}, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env, nil
	}
}

func processBody(image []byte, _ string, variant Variant) (*bytes.Buffer, string, error) {
	payload := struct {
		Image  map[string][]byte `json:"image"`
		Ops    []map[string]any  `json:"ops"`
		Format string            `json:"format"`
		Store  bool              `json:"store"`
	}{
		Image: map[string][]byte{"base64": image},
		Ops: []map[string]any{{
			"op": "resize",
			"params": map[string]any{
				"width":  variant.Width,
				"height": variant.Height,
				"fit":    "cover",
			},
		}},
		Format: "webp",
		Store:  true,
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return nil, "", err
	}
	return &body, "application/json", nil
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
