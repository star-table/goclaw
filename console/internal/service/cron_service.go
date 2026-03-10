package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/cron"
)

// CronService wraps cron.Service for REST API
type CronService struct {
	service *cron.Service
}

// NewCronService creates a new cron service wrapper
func NewCronService(svc *cron.Service) *CronService {
	return &CronService{
		service: svc,
	}
}

// ListJobs returns all cron jobs
func (s *CronService) ListJobs() []*model.CronJobSpecOutput {
	if s.service == nil {
		return []*model.CronJobSpecOutput{}
	}

	jobs := s.service.ListJobs()
	result := make([]*model.CronJobSpecOutput, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, convertCronJobToOutput(job))
	}
	return result
}

// GetJob returns a specific job by ID
func (s *CronService) GetJob(id string) (*model.CronJobView, error) {
	if s.service == nil {
		return nil, ErrJobNotFound
	}

	job, err := s.service.GetJob(id)
	if err != nil {
		return nil, ErrJobNotFound
	}

	// Get run logs
	runLogs, _ := s.service.GetRunLogs(id, cron.RunLogFilter{Limit: 50})
	runs := make([]model.CronJobRun, 0, len(runLogs))
	for _, log := range runLogs {
		runs = append(runs, model.CronJobRun{
			ID:        log.RunID,
			JobID:     log.JobID,
			StartTime: log.StartedAt,
			EndTime:   log.FinishedAt,
			Status:    log.Status,
			Output:    "",
			Error:     log.Error,
		})
	}

	return &model.CronJobView{
		Job:   *convertCronJobToOutput(job),
		Runs:  runs,
		State: convertJobState(&job.State),
	}, nil
}

// CreateJob creates a new cron job
func (s *CronService) CreateJob(req *model.CronJobRequest) (*model.CronJobSpecOutput, error) {
	if s.service == nil {
		return nil, ErrServiceNotAvailable
	}

	job := convertRequestToCronJob(req)
	if job.ID == "" {
		job.ID = "job-" + uuid.New().String()[:8]
	}

	if err := s.service.AddJob(job); err != nil {
		return nil, err
	}

	return convertCronJobToOutput(job), nil
}

// UpdateJob updates a cron job
func (s *CronService) UpdateJob(id string, req *model.CronJobRequest) (*model.CronJobSpecOutput, error) {
	if s.service == nil {
		return nil, ErrServiceNotAvailable
	}

	err := s.service.UpdateJob(id, func(job *cron.Job) error {
		job.Name = req.Name
		job.State.Enabled = req.Enabled
		job.Schedule = convertModelScheduleToCron(req.Schedule)
		job.Payload = convertRequestToPayload(req)
		return nil
	})

	if err != nil {
		return nil, ErrJobNotFound
	}

	updatedJob, _ := s.service.GetJob(id)
	return convertCronJobToOutput(updatedJob), nil
}

// DeleteJob deletes a cron job
func (s *CronService) DeleteJob(id string) error {
	if s.service == nil {
		return ErrServiceNotAvailable
	}

	if err := s.service.RemoveJob(id); err != nil {
		return ErrJobNotFound
	}
	return nil
}

// PauseJob pauses a cron job
func (s *CronService) PauseJob(id string) error {
	if s.service == nil {
		return ErrServiceNotAvailable
	}

	return s.service.DisableJob(id)
}

// ResumeJob resumes a cron job
func (s *CronService) ResumeJob(id string) error {
	if s.service == nil {
		return ErrServiceNotAvailable
	}

	return s.service.EnableJob(id)
}

// RunJob triggers a cron job manually
func (s *CronService) RunJob(id string) error {
	if s.service == nil {
		return ErrServiceNotAvailable
	}

	return s.service.RunJob(context.Background(), id, true)
}

