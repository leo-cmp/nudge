package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ReminderType string

const (
	TypeInstant   ReminderType = "instant"
	TypeScheduled ReminderType = "scheduled"
	TypeRecurring ReminderType = "recurring"
)

type ReminderStatus string

const (
	StatusPending   ReminderStatus = "pending"
	StatusSent      ReminderStatus = "sent"
	StatusCancelled ReminderStatus = "cancelled"
)

type Reminder struct {
	ID          string         `json:"id"`
	Message     string         `json:"message"`
	Type        ReminderType   `json:"type"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"`
	CronPattern *string        `json:"cron_pattern,omitempty"`
	Status      ReminderStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (d *DB) CreateReminder(r *Reminder) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now

	if r.Status == "" {
		r.Status = StatusPending
	}

	query := `
	INSERT INTO reminders (id, message, type, scheduled_at, cron_pattern, status, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.Exec(query, r.ID, r.Message, r.Type, r.ScheduledAt, r.CronPattern, r.Status, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert reminder: %w", err)
	}
	return nil
}

func (d *DB) GetPendingDueReminders(now time.Time) ([]Reminder, error) {
	query := `
	SELECT id, message, type, scheduled_at, cron_pattern, status, created_at, updated_at
	FROM reminders
	WHERE status = 'pending' AND (scheduled_at IS NULL OR scheduled_at <= ?)
	ORDER BY scheduled_at ASC
	`

	rows, err := d.Query(query, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to query pending reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var r Reminder
		var scheduledAt sql.NullTime
		var cronPattern sql.NullString

		err := rows.Scan(&r.ID, &r.Message, &r.Type, &scheduledAt, &cronPattern, &r.Status, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan reminder: %w", err)
		}

		if scheduledAt.Valid {
			r.ScheduledAt = &scheduledAt.Time
		}
		if cronPattern.Valid {
			r.CronPattern = &cronPattern.String
		}

		reminders = append(reminders, r)
	}

	return reminders, nil
}

func (d *DB) MarkAsSent(id string) error {
	query := `UPDATE reminders SET status = ?, updated_at = ? WHERE id = ?`
	_, err := d.Exec(query, StatusSent, time.Now().UTC(), id)
	return err
}

func (d *DB) UpdateNextScheduledTime(id string, nextScheduled time.Time) error {
	query := `UPDATE reminders SET scheduled_at = ?, updated_at = ? WHERE id = ?`
	_, err := d.Exec(query, nextScheduled.UTC(), time.Now().UTC(), id)
	return err
}

func (d *DB) ListReminders(status string) ([]Reminder, error) {
	query := `
	SELECT id, message, type, scheduled_at, cron_pattern, status, created_at, updated_at
	FROM reminders
	`
	var args []interface{}
	if status != "" && status != "all" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT 50`

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var r Reminder
		var scheduledAt sql.NullTime
		var cronPattern sql.NullString

		err := rows.Scan(&r.ID, &r.Message, &r.Type, &scheduledAt, &cronPattern, &r.Status, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan reminder: %w", err)
		}

		if scheduledAt.Valid {
			r.ScheduledAt = &scheduledAt.Time
		}
		if cronPattern.Valid {
			r.CronPattern = &cronPattern.String
		}

		reminders = append(reminders, r)
	}

	return reminders, nil
}

func (d *DB) CancelReminder(id string) error {
	query := `UPDATE reminders SET status = ?, updated_at = ? WHERE id = ?`
	res, err := d.Exec(query, StatusCancelled, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("reminder with id %s not found", id)
	}
	return nil
}
