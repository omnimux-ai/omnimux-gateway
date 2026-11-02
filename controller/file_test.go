package controller

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	gingzip "github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	test1x1PNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
)

func setupFileTestEnv(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	previousDB := model.DB
	previousDriver := storage.GetStorageDriver()
	previousAddress := system_setting.ServerAddress
	previousTrustedURLs := common.SessionCookieTrustedURLs
	model.DB = db
	require.NoError(t, model.DB.AutoMigrate(&model.File{}, &model.User{}))
	require.NoError(t, model.DB.Create(&model.User{Id: 42, Username: "file-test-owner", Status: 1}).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		model.DB = previousDB
		storage.SetGlobalDriver(previousDriver)
		system_setting.ServerAddress = previousAddress
		common.SessionCookieTrustedURLs = previousTrustedURLs
		assert.NoError(t, sqlDB.Close())
	})
	system_setting.ServerAddress = "https://app.example"
	common.SessionCookieTrustedURLs = nil
	t.Setenv("FILE_PUBLIC_BASE_URL", "")

	tempDir := t.TempDir()
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(tempDir))

	router := gin.New()
	router.Use(gin.Recovery())

	// Simulated auth middleware for tests: injects user id=42
	authMiddleware := func(c *gin.Context) {
		c.Set("id", 42)
		c.Set("group", "default")
		c.Next()
	}

	for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
		api := router.Group(prefix)
		if prefix == "/api/v1/files" {
			api.Use(gingzip.Gzip(gingzip.DefaultCompression))
		}
		api.GET("/download/:file_id", FileDownload)
		authed := api.Group("/")
		authed.Use(authMiddleware)
		authed.POST("/upload/base64", FileUploadBase64)
		authed.POST("/upload/stream", FileUploadStream)
		authed.POST("/upload/url", FileUploadURL)
		authed.POST("/upload/presign", FilePresignUpload)
		authed.POST("/upload/confirm", FileConfirmUpload)
		authed.GET("/quota", FileGetQuota)
	}

	return router
}

// assertIsolatedFileResponse checks the browser boundary and exact streamed content.
func assertIsolatedFileResponse(t *testing.T, response *httptest.ResponseRecorder, data []byte, mimeType string) {
	t.Helper()
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "sandbox; default-src 'none'", response.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", response.Header().Get("Referrer-Policy"))
	assert.Equal(t, mimeType, response.Header().Get("Content-Type"))
	assert.Equal(t, fmt.Sprint(len(data)), response.Header().Get("Content-Length"))
	assert.Contains(t, response.Header().Get("Content-Disposition"), "inline;")
	assert.Empty(t, response.Header().Get("Location"))
	assert.Equal(t, data, response.Body.Bytes())
}

type fileStorageTestDriver struct {
	storage.StorageDriver
	publicURL string
	uploadURL *string
	r2        bool
	mimeType  string
	reads     int
	uploads   int
	deletes   int
	lastKey   string
	deleteErr error
}

func (d *fileStorageTestDriver) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	d.uploads++
	d.lastKey = key
	publicURL, err := d.StorageDriver.Upload(ctx, key, data, contentType)
	if d.uploadURL != nil {
		return *d.uploadURL, err
	}
	if d.publicURL != "" {
		publicURL = d.publicURL
	}
	return publicURL, err
}

func (d *fileStorageTestDriver) Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	d.reads++
	reader, mimeType, size, err := d.StorageDriver.Get(ctx, key)
	if d.mimeType != "" {
		mimeType = d.mimeType
	}
	return reader, mimeType, size, err
}

func (d *fileStorageTestDriver) GetPublicURL(key string) string {
	d.lastKey = key
	if d.publicURL != "" {
		return d.publicURL
	}
	return d.StorageDriver.GetPublicURL(key)
}

func (d *fileStorageTestDriver) Delete(ctx context.Context, key string) error {
	d.deletes++
	if d.deleteErr != nil {
		return d.deleteErr
	}
	return d.StorageDriver.Delete(ctx, key)
}

func (d *fileStorageTestDriver) IsR2() bool { return d.r2 }

