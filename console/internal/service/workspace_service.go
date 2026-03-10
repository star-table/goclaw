package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/goclaw/console/internal/model"
	"github.com/smallnest/goclaw/memory"
)

// WorkspaceService manages workspace files and memories
type WorkspaceService struct {
	store        memory.Store
	workspaceDir string
	mu           sync.RWMutex
}

// NewWorkspaceService creates a new workspace service
func NewWorkspaceService(store memory.Store, workspaceDir string) *WorkspaceService {
	if workspaceDir == "" {
		homeDir, _ := os.UserHomeDir()
		workspaceDir = filepath.Join(homeDir, ".goclaw", "workspace")
	}

	// Ensure workspace directory exists
	os.MkdirAll(workspaceDir, 0755)
	os.MkdirAll(filepath.Join(workspaceDir, "memory"), 0755)

	return &WorkspaceService{
		store:        store,
		workspaceDir: workspaceDir,
	}
}

// ListFiles returns all workspace files
func (s *WorkspaceService) ListFiles() []*model.MdFileInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.MdFileInfo, 0)

	// Read files from workspace directory
	filesDir := s.workspaceDir
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			result = append(result, &model.MdFileInfo{
				Filename:     name,
				Path:         filepath.Join(filesDir, name),
				Size:         info.Size(),
				CreatedTime:  info.ModTime().Format(time.RFC3339),
				ModifiedTime: info.ModTime().Format(time.RFC3339),
			})
		}
	}

	return result
}

// GetFile returns a file by name
func (s *WorkspaceService) GetFile(name string) (*model.MdFileContent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.workspaceDir, name)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	return &model.MdFileContent{
		Content: string(content),
	}, nil
}

// SaveFile saves a file
func (s *WorkspaceService) SaveFile(name string, content string) *model.SaveFileResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.workspaceDir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return &model.SaveFileResponse{Written: false}
	}

	return &model.SaveFileResponse{Written: true}
}

// ListMemories returns all memory files
func (s *WorkspaceService) ListMemories() []*model.MdFileInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.MdFileInfo, 0)

	// Read memory files from memory directory
	memoryDir := filepath.Join(s.workspaceDir, "memory")
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			result = append(result, &model.MdFileInfo{
				Filename:     name,
				Path:         filepath.Join(memoryDir, name),
				Size:         info.Size(),
				CreatedTime:  info.ModTime().Format(time.RFC3339),
				ModifiedTime: info.ModTime().Format(time.RFC3339),
			})
		}
	}

	// Also check memory store if available
	if s.store != nil {
		memories, _ := s.store.List(func(ve *memory.VectorEmbedding) bool {
			return ve.Source == memory.MemorySourceDaily
		})
		for _, m := range memories {
			// Check if already added from file system
			found := false
			for _, r := range result {
				if r.Filename == m.ID+".md" {
					found = true
					break
				}
			}
			if !found && m.Metadata.FilePath != "" {
				result = append(result, &model.MdFileInfo{
					Filename:     m.ID,
					Path:         m.Metadata.FilePath,
					Size:         int64(len(m.Text)),
					CreatedTime:  m.UpdatedAt.Format(time.RFC3339),
					ModifiedTime: m.UpdatedAt.Format(time.RFC3339),
				})
			}
		}
	}

	return result
}

// GetMemory returns a memory file by date
func (s *WorkspaceService) GetMemory(date string) (*model.MdFileContent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try to read from file system first
	filePath := filepath.Join(s.workspaceDir, "memory", date+".md")
	content, err := os.ReadFile(filePath)
	if err == nil {
		return &model.MdFileContent{
			Content: string(content),
		}, nil
	}

	// Try memory store if available
	if s.store != nil {
		memories, _ := s.store.List(func(ve *memory.VectorEmbedding) bool {
			return ve.Source == memory.MemorySourceDaily &&
				(ve.ID == date || strings.Contains(ve.Metadata.FilePath, date))
		})
		if len(memories) > 0 {
			m := memories[0]
			return &model.MdFileContent{
				Content: m.Text,
			}, nil
		}
	}

	return nil, ErrMemoryNotFound
}

