package integration

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type assetsTestTransport func(*http.Request) (*http.Response, error)

func (f assetsTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type assetsTestBody struct {
	io.ReadCloser
	started chan struct{}
}

func (b *assetsTestBody) Read(p []byte) (int, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	return b.ReadCloser.Read(p)
}

func TestDoc2TextHonorsCancellation(t *testing.T) {
	for _, phase := range []string{"headers", "body"} {
		t.Run(phase, func(t *testing.T) {
			started := make(chan struct{})
			requestCanceled := make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if phase == "body" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				close(started)
				select {
				case <-r.Context().Done():
					close(requestCanceled)
				case <-release:
				}
			}))
			defer func() {
				close(release)
				server.Close()
			}()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service := NewAssetsService(server.URL).(*DefaultAssetsService)
			waitForStart := started
			if phase == "body" {
				reading := make(chan struct{})
				service.client.Transport = assetsTestTransport(func(r *http.Request) (*http.Response, error) {
					response, err := http.DefaultTransport.RoundTrip(r)
					if err == nil {
						response.Body = &assetsTestBody{ReadCloser: response.Body, started: reading}
					}
					return response, err
				})
				waitForStart = reading
			}
			result := make(chan error, 1)
			go func() {
				_, err := service.Doc2Text(ctx, "fileuid1")
				result <- err
			}()
			timeout := time.NewTimer(3 * time.Second)
			defer timeout.Stop()
			select {
			case <-waitForStart:
			case <-timeout.C:
				t.Fatal("document request did not start")
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("expected cancellation error, got %v", err)
				}
			case <-timeout.C:
				t.Fatal("Doc2Text did not return after cancellation")
			}
			select {
			case <-requestCanceled:
			case <-timeout.C:
				t.Fatal("cancellation did not reach the HTTP server")
			}
		})
	}
}

func TestDoc2TextReturnsDocumentText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/doc2text/fileuid1" {
			t.Errorf("unexpected document request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte("document text"))
	}))
	defer server.Close()
	text, err := NewAssetsService(server.URL).Doc2Text(context.Background(), "fileuid1")
	if err != nil || text != "document text" {
		t.Fatalf("unexpected document response: text=%q err=%v", text, err)
	}
}