func newFileUploadRequest(t *testing.T, prefix, ingress, mimeType string, data []byte) *http.Request {
	t.Helper()
	var request *http.Request
	switch ingress {
	case "base64":
		body, err := common.Marshal(gin.H{"base64_data": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), "file_name": "upload.html"})
		require.NoError(t, err)
		request = httptest.NewRequest(http.MethodPost, prefix+"/upload/base64", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	case "stream":
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": {`form-data; name="file"; filename="upload.html"`},
			"Content-Type":        {mimeType + "; charset=utf-8"},
		})
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		request = httptest.NewRequest(http.MethodPost, prefix+"/upload/stream", body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
	case "url":
		settings := system_setting.GetFetchSetting()
		previousAllow, previousPorts := settings.AllowPrivateIp, settings.AllowedPorts
		settings.AllowPrivateIp, settings.AllowedPorts = true, []string{"1-65535"}
		t.Cleanup(func() { settings.AllowPrivateIp, settings.AllowedPorts = previousAllow, previousPorts })
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", mimeType+"; charset=utf-8")
			_, _ = w.Write(data)
		}))
		t.Cleanup(server.Close)
		body, err := common.Marshal(gin.H{"file_url": server.URL + "/upload.html"})
		require.NoError(t, err)
		request = httptest.NewRequest(http.MethodPost, prefix+"/upload/url", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	default:
		t.Fatalf("unsupported upload ingress %q", ingress)
	}
	return request
}

func TestFileUploadsIsolateActiveContent(t *testing.T) {
	for _, ingress := range []string{"base64", "stream", "url"} {
		for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
			for _, sample := range []struct {
				name, mimeType, body string
			}{
				{"html", "text/html", `<script>fetch('/api/user/auth/refresh',{method:'POST'})</script>`},
				{"svg", "image/svg+xml", `<svg xmlns="http://www.w3.org/2000/svg" onload="fetch('/api/user/auth/refresh')"/>`},
				{"declared_png", "image/png", `<html><script>fetch('/api/user/auth/refresh')</script></html>`},
			} {
				t.Run(ingress+prefix+"/"+sample.name, func(t *testing.T) {
					router := setupFileTestEnv(t)
					request := newFileUploadRequest(t, prefix, ingress, sample.mimeType, []byte(sample.body))
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					require.Equal(t, http.StatusOK, response.Code, response.Body.String())
					var uploaded dto.FileUploadResponse
					require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
					require.True(t, uploaded.Success)
					require.NotNil(t, uploaded.Data)
					assert.Equal(t, uploaded.Data.DownloadURL, uploaded.Data.FileURL)
					for _, alias := range []string{"/api/v1/files", "/v1/files"} {
						download := httptest.NewRecorder()
						router.ServeHTTP(download, httptest.NewRequest(http.MethodGet, alias+"/download/"+uploaded.Data.FileID, nil))
						assertIsolatedFileResponse(t, download, []byte(sample.body), sample.mimeType)
					}
				})
			}
		}
	}
}

