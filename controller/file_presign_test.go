package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fork: r2-file-storage-presign — the ticket half of the upload surface.
//
// The driver below hands out a ticket that points at a real in-process PUT
// endpoint, so presign → upload → confirm runs as one loop with no network and
// no cloud credentials.
type presignTestObject struct {
	body        []byte
	contentType string
}

type presignTestDriver struct {
	objects    map[string]presignTestObject
	server     *httptest.Server
	publicURL  string
	presigns   int
	uploads    int
	deletes    int
	presignErr error
}

func newPresignTestDriver(t *testing.T, publicURL string) *presignTestDriver {
	t.Helper()
	driver := &presignTestDriver{
		objects:   map[string]presignTestObject{},
		publicURL: strings.TrimRight(publicURL, "/"),
	}
	driver.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		driver.objects[strings.TrimPrefix(r.URL.Path, "/")] = presignTestObject{
			body:        body,
			contentType: r.Header.Get("Content-Type"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(driver.server.Close)
	return driver
}

func (d *presignTestDriver) put(t *testing.T, key string, body []byte, contentType string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, d.server.URL+"/"+strings.TrimLeft(key, "/"), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	resp, err := d.server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (d *presignTestDriver) Upload(context.Context, string, []byte, string) (string, error) {
	d.uploads++
	return "", fmt.Errorf("relay upload is not part of this path")
}

func (d *presignTestDriver) Get(_ context.Context, key string) (io.ReadCloser, string, int64, error) {
	object, ok := d.objects[strings.TrimPrefix(key, "/")]
	if !ok {
		return nil, "", 0, fmt.Errorf("object not found: %s", key)
	}
	contentType := object.contentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return io.NopCloser(bytes.NewReader(object.body)), contentType, int64(len(object.body)), nil
}

func (d *presignTestDriver) Delete(_ context.Context, key string) error {
	d.deletes++
	delete(d.objects, strings.TrimPrefix(key, "/"))
	return nil
}

func (d *presignTestDriver) GetPublicURL(key string) string {
	if d.publicURL == "" {
		// A deployment with no configured public origin: the gateway serves the
		// object itself, exactly as the relay route does.
		return ""
	}
	return d.publicURL + "/" + strings.TrimLeft(key, "/")
}

func (d *presignTestDriver) IsR2() bool { return true }

func (d *presignTestDriver) PresignPut(_ context.Context, key string, contentType string, ttl time.Duration) (*storage.PresignResult, error) {
	if d.presignErr != nil {
		return nil, d.presignErr
	}
	d.presigns++
	return &storage.PresignResult{
		URL:       d.server.URL + "/" + strings.TrimLeft(key, "/"),
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

type presignTicket struct {
	FileID        string            `json:"file_id"`
	UploadURL     string            `json:"upload_url"`
	UploadMethod  string            `json:"upload_method"`
	UploadHeaders map[string]string `json:"upload_headers"`
	ResourceURL   string            `json:"resource_url"`
	DownloadURL   string            `json:"download_url"`
	ExpiresAt     string            `json:"expires_at"`
}

type presignEnvelope struct {
	Success bool          `json:"success"`
	Code    int           `json:"code"`
	Msg     string        `json:"msg"`
	Data    presignTicket `json:"data"`
}

func postJSON(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func requestTicket(t *testing.T, router http.Handler, body map[string]any) presignEnvelope {
	t.Helper()
	recorder := postJSON(t, router, "/api/v1/files/upload/presign", body)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var envelope presignEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	return envelope
}

func ticketBody() map[string]any {
	return map[string]any{
		"file_name":   "clip.mp4",
		"mime_type":   "video/mp4",
		"file_size":   4,
		"upload_path": "videos",
	}
}

func TestFilePresignUpload_UploadsStraightToStorageAndConfirms(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)

	envelope := requestTicket(t, router, ticketBody())
	ticket := envelope.Data
	assert.Equal(t, http.MethodPut, ticket.UploadMethod)
	assert.NotEmpty(t, ticket.UploadURL)
	assert.Equal(t, "https://cdn.example/videos/"+ticket.FileID+"_clip.mp4", ticket.ResourceURL)
	assert.NotEmpty(t, ticket.DownloadURL)
	assert.NotEmpty(t, ticket.ExpiresAt)

	// The gateway issued a ticket and kept no other trace: the row is pending.
	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusUploadPending, record.Status)
	count, err := model.CountActiveUserFiles(42)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "a pending ticket must not consume quota")

	// The client uploads the bytes itself; the gateway never sees them.
	driver.put(t, record.StorageKey, []byte("data"), "video/mp4")
	assert.Equal(t, 0, driver.uploads, "the relay path must not carry the bytes")

	recorder := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{
		"file_id":   ticket.FileID,
		"file_size": 4,
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var confirmed dtoFileUploadEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &confirmed))
	require.True(t, confirmed.Success)
	assert.Equal(t, ticket.FileID, confirmed.Data.FileID)
	assert.Equal(t, int64(4), confirmed.Data.FileSize)
	assert.Equal(t, "video/mp4", confirmed.Data.MimeType)
	assert.Equal(t, ticket.ResourceURL, confirmed.Data.FileURL)

	confirmedRecord, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusActive, confirmedRecord.Status)
	count, err = model.CountActiveUserFiles(42)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

type dtoFileUploadEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		FileID   string `json:"file_id"`
		FileSize int64  `json:"file_size"`
		MimeType string `json:"mime_type"`
		FileURL  string `json:"file_url"`
	} `json:"data"`
}

