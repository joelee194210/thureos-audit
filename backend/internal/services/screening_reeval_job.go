package services

import (
	"context"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

// ScreeningReevalJob reabre descartes de screening vencidos (whitelist
// expirado): los vuelve a correr contra Watchman y, si siguen dando
// match/review, avisa — un descarte de hace 180+ días no debe quedar
// congelado en falso positivo para siempre. Mismo patrón de polling que
// SLAEscalationJob (Tarea 5 del roadmap P1).
type ScreeningReevalJob struct {
	cron     *cron.Cron
	repo     *repository.ScreeningRepository
	client   *WatchmanClient
	notifier *NotificationService
	mu       sync.RWMutex
	running  bool
}

func NewScreeningReevalJob(repo *repository.ScreeningRepository, client *WatchmanClient, notifier *NotificationService) *ScreeningReevalJob {
	return &ScreeningReevalJob{repo: repo, client: client, notifier: notifier}
}

// Start launches the job. Blocks until ctx is cancelled — call with `go`.
func (j *ScreeningReevalJob) Start(ctx context.Context) {
	j.cron = cron.New()
	// Diario alcanza: un vencimiento de 180 días no necesita chequeo por
	// minuto como el de SLA (que vence en horas).
	if _, err := j.cron.AddFunc("0 4 * * *", func() {
		j.checkAndReeval(ctx)
	}); err != nil {
		log.Printf("screening reeval: registrando chequeo: %v", err)
	}

	j.mu.Lock()
	j.running = true
	j.mu.Unlock()

	log.Println("Screening reeval job started — checking expired whitelist daily at 04:00")
	j.cron.Start()

	<-ctx.Done()

	log.Println("Screening reeval job stopping...")
	stopCtx := j.cron.Stop()
	<-stopCtx.Done()

	j.mu.Lock()
	j.running = false
	j.mu.Unlock()
	log.Println("Screening reeval job stopped")
}

func (j *ScreeningReevalJob) checkAndReeval(ctx context.Context) {
	expired, err := j.repo.FindExpiredWhitelist(ctx)
	if err != nil {
		log.Printf("screening reeval: buscando whitelist vencido: %v", err)
		return
	}
	for _, result := range expired {
		matches, searchErr := j.client.Search(ctx, result.Query, WatchmanSearchOptions{Limit: 20, MinMatch: 0.5})
		if searchErr != nil {
			// Proveedor caído: no reabrir el descarte del analista por un
			// error de infraestructura — se reintenta en el próximo corrido
			// diario, sin tocar whitelist_expires_at.
			log.Printf("screening reeval: Watchman no disponible para %s, se reintenta mañana: %v", result.ID.Hex(), searchErr)
			continue
		}
		outcome := ClassifyScreeningOutcome(matches, searchErr)

		if err := j.repo.UpdateAfterReeval(ctx, result.ID, outcome.Status, outcome.StrongestMatch, matches); err != nil {
			log.Printf("screening reeval: actualizando %s: %v", result.ID.Hex(), err)
			continue
		}

		stillFlagged := outcome.Status == models.ScreeningMatch || outcome.Status == models.ScreeningReview
		if stillFlagged && j.notifier != nil {
			result.Status = outcome.Status
			result.Matches = matches
			if err := j.notifier.TriggerScreeningStale(result); err != nil {
				log.Printf("screening reeval: avisando %s: %v", result.ID.Hex(), err)
			}
		}
	}
}

func (j *ScreeningReevalJob) Status() map[string]interface{} {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return map[string]interface{}{"running": j.running}
}
