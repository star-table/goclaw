package model

// MdFileInfo represents markdown file info (matches Python MdFileInfo)
type MdFileInfo struct {
	Filename     string `json:"filename"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	CreatedTime  string `json:"created_time"`
	ModifiedTime string `json:"modified_time"`
}

// MdFileContent represents markdown file content (matches Python MdFileContent)
type MdFileContent struct {
	Content string `json:"content"`
}

// SaveFileRequest represents a request to save file content
type SaveFileRequest struct {
	Content string `json:"content"`
}

// SaveFileResponse represents save file response (matches Python {"written": true})
type SaveFileResponse struct {
	Written bool `json:"written"`
}

// SaveMemoryResponse represents save memory response
type SaveMemoryResponse struct {
	Written bool `json:"written"`
}

// UploadFileResponse represents upload file response
type UploadFileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Path    string `json:"path"`
}
