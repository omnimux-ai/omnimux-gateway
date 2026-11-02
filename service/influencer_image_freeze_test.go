package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func imageFreezeEnvironment(t *testing.T, data []byte) (*frozenTestDriver, *http.Client, *int) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.File{}, &model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 41, Username: "image-freeze-owner", Status: 1}).Error)
	oldDB, oldDriver := model.DB, storage.GetStorageDriver()
	setting := system_setting.GetFetchSetting()
	oldSetting := *setting
	*setting = system_setting.FetchSetting{}
	model.DB = db
	driver := &frozenTestDriver{objects: map[string][]byte{}}
	storage.SetGlobalDriver(driver)
	calls := new(int)
	client := &http.Client{Transport: referenceFetchTransport(func(req *http.Request) (*http.Response, error) {
		*calls = *calls + 1
		assert.Empty(t, req.Header.Get("Authorization"))
		assert.Empty(t, req.Header.Get("Cookie"))
		body := data
		if *calls > 1 {
			body = []byte("changed-source")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body)), Request: req}, nil
	})}
	t.Cleanup(func() {
		model.DB = oldDB
		storage.SetGlobalDriver(oldDriver)
		*setting = oldSetting
		sqlDB, err := db.DB()
		require.NoError(t, err)
		assert.NoError(t, sqlDB.Close())
	})
	return driver, client, calls
}

func imageFreezePNG(t *testing.T, width, height int, alpha func(int, int) uint8) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 120, G: 160, B: 200, A: alpha(x, y)})
		}
	}
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}

func TestInfluencerImageFreezesSingleValidatedOriginalAndRetention(t *testing.T) {
	original := imageFreezePNG(t, 2, 3, func(int, int) uint8 { return 255 })
	driver, client, calls := imageFreezeEnvironment(t, original)
	expiry := time.Now().Add(23 * time.Minute)
	prepared, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/mutable.png", expiry, client)
	require.NoError(t, err)
	assert.Equal(t, 1, *calls)
	assert.Equal(t, 2, prepared.Width)
	assert.Equal(t, 3, prepared.Height)
	assert.Equal(t, "png", prepared.Format)
	assert.Equal(t, expiry.Unix(), prepared.ExpiresAt)
	digest := sha256.Sum256(original)
	assert.Equal(t, fmt.Sprintf("%x", digest), prepared.SHA256)
	file, err := model.GetFileByFileId(prepared.FileID)
	require.NoError(t, err)
	assert.True(t, expiry.Equal(file.ExpiresAt), "freezing must retain the original actual expiry")
	assert.Equal(t, model.FileStatusActive, file.Status)
	assert.Equal(t, original, driver.objects[file.StorageKey])
	got, err := ReadInfluencerFrozenImage(context.Background(), 41, prepared.InfluencerFrozenImage)
	require.NoError(t, err)
	assert.Equal(t, original, got)
	assert.Equal(t, 1, *calls, "restoration reads storage, never a changed source URL")
	prepared.Discard()
	prepared.Discard()
	assert.Empty(t, driver.objects)
	var count int64
	require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestInfluencerImageFreezeRejectsBeforeStorageOrCleansFailedUpload(t *testing.T) {
	valid := imageFreezePNG(t, 2, 2, func(int, int) uint8 { return 255 })
	for _, name := range []string{"no-owner", "no-expiry", "expired", "private-source", "invalid-image", "partial-upload", "wrong-storage-url", "confirmation-failure", "cancel-upload"} {
		t.Run(name, func(t *testing.T) {
			data := valid
			if name == "invalid-image" {
				data = []byte("not-image")
			}
			driver, client, calls := imageFreezeEnvironment(t, data)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			owner, raw, expiry := 41, "https://8.8.8.8/source", time.Now().Add(time.Hour)
			switch name {
			case "no-owner":
				owner = 0
			case "no-expiry":
				expiry = time.Time{}
			case "expired":
				expiry = time.Now().Add(-time.Hour)
			case "private-source":
				raw = "https://127.0.0.1/source"
			case "partial-upload":
				driver.failUpload = true
			case "wrong-storage-url":
				driver.mismatch = true
			case "confirmation-failure":
				require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("fail-image-confirm", func(db *gorm.DB) { db.AddError(fmt.Errorf("confirmation failed")) }))
			case "cancel-upload":
				driver.onUpload = func(context.Context) { cancel() }
			}
			prepared, err := FreezeInfluencerImage(ctx, owner, raw, expiry, client)
			require.Error(t, err)
			assert.Empty(t, prepared.URL)
			assert.Empty(t, driver.objects)
			var count int64
			require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
			assert.Zero(t, count)
			if name == "no-owner" || name == "no-expiry" || name == "expired" || name == "private-source" {
				assert.Zero(t, *calls)
			}
		})
	}
}

