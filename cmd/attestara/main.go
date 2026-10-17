package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

type health struct {
	SchemaVersion int    `json:"schemaVersion"`
	Status        string `json:"status"`
	Scope         string `json:"scope"`
	Version       string `json:"version"`
	GoVersion     string `json:"goVersion"`
}

func currentHealth() health { return health{1, "ok", "engine-process", "0.0.1", runtime.Version()} }

func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/healthz" {
			w.WriteHeader(404)
			io.WriteString(w, `{"error":"not_found"}`)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(405)
			io.WriteString(w, `{"error":"method_not_allowed"}`)
			return
		}
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(currentHealth())
		}
	})
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("attestara", flag.ContinueOnError)
	flags.SetOutput(stderr)
	address := flags.String("listen", "127.0.0.1:0", "loopback listen address")
	version := flags.Bool("version", false, "print engine process version as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments; user source execution is unsupported")
	}
	if *version {
		return json.NewEncoder(stdout).Encode(currentHealth())
	}
	host, port, err := net.SplitHostPort(*address)
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || number < 0 || number > 65535 || (host != "127.0.0.1" && host != "::1") {
		return errors.New("listen address must be numeric loopback with port 0..65535")
	}
	if ctx.Err() != nil {
		return nil
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 16384}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	if err := json.NewEncoder(stdout).Encode(struct {
		Event   string `json:"event"`
		Address string `json:"address"`
	}{"listening", listener.Addr().String()}); err != nil {
		server.Close()
		return err
	}
	select {
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		// https://pkg.go.dev/net/http#Server.Shutdown: wait for shutdown before exiting.
		deadline, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(deadline); err != nil {
			server.Close()
			return err
		}
	}
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
