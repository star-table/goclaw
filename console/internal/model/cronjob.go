package model

import "time"

// CronJobSchedule represents a cron job schedule
type CronJobSchedule struct {
	Type     string `json:"type"`
	Cron     string `json:"cron,omitempty"`
	Interval int    `json:"interval,omitempty"`
	At       string `json:"at,omitempty"`
}

// CronJobDispatch represents dispatch configuration
type CronJobDispatch struct {
	Type         string                 `json:"type"`
	Channel      string                 `json:"channel,omitempty"`
	Target       map[string]interface{} `json:"target,omitempty"`
	Mode         string                 `json:"mode,omitempty"`
	Meta         map[string]interface{} `json:"meta,omitempty"`
	WebhookURL   string                 `json:"webhook_url,omitempty"`
	WebhookToken string                 `json:"webhook_token,omitempty"`
}

// CronJobRuntime represents runtime configuration
type CronJobRuntime struct {
	MaxConcurrency      int `json:"max_concurrency,omitempty"`
	TimeoutSeconds      int `json:"timeout_seconds,omitempty"`
	MisfireGraceSeconds int `json:"misfire_grace_seconds,omitempty"`
}

// CronJobRequest represents request body for creating/updating cron job
type CronJobRequest struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Enabled   bool                   `json:"enabled"`
	Schedule  CronJobSchedule        `json:"schedule"`
	TaskType  string                 `json:"task_type"`
	Text      string                 `json:"text,omitempty"`
	Request   *AgentProcessRequest   `json:"request,omitempty"`
	Dispatch  CronJobDispatch        `json:"dispatch"`
	Runtime   CronJobRuntime         `json:"runtime"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// CronJobSpecOutput represents cron job spec output
type CronJobSpecOutput struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Enabled   bool                   `json:"enabled"`
	Schedule  CronJobSchedule        `json:"schedule"`
	TaskType  string                 `json:"task_type"`
	Text      string                 `json:"text,omitempty"`
	Request   *AgentProcessRequest   `json:"request,omitempty"`
	Dispatch  CronJobDispatch        `json:"dispatch"`
	Runtime   CronJobRuntime         `json:"runtime"`
	NextRun   *time.Time             `json:"next_run,omitempty"`
	LastRun   *time.Time             `json:"last_run,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// CronJobRun represents a cron job run record
type CronJobRun struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Status    string    `json:"status"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
}

// CronJobState represents cron job state
type CronJobState struct {
	Status     string     `json:"status"`
	LastResult string     `json:"last_result"`
	NextRun    *time.Time `json:"next_run,omitempty"`
}

// CronJobView represents cron job view with runs and state
type CronJobView struct {
	Job   CronJobSpecOutput `json:"job"`
	Runs  []CronJobRun      `json:"runs"`
	State CronJobState      `json:"state"`
}
