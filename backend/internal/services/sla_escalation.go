package services

import (
	"context"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/thureos/compliance/internal/repository"
)

// SLAEscalationJob avisa (no transiciona status) cuando el plazo de SLA de
// un caso vence. Mismo patrón de polling que Scheduler/MonitorPuller: un
// cron entry corre cada minuto en vez de uno por caso.
type SLAEscalationJob struct {
	cron        *cron.Cron
	redFlagRepo *repository.RedFlagRepository
	notifier    *NotificationService
	mu          sync.RWMutex
	running     bool
}

func NewSLAEscalationJob(redFlagRepo *repository.RedFlagRepository, notifier *NotificationService) *SLAEscalationJob {
	return &SLAEscalationJob{redFlagRepo: redFlagRepo, notifier: notifier}
}

// Start launches the job. Blocks until ctx is cancelled — call with `go`.
func (j *SLAEscalationJob) Start(ctx context.Context) {
	j.cron = cron.New()
	if _, err := j.cron.AddFunc("* * * * *", func() {
		j.checkAndEscalate(ctx)
	}); err != nil {
		log.Printf("sla escalation: registrando chequeo: %v", err)
	}

	j.mu.Lock()
	j.running = true
	j.mu.Unlock()

	log.Println("SLA escalation job started — checking overdue cases every minute")
	j.cron.Start()

	<-ctx.Done()

	log.Println("SLA escalation job stopping...")
	stopCtx := j.cron.Stop()
	<-stopCtx.Done()

	j.mu.Lock()
	j.running = false
	j.mu.Unlock()
	log.Println("SLA escalation job stopped")
}

func (j *SLAEscalationJob) checkAndEscalate(ctx context.Context) {
	overdue, err := j.redFlagRepo.FindOverdueUnnotified(ctx)
	if err != nil {
		log.Printf("sla escalation: buscando casos vencidos: %v", err)
		return
	}
	for _, rf := range overdue {
		// Solo se marca como notificado si el aviso salió de verdad — si
		// falla (sin canales configurados, cola caída), se reintenta en el
		// próximo tick en vez de perder el aviso en silencio.
		if err := j.notifier.TriggerSLABreach(rf); err != nil {
			log.Printf("sla escalation: avisando %s: %v", rf.ID.Hex(), err)
			continue
		}
		if err := j.redFlagRepo.MarkEscalationNotified(ctx, rf.ID); err != nil {
			log.Printf("sla escalation: marcando %s como notificado: %v", rf.ID.Hex(), err)
		}
	}
}

func (j *SLAEscalationJob) Status() map[string]interface{} {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return map[string]interface{}{"running": j.running}
}
