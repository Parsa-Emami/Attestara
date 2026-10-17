package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestHealthContractAndAdmission(t *testing.T) {
	var fixture struct {
		Health health `json:"health"`
	}
	data, err := os.ReadFile("../../tests/fixtures/attestara/C03/positive.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "HEAD", "POST"} {
		rec := httptest.NewRecorder()
		handler().ServeHTTP(rec, httptest.NewRequest(method, "/healthz", nil))
		if method == "POST" {
			if rec.Code != 405 {
				t.Fatal(rec.Code)
			}
			continue
		}
		if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(rec)
		}
		if method == "HEAD" {
			if rec.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
			continue
		}
		var actual health
		if err := json.Unmarshal(rec.Body.Bytes(), &actual); err != nil {
			t.Fatal(err)
		}
		if actual != fixture.Health {
			t.Fatalf("health: %+v", actual)
		}
	}
	rec := httptest.NewRecorder()
	handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/run", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatal("unexpected execution route")
	}
}

func TestRealListenerShutdownAndOccupiedPort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, writer, io.Discard); writer.Close() }()
	var event struct{ Event, Address string }
	if err := json.NewDecoder(reader).Decode(&event); err != nil {
		t.Fatal(err)
	}
	if event.Event != "listening" {
		t.Fatal(event)
	}
	client := &http.Client{Timeout: time.Second}
	res, err := client.Get("http://" + event.Address + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	if err := run(context.Background(), []string{"--listen", event.Address}, io.Discard, io.Discard); err == nil {
		t.Fatal("occupied port accepted")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("shutdown deadline")
	}
	client.CloseIdleConnections()
	listener, err := net.Listen("tcp", event.Address)
	if err != nil {
		t.Fatal("listener retained after shutdown", err)
	}
	listener.Close()
}

func TestVersionAndInvalidLaunch(t *testing.T) {
	for _, args := range [][]string{{"--listen", "0.0.0.0:0"}, {"--listen", "localhost:0"}, {"--listen", "127.0.0.1:65536"}, {"payload.sh"}, {"--unknown"}} {
		if err := run(context.Background(), args, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := run(context.Background(), []string{"--version"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}
