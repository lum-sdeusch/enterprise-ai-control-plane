// eacp-adapter is the policy enforcement orchestrator of the Enterprise AI Control Plane.
//
// It serves agentgateway's ExtMCP gRPC hook (every MCP request and response phase), the LLM
// prompt-guard webhook and routing callout, the approval page, and health endpoints. See
// docs/design.md section 5 and the package comments under internal/.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	gen "github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/gen/extmcp"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/approval"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/audit"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/config"
	extmcpsrv "github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/extmcp"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/guardrails"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/llm"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/pipeline"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/policy"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/taint"
	"github.com/lum-sdeusch/enterprise-ai-control-plane/adapter/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("eacp-adapter exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.FromEnv()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Setup(ctx, os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), "eacp-adapter", config.Version)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	pol := policy.NewHTTPClient(cfg.OPAURL, cfg.OPATimeout)
	gr := guardrails.NewHTTPClient(cfg.GuardrailsURL, cfg.GuardrailsTimeout)

	var ts taint.Store
	var as approval.Store
	if cfg.ValkeyAddr != "" {
		vk := taint.NewValkey(cfg.ValkeyAddr, cfg.TaintTTL, cfg.RouteCacheTTL)
		if err := vk.Ping(ctx); err != nil {
			return fmt.Errorf("valkey %s: %w", cfg.ValkeyAddr, err)
		}
		ts = vk
		as = approval.NewValkey(cfg.ValkeyAddr, cfg.ApprovalTTL)
		log.Info("state store: valkey", "addr", cfg.ValkeyAddr)
	} else {
		ts = taint.NewMemory()
		as = approval.NewMemory()
		log.Warn("state store: in-memory (single replica only)")
	}

	var aud audit.Writer = audit.NewStdout()
	if cfg.ClickHouseURL != "" {
		aud = audit.Multi{audit.NewClickHouse(cfg.ClickHouseURL, cfg.ClickHouseUser, cfg.ClickHousePassword, cfg.AuditTimeout), audit.NewStdout()}
		log.Info("audit store: clickhouse", "url", cfg.ClickHouseURL, "required", cfg.AuditRequired)
	} else {
		log.Warn("audit store: stdout only", "required", cfg.AuditRequired)
	}

	agents := &agentClientsCache{pol: pol, log: log}
	agents.refresh(ctx)
	go agents.loop(ctx, time.Minute)

	p := pipeline.New(pipeline.Deps{
		Cfg: cfg, Policy: pol, Guardrails: gr, Taint: ts, Approvals: as, Audit: aud,
		AgentClients: agents.get,
	})

	// gRPC: the ExtMCP hook.
	grpcServer := grpc.NewServer()
	gen.RegisterExtMcpServer(grpcServer, &extmcpsrv.Server{P: p, Log: log})
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, hs)
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}

	// HTTP: LLM hooks, approvals, health.
	mux := http.NewServeMux()
	(&llm.Handler{P: p, Log: log}).Register(mux)
	web, err := approval.NewWeb(ctx, cfg, as, aud, log)
	if err != nil {
		return err
	}
	web.Register(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		rctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if _, err := pol.AgentClients(rctx); err != nil {
			http.Error(w, "policy not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	errCh := make(chan error, 2)
	go func() {
		log.Info("grpc listening", "addr", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(lis)
	}()
	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-errCh:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.GracefulStop()
	return httpServer.Shutdown(shutdownCtx)
}

// agentClientsCache keeps the client-id to agent-name map from policy data fresh so identity
// resolution never waits on OPA in the request path.
type agentClientsCache struct {
	pol policy.Client
	log *slog.Logger
	mu  sync.RWMutex
	m   map[string]string
}

func (c *agentClientsCache) get() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.m
}

func (c *agentClientsCache) refresh(ctx context.Context) {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	m, err := c.pol.AgentClients(rctx)
	if err != nil {
		c.log.Warn("agent clients refresh failed", "err", err)
		return
	}
	c.mu.Lock()
	c.m = m
	c.mu.Unlock()
}

func (c *agentClientsCache) loop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.refresh(ctx)
		}
	}
}
