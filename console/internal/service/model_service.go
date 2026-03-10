package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smallnest/goclaw/console/internal/model"
)

// LocalModelService manages local models with file-based persistence
type LocalModelService struct {
	models        map[string]*model.LocalModelResponse
	downloadTasks map[string]*model.DownloadTaskResponse
	modelsPath    string // 模型存储路径
	configPath    string // 配置文件路径
	mu            sync.RWMutex
}

// NewLocalModelService creates a new local model service
func NewLocalModelService(workspaceDir string) *LocalModelService {
	s := &LocalModelService{
		models:        make(map[string]*model.LocalModelResponse),
		downloadTasks: make(map[string]*model.DownloadTaskResponse),
		modelsPath:    filepath.Join(workspaceDir, "models"),
		configPath:    filepath.Join(workspaceDir, "local_models.json"),
	}

	// 确保目录存在
	os.MkdirAll(s.modelsPath, 0755)

	// 加载已保存的模型
	s.loadModels()

	return s
}

// loadModels 从配置文件加载模型列表
func (s *LocalModelService) loadModels() {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return
	}

	var models []*model.LocalModelResponse
	if err := json.Unmarshal(data, &models); err != nil {
		return
	}

	for _, m := range models {
		s.models[m.ID] = m
	}
}

// saveModels 保存模型列表到配置文件
func (s *LocalModelService) saveModels() error {
	models := make([]*model.LocalModelResponse, 0, len(s.models))
	for _, m := range s.models {
		models = append(models, m)
	}

	data, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configPath, data, 0644)
}

// ListModels returns all local models
func (s *LocalModelService) ListModels(backend string) []*model.LocalModelResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.LocalModelResponse, 0)
	for _, m := range s.models {
		if backend != "" && m.Backend != backend {
			continue
		}
		result = append(result, m)
	}
	return result
}

// DeleteModel deletes a local model
func (s *LocalModelService) DeleteModel(id string) (*model.DeleteLocalModelResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.models[id]
	if !ok {
		return nil, ErrLocalModelNotFound
	}

	// 删除模型文件
	if m.LocalPath != "" {
		os.Remove(m.LocalPath)
	}

	delete(s.models, id)
	s.saveModels()

	return &model.DeleteLocalModelResponse{
		Status:  "deleted",
		ModelID: id,
	}, nil
}

// StartDownload starts a model download
func (s *LocalModelService) StartDownload(req *model.LocalModelDownloadRequest) *model.DownloadTaskResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := "task-" + uuid.New().String()[:8]
	task := &model.DownloadTaskResponse{
		TaskID:   taskID,
		Status:   "pending",
		RepoID:   req.RepoID,
		Filename: req.Filename,
		Backend:  req.Backend,
		Source:   req.Source,
		Error:    "",
		Result:   nil,
	}

	s.downloadTasks[taskID] = task

	// 异步执行下载
	go s.executeDownload(taskID, req)

	return task
}

// executeDownload 执行实际的模型下载
func (s *LocalModelService) executeDownload(taskID string, req *model.LocalModelDownloadRequest) {
	s.mu.Lock()
	task := s.downloadTasks[taskID]
	if task == nil {
		s.mu.Unlock()
		return
	}
	task.Status = "downloading"
	s.mu.Unlock()

	// 构建下载 URL
	var downloadURL string
	switch req.Source {
	case "huggingface":
		downloadURL = fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", req.RepoID, req.Filename)
	default:
		downloadURL = fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", req.RepoID, req.Filename)
	}

	// 创建本地文件路径
	modelID := strings.ReplaceAll(req.RepoID, "/", "-") + "-" + strings.ReplaceAll(req.Filename, ".", "-")
	localPath := filepath.Join(s.modelsPath, modelID, req.Filename)
	os.MkdirAll(filepath.Dir(localPath), 0755)

	// 执行下载
	resp, err := http.Get(downloadURL)
	if err != nil {
		s.mu.Lock()
		task.Status = "failed"
		task.Error = fmt.Sprintf("Download failed: %v", err)
		s.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.mu.Lock()
		task.Status = "failed"
		task.Error = fmt.Sprintf("Download failed: HTTP %d", resp.StatusCode)
		s.mu.Unlock()
		return
	}

	// 创建文件
	out, err := os.Create(localPath)
	if err != nil {
		s.mu.Lock()
		task.Status = "failed"
		task.Error = fmt.Sprintf("Failed to create file: %v", err)
		s.mu.Unlock()
		return
	}
	defer out.Close()

	// 复制内容并跟踪进度
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		s.mu.Lock()
		task.Status = "failed"
		task.Error = fmt.Sprintf("Failed to save file: %v", err)
		s.mu.Unlock()
		os.Remove(localPath)
		return
	}

	// 获取文件大小
	fileInfo, _ := os.Stat(localPath)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建模型记录
	m := &model.LocalModelResponse{
		ID:          modelID,
		RepoID:      req.RepoID,
		Filename:    req.Filename,
		Backend:     req.Backend,
		Source:      req.Source,
		FileSize:    fileInfo.Size(),
		LocalPath:   localPath,
		DisplayName: fmt.Sprintf("%s/%s", req.RepoID, req.Filename),
	}
	s.models[modelID] = m
	s.saveModels()

	// 更新任务状态
	task.Status = "completed"
	task.Result = map[string]interface{}{
		"model_id":   modelID,
		"local_path": localPath,
		"file_size":  fileInfo.Size(),
	}
}