func TestFileDownloadIsolatesLegacyMIMEAndPreservesBytes(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString(test1x1PNG)
	require.NoError(t, err)
	utf16HTML := []byte{0xff, 0xfe}
	for _, char := range []byte("<script>alert(1)</script>") {
		utf16HTML = append(utf16HTML, char, 0)
	}
	for _, sample := range []struct {
		name, storedMIME, storageMIME, expectedMIME string
		body                                        []byte
	}{
		{"html_parameters", "Text/HTML; charset=utf-8", "", "Text/HTML; charset=utf-8", []byte("<script>alert(1)</script>")},
		{"html_utf16", "text/html; charset=utf-16le", "", "text/html; charset=utf-16le", utf16HTML},
		{"xhtml", "application/xhtml+xml", "", "application/xhtml+xml", []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><script>alert(1)</script></html>`)},
		{"xml", "application/xml", "", "application/xml", []byte(`<?xml-stylesheet type="text/xsl" href="/api/user/self"?><root/>`)},
		{"javascript", "application/javascript", "", "application/javascript", []byte("fetch('/api/user/self')")},
		{"unknown_mime", "x-custom/active", "", "x-custom/active", []byte("<script>alert(1)</script>")},
		{"empty_db_mime", "", "text/html", "text/html", []byte("<script>alert(1)</script>")},
		{"octet_db_mime", "application/octet-stream", "image/svg+xml", "image/svg+xml", []byte(`<svg onload="alert(1)"/>`)},
		{"both_mime_empty", "", "", "application/octet-stream", []byte("<script>alert(1)</script>")},
		{"plain_text", "text/plain", "text/html", "text/plain", []byte("<script>alert(1)</script>")},
		{"png_misleading_name", "image/png", "text/html", "image/png", png},
		{"video_bytes", "video/mp4", "", "video/mp4", []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0}},
		{"audio_bytes", "audio/wav", "", "audio/wav", []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x40\x1f\x00\x00\x01\x00\x08\x00data\x00\x00\x00\x00")},
		{"pdf_bytes", "application/pdf", "", "application/pdf", []byte("%PDF-1.7\n%%EOF")},
	} {
		t.Run(sample.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := &fileStorageTestDriver{StorageDriver: storage.GetStorageDriver(), r2: true, mimeType: sample.storageMIME}
			storage.SetGlobalDriver(driver)
			_, err := driver.Upload(context.Background(), "legacy", sample.body, sample.storageMIME)
			require.NoError(t, err)
			require.NoError(t, model.CreateFile(&model.File{
				FileId: "legacy", UserId: 42, FileName: "旧文件\".html", StorageKey: "legacy",
				MimeType: sample.storedMIME, FileSize: int64(len(sample.body)),
			}))
			for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
				request := httptest.NewRequest(http.MethodGet, prefix+"/download/legacy", nil)
				request.Header.Set("Range", "bytes=0-3")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				// StorageDriver still supplies the full object; do not invent partial-content support.
				assertIsolatedFileResponse(t, response, sample.body, sample.expectedMIME)
				assert.Empty(t, response.Header().Get("Content-Range"))
			}
			assert.Equal(t, 2, driver.reads)
		})
	}
}

func TestFileUploadBase64URLSafeSniffedHTMLRemainsIsolated(t *testing.T) {
	router := setupFileTestEnv(t)
	body := append([]byte("<!DOCTYPE html><script>alert(1)</script><!--"), bytes.Repeat([]byte{0xfb}, 6)...)
	encoded := base64.URLEncoding.EncodeToString(body)
	_, err := base64.StdEncoding.DecodeString(encoded)
	require.Error(t, err, "fixture must exercise the URL-safe decoder")
	requestBody, err := common.Marshal(gin.H{"base64_data": encoded})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/v1/files/upload/base64", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var uploaded dto.FileUploadResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
	require.NotNil(t, uploaded.Data)
	assert.Equal(t, "text/html", uploaded.Data.MimeType)
	for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
		download := httptest.NewRecorder()
		router.ServeHTTP(download, httptest.NewRequest(http.MethodGet, prefix+"/download/"+uploaded.Data.FileID, nil))
		assertIsolatedFileResponse(t, download, body, "text/html")
	}
}

func TestFileDownloadSecurityHeadersSurviveCompression(t *testing.T) {
	router := setupFileTestEnv(t)
	body := []byte(strings.Repeat("<script>alert(1)</script>", 20))
	_, err := storage.GetStorageDriver().Upload(context.Background(), "compressed", body, "text/html")
	require.NoError(t, err)
	require.NoError(t, model.CreateFile(&model.File{FileId: "compressed", UserId: 42, FileName: "page.html", StorageKey: "compressed", MimeType: "text/html"}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/files/download/compressed", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "gzip", response.Header().Get("Content-Encoding"))
	assert.Equal(t, "sandbox; default-src 'none'", response.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", response.Header().Get("Referrer-Policy"))
	reader, err := gzip.NewReader(response.Body)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, reader.Close()) })
	decoded, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, body, decoded)
}

func TestFilePublicURLsRespectApplicationOrigins(t *testing.T) {
	for _, candidate := range []struct {
		name, publicURL, requestURL, forwardedProto, serverAddress string
		trustedURLs                                                []string
		allowed                                                    bool
	}{
		{name: "request_origin", publicURL: "https://app.example/raw", requestURL: "https://app.example"},
		{name: "request_case_default_port", publicURL: "https://APP.EXAMPLE:443/raw", requestURL: "https://app.example"},
		{name: "browser_zero_padded_port", publicURL: "https://app.example:0443/raw", requestURL: "https://app.example"},
		{name: "browser_short_ipv4", publicURL: "http://127.1/raw", requestURL: "http://127.0.0.1"},
		{name: "browser_hex_ipv4", publicURL: "http://0x7f000001/raw", requestURL: "http://127.0.0.1"},
		{name: "browser_expanded_ipv6", publicURL: "http://[0:0:0:0:0:0:0:1]/raw", requestURL: "http://[::1]"},
		{name: "browser_unicode_host", publicURL: "https://bücher.example/raw", requestURL: "https://xn--bcher-kva.example"},
		{name: "browser_application_port", publicURL: "https://canonical.example/raw", requestURL: "https://app.example", serverAddress: "https://canonical.example:0443"},
		{name: "browser_application_unicode", publicURL: "https://xn--bcher-kva.example/raw", requestURL: "https://app.example", serverAddress: "https://bücher.example"},
		{name: "browser_request_ipv4", publicURL: "http://127.0.0.1/raw", requestURL: "http://127.1"},
		{name: "browser_trusted_port", publicURL: "https://trusted.example/raw", requestURL: "https://app.example", trustedURLs: []string{"https://trusted.example:0443"}},
		{name: "request_http_default_port", publicURL: "http://APP.EXAMPLE:80/raw", requestURL: "http://app.example"},
		{name: "forwarded_https", publicURL: "https://app.example/raw", requestURL: "http://app.example", forwardedProto: "https"},
		{name: "canonical_origin", publicURL: "https://CANONICAL.EXAMPLE:443/raw", requestURL: "https://app.example", serverAddress: "https://canonical.example/dashboard/"},
		{name: "trusted_origin", publicURL: "https://TRUSTED.EXAMPLE:443/raw", requestURL: "https://app.example", trustedURLs: []string{"https://trusted.example"}},
		{name: "empty", requestURL: "https://app.example"},
		{name: "relative", publicURL: "/raw", requestURL: "https://app.example"},
		{name: "protocol_relative", publicURL: "//cdn.example/raw", requestURL: "https://app.example"},
		{name: "userinfo", publicURL: "https://user@cdn.example/raw", requestURL: "https://app.example"},
		{name: "non_http", publicURL: "javascript:alert(1)", requestURL: "https://app.example"},
		{name: "missing_host", publicURL: "https:///raw", requestURL: "https://app.example"},
		{name: "bad_escape", publicURL: "https://cdn.example/%zz", requestURL: "https://app.example"},
		{name: "bad_port", publicURL: "https://cdn.example:invalid/raw", requestURL: "https://app.example"},
		{name: "fragment", publicURL: "https://cdn.example/raw#part", requestURL: "https://app.example"},
		{name: "malformed_application", publicURL: "https://cdn.example/raw", requestURL: "https://app.example", serverAddress: ":invalid"},
		{name: "isolated_origin", publicURL: "https://cdn.example/raw", requestURL: "https://app.example", allowed: true},
		{name: "isolated_case_default_port", publicURL: "https://CDN.EXAMPLE:443/raw", requestURL: "https://app.example", allowed: true},
		{name: "same_host_other_port", publicURL: "https://app.example:444/raw", requestURL: "https://app.example"},
		{name: "same_host_other_scheme", publicURL: "http://app.example/raw", requestURL: "https://app.example"},
		{name: "trusted_host_other_port", publicURL: "http://trusted.example:8080/raw", requestURL: "https://app.example", trustedURLs: []string{"https://trusted.example"}},
		{name: "same_host_trailing_dot", publicURL: "https://app.example./raw", requestURL: "https://app.example"},
		{name: "isolated_ipv4", publicURL: "http://192.0.2.2/raw", requestURL: "http://192.0.2.1", allowed: true},
		{name: "isolated_ipv6", publicURL: "http://[2001:db8::2]/raw", requestURL: "http://[2001:db8::1]", allowed: true},
		{name: "isolated_query", publicURL: "https://cdn.example/raw?signature=fixture", requestURL: "https://app.example", allowed: true},
	} {
		for _, r2 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/r2=%t", candidate.name, r2), func(t *testing.T) {
				router := setupFileTestEnv(t)
				system_setting.ServerAddress = candidate.serverAddress
				common.SessionCookieTrustedURLs = candidate.trustedURLs
				driver := &fileStorageTestDriver{StorageDriver: storage.GetStorageDriver(), publicURL: candidate.publicURL, r2: r2}
				storage.SetGlobalDriver(driver)
				body := []byte("<script>alert(1)</script>")
				encoded, err := common.Marshal(gin.H{"base64_data": "data:text/html;base64," + base64.StdEncoding.EncodeToString(body)})
				require.NoError(t, err)
				request := httptest.NewRequest(http.MethodPost, candidate.requestURL+"/api/v1/files/upload/base64", bytes.NewReader(encoded))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Forwarded-Proto", candidate.forwardedProto)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				fileID := "legacy"
				if candidate.publicURL != "" && !candidate.allowed {
					require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
					assert.Zero(t, driver.uploads)
					var count int64
					require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
					assert.Zero(t, count)
					require.NotEmpty(t, driver.lastKey)
					_, _, _, err := driver.StorageDriver.Get(context.Background(), driver.lastKey)
					assert.ErrorIs(t, err, os.ErrNotExist)
					// Old raw objects are seeded separately; rejected uploads create nothing.
					_, err = driver.StorageDriver.Upload(context.Background(), "legacy", body, "text/html")
					require.NoError(t, err)
					require.NoError(t, model.CreateFile(&model.File{FileId: fileID, UserId: 42, StorageKey: "legacy", FileName: "old.html", MimeType: "text/html"}))
				} else {
					require.Equal(t, http.StatusOK, response.Code, response.Body.String())
					var uploaded dto.FileUploadResponse
					require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
					require.NotNil(t, uploaded.Data)
					fileID = uploaded.Data.FileID
					expectedURL := uploaded.Data.DownloadURL
					if candidate.allowed {
						expectedURL = candidate.publicURL
					}
					assert.Equal(t, expectedURL, uploaded.Data.FileURL)
					record, err := model.GetFileByFileId(fileID)
					require.NoError(t, err)
					assert.Equal(t, expectedURL, record.FileUrl)
					assert.Equal(t, 1, driver.uploads)
				}
				for _, alias := range []string{"/api/v1/files", "/v1/files"} {
					request := httptest.NewRequest(http.MethodGet, candidate.requestURL+alias+"/download/"+fileID, nil)
					request.Header.Set("X-Forwarded-Proto", candidate.forwardedProto)
					download := httptest.NewRecorder()
					router.ServeHTTP(download, request)
					if r2 && candidate.allowed {
						assert.Equal(t, http.StatusFound, download.Code)
						assert.Equal(t, candidate.publicURL, download.Header().Get("Location"))
					} else {
						assertIsolatedFileResponse(t, download, body, "text/html")
					}
				}
				if r2 && candidate.allowed {
					assert.Zero(t, driver.reads)
				} else {
					assert.Equal(t, 2, driver.reads)
				}
			})
		}
	}
}

func TestFileUnsafePublicStorageCreatesNoRawHTTPObject(t *testing.T) {
	router := setupFileTestEnv(t)
	directory := t.TempDir()
	rawFiles := http.StripPrefix("/raw/", http.FileServer(http.Dir(directory)))
	router.GET("/raw/*path", gin.WrapH(rawFiles))
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	t.Setenv("FILE_PUBLIC_BASE_URL", server.URL+"/raw")
	nativeDriver := storage.NewLocalStorageDriver(directory)
	driver := &fileStorageTestDriver{StorageDriver: nativeDriver}
	storage.SetGlobalDriver(driver)

	// Prove this is a real unsafe raw serving path, not a URL-only storage stub.
	control := []byte("<!DOCTYPE html><script>alert(1)</script>")
	_, err := nativeDriver.Upload(context.Background(), "control.html", control, "text/html")
	require.NoError(t, err)
	controlResponse, err := server.Client().Get(server.URL + "/raw/control.html")
	require.NoError(t, err)
	controlBody, err := io.ReadAll(controlResponse.Body)
	require.NoError(t, controlResponse.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, controlResponse.StatusCode)
	assert.Contains(t, controlResponse.Header.Get("Content-Type"), "text/html")
	assert.Empty(t, controlResponse.Header.Get("Content-Security-Policy"))
	assert.Equal(t, control, controlBody)
	require.NoError(t, nativeDriver.Delete(context.Background(), "control.html"))
	png, err := base64.StdEncoding.DecodeString(test1x1PNG)
	require.NoError(t, err)

	for _, ingress := range []string{"base64", "stream", "url"} {
		for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
			for _, sample := range []struct {
				name, mimeType string
				body           []byte
			}{{"html", "text/html", control}, {"png", "image/png", png}} {
				t.Run(ingress+prefix+"/"+sample.name, func(t *testing.T) {
					request := newFileUploadRequest(t, server.URL+prefix, ingress, sample.mimeType, sample.body)
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
					assert.Contains(t, response.Body.String(), "公开存储地址必须使用独立于本站的域名")
					assert.Zero(t, driver.uploads)
					assert.Zero(t, driver.deletes)
					var count int64
					require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
					assert.Zero(t, count)
					require.NotEmpty(t, driver.lastKey)
					rawResponse, err := server.Client().Get(nativeDriver.GetPublicURL(driver.lastKey))
					require.NoError(t, err)
					_, err = io.Copy(io.Discard, rawResponse.Body)
					require.NoError(t, rawResponse.Body.Close())
					require.NoError(t, err)
					assert.Equal(t, http.StatusNotFound, rawResponse.StatusCode)
					entries, err := os.ReadDir(directory)
					require.NoError(t, err)
					assert.Empty(t, entries)
				})
			}
		}
	}
}

func TestFileUploadPublicURLMismatchFailsAndCleansCreatedObject(t *testing.T) {
	for _, sample := range []struct {
		name, expectedURL, returnedURL string
		deleteFailure                  bool
	}{
		{"private_became_public", "", "https://app.example/raw", false},
		{"isolated_became_application", "https://cdn.example/raw", "https://app.example/raw", false},
		{"isolated_became_private", "https://cdn.example/raw", "", false},
		{"changed_isolated_url", "https://cdn.example/raw", "https://other.example/raw", false},
		{"cleanup_failure_is_reported", "https://cdn.example/raw", "https://app.example/raw", true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := &fileStorageTestDriver{
				StorageDriver: storage.GetStorageDriver(), publicURL: sample.expectedURL, uploadURL: &sample.returnedURL, r2: true,
			}
			if sample.deleteFailure {
				driver.deleteErr = errors.New("fixture delete failure")
			}
			storage.SetGlobalDriver(driver)
			request := newFileUploadRequest(t, "https://app.example/api/v1/files", "base64", "text/html", []byte("<script>alert(1)</script>"))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Equal(t, 1, driver.uploads)
			assert.Equal(t, 1, driver.deletes)
			var count int64
			require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
			assert.Zero(t, count)
			reader, _, _, err := driver.StorageDriver.Get(context.Background(), driver.lastKey)
			if sample.deleteFailure {
				require.NoError(t, err)
				require.NoError(t, reader.Close())
				assert.Contains(t, response.Body.String(), "删除已上传文件失败")
				assert.NotContains(t, response.Body.String(), "文件已删除")
			} else {
				assert.ErrorIs(t, err, os.ErrNotExist)
				assert.Contains(t, response.Body.String(), "文件已删除")
			}
		})
	}
}

func TestFileDownloadRejectsUnavailableRecordsBeforePublicRedirect(t *testing.T) {
	for _, state := range []string{"missing", "expired", "deleted"} {
		t.Run(state, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := &fileStorageTestDriver{StorageDriver: storage.GetStorageDriver(), r2: true, publicURL: "https://cdn.example/raw"}
			storage.SetGlobalDriver(driver)
			if state != "missing" {
				record := &model.File{FileId: state, UserId: 42, FileName: "old.html", StorageKey: state, MimeType: "text/html", Status: model.FileStatusDeleted}
				if state == "expired" {
					record.Status = model.FileStatusActive
					record.ExpiresAt = time.Now().Add(-time.Hour)
				}
				require.NoError(t, model.CreateFile(record))
			}
			for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, prefix+"/download/"+state, nil))
				assert.Equal(t, http.StatusNotFound, response.Code)
				assert.Empty(t, response.Header().Get("Location"))
			}
			assert.Zero(t, driver.reads)
		})
	}
}

func TestFileUploadBase64_Success(t *testing.T) {
	router := setupFileTestEnv(t)

	reqBody := fmt.Sprintf(`{"base64_data":"data:image/png;base64,%s","upload_path":"avatars","file_name":"my_avatar.png"}`, test1x1PNG)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/base64", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp dto.FileUploadResponse
	err := common.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, 200, resp.Code)
	require.NotNil(t, resp.Data)
	assert.Equal(t, "my_avatar.png", resp.Data.FileName)
	assert.Equal(t, "avatars", resp.Data.UploadPath)
	assert.Equal(t, "image/png", resp.Data.MimeType)
	assert.True(t, strings.HasPrefix(resp.Data.FileID, "file_"))
	assert.NotEmpty(t, resp.Data.FileURL)
	assert.NotEmpty(t, resp.Data.DownloadURL)

	// Verify expiration: ~72 hours
	uploadTime, err := time.Parse(time.RFC3339, resp.Data.UploadTime)
	require.NoError(t, err)
	expiresAt, err := time.Parse(time.RFC3339, resp.Data.ExpiresAt)
	require.NoError(t, err)
	diff := expiresAt.Sub(uploadTime)
	assert.InDelta(t, 72*time.Hour, diff, float64(2*time.Minute))

	// Verify DB record
	fileRecord, err := model.GetFileByFileId(resp.Data.FileID)
	require.NoError(t, err)
	assert.Equal(t, 42, fileRecord.UserId)
	assert.Equal(t, model.FileStatusActive, fileRecord.Status)
}

func TestFileUploadStream_Success(t *testing.T) {
	router := setupFileTestEnv(t)

	pngBytes, err := base64.StdEncoding.DecodeString(test1x1PNG)
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test_stream.png")
	require.NoError(t, err)
	_, err = part.Write(pngBytes)
	require.NoError(t, err)

	_ = writer.WriteField("upload_path", "test_uploads")
	_ = writer.WriteField("file_name", "custom_stream.png")
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/stream", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp dto.FileUploadResponse
	err = common.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	require.NotNil(t, resp.Data)
	assert.Equal(t, "custom_stream.png", resp.Data.FileName)
	assert.Equal(t, "test_uploads", resp.Data.UploadPath)
	assert.Equal(t, "image/png", resp.Data.MimeType)
}

func TestFileUploadURL_Success(t *testing.T) {
	router := setupFileTestEnv(t)

	fetchSetting := system_setting.GetFetchSetting()
	origAllow := fetchSetting.AllowPrivateIp
	origPorts := fetchSetting.AllowedPorts
	fetchSetting.AllowPrivateIp = true
	fetchSetting.AllowedPorts = append(fetchSetting.AllowedPorts, "1-65535")
	defer func() {
		fetchSetting.AllowPrivateIp = origAllow
		fetchSetting.AllowedPorts = origPorts
	}()

	pngBytes, err := base64.StdEncoding.DecodeString(test1x1PNG)
	require.NoError(t, err)

	// Mock server that provides the remote file
	mockServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "image/png")
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write(pngBytes)
	}))
	defer mockServer.Close()

	reqBody := fmt.Sprintf(`{"file_url":"%s/sample.png","upload_path":"remote","file_name":"saved.png"}`, mockServer.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/url", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp dto.FileUploadResponse
	err = common.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	require.NotNil(t, resp.Data)
	assert.Equal(t, "saved.png", resp.Data.FileName)
	assert.Equal(t, "remote", resp.Data.UploadPath)
	assert.Equal(t, "image/png", resp.Data.MimeType)
}

func TestFileQuotaAndLimitEnforcement(t *testing.T) {
	router := setupFileTestEnv(t)

	// 1. Initial quota check
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/quota", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var quotaResp dto.FileQuotaResponse
	err := common.Unmarshal(w.Body.Bytes(), &quotaResp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), quotaResp.Data.UsedFiles)
	assert.Equal(t, int64(2000), quotaResp.Data.MaxFiles)
	assert.Equal(t, int64(2000), quotaResp.Data.RemainFiles)

	// 2. Upload one file
	reqBody := fmt.Sprintf(`{"base64_data":"data:image/png;base64,%s"}`, test1x1PNG)
	reqUpload := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/base64", strings.NewReader(reqBody))
	reqUpload.Header.Set("Content-Type", "application/json")
	wUpload := httptest.NewRecorder()
	router.ServeHTTP(wUpload, reqUpload)
	require.Equal(t, http.StatusOK, wUpload.Code)

	// 3. Re-check quota
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	err = common.Unmarshal(w.Body.Bytes(), &quotaResp)
	require.NoError(t, err)
	assert.Equal(t, int64(1), quotaResp.Data.UsedFiles)
	assert.Equal(t, int64(1999), quotaResp.Data.RemainFiles)

	// 4. Test Quota Exhaustion (2000 files)
	// Seed 1999 more files for user 42
	now := time.Now()
	var dummyFiles []model.File
	for i := 0; i < 1999; i++ {
		dummyFiles = append(dummyFiles, model.File{
			FileId:     fmt.Sprintf("dummy_%d", i),
			UserId:     42,
			FileName:   "dummy.png",
			FileSize:   10,
			MimeType:   "image/png",
			StorageKey: fmt.Sprintf("dummy_%d.png", i),
			FileUrl:    "https://example.com/dummy",
			CreatedAt:  now,
			ExpiresAt:  now.Add(72 * time.Hour),
			Status:     model.FileStatusActive,
		})
	}
	require.NoError(t, model.DB.CreateInBatches(dummyFiles, 500).Error)

	// Total active files is now 2000
	count, err := model.CountActiveUserFiles(42)
	require.NoError(t, err)
	require.Equal(t, int64(2000), count)

	// Attempting to upload another file must fail with 429
	wUploadExhausted := httptest.NewRecorder()
	reqUpload2 := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/base64", strings.NewReader(reqBody))
	reqUpload2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wUploadExhausted, reqUpload2)

	require.Equal(t, http.StatusTooManyRequests, wUploadExhausted.Code)
	assert.Contains(t, wUploadExhausted.Body.String(), "文件配额已耗尽")
}

func TestFileDownloadAndExpiration(t *testing.T) {
	router := setupFileTestEnv(t)

	// 1. Upload a file
	reqBody := fmt.Sprintf(`{"base64_data":"data:image/png;base64,%s","file_name":"download_me.png"}`, test1x1PNG)
	reqUpload := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/base64", strings.NewReader(reqBody))
	reqUpload.Header.Set("Content-Type", "application/json")
	wUpload := httptest.NewRecorder()
	router.ServeHTTP(wUpload, reqUpload)
	require.Equal(t, http.StatusOK, wUpload.Code)

	var resp dto.FileUploadResponse
	err := common.Unmarshal(wUpload.Body.Bytes(), &resp)
	require.NoError(t, err)
	fileID := resp.Data.FileID

	// 2. Download valid file
	reqDownload := httptest.NewRequest(http.MethodGet, "/api/v1/files/download/"+fileID, nil)
	wDownload := httptest.NewRecorder()
	router.ServeHTTP(wDownload, reqDownload)

	require.Equal(t, http.StatusOK, wDownload.Code)
	assert.Equal(t, "image/png", wDownload.Header().Get("Content-Type"))

	// 3. Mark file expired in DB
	require.NoError(t, model.DB.Model(&model.File{}).Where("file_id = ?", fileID).Update("expires_at", time.Now().Add(-1*time.Hour)).Error)

	// 4. Download expired file must return 404
	wDownloadExpired := httptest.NewRecorder()
	router.ServeHTTP(wDownloadExpired, reqDownload)
	require.Equal(t, http.StatusNotFound, wDownloadExpired.Code)
	assert.Contains(t, wDownloadExpired.Body.String(), "文件已过期")
}

func TestR2StorageDriver_Mock(t *testing.T) {
	// Verify R2 driver public URL construction and initialization logic
	driver, err := storage.NewR2StorageDriver(
		"https://example.r2.cloudflarestorage.com",
		"my-bucket",
		"mock-ak",
		"mock-sk",
		"https://files.omnimux.ai",
	)
	require.NoError(t, err)
	assert.True(t, driver.IsR2())
	assert.Equal(t, "https://files.omnimux.ai/photos/sample.jpg", driver.GetPublicURL("photos/sample.jpg"))
}
