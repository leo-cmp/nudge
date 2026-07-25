package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/leomaciel/nudge/internal/db"
	"github.com/leomaciel/nudge/internal/telegram"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	database   *db.DB
	notifier   *telegram.Notifier
	cronParser cron.Parser
	interval   time.Duration
}

func NewScheduler(database *db.DB, notifier *telegram.Notifier) *Scheduler {
	return &Scheduler{
		database:   database,
		notifier:   notifier,
		cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		interval:   15 * time.Second,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Println("[Scheduler] Started background worker loop")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Scheduler] Stopping worker loop")
			return
		case now := <-ticker.C:
			s.processDueReminders(now)
		}
	}
}

func (s *Scheduler) processDueReminders(now time.Time) {
	reminders, err := s.database.GetPendingDueReminders(now)
	if err != nil {
		log.Printf("[Scheduler] Error fetching pending reminders: %v\n", err)
		return
	}

	for _, r := range reminders {
		log.Printf("[Scheduler] Processing reminder %s (type: %s, msg: %q)\n", r.ID, r.Type, r.Message)

		// Send notification via Telegram
		err := s.notifier.SendNotification(r.Message)
		if err != nil {
			log.Printf("[Scheduler] Failed to send Telegram notification for reminder %s: %v\n", r.ID, err)
			continue
		}

		// Update database based on type
		if r.Type == db.TypeRecurring && r.CronPattern != nil && *r.CronPattern != "" {
			schedule, err := s.cronParser.Parse(*r.CronPattern)
			if err != nil {
				log.Printf("[Scheduler] Error parsing cron pattern %q for reminder %s: %v. Marking as sent.\n", *r.CronPattern, r.ID, err)
				_ = s.database.MarkAsSent(r.ID)
				continue
			}

			nextTime := schedule.Next(now)
			log.Printf("[Scheduler] Rescheduling recurring reminder %s for next time: %s\n", r.ID, nextTime.Format(time.RFC3339))
			if err := s.database.UpdateNextScheduledTime(r.ID, nextTime); err != nil {
				log.Printf("[Scheduler] Error updating next scheduled time for %s: %v\n", r.ID, err)
			}
		} else {
			// Instant or one-time Scheduled reminder
			if err := s.database.MarkAsSent(r.ID); err != nil {
				log.Printf("[Scheduler] Error marking reminder %s as sent: %v\n", r.ID, err)
			}
		}
	}
}