// GetDownloadStatus returns all download tasks
func (s *LocalModelService) GetDownloadStatus(backend string) []*model.DownloadTaskResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.DownloadTaskResponse, 0)
	for _, task := range s.downloadTasks {
		if backend != "" && task.Backend != backend {
			continue
		}
		result = append(result, task)
	}
	return result
}

// CancelDownload cancels a download task
func (s *LocalModelService) CancelDownload(taskID string) (*model.CancelDownloadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.downloadTasks[taskID]
	if !ok {
		return nil, ErrDownloadTaskNotFound
	}

	// 只能取消 pending 或 downloading 状态的任务
	if task.Status == "completed" {
		return nil, fmt.Errorf("cannot cancel completed task")
	}

	task.Status = "cancelled"
	delete(s.downloadTasks, taskID)

	return &model.CancelDownloadResponse{
		Status: "cancelled",
		TaskID: taskID,
	}, nil
}

// ErrLocalModelNotFound is returned when a local model is not found
var ErrLocalModelNotFound = &LocalModelNotFoundError{}

type LocalModelNotFoundError struct{}

func (e *LocalModelNotFoundError) Error() string {
	return "local model not found"
}

// ErrDownloadTaskNotFound is returned when a download task is not found
var ErrDownloadTaskNotFound = &DownloadTaskNotFoundError{}

type DownloadTaskNotFoundError struct{}

func (e *DownloadTaskNotFoundError) Error() string {
	return "download task not found"
}

// OllamaModelService manages Ollama models via Ollama API
type OllamaModelService struct {
	ollamaURL     string
	models        map[string]*model.OllamaModelResponse
	downloadTasks map[string]*model.OllamaDownloadTaskResponse
	mu            sync.RWMutex
}

// NewOllamaModelService creates a new Ollama model service
func NewOllamaModelService() *OllamaModelService {
	return &OllamaModelService{
		ollamaURL:     getEnvOrDefault("OLLAMA_URL", "http://localhost:11434"),
		models:        make(map[string]*model.OllamaModelResponse),
		downloadTasks: make(map[string]*model.OllamaDownloadTaskResponse),
	}
}

