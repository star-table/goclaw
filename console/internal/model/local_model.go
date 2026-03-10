package model

// LocalModelResponse represents a local model
type LocalModelResponse struct {
	ID          string `json:"id"`
	RepoID      string `json:"repo_id"`
	Filename    string `json:"filename"`
	Backend     string `json:"backend"`
	Source      string `json:"source"`
	FileSize    int64  `json:"file_size"`
	LocalPath   string `json:"local_path"`
	DisplayName string `json:"display_name"`
}

// LocalModelDownloadRequest represents a request to download a local model
type LocalModelDownloadRequest struct {
	RepoID   string `json:"repo_id"`
	Filename string `json:"filename"`
	Backend  string `json:"backend"`
	Source   string `json:"source"`
}

// DownloadTaskResponse represents a download task
type DownloadTaskResponse struct {
	TaskID   string                 `json:"task_id"`
	Status   string                 `json:"status"`
	RepoID   string                 `json:"repo_id"`
	Filename string                 `json:"filename"`
	Backend  string                 `json:"backend"`
	Source   string                 `json:"source"`
	Error    string                 `json:"error"`
	Result   map[string]interface{} `json:"result,omitempty"`
}

// CancelDownloadResponse represents cancel download response
type CancelDownloadResponse struct {
	Status string `json:"status"`
	TaskID string `json:"task_id"`
}

// DeleteLocalModelResponse represents delete local model response
type DeleteLocalModelResponse struct {
	Status  string `json:"status"`
	ModelID string `json:"model_id"`
}
