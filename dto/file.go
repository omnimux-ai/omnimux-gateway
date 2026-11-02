package dto

// FileUploadBase64Request represents the request body for Base64 file upload.
type FileUploadBase64Request struct {
	Base64Data string `json:"base64_data" binding:"required"`
	UploadPath string `json:"upload_path"`
	FileName   string `json:"file_name"`
}

// FileUploadURLRequest represents the request body for URL file upload.
type FileUploadURLRequest struct {
	FileURL    string `json:"file_url" binding:"required"`
	UploadPath string `json:"upload_path"`
	FileName   string `json:"file_name"`
}

// FilePresignRequest asks for a direct-upload ticket for one object.
type FilePresignRequest struct {
	FileName   string `json:"file_name" binding:"required"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	UploadPath string `json:"upload_path"`
}

// FilePresignInfoData is the ticket the client uses to upload one object and
// the public address the object will carry once confirmed.
type FilePresignInfoData struct {
	FileID        string            `json:"file_id"`
	UploadURL     string            `json:"upload_url"`
	UploadMethod  string            `json:"upload_method"`
	UploadHeaders map[string]string `json:"upload_headers"`
	ResourceURL   string            `json:"resource_url"`
	DownloadURL   string            `json:"download_url"`
	ExpiresAt     string            `json:"expires_at"`
}

// FilePresignResponse represents the standard API response for a upload ticket.
type FilePresignResponse struct {
	Success bool                 `json:"success"`
	Code    int                  `json:"code"`
	Msg     string               `json:"msg"`
	Data    *FilePresignInfoData `json:"data,omitempty"`
}

// FileConfirmUploadRequest reports that a direct upload finished.
type FileConfirmUploadRequest struct {
	FileID   string `json:"file_id" binding:"required"`
	FileSize int64  `json:"file_size"`
}

// FileInfoData represents uploaded file details in Evolink response format.
type FileInfoData struct {
	FileID       string `json:"file_id"`
	FileName     string `json:"file_name"`
	OriginalName string `json:"original_name"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	UploadPath   string `json:"upload_path"`
	FileURL      string `json:"file_url"`
	DownloadURL  string `json:"download_url"`
	UploadTime   string `json:"upload_time"`
	ExpiresAt    string `json:"expires_at"`
	// VideoMetadata is present only when the server persisted a browser
	// declaration for an influencer-reference upload.
	VideoMetadata *InfluencerVideoMetadata `json:"video_metadata,omitempty"`
}

// FileUploadResponse represents the standard API response for file upload.
type FileUploadResponse struct {
	Success bool          `json:"success"`
	Code    int           `json:"code"`
	Msg     string        `json:"msg"`
	Data    *FileInfoData `json:"data,omitempty"`
}

// FileQuotaData represents user file quota usage and limits.
type FileQuotaData struct {
	UserGroup   string `json:"user_group"`
	UsedFiles   int64  `json:"used_files"`
	MaxFiles    int64  `json:"max_files"`
	RemainFiles int64  `json:"remain_files"`
	IsCustom    bool   `json:"is_custom"`
	QuotaReason string `json:"quota_reason"`
}

// FileQuotaResponse represents the standard API response for file quota.
type FileQuotaResponse struct {
	Success bool           `json:"success"`
	Code    int            `json:"code"`
	Data    *FileQuotaData `json:"data"`
}
