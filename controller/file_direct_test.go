package controller

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type temporaryHeadFileDriver struct {
	*presignTestDriver
	fail bool
}

func (driver *temporaryHeadFileDriver) Head(ctx context.Context, key string) (*storage.ObjectMetadata, error) {
	if driver.fail {
		return nil, errors.New("temporary timeout")
	}
	reader, contentType, size, err := driver.Get(ctx, key)
	if err != nil {
		return nil, storage.ErrObjectNotFound
	}
	reader.Close()
	return &storage.ObjectMetadata{ContentType: contentType, Size: size}, nil
}

func TestFileConfirmUploadCanRetryTemporaryMetadataFailure(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &temporaryHeadFileDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example"), fail: true}
	storage.SetGlobalDriver(driver)
	ticket := requestTicket(t, router, ticketBody()).Data
	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	driver.put(t, record.StorageKey, []byte("data"), "video/mp4")
	first := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "file_size": 4})
	assert.Equal(t, http.StatusServiceUnavailable, first.Code)
	record, err = model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusUploadPending, record.Status)
	driver.fail = false
	second := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "file_size": 4})
	assert.Equal(t, http.StatusOK, second.Code)
}

func TestFilePresignUploadCapsWriteLifetimeBelowRetention(t *testing.T) {
	t.Setenv(fileUploadPresignTTLEnv, "604800")
	assert.Less(t, fileUploadPresignTTL(), model.DefaultFileExpireDuration-time.Hour)
}

func TestFileConfirmUploadCannotReplaceTicketDeclaredSize(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)
	ticket := requestTicket(t, router, ticketBody()).Data
	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	driver.put(t, record.StorageKey, []byte("five!"), "video/mp4")
	response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "file_size": 5})
	assert.Equal(t, http.StatusConflict, response.Code)
	record, err = model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.NotEqual(t, model.FileStatusActive, record.Status)
}

func TestFileConfirmUploadRejectsContentTypeChangedFromTicket(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := newPresignTestDriver(t, "https://cdn.example")
	storage.SetGlobalDriver(driver)
	ticket := requestTicket(t, router, ticketBody()).Data
	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	driver.put(t, record.StorageKey, []byte("data"), "text/html")
	response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "file_size": 4})
	assert.Equal(t, http.StatusConflict, response.Code)
}