// SaveMemory saves a memory file
func (s *WorkspaceService) SaveMemory(date string, content string) *model.SaveMemoryResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Save to file system
	memoryDir := filepath.Join(s.workspaceDir, "memory")
	os.MkdirAll(memoryDir, 0755)
	filePath := filepath.Join(memoryDir, date+".md")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return &model.SaveMemoryResponse{Written: false}
	}

	// Also add to memory store if available
	if s.store != nil {
		embedding := &memory.VectorEmbedding{
			ID:        date,
			Text:      content,
			Source:    memory.MemorySourceDaily,
			Type:      memory.MemoryTypeContext,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata: memory.MemoryMetadata{
				FilePath: filePath,
			},
		}
		_ = s.store.Add(embedding)
	}

	return &model.SaveMemoryResponse{Written: true}
}

// UploadFile handles file upload
func (s *WorkspaceService) UploadFile(name string, content string) *model.UploadFileResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.workspaceDir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return &model.UploadFileResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to upload file: %v", err),
			Path:    filePath,
		}
	}

	return &model.UploadFileResponse{
		Success: true,
		Message: "File uploaded successfully",
		Path:    filePath,
	}
}

// DownloadWorkspace creates a zip of the workspace and returns the buffer and filename
func (s *WorkspaceService) DownloadWorkspace() (*bytes.Buffer, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create zip buffer
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Walk through workspace directory
	err := filepath.Walk(s.workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the zip file itself if it exists
		if strings.HasSuffix(path, ".zip") {
			return nil
		}

		// Get relative path from workspace
		relPath, err := filepath.Rel(s.workspaceDir, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for zip
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			// Create directory entry in zip
			_, err = zipWriter.Create(relPath + "/")
			return err
		}

		// Create file entry in zip
		fileWriter, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// Open and copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(fileWriter, file)
		return err
	})

	if err != nil {
		return nil, "", err
	}

	// Close zip writer
	if err := zipWriter.Close(); err != nil {
		return nil, "", err
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("goclaw_workspace_%s.zip", timestamp)

	return buf, filename, nil
}

// UploadWorkspace extracts a zip file into the workspace (merge mode)
func (s *WorkspaceService) UploadWorkspace(zipData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate zip
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("invalid zip file: %w", err)
	}

	// Check for path traversal
	for _, file := range zipReader.File {
		// Clean the path and check for traversal
		cleanPath := filepath.Clean(file.Name)
		if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "../") {
			return fmt.Errorf("zip contains unsafe path: %s", file.Name)
		}
	}

	// Extract files (merge mode: overwrite existing, keep others)
	for _, file := range zipReader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			dirPath := filepath.Join(s.workspaceDir, file.Name)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", file.Name, err)
			}
			continue
		}

		// Create target path
		targetPath := filepath.Join(s.workspaceDir, file.Name)

		// Ensure parent directory exists
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", file.Name, err)
		}

		// Open file from zip
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file %s in zip: %w", file.Name, err)
		}

		// Create target file
		targetFile, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to create file %s: %w", targetPath, err)
		}

		// Copy content
		_, err = io.Copy(targetFile, rc)
		rc.Close()
		targetFile.Close()

		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", file.Name, err)
		}
	}

	return nil
}

// ErrFileNotFound is returned when a file is not found
var ErrFileNotFound = &FileNotFoundError{}

type FileNotFoundError struct{}

func (e *FileNotFoundError) Error() string {
	return "file not found"
}

// ErrMemoryNotFound is returned when a memory is not found
var ErrMemoryNotFound = &MemoryNotFoundError{}

type MemoryNotFoundError struct{}

func (e *MemoryNotFoundError) Error() string {
	return "memory not found"
}
