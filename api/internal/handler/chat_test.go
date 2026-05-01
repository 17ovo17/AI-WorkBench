package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestChatStreamReturnsAfterDoneWithoutUpstreamEOF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	doneWritten := make(chan struct{})
	releaseUpstream := make(chan struct{})
	upstream := newBlockingDoneChatUpstream(t, doneWritten, releaseUpstream)
	defer upstream.Close()
	defer close(releaseUpstream)
	configureChatTestProvider(t, upstream.URL+"/v1")
	t.Cleanup(viper.Reset)

	sessionID := "chat-stream-test-" + time.Now().Format("20060102150405.000000000")
	defer store.DeleteChatSession(sessionID)
	w, finished := performStreamingChatRequest(t, sessionID)

	assertSignal(t, doneWritten, "upstream did not write done marker")
	assertSignal(t, finished, "chat handler did not return after [DONE]")
	assertStreamingChatResponse(t, w)
}

func TestChatStreamWritesDoneWhenUpstreamClosesWithoutDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer upstream.Close()
	configureChatTestProvider(t, upstream.URL+"/v1")
	t.Cleanup(viper.Reset)

	sessionID := "chat-stream-eof-test-" + time.Now().Format("20060102150405.000000000")
	defer store.DeleteChatSession(sessionID)
	w, finished := performStreamingChatRequest(t, sessionID)

	assertSignal(t, finished, "chat handler did not return after upstream EOF")
	assertStreamingChatResponse(t, w)
}

func newBlockingDoneChatUpstream(t *testing.T, doneWritten, releaseUpstream chan struct{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n"))
		_, _ = w.Write([]byte("data: [DONE]\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		close(doneWritten)
		<-releaseUpstream
	}))
}

func configureChatTestProvider(t *testing.T, baseURL string) {
	t.Helper()
	viper.Reset()
	viper.Set("ai.base_url", baseURL)
	viper.Set("ai.api_key", "test-key")
}

func performStreamingChatRequest(t *testing.T, sessionID string) (*httptest.ResponseRecorder, <-chan struct{}) {
	t.Helper()
	body, err := json.Marshal(model.ChatRequest{
		SessionID: sessionID,
		Model:     "test-model",
		Messages:  []model.Message{{Role: "user", Content: "ping"}},
		Stream:    true,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	r := gin.New()
	r.POST("/api/v1/chat", Chat)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		r.ServeHTTP(w, req)
		close(finished)
	}()
	return w, finished
}

func assertSignal(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal(message)
	}
}

func assertStreamingChatResponse(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "data: [DONE]") {
		t.Fatalf("response missing done marker: %q", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "pong") {
		t.Fatalf("response missing streamed content: %q", w.Body.String())
	}
}
