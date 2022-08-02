package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"text/template"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(2)
	}
}

func run(ctx context.Context) (err error) {
	var (
		addr                = strEnv("HTTP_ADDRESS", ":8080")
		responseTemplate    = strEnv("RESPONSE_TEMPLATE", "hello\n")
		responseDelay       = durEnv("RESPONSE_DELAY", 0)
		gracePeriod         = durEnv("GRACE_PERIOD", time.Minute)
		signalReactionDelay = durEnv("SIGNAL_REACTION_DELAY", 0)
	)

	fmt.Printf("cfg:\n")
	fmt.Printf("  httpAddress:         %q\n", addr)
	fmt.Printf("  responseTemplate:    %q\n", responseTemplate)
	fmt.Printf("  responseDelay:       %s\n", responseDelay)
	fmt.Printf("  gracePeriod:         %s\n", gracePeriod)
	fmt.Printf("  signalReactionDelay: %s\n", signalReactionDelay)

	tmpl, err := template.New("message").
		Funcs(template.FuncMap{
			"env": os.Getenv,
		}).Parse(responseTemplate)
	if err != nil {
		return fmt.Errorf("parse response template: %w", err)
	}

	terminationSignalReceived := int64(0)
	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.LoadInt64(&terminationSignalReceived) == 1 {
				fmt.Println("WARNING: Incoming request in shutdown phase")
			}

			time.Sleep(responseDelay)
			tmpl.Execute(w, nil)
		}),
	}

	var g errgroup.Group
	g.Go(func() error { return srv.ListenAndServe() })
	g.Go(func() error {
		<-ctx.Done()

		fmt.Println("Termination signal received")
		atomic.StoreInt64(&terminationSignalReceived, 1)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracePeriod)
		defer cancel()

		if signalReactionDelay != 0 {
			fmt.Printf("Shutdown after %s...\n", signalReactionDelay)
			time.Sleep(signalReactionDelay)
		}

		fmt.Println("Shutting down...")
		return srv.Shutdown(shutdownCtx)
	})

	err = g.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func strEnv(env string, def string) string {
	v, ok := os.LookupEnv(env)
	if !ok {
		return def
	}

	return v
}

func durEnv(env string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(env)
	if !ok {
		return def
	}

	dur, err := time.ParseDuration(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse env %q: %s\n", env, err)
		os.Exit(2)
	}

	return dur
}