func TestFilePresignUpload_RefusesWhenStorageCannotIssueTickets(t *testing.T) {
	router := setupFileTestEnv(t) // local disk driver: no presigning, no public origin

	recorder := postJSON(t, router, "/api/v1/files/upload/presign", ticketBody())
	assert.Equal(t, http.StatusNotImplemented, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), "/upload/stream")
}

func TestFilePresignUpload_RefusesAPublicURLOnTheApplicationHost(t *testing.T) {
	router := setupFileTestEnv(t)
	storage.SetGlobalDriver(newPresignTestDriver(t, "https://app.example"))

	recorder := postJSON(t, router, "/api/v1/files/upload/presign", ticketBody())
	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
}

func TestFilePresignUpload_PublishesThroughTheGatewayWithoutAPublicOrigin(t *testing.T) {
	// Production runs with an empty public origin: the relay route falls back to
	// the gateway download address, and direct upload must not be refused there.
	router := setupFileTestEnv(t)
	storage.SetGlobalDriver(newPresignTestDriver(t, ""))

	ticket := requestTicket(t, router, ticketBody()).Data
	assert.NotEmpty(t, ticket.UploadURL, "a ticket must still be issued")
	assert.Contains(t, ticket.ResourceURL, "/api/v1/files/download/"+ticket.FileID)

	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, ticket.ResourceURL, record.FileUrl)
}

func TestFilePresignUpload_ValidatesItsInput(t *testing.T) {
	router := setupFileTestEnv(t)
	storage.SetGlobalDriver(newPresignTestDriver(t, "https://cdn.example"))

	cases := []struct {
		name string
		body map[string]any
	}{
		{name: "missing file name", body: map[string]any{"mime_type": "video/mp4", "file_size": 4}},
		{name: "missing content type", body: map[string]any{"file_name": "clip.mp4", "file_size": 4}},
		{name: "size beyond the video cap", body: map[string]any{"file_name": "clip.mp4", "mime_type": "video/mp4", "file_size": testVideoUploadMaxBytes + 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := postJSON(t, router, "/api/v1/files/upload/presign", tc.body)
			assert.GreaterOrEqual(t, recorder.Code, http.StatusBadRequest)
			assert.Less(t, recorder.Code, http.StatusInternalServerError)
		})
	}
}

func TestFileConfirmUpload_RejectsAnUploadThatNeverArrived(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)
	ticket := requestTicket(t, router, ticketBody()).Data

	recorder := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{
		"file_id":   ticket.FileID,
		"file_size": 4,
	})
	assert.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())

	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusUploadRejected, record.Status)
}

func TestFileConfirmUpload_RejectsAndRemovesAMismatchedObject(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)
	ticket := requestTicket(t, router, ticketBody()).Data

	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	driver.put(t, record.StorageKey, []byte("five!"), "video/mp4")

	recorder := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{
		"file_id":   ticket.FileID,
		"file_size": 4,
	})
	assert.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	_, present := driver.objects[record.StorageKey]
	assert.False(t, present, "a rejected upload must not leave an object behind")

	record, err = model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusUploadRejected, record.Status)
}

func TestFileConfirmUpload_IsSingleUse(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)
	ticket := requestTicket(t, router, ticketBody()).Data

	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	driver.put(t, record.StorageKey, []byte("data"), "video/mp4")

	first := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "file_size": 4})
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	second := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "file_size": 4})
	assert.Equal(t, http.StatusConflict, second.Code, second.Body.String())
}

func TestFileConfirmUpload_DoesNotConfirmSomeoneElsesTicket(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)

	foreign := &model.File{
		FileId:     "file_foreign",
		UserId:     43,
		FileName:   "clip.mp4",
		FileSize:   4,
		MimeType:   "video/mp4",
		StorageKey: "videos/file_foreign_clip.mp4",
		FileUrl:    "https://cdn.example/videos/file_foreign_clip.mp4",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Minute),
		Status:     model.FileStatusUploadPending,
	}
	require.NoError(t, model.CreateFile(foreign))
	driver.put(t, foreign.StorageKey, []byte("data"), "video/mp4")

	recorder := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": foreign.FileId, "file_size": 4})
	assert.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())

	stored, err := model.GetFileByFileId(foreign.FileId)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusUploadPending, stored.Status, "another user's ticket must be untouched")
}

func TestFileConfirmUpload_RefusesWhenQuotaFilledUpAfterTheTicket(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)
	ticket := requestTicket(t, router, ticketBody()).Data

	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	driver.put(t, record.StorageKey, []byte("data"), "video/mp4")

	fillQuota(t, 42)

	recorder := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "file_size": 4})
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code, recorder.Body.String())

	released, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusUploadRejected, released.Status)
}

// fillQuota parks the user at the file ceiling with active, unexpired records.
func fillQuota(t *testing.T, userId int) {
	t.Helper()
	now := time.Now()
	files := make([]model.File, 0, model.DefaultUserMaxFiles)
	for i := 0; i < model.DefaultUserMaxFiles; i++ {
		files = append(files, model.File{
			FileId:     fmt.Sprintf("file_quota_%d", i),
			UserId:     userId,
			FileName:   "held.bin",
			FileSize:   1,
			MimeType:   "application/octet-stream",
			StorageKey: fmt.Sprintf("files/held_%d.bin", i),
			FileUrl:    "https://cdn.example/held.bin",
			CreatedAt:  now,
			ExpiresAt:  now.Add(model.DefaultFileExpireDuration),
			Status:     model.FileStatusActive,
		})
	}
	require.NoError(t, model.DB.CreateInBatches(files, 500).Error)
}
