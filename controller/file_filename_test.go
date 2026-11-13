package controller

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileUploadSpecialFilenamePublicDownload(t *testing.T) {
	router := setupFileTestEnv(t)
	directory := t.TempDir()
	media := httptest.NewServer(http.FileServer(http.Dir(directory)))
	t.Cleanup(media.Close)
	t.Setenv("FILE_PUBLIC_BASE_URL", media.URL)
	driver := storage.NewLocalStorageDriver(directory)
	storage.SetGlobalDriver(driver)
	filename := "Marcel Duvall_love don’t pay #DuTime 中文?%.mp4"
	payload := []byte("reference upload round-trip")
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(payload)
	require.NoError(t, err)
	require.NoError(t, form.Close())
	request := httptest.NewRequest(http.MethodPost, "https://app.example/api/v1/files/upload/stream", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var uploaded dto.FileUploadResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
	require.NotNil(t, uploaded.Data)
	assert.Equal(t, filename, uploaded.Data.OriginalName)
	parsed, err := url.Parse(uploaded.Data.FileURL)
	require.NoError(t, err)
	assert.Empty(t, parsed.RawQuery)
	assert.Empty(t, parsed.Fragment)
	download, err := media.Client().Get(uploaded.Data.FileURL)
	require.NoError(t, err)
	defer download.Body.Close()
	require.Equal(t, http.StatusOK, download.StatusCode)
	received, err := io.ReadAll(download.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, received)
	require.NoError(t, driver.Delete(context.Background(), parsed.Path[1:]))
}