func TestInfluencerImageFailedCleanupRetainsOriginalExpiryRecord(t *testing.T) {
	driver, client, _ := imageFreezeEnvironment(t, imageFreezePNG(t, 2, 2, func(int, int) uint8 { return 255 }))
	driver.failUpload, driver.failDelete = true, true
	expiry := time.Now().Add(7 * time.Minute)
	_, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/source", expiry, client)
	require.Error(t, err)
	require.Len(t, driver.objects, 1)
	var file model.File
	require.NoError(t, model.DB.First(&file).Error)
	assert.Equal(t, model.FileStatusUploadPending, file.Status)
	assert.True(t, expiry.Equal(file.ExpiresAt))
	assert.NotEmpty(t, file.StorageKey, "failed cleanup must stay reachable by the existing expiry service")
}

func TestInfluencerImageRestoreFailsClosedForChangedOwnershipIdentityAndBytes(t *testing.T) {
	for _, name := range []string{"owner", "file-id", "status", "expired", "expiry-extended", "key-traversal", "not-frozen", "url", "mime", "metadata", "size", "hash", "invalid-hash", "bytes-changed", "ambiguous-url"} {
		t.Run(name, func(t *testing.T) {
			driver, client, calls := imageFreezeEnvironment(t, imageFreezePNG(t, 2, 3, func(int, int) uint8 { return 255 }))
			prepared, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/source", time.Now().Add(time.Hour), client)
			require.NoError(t, err)
			snapshot, owner := prepared.InfluencerFrozenImage, 41
			file, err := model.GetFileByFileId(snapshot.FileID)
			require.NoError(t, err)
			switch name {
			case "owner":
				owner = 42
			case "file-id":
				snapshot.FileID = "another-file"
			case "status":
				require.NoError(t, model.DB.Model(file).Update("status", model.FileStatusDeleted).Error)
			case "expired":
				snapshot.ExpiresAt = time.Now().Add(-time.Hour).Unix()
			case "expiry-extended":
				require.NoError(t, model.DB.Model(file).Update("expires_at", file.ExpiresAt.Add(time.Hour)).Error)
			case "key-traversal":
				require.NoError(t, model.DB.Model(file).Update("storage_key", "influencer-image/frozen/../x").Error)
			case "not-frozen":
				require.NoError(t, model.DB.Model(file).Update("upload_path", "user-uploads").Error)
			case "url":
				snapshot.URL += "?new-capability=untrusted"
			case "mime":
				require.NoError(t, model.DB.Model(file).Update("mime_type", "text/html").Error)
			case "metadata":
				snapshot.Width++
			case "size":
				require.NoError(t, model.DB.Model(file).Update("file_size", file.FileSize+1).Error)
			case "hash":
				snapshot.SHA256 = strings.Repeat("a", 64)
			case "invalid-hash":
				snapshot.SHA256 = "not-sha256"
			case "bytes-changed":
				driver.objects[file.StorageKey][len(driver.objects[file.StorageKey])-1] ^= 1
			case "ambiguous-url":
				duplicate := *file
				duplicate.Id = 0
				duplicate.FileId += "-duplicate"
				require.NoError(t, model.DB.Create(&duplicate).Error)
			}
			_, err = ReadInfluencerFrozenImage(context.Background(), owner, snapshot)
			require.Error(t, err)
			assert.Equal(t, 1, *calls)
		})
	}
}

