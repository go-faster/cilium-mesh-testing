package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	ListenAddr string
	TargetAddr string
	Timeout    time.Duration
	Workers    int
	Sleep      time.Duration
}

func (cfg *Config) RegisterFlags(fset *flag.FlagSet) {
	fset.StringVar(&cfg.ListenAddr, "listen-addr", ":8080", "http listen address (for /metrics)")
	fset.StringVar(&cfg.TargetAddr, "target-addr", "", "deathstar http address")
	fset.DurationVar(&cfg.Timeout, "timeout", time.Second*5, "deathstar request timeout")
	fset.IntVar(&cfg.Workers, "n", 1, "number of workers")
	fset.DurationVar(&cfg.Sleep, "sleep", 0, "sleep duration between requests")
}

func (cfg *Config) Print() {
	fmt.Printf("cfg:\n")
	fmt.Printf("  listenAddress:  %q\n", cfg.ListenAddr)
	fmt.Printf("  targetAddress:  %q\n", cfg.TargetAddr)
	fmt.Printf("  requestTimeout: %q\n", cfg.Timeout)
	fmt.Printf("  workersNumber:  %d\n", cfg.Workers)
	fmt.Printf("  sleepDuration:  %s\n", cfg.Sleep)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var cfg Config
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()
	cfg.Print()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg Config) error {
	tf := NewTieFighter(cfg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", tf.MetricsHandler())
	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return tf.Run(ctx) })
	g.Go(func() error {
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	g.Go(func() error {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
		defer cancel()

		return srv.Shutdown(ctx)
	})

	return g.Wait()
}

type TieFighter struct {
	cfg        Config
	httpClient *http.Client

	promRegistry *prometheus.Registry
	promStats    *prometheus.CounterVec
	promErrors   prometheus.Counter
}

func NewTieFighter(cfg Config) *TieFighter {
	stats := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tiefighter_requests",
	}, []string{"cluster", "pod"})
	errors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tiefighter_errors",
	})

	registry := prometheus.NewRegistry()
	registry.MustRegister(stats)
	registry.MustRegister(errors)

	return &TieFighter{
		promRegistry: registry,
		promStats:    stats,
		promErrors:   errors,

		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

func (tf *TieFighter) Run(ctx context.Context) error {
	if err := tf.sendRequest(ctx); err != nil {
		return fmt.Errorf("ping server: %w", err)
	}

	var g errgroup.Group
	for i := 0; i < tf.cfg.Workers; i++ {
		g.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()

				default:
					if tf.cfg.Sleep > 0 {
						time.Sleep(tf.cfg.Sleep)
					}

					if err := tf.sendRequest(ctx); err != nil {
						tf.promErrors.Inc()
						fmt.Printf("Request failed: %s\n", err)
						continue
					}
				}
			}
		})
	}

	return g.Wait()
}

func (tf *TieFighter) sendRequest(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tf.cfg.TargetAddr, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := tf.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var response struct {
		Cluster string `json:"cluster"`
		Pod     string `json:"pod"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	tf.promStats.With(prometheus.Labels{
		"cluster": response.Cluster,
		"pod":     response.Pod,
	}).Inc()

	return nil
}

func (tf *TieFighter) MetricsHandler() http.Handler {
	return promhttp.InstrumentMetricHandler(
		tf.promRegistry,
		promhttp.HandlerFor(tf.promRegistry, promhttp.HandlerOpts{}),
	)
}
