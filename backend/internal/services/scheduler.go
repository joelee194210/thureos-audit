package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
)

// Scheduler runs rule evaluations on a per-rule cron schedule.
// It polls every minute for due rules and processes day-by-day backlogs.
type Scheduler struct {
	cron        *cron.Cron
	ruleRepo    *repository.RuleRepository
	monitorRepo *repository.MonitorRepository
	ruleEngine  *RuleEngine
	mu          sync.RWMutex
	running     bool
	rulesActive int
}

func NewScheduler(
	ruleRepo *repository.RuleRepository,
	monitorRepo *repository.MonitorRepository,
	ruleEngine *RuleEngine,
) *Scheduler {
	return &Scheduler{
		ruleRepo:    ruleRepo,
		monitorRepo: monitorRepo,
		ruleEngine:  ruleEngine,
	}
}

// Start launches the scheduler. It runs a single cron entry every minute
// that polls for due rules. Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	s.cron = cron.New()

	s.cron.AddFunc("* * * * *", func() {
		s.checkAndFireRules(ctx)
	})

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	log.Println("Scheduler started — checking rules every minute")
	s.cron.Start()

	<-ctx.Done()

	log.Println("Scheduler stopping...")
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	log.Println("Scheduler stopped")
}

// checkAndFireRules finds all scheduled rules that are due and evaluates them
func (s *Scheduler) checkAndFireRules(ctx context.Context) {
	rules, err := s.ruleRepo.FindScheduled(ctx)
	if err != nil {
		log.Printf("Scheduler: error finding scheduled rules: %v", err)
		return
	}

	s.mu.Lock()
	s.rulesActive = len(rules)
	s.mu.Unlock()

	now := time.Now()
	for _, rule := range rules {
		// Skip if not yet due
		if rule.Schedule.NextRun != nil && now.Before(*rule.Schedule.NextRun) {
			continue
		}

		// Launch evaluation in a goroutine so we don't block the ticker
		go func(r models.Rule) {
			s.evaluateRuleWithBacklog(ctx, r)
		}(rule)
	}
}

// evaluateRuleWithBacklog processes day-by-day from lastEvaluated+1 to yesterday.
// If lastEvaluated is nil, it only processes yesterday.
func (s *Scheduler) evaluateRuleWithBacklog(ctx context.Context, rule models.Rule) {
	monitor, err := s.monitorRepo.FindByID(ctx, rule.MonitorID)
	if err != nil {
		log.Printf("Scheduler: monitor not found for rule %s: %v", rule.ID.Hex(), err)
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.Add(-24 * time.Hour)

	// Determine start date: day after last evaluation, or yesterday if never evaluated
	var startDate time.Time
	if rule.Schedule.LastEvaluated != nil {
		lastEval := *rule.Schedule.LastEvaluated
		startDate = time.Date(lastEval.Year(), lastEval.Month(), lastEval.Day(), 0, 0, 0, 0, lastEval.Location())
		startDate = startDate.Add(24 * time.Hour) // day after last eval
	} else {
		startDate = yesterday
	}

	// Don't process future dates — stop at yesterday
	if startDate.After(yesterday) {
		startDate = yesterday
	}

	totalAlerts := 0
	daysProcessed := 0

	// Process day by day
	for day := startDate; !day.After(yesterday); day = day.Add(24 * time.Hour) {
		dayEnd := day.Add(24 * time.Hour)
		alerts, err := s.ruleEngine.EvaluateRuleForDateRange(ctx, monitor, rule, day, dayEnd)
		if err != nil {
			log.Printf("Scheduler: error evaluating rule %s for %s: %v",
				rule.ID.Hex(), day.Format("2006-01-02"), err)
			continue
		}
		totalAlerts += len(alerts)
		daysProcessed++
	}

	// Calculate next run using the cron expression
	nextRun := CalculateNextRun(rule.Schedule.CronExpr)

	// Update schedule state
	if err := s.ruleRepo.UpdateScheduleState(ctx, rule.ID, yesterday, nextRun); err != nil {
		log.Printf("Scheduler: error updating schedule state for rule %s: %v", rule.ID.Hex(), err)
	}

	log.Printf("Scheduler: rule '%s' — %d days processed, %d alerts generated, next run: %s",
		rule.Name, daysProcessed, totalAlerts, nextRun.Format("2006-01-02 15:04"))
}

// CalculateNextRun parses a cron expression and returns the next scheduled time
func CalculateNextRun(cronExpr string) time.Time {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		// Fallback: next day at 6 AM
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day()+1, 6, 0, 0, 0, now.Location())
	}
	return schedule.Next(time.Now())
}

// Status returns current scheduler state for the settings endpoint
func (s *Scheduler) Status() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"running":     s.running,
		"rulesActive": s.rulesActive,
	}
}