func TestInfluencerImageMaskRequiresActualPNGSelectionOfOriginalDimensions(t *testing.T) {
	for _, name := range []string{"valid", "all-transparent", "all-opaque", "only-partial-alpha", "wrong-dimensions", "JPEG", "source-JPEG", "source-eXIf", "mask-eXIf", "source-text-with-eXIf", "mask-text-with-eXIf"} {
		t.Run(name, func(t *testing.T) {
			source := imageFreezePNG(t, 2, 2, func(int, int) uint8 { return 255 })
			switch name {
			case "source-JPEG":
				var out bytes.Buffer
				require.NoError(t, jpeg.Encode(&out, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil))
				source = out.Bytes()
			case "source-eXIf":
				source = imageFreezePNGChunk(source, "eXIf", []byte("II*\x00\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
			case "source-text-with-eXIf":
				source = imageFreezePNGChunk(source, "tEXt", []byte("Comment\x00literal eXIf is not orientation metadata"))
			}
			_, sourceClient, _ := imageFreezeEnvironment(t, source)
			prepared, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/source", time.Now().Add(time.Hour), sourceClient)
			require.NoError(t, err)
			width := 2
			if name == "wrong-dimensions" {
				width = 3
			}
			mask := imageFreezePNG(t, width, 2, func(x, y int) uint8 {
				switch name {
				case "all-transparent":
					return 0
				case "all-opaque", "JPEG":
					return 255
				case "only-partial-alpha":
					return 128
				}
				if x == 0 && y == 0 {
					return 0
				}
				return 255
			})
			if name == "JPEG" {
				var out bytes.Buffer
				require.NoError(t, jpeg.Encode(&out, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil))
				mask = out.Bytes()
			}
			if name == "mask-eXIf" {
				mask = imageFreezePNGChunk(mask, "eXIf", []byte("II*\x00\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
			}
			if name == "mask-text-with-eXIf" {
				mask = imageFreezePNGChunk(mask, "tEXt", []byte("Comment\x00literal eXIf is not orientation metadata"))
			}
			maskClient := &http.Client{Transport: referenceFetchTransport(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: int64(len(mask)), Body: io.NopCloser(bytes.NewReader(mask)), Request: req}, nil
			})}
			frozenMask, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/mask", time.Now().Add(time.Hour), maskClient)
			require.NoError(t, err)
			err = ValidateInfluencerFrozenImageMask(context.Background(), 41, prepared.InfluencerFrozenImage, frozenMask.InfluencerFrozenImage)
			if name == "valid" || name == "source-text-with-eXIf" || name == "mask-text-with-eXIf" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func imageFreezePNGChunk(data []byte, kind string, body []byte) []byte {
	chunk := make([]byte, len(body)+12)
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(body)))
	copy(chunk[4:8], []byte(kind))
	copy(chunk[8:], body)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	// Encoded fixture PNGs end in the 12-byte IEND chunk.
	out := append([]byte{}, data[:len(data)-12]...)
	out = append(out, chunk...)
	return append(out, data[len(data)-12:]...)
}

func TestInfluencerImageMaskRawByteLimitPreservesSourceAllowance(t *testing.T) {
	for _, tc := range []struct {
		name        string
		sourceBytes int
		maskBytes   int
		want        string
	}{
		{name: "mask below decimal limit", maskBytes: 3999999},
		{name: "mask at decimal limit", maskBytes: 4000000, want: "4,000,000"},
		{name: "mask below binary limit", maskBytes: 4194303, want: "4,000,000"},
		{name: "mask at old shared limit", maskBytes: 5242880, want: "4,000,000"},
		{name: "source at original limit", sourceBytes: 5242880},
		{name: "source above original limit", sourceBytes: 5242881, want: "5 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := imageFreezePNG(t, 2, 2, func(int, int) uint8 { return 255 })
			if tc.sourceBytes > 0 {
				source = imageFreezePNGChunk(source, "tEXt", append([]byte("Comment\x00"), make([]byte, tc.sourceBytes-len(source)-20)...))
				require.Len(t, source, tc.sourceBytes)
			}
			_, client, _ := imageFreezeEnvironment(t, source)
			frozenSource, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/source.png", time.Now().Add(time.Hour), client)
			if tc.sourceBytes > 5242880 {
				require.ErrorContains(t, err, tc.want)
				return
			}
			require.NoError(t, err)
			mask := imageFreezePNG(t, 2, 2, func(x, y int) uint8 {
				if x == 0 && y == 0 {
					return 0
				}
				return 255
			})
			if tc.maskBytes > 0 {
				mask = imageFreezePNGChunk(mask, "tEXt", append([]byte("Comment\x00"), make([]byte, tc.maskBytes-len(mask)-20)...))
				require.Len(t, mask, tc.maskBytes)
			}
			maskClient := &http.Client{Transport: referenceFetchTransport(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: int64(len(mask)), Body: io.NopCloser(bytes.NewReader(mask)), Request: req}, nil
			})}
			frozenMask, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/mask.png", time.Now().Add(time.Hour), maskClient)
			require.NoError(t, err)
			err = ValidateInfluencerFrozenImageMask(context.Background(), 41, frozenSource.InfluencerFrozenImage, frozenMask.InfluencerFrozenImage)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
			for _, item := range []struct {
				snapshot InfluencerFrozenImage
				original []byte
			}{{frozenSource.InfluencerFrozenImage, source}, {frozenMask.InfluencerFrozenImage, mask}} {
				actual, err := ReadInfluencerFrozenImage(context.Background(), 41, item.snapshot)
				require.NoError(t, err)
				assert.Equal(t, item.original, actual, "size validation never rewrites frozen input")
			}
			t.Logf("source_bytes=%d mask_bytes=%d rejected=%t", len(source), len(mask), tc.want != "")
		})
	}
}

func TestInfluencerImageFreezeChecksEveryRedirectOwnershipAndTightensExpiry(t *testing.T) {
	for _, name := range []string{"foreign-upload", "foreign-native-artifact", "missing-managed-upload", "owned-upload-shorter", "external-public"} {
		t.Run(name, func(t *testing.T) {
			original := imageFreezePNG(t, 2, 2, func(int, int) uint8 { return 255 })
			driver, _, _ := imageFreezeEnvironment(t, original)
			initialExpiry, targetExpiry := time.Now().Add(time.Hour), time.Now().Add(17*time.Minute)
			target := driver.GetPublicURL("uploads/redirect-target.png")
			if name == "foreign-upload" || name == "owned-upload-shorter" {
				owner := 41
				if name == "foreign-upload" {
					owner = 42
				}
				require.NoError(t, model.CreateFile(&model.File{FileId: "file-redirect-target", UserId: owner, FileName: "redirect-target.png", StorageKey: "uploads/redirect-target.png", FileSize: int64(len(original)), MimeType: "image/png", FileUrl: target, CreatedAt: time.Now(), ExpiresAt: targetExpiry, Status: model.FileStatusActive}))
			}
			if name == "foreign-native-artifact" {
				require.NoError(t, model.DB.AutoMigrate(&model.Task{}))
				oldSecret, oldAddress := common.CryptoSecret, system_setting.TaskPublicAddress
				common.CryptoSecret, system_setting.TaskPublicAddress = "redirect-image-test-secret", "https://8.8.4.4/media"
				t.Cleanup(func() { common.CryptoSecret, system_setting.TaskPublicAddress = oldSecret, oldAddress })
				require.NoError(t, model.DB.Create(&model.Task{TaskID: "foreign-redirect-task", UserId: 42, Status: model.TaskStatusSuccess}).Error)
				var err error
				target, err = BuildTaskArtifactContentURL("foreign-redirect-task", "image-main")
				require.NoError(t, err)
			}
			if name == "external-public" {
				target = "https://8.8.4.4/public/external.png"
			}
			gets := 0
			client := &http.Client{Transport: referenceFetchTransport(func(req *http.Request) (*http.Response, error) {
				gets++
				if gets == 1 {
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {target}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: int64(len(original)), Body: io.NopCloser(bytes.NewReader(original)), Request: req}, nil
			})}
			prepared, err := FreezeInfluencerImage(context.Background(), 41, "https://8.8.8.8/start.png", initialExpiry, client)
			if name != "owned-upload-shorter" && name != "external-public" {
				require.Error(t, err)
				assert.Equal(t, 1, gets, "foreign managed content must not be requested")
				assert.Empty(t, driver.objects)
				var frozen int64
				require.NoError(t, model.DB.Model(&model.File{}).Where("upload_path = ?", influencerFrozenImagePrefix).Count(&frozen).Error)
				assert.Zero(t, frozen)
				assert.NotContains(t, err.Error(), "access=")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 2, gets)
			wantExpiry := initialExpiry
			if name == "owned-upload-shorter" {
				wantExpiry = targetExpiry
			}
			assert.Equal(t, wantExpiry.Unix(), prepared.ExpiresAt)
			file, err := model.GetFileByFileId(prepared.FileID)
			require.NoError(t, err)
			assert.True(t, wantExpiry.Equal(file.ExpiresAt), "redirect retention must never extend either original source")
			assert.Equal(t, original, driver.objects[file.StorageKey])
		})
	}
}

func TestInfluencerImageFreezeNeverExtendsRegisteredSourceRetention(t *testing.T) {
	original := imageFreezePNG(t, 2, 2, func(int, int) uint8 { return 255 })
	driver, client, gets := imageFreezeEnvironment(t, original)
	actualExpiry := time.Now().Add(5 * time.Minute)
	raw := driver.GetPublicURL("uploads/original.png")
	require.NoError(t, model.CreateFile(&model.File{FileId: "file-actual-retention", UserId: 41, FileName: "original.png", StorageKey: "uploads/original.png", FileSize: int64(len(original)), MimeType: "image/png", FileUrl: raw, CreatedAt: time.Now(), ExpiresAt: actualExpiry, Status: model.FileStatusActive}))
	prepared, err := FreezeInfluencerImage(context.Background(), 41, raw, time.Now().Add(model.DefaultFileExpireDuration), client)
	require.NoError(t, err)
	assert.Equal(t, 1, *gets)
	assert.Equal(t, actualExpiry.Unix(), prepared.ExpiresAt)
	file, err := model.GetFileByFileId(prepared.FileID)
	require.NoError(t, err)
	assert.True(t, actualExpiry.Equal(file.ExpiresAt))
}

type imageSourcePrefixDriver struct {
	*frozenTestDriver
	base string
}

func (driver *imageSourcePrefixDriver) GetPublicURL(key string) string {
	return strings.TrimRight(driver.base, "/") + "/" + key
}

func TestInfluencerImageSourceExpiryKeepsManagedOwnershipAndAllowsExternalPublicURL(t *testing.T) {
	for _, name := range []string{"owned-upload", "foreign-upload", "expired-upload", "deleted-upload", "ambiguous-upload", "missing-managed-upload", "managed-alternate-port", "managed-normalized-path", "public-external", "same-host-outside-storage-prefix", "registered-external-foreign", "private-IP", "no-owner", "database-failure"} {
		t.Run(name, func(t *testing.T) {
			driver, _, _ := imageFreezeEnvironment(t, []byte("not fetched"))
			prefixDriver := &imageSourcePrefixDriver{frozenTestDriver: driver, base: "https://8.8.4.4/files"}
			storage.SetGlobalDriver(prefixDriver)
			raw, owner := prefixDriver.GetPublicURL("uploads/original.png"), 41
			expiry := time.Now().Add(29 * time.Minute)
			file := model.File{FileId: "file-original", UserId: 41, FileName: "original.png", FileSize: 123, MimeType: "image/png", StorageKey: "uploads/original.png", FileUrl: raw, CreatedAt: time.Now(), ExpiresAt: expiry, Status: model.FileStatusActive}
			createRecord, accepted := true, false
			switch name {
			case "owned-upload":
				accepted = true
			case "foreign-upload":
				file.UserId = 42
			case "expired-upload":
				file.ExpiresAt = time.Now().Add(-time.Hour)
			case "deleted-upload":
				file.Status = model.FileStatusDeleted
			case "missing-managed-upload":
				createRecord = false
			case "managed-alternate-port":
				createRecord = false
				raw = "https://8.8.4.4:443/files/uploads/original.png"
			case "managed-normalized-path":
				createRecord = false
				raw = "https://8.8.4.4/files/another/../uploads/original.png"
			case "public-external":
				createRecord, accepted = false, true
				raw = "https://8.8.8.8/public/reference.png"
			case "same-host-outside-storage-prefix":
				createRecord, accepted = false, true
				raw = "https://8.8.4.4/public/reference.png"
			case "registered-external-foreign":
				raw = "https://8.8.8.8/public/reference.png"
				file.FileUrl, file.UserId = raw, 42
			case "private-IP":
				createRecord = false
				raw = "https://127.0.0.1/private.png"
			case "no-owner":
				owner = 0
			case "database-failure":
				createRecord = false
				require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register("image-source-db-failure", func(db *gorm.DB) { db.AddError(fmt.Errorf("database unavailable")) }))
			}
			if createRecord {
				require.NoError(t, model.CreateFile(&file))
			}
			if name == "ambiguous-upload" {
				duplicate := file
				duplicate.Id, duplicate.FileId = 0, "file-duplicate"
				require.NoError(t, model.CreateFile(&duplicate))
			}
			before := time.Now()
			got, err := ResolveInfluencerImageSourceExpiry(context.Background(), owner, raw)
			if !accepted {
				require.Error(t, err)
				assert.True(t, got.IsZero())
				return
			}
			require.NoError(t, err)
			if name == "owned-upload" {
				assert.True(t, expiry.Equal(got), "managed uploads retain their actual expiry")
			} else {
				assert.False(t, got.Before(before.Add(model.DefaultFileExpireDuration)))
				assert.False(t, got.After(time.Now().Add(model.DefaultFileExpireDuration)))
			}
		})
	}
}

