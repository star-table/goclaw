package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/console/internal/service"
)

// SkillHandler handles skill API endpoints
type SkillHandler struct {
	skillSvc      *service.SkillService
	skillImporter *service.SkillImporter
}

// NewSkillHandler creates a new skill handler
func NewSkillHandler(skillSvc *service.SkillService) *SkillHandler {
	return &SkillHandler{
		skillSvc:      skillSvc,
		skillImporter: service.NewSkillImporter(),
	}
}

// HandleListSkills handles GET /api/skills
// Returns all skills with their enabled status
func (h *SkillHandler) HandleListSkills(c *gin.Context) {
	skills := h.skillSvc.ListSkills()
	c.JSON(http.StatusOK, skills)
}

// HandleListAvailableSkills handles GET /api/skills/available
// Returns only enabled/active skills
func (h *SkillHandler) HandleListAvailableSkills(c *gin.Context) {
	skills := h.skillSvc.ListAvailableSkills()
	c.JSON(http.StatusOK, skills)
}

// HandleCreateSkill handles POST /api/skills
// Creates a new customized skill
func (h *SkillHandler) HandleCreateSkill(c *gin.Context) {
	var req model.CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	response := h.skillSvc.CreateSkill(&req)
	c.JSON(http.StatusOK, response)
}

// HandleEnableSkill handles POST /api/skills/:skillName/enable
// Enables a skill by copying to active_skills directory
func (h *SkillHandler) HandleEnableSkill(c *gin.Context) {
	skillName := c.Param("skillName")

	response, err := h.skillSvc.EnableSkill(skillName)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleDisableSkill handles POST /api/skills/:skillName/disable
// Disables a skill by removing from active_skills directory
func (h *SkillHandler) HandleDisableSkill(c *gin.Context) {
	skillName := c.Param("skillName")

	response, err := h.skillSvc.DisableSkill(skillName)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleBatchEnableSkills handles POST /api/skills/batch-enable
// Enables multiple skills at once
func (h *SkillHandler) HandleBatchEnableSkills(c *gin.Context) {
	var req model.BatchSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Try legacy format (array of strings)
		var names []string
		if err2 := c.ShouldBindJSON(&names); err2 != nil {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
			return
		}
		h.skillSvc.BatchEnableSkills(names)
		c.JSON(http.StatusOK, gin.H{"enabled": true})
		return
	}

	h.skillSvc.BatchEnableSkills(req.Names)
	c.JSON(http.StatusOK, gin.H{"enabled": true})
}

// HandleBatchDisableSkills handles POST /api/skills/batch-disable
// Disables multiple skills at once
func (h *SkillHandler) HandleBatchDisableSkills(c *gin.Context) {
	var req model.BatchSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Try legacy format (array of strings)
		var names []string
		if err2 := c.ShouldBindJSON(&names); err2 != nil {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
			return
		}
		h.skillSvc.BatchDisableSkills(names)
		c.JSON(http.StatusOK, gin.H{"disabled": true})
		return
	}

	h.skillSvc.BatchDisableSkills(req.Names)
	c.JSON(http.StatusOK, gin.H{"disabled": true})
}

// HandleDeleteSkill handles DELETE /api/skills/:skillName
// Deletes a customized skill
func (h *SkillHandler) HandleDeleteSkill(c *gin.Context) {
	skillName := c.Param("skillName")

	response, err := h.skillSvc.DeleteSkill(skillName)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleSearchHub handles GET /api/skills/hub/search
// Searches for skills in the hub
func (h *SkillHandler) HandleSearchHub(c *gin.Context) {
	query := c.Query("q")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	skills := h.skillSvc.SearchHubSkills(query, limit)
	c.JSON(http.StatusOK, skills)
}

// HandleInstallSkill handles POST /api/skills/hub/install
// Installs a skill from the hub
func (h *SkillHandler) HandleInstallSkill(c *gin.Context) {
	var req model.InstallSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	response := h.skillSvc.InstallSkill(&req)
	c.JSON(http.StatusOK, response)
}

// HandleImportSkill handles POST /api/skills/import
// Imports a skill from a URL (skills.sh, GitHub, skillsmp, clawhub)
func (h *SkillHandler) HandleImportSkill(c *gin.Context) {
	var req struct {
		URL       string `json:"url" binding:"required"`
		Version   string `json:"version"`
		Enable    bool   `json:"enable"`
		Overwrite bool   `json:"overwrite"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	result, err := h.skillImporter.InstallSkillFromHub(req.URL, req.Version, req.Enable, req.Overwrite, h.skillSvc)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleSearchSkillsHub handles GET /api/skills/hub/search
// Searches for skills in the skills hub
func (h *SkillHandler) HandleSearchSkillsHub(c *gin.Context) {
	query := c.Query("q")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	results, err := h.skillImporter.SearchHubSkills(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// HandleLoadSkillFile handles GET /api/skills/:skillName/files/:source/*filePath
// Loads a file from a skill (SKILL.md, references, or scripts)
func (h *SkillHandler) HandleLoadSkillFile(c *gin.Context) {
	skillName := c.Param("skillName")
	source := c.Param("source")
	filePath := c.Param("filePath")

	// Clean up the file path (remove leading slash)
	if len(filePath) > 0 && filePath[0] == '/' {
		filePath = filePath[1:]
	}

	content, err := h.skillSvc.LoadSkillFile(skillName, source, filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, content)
}

// HandleSyncSkills handles POST /api/skills/sync
// Syncs skills from skills directory to active_skills directory
func (h *SkillHandler) HandleSyncSkills(c *gin.Context) {
	var req struct {
		SkillNames []string `json:"skill_names"`
		Force      bool     `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	result := h.skillSvc.SyncSkillsToActive(req.SkillNames, req.Force)
	c.JSON(http.StatusOK, result)
}

// HandleValidateSkill handles POST /api/skills/:skillName/validate
// Validates a skill by name
func (h *SkillHandler) HandleValidateSkill(c *gin.Context) {
	skillName := c.Param("skillName")
	result := h.skillSvc.ValidateSkill(skillName)
	c.JSON(http.StatusOK, result)
}