// ListModels returns all Ollama models by calling Ollama API
func (s *OllamaModelService) ListModels() []*model.OllamaModelResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 尝试从 Ollama API 获取模型列表
	resp, err := http.Get(s.ollamaURL + "/api/tags")
	if err != nil {
		// 如果 Ollama 不可用，返回缓存的模型
		result := make([]*model.OllamaModelResponse, 0, len(s.models))
		for _, m := range s.models {
			result = append(result, m)
		}
		return result
	}
	defer resp.Body.Close()

	// 解析 Ollama 响应
	var ollamaResp struct {
		Models []struct {
			Name       string    `json:"name"`
			Size       int64     `json:"size"`
			Digest     string    `json:"digest"`
			ModifiedAt time.Time `json:"modified_at"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		result := make([]*model.OllamaModelResponse, 0, len(s.models))
		for _, m := range s.models {
			result = append(result, m)
		}
		return result
	}

	// 更新缓存
	s.models = make(map[string]*model.OllamaModelResponse)
	result := make([]*model.OllamaModelResponse, 0, len(ollamaResp.Models))
	for _, m := range ollamaResp.Models {
		ollamaModel := &model.OllamaModelResponse{
			Name:        m.Name,
			Size:        m.Size,
			Digest:      m.Digest,
			CreatedAt:   m.ModifiedAt,
			ModifiedAt:  m.ModifiedAt,
		}
		s.models[m.Name] = ollamaModel
		result = append(result, ollamaModel)
	}

	return result
}

// DeleteModel deletes an Ollama model via API
func (s *OllamaModelService) DeleteModel(name string) (*model.DeleteOllamaModelResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 调用 Ollama API 删除模型
	req := map[string]string{"name": name}
	jsonData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("DELETE", s.ollamaURL+"/api/delete", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		// 如果 API 不可用，从缓存删除
		if _, ok := s.models[name]; !ok {
			return nil, ErrOllamaModelNotFound
		}
		delete(s.models, name)
		return &model.DeleteOllamaModelResponse{
			Status: "deleted",
			Name:   name,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to delete model: HTTP %d", resp.StatusCode)
	}

	delete(s.models, name)
	return &model.DeleteOllamaModelResponse{
		Status: "deleted",
		Name:   name,
	}, nil
}

// StartDownload starts an Ollama model download (pull)
func (s *OllamaModelService) StartDownload(req *model.OllamaDownloadRequest) *model.OllamaDownloadTaskResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := "task-" + uuid.New().String()[:8]
	task := &model.OllamaDownloadTaskResponse{
		TaskID:    taskID,
		Name:      req.Name,
		Status:    "pending",
		Progress:  0,
		Completed: 0,
		Total:     0,
		Speed:     "",
		StartTime: time.Now(),
		EndTime:   nil,
		Error:     "",
	}

	s.downloadTasks[taskID] = task

	// 异步执行下载
	go s.executeOllamaPull(taskID, req.Name)

	return task
}

// executeOllamaPull 执行 Ollama pull 命令
func (s *OllamaModelService) executeOllamaPull(taskID, modelName string) {
	s.mu.Lock()
	task := s.downloadTasks[taskID]
	if task == nil {
		s.mu.Unlock()
		return
	}
	task.Status = "downloading"
	s.mu.Unlock()

	// 调用 Ollama API pull
	req := map[string]string{"name": modelName}
	jsonData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", s.ollamaURL+"/api/pull", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		s.mu.Lock()
		task.Status = "failed"
		task.Error = fmt.Sprintf("Failed to pull model: %v", err)
		task.EndTime = ptrTime(time.Now())
		s.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	// 解析流式响应
	decoder := json.NewDecoder(resp.Body)
	for {
		var pullResp struct {
			Status    string `json:"status"`
			Digest    string `json:"digest"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
		}

		if err := decoder.Decode(&pullResp); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		s.mu.Lock()
		task.Status = pullResp.Status
		task.Total = pullResp.Total
		task.Completed = pullResp.Completed
		if pullResp.Total > 0 {
			task.Progress = int(float64(pullResp.Completed) / float64(pullResp.Total) * 100)
		}

		if pullResp.Status == "success" {
			task.Status = "completed"
			task.EndTime = ptrTime(time.Now())

			// 添加到模型列表
			s.models[modelName] = &model.OllamaModelResponse{
				Name:       modelName,
				Size:       pullResp.Total,
				Digest:     pullResp.Digest,
				CreatedAt:  time.Now(),
				ModifiedAt: time.Now(),
			}
		}
		s.mu.Unlock()
	}
}

// GetDownloadStatus returns all Ollama download tasks
func (s *OllamaModelService) GetDownloadStatus() []*model.OllamaDownloadTaskResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.OllamaDownloadTaskResponse, 0, len(s.downloadTasks))
	for _, task := range s.downloadTasks {
		result = append(result, task)
	}
	return result
}

// CancelDownload cancels an Ollama download task
func (s *OllamaModelService) CancelDownload(taskID string) (*model.CancelOllamaDownloadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.downloadTasks[taskID]
	if !ok {
		return nil, ErrOllamaDownloadTaskNotFound
	}

	task.Status = "cancelled"
	task.EndTime = ptrTime(time.Now())
	delete(s.downloadTasks, taskID)

	return &model.CancelOllamaDownloadResponse{
		Status: "cancelled",
		TaskID: taskID,
	}, nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

// ErrOllamaModelNotFound is returned when an Ollama model is not found
var ErrOllamaModelNotFound = &OllamaModelNotFoundError{}

type OllamaModelNotFoundError struct{}

func (e *OllamaModelNotFoundError) Error() string {
	return "ollama model not found"
}

// ErrOllamaDownloadTaskNotFound is returned when an Ollama download task is not found
var ErrOllamaDownloadTaskNotFound = &OllamaDownloadTaskNotFoundError{}

type OllamaDownloadTaskNotFoundError struct{}

func (e *OllamaDownloadTaskNotFoundError) Error() string {
	return "ollama download task not found"
}

// InitializeDefaultLocalModels creates default local models (for backward compatibility)
func (s *LocalModelService) InitializeDefaultLocalModels() {
	// Models are loaded from config file
}

// InitializeDefaultOllamaModels creates default Ollama models
func (s *OllamaModelService) InitializeDefaultOllamaModels() {
	// Ollama models are fetched from Ollama API
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