func TestInfluencerImageSourceExpiryRequiresOwnedSignedRetrievableNativeArtifact(t *testing.T) {
	for _, name := range []string{"owned-native", "escaped-native-ID", "foreign-native", "failed-native", "discarded-native", "missing-native", "ambiguous-native", "unsigned-native", "forged-access", "wrong-configured-origin", "wrong-path", "encoded-forged-path", "duplicate-access", "extra-query", "fragment"} {
		t.Run(name, func(t *testing.T) {
			_, _, _ = imageFreezeEnvironment(t, []byte("not fetched"))
			require.NoError(t, model.DB.AutoMigrate(&model.Task{}))
			oldSecret, oldPublic, oldServer := common.CryptoSecret, system_setting.TaskPublicAddress, system_setting.ServerAddress
			common.CryptoSecret = "image-source-test-signature-secret"
			system_setting.TaskPublicAddress, system_setting.ServerAddress = "https://8.8.8.8/media", "https://8.8.4.4/app"
			t.Cleanup(func() {
				common.CryptoSecret, system_setting.TaskPublicAddress, system_setting.ServerAddress = oldSecret, oldPublic, oldServer
			})
			task := model.Task{TaskID: "native-image-source", UserId: 41, Status: model.TaskStatusSuccess}
			if name == "escaped-native-ID" {
				task.TaskID = "native/image-source"
			}
			if name == "foreign-native" {
				task.UserId = 42
			}
			if name == "failed-native" {
				task.Status = model.TaskStatusFailure
			}
			if name == "discarded-native" {
				task.PrivateData.ResultDiscarded = true
			}
			if name != "missing-native" {
				require.NoError(t, model.DB.Create(&task).Error)
			}
			if name == "ambiguous-native" {
				duplicate := task
				duplicate.ID, duplicate.UserId = 0, 42
				require.NoError(t, model.DB.Create(&duplicate).Error)
			}
			raw, err := BuildTaskArtifactContentURL(task.TaskID, "image-main")
			require.NoError(t, err)
			parsed, err := url.Parse(raw)
			require.NoError(t, err)
			switch name {
			case "unsigned-native":
				parsed.RawQuery = ""
				raw = parsed.String()
			case "forged-access":
				parsed.RawQuery = "access=" + strings.Repeat("a", 43)
				raw = parsed.String()
			case "wrong-configured-origin":
				raw = strings.Replace(raw, "https://8.8.8.8/media", "https://8.8.4.4/app", 1)
			case "wrong-path":
				raw = strings.Replace(raw, "/artifacts/", "/artifacts/../artifacts/", 1)
			case "encoded-forged-path":
				raw = strings.Replace(raw, "/v1/tasks/", "/v1%2ftasks/", 1)
			case "duplicate-access":
				raw += "&access=" + parsed.Query().Get(TaskArtifactAccessQueryParameter)
			case "extra-query":
				raw += "&untrusted=extra"
			case "fragment":
				raw += "#untrusted"
			}
			before := time.Now()
			got, err := ResolveInfluencerImageSourceExpiry(context.Background(), 41, raw)
			if name == "owned-native" || name == "escaped-native-ID" {
				require.NoError(t, err)
				assert.False(t, got.Before(before.Add(model.DefaultFileExpireDuration)))
				assert.False(t, got.After(time.Now().Add(model.DefaultFileExpireDuration)))
			} else {
				require.Error(t, err)
				assert.True(t, got.IsZero())
			}
		})
	}
}

func TestInfluencerImageSourceExpiryHonorsCancellationBeforeLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ResolveInfluencerImageSourceExpiry(ctx, 41, "https://8.8.8.8/reference.png")
	assert.ErrorIs(t, err, context.Canceled)
}
