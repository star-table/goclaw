package model

import "time"

// OllamaModelResponse represents an Ollama model
type OllamaModelResponse struct {
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	Digest      string    `json:"digest"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

// OllamaDownloadRequest represents a request to download an Ollama model
type OllamaDownloadRequest struct {
	Name string `json:"name"`
}

// OllamaDownloadTaskResponse represents an Ollama download task
type OllamaDownloadTaskResponse struct {
	TaskID    string     `json:"task_id"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	Progress  int        `json:"progress"`
	Completed int64      `json:"completed"`
	Total     int64      `json:"total"`
	Speed     string     `json:"speed"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	Error     string     `json:"error"`
}

// CancelOllamaDownloadResponse represents cancel Ollama download response
type CancelOllamaDownloadResponse struct {
	Status string `json:"status"`
	TaskID string `json:"task_id"`
}

// DeleteOllamaModelResponse represents delete Ollama model response
type DeleteOllamaModelResponse struct {
	Status string `json:"status"`
	Name   string `json:"name"`
}
