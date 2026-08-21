package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
)

// maxPullResponseBytes caps how much of an external API's response body we
// read into memory — a misconfigured or malicious external API could
// otherwise return an unbounded body and exhaust memory.
const maxPullResponseBytes = 10 * 1024 * 1024 // 10MB

// MonitorPuller periodically fetches data from external APIs for monitors
// configured with SourceType "api" and SourceConfig.Mode "pull". Same
// polling pattern as Scheduler: one cron entry checks every minute for
// monitors whose NextPullAt is due, instead of one cron entry per monitor.
type MonitorPuller struct {
	cron             *cron.Cron
	monitorRepo      *repository.MonitorRepository
	ingestionService *IngestionService
	jobQueue         *JobQueue
	httpClient       *http.Client
	mu               sync.RWMutex
	running          bool
}

func NewMonitorPuller(monitorRepo *repository.MonitorRepository, ingestionService *IngestionService, jobQueue *JobQueue) *MonitorPuller {
	return &MonitorPuller{
		monitorRepo:      monitorRepo,
		ingestionService: ingestionService,
		jobQueue:         jobQueue,
		httpClient:       newSafeHTTPClient(),
	}
}

// newSafeHTTPClient returns an http.Client whose DialContext refuses to
// connect to private, loopback, or link-local addresses (including the
// 169.254.169.254 cloud metadata range) — PullURL is user-configured, so
// without this check the puller is a textbook SSRF vector into internal
// infrastructure. The check runs at dial time against the resolved IP, not
// just against the hostname, so a DNS answer that changes between lookup
// and connect (DNS rebinding) can't bypass it.
func newSafeHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("resolving %s: %w", host, err)
			}
			for _, ip := range ips {
				if isPrivateOrReservedIP(ip) {
					return nil, fmt.Errorf("refusing to connect to private/reserved address %s", ip)
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}

func isPrivateOrReservedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// Start launches the puller. Blocks until ctx is cancelled — call with `go`.
func (p *MonitorPuller) Start(ctx context.Context) {
	p.cron = cron.New()
	p.cron.AddFunc("* * * * *", func() {
		p.checkAndPull(ctx)
	})

	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	log.Println("Monitor puller started — checking API pull monitors every minute")
	p.cron.Start()

	<-ctx.Done()

	log.Println("Monitor puller stopping...")
	stopCtx := p.cron.Stop()
	<-stopCtx.Done()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
	log.Println("Monitor puller stopped")
}

func (p *MonitorPuller) checkAndPull(ctx context.Context) {
	monitors, err := p.monitorRepo.FindPullMonitors(ctx)
	if err != nil {
		log.Printf("Monitor puller: error finding pull monitors: %v", err)
		return
	}

	now := time.Now()
	for _, m := range monitors {
		if m.SourceConfig.NextPullAt != nil && now.Before(*m.SourceConfig.NextPullAt) {
			continue
		}
		// Reclama el slot ANTES de lanzar el pull: avanza NextPullAt de
		// inmediato (no solo al terminar, en recordResult) para que el
		// siguiente tick de cron no vuelva a seleccionar este monitor
		// mientras el pull en curso todavía no respondió. recordResult
		// vuelve a escribir NextPullAt al final con el valor correcto
		// basado en el momento real de finalización, sobrescribiendo
		// este reclamo optimista.
		claimedNext := p.nextPullTime(m)
		if err := p.monitorRepo.Update(ctx, m.ID, bson.M{"source_config.next_pull_at": claimedNext}); err != nil {
			log.Printf("Monitor puller: failed to claim pull slot for monitor %s: %v", m.ID.Hex(), err)
			continue
		}
		go func(monitor models.Monitor) {
			p.pullOne(ctx, monitor)
		}(m)
	}
}

func (p *MonitorPuller) pullOne(ctx context.Context, monitor models.Monitor) {
	cfg := monitor.SourceConfig
	method := cfg.PullMethod
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.PullURL, nil)
	if err != nil {
		p.recordResult(ctx, monitor, 0, fmt.Errorf("building request: %w", err))
		return
	}

	switch cfg.PullAuthType {
	case models.APIAuthAPIKey:
		req.Header.Set(cfg.PullAuthHeaderName, cfg.PullAuthValue)
	case models.APIAuthBearer:
		req.Header.Set("Authorization", "Bearer "+cfg.PullAuthValue)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.recordResult(ctx, monitor, 0, fmt.Errorf("requesting %s: %w", cfg.PullURL, err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		p.recordResult(ctx, monitor, 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
		return
	}

	count, err := p.ingestionService.IngestJSON(ctx, &monitor, io.LimitReader(resp.Body, maxPullResponseBytes))
	if err != nil {
		p.recordResult(ctx, monitor, 0, fmt.Errorf("ingesting response: %w", err))
		return
	}

	if p.jobQueue != nil {
		_ = p.jobQueue.Enqueue(ctx, EvalJob{MonitorID: monitor.ID.Hex()})
	}

	p.recordResult(ctx, monitor, count, nil)
}

func (p *MonitorPuller) nextPullTime(monitor models.Monitor) time.Time {
	interval := monitor.SourceConfig.PullIntervalMinutes
	if interval < 1 {
		interval = 60
	}
	return time.Now().Add(time.Duration(interval) * time.Minute)
}

// recordResult writes LastPullAt/LastPullStatus/LastPullError and recomputes
// NextPullAt — on both success and failure, so a failing external API
// doesn't get hammered every minute; it waits a full interval before retrying.
func (p *MonitorPuller) recordResult(ctx context.Context, monitor models.Monitor, count int, pullErr error) {
	now := time.Now()
	next := p.nextPullTime(monitor)
	status := "ok"
	errMsg := ""
	if pullErr != nil {
		status = "error"
		errMsg = pullErr.Error()
		log.Printf("Monitor puller: pull failed for monitor %s: %v", monitor.Name, pullErr)
	} else {
		log.Printf("Monitor puller: pulled %d records for monitor %s", count, monitor.Name)
	}

	err := p.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"source_config.last_pull_at":     now,
		"source_config.last_pull_status": status,
		"source_config.last_pull_error":  errMsg,
		"source_config.next_pull_at":     next,
	})
	if err != nil {
		log.Printf("Monitor puller: failed to record pull result for monitor %s: %v", monitor.ID.Hex(), err)
	}
}

// Status reports whether the puller's cron loop is running — same shape as
// Scheduler.Status(), consumed by the /settings endpoint if it's ever added there.
func (p *MonitorPuller) Status() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return map[string]interface{}{"running": p.running}
}