// GetJobState returns the state of a cron job
func (s *CronService) GetJobState(id string) (*model.CronJobState, error) {
	if s.service == nil {
		return nil, ErrServiceNotAvailable
	}

	job, err := s.service.GetJob(id)
	if err != nil {
		return nil, ErrJobNotFound
	}

	state := convertJobState(&job.State)
	return &state, nil
}

// GetStatus returns the cron service status
func (s *CronService) GetStatus() map[string]interface{} {
	if s.service == nil {
		return map[string]interface{}{
			"running": false,
			"error":   "service not available",
		}
	}
	return s.service.GetStatus()
}

// Helper functions to convert between types

func convertCronJobToOutput(job *cron.Job) *model.CronJobSpecOutput {
	output := &model.CronJobSpecOutput{
		ID:        job.ID,
		Name:      job.Name,
		Enabled:   job.State.Enabled,
		Schedule:  convertCronScheduleToModel(job.Schedule),
		TaskType:  string(job.Payload.Type),
		Text:      job.Payload.Message,
		Request:   nil,
		Dispatch:  model.CronJobDispatch{},
		Runtime:   model.CronJobRuntime{},
		NextRun:   job.State.NextRunAt,
		LastRun:   job.State.LastRunAt,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}

	if job.Delivery != nil {
		output.Dispatch = model.CronJobDispatch{
			Mode:         string(job.Delivery.Mode),
			WebhookURL:   job.Delivery.WebhookURL,
			WebhookToken: job.Delivery.WebhookToken,
		}
	}

	return output
}

func convertCronScheduleToModel(s cron.Schedule) model.CronJobSchedule {
	return model.CronJobSchedule{
		Type:     string(s.Type),
		Cron:     s.CronExpression,
		Interval: int(s.EveryDuration / time.Second),
		At:       s.At.Format(time.RFC3339),
	}
}

func convertModelScheduleToCron(s model.CronJobSchedule) cron.Schedule {
	result := cron.Schedule{
		Type:           cron.ScheduleType(s.Type),
		CronExpression: s.Cron,
		EveryDuration:  time.Duration(s.Interval) * time.Second,
	}
	if s.At != "" {
		result.At, _ = time.Parse(time.RFC3339, s.At)
	}
	return result
}

func convertRequestToCronJob(req *model.CronJobRequest) *cron.Job {
	now := time.Now()
	job := &cron.Job{
		ID:            req.ID,
		Name:          req.Name,
		Schedule:      convertModelScheduleToCron(req.Schedule),
		SessionTarget: cron.SessionTargetMain,
		WakeMode:      cron.WakeModeNow,
		Payload:       convertRequestToPayload(req),
		State: cron.JobState{
			Enabled: req.Enabled,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if req.Dispatch.Mode != "" {
		job.Delivery = &cron.Delivery{
			Mode:         cron.DeliveryMode(req.Dispatch.Mode),
			WebhookURL:   req.Dispatch.WebhookURL,
			WebhookToken: req.Dispatch.WebhookToken,
		}
	}

	return job
}

func convertRequestToPayload(req *model.CronJobRequest) cron.Payload {
	return cron.Payload{
		Type:    cron.PayloadType(req.TaskType),
		Message: req.Text,
	}
}

func convertJobState(state *cron.JobState) model.CronJobState {
	status := "idle"
	if state.RunningAt != nil {
		status = "running"
	}

	lastResult := "success"
	if state.LastStatus == "error" {
		lastResult = "error"
	}

	return model.CronJobState{
		Status:     status,
		LastResult: lastResult,
		NextRun:    state.NextRunAt,
	}
}

// ErrJobNotFound is returned when a job is not found
var ErrJobNotFound = &JobNotFoundError{}

type JobNotFoundError struct{}

func (e *JobNotFoundError) Error() string {
	return "job not found"
}

// ErrServiceNotAvailable is returned when the service is not available
var ErrServiceNotAvailable = &ServiceNotAvailableError{}

type ServiceNotAvailableError struct{}

func (e *ServiceNotAvailableError) Error() string {
	return "service not available"
}
