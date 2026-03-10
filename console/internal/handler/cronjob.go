package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// CronJobHandler handles cron job API endpoints
type CronJobHandler struct {
	cronSvc *service.CronService
}

// NewCronJobHandler creates a new cron job handler
func NewCronJobHandler(cronSvc *service.CronService) *CronJobHandler {
	return &CronJobHandler{
		cronSvc: cronSvc,
	}
}

// HandleListJobs handles GET /api/cron/jobs
func (h *CronJobHandler) HandleListJobs(c *gin.Context) {
	jobs := h.cronSvc.ListJobs()
	c.JSON(http.StatusOK, jobs)
}

// HandleCreateJob handles POST /api/cron/jobs
func (h *CronJobHandler) HandleCreateJob(c *gin.Context) {
	var req model.CronJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	job, err := h.cronSvc.CreateJob(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

// HandleGetJob handles GET /api/cron/jobs/:jobId
func (h *CronJobHandler) HandleGetJob(c *gin.Context) {
	jobID := c.Param("jobId")

	view, err := h.cronSvc.GetJob(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, view)
}

// HandleUpdateJob handles PUT /api/cron/jobs/:jobId
func (h *CronJobHandler) HandleUpdateJob(c *gin.Context) {
	jobID := c.Param("jobId")

	var req model.CronJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	job, err := h.cronSvc.UpdateJob(jobID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, job)
}

// HandleDeleteJob handles DELETE /api/cron/jobs/:jobId
func (h *CronJobHandler) HandleDeleteJob(c *gin.Context) {
	jobID := c.Param("jobId")

	if err := h.cronSvc.DeleteJob(jobID); err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandlePauseJob handles POST /api/cron/jobs/:jobId/pause
func (h *CronJobHandler) HandlePauseJob(c *gin.Context) {
	jobID := c.Param("jobId")

	if err := h.cronSvc.PauseJob(jobID); err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleResumeJob handles POST /api/cron/jobs/:jobId/resume
func (h *CronJobHandler) HandleResumeJob(c *gin.Context) {
	jobID := c.Param("jobId")

	if err := h.cronSvc.ResumeJob(jobID); err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleRunJob handles POST /api/cron/jobs/:jobId/run
func (h *CronJobHandler) HandleRunJob(c *gin.Context) {
	jobID := c.Param("jobId")

	if err := h.cronSvc.RunJob(jobID); err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleGetJobState handles GET /api/cron/jobs/:jobId/state
func (h *CronJobHandler) HandleGetJobState(c *gin.Context) {
	jobID := c.Param("jobId")

	state, err := h.cronSvc.GetJobState(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}
