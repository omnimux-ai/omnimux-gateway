package storage

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicURLPreservesObjectKey(t *testing.T) {
	for _, key := range []string{
		"videos/Marcel Duvall_love don’t pay #DuTime_7691175798727806239.mp4",
		"photos/中文 ?draft=1&x=2#封面%.png",
		"videos/nested/already%23encoded+clip.mp4",
		"videos/https://other.example/clip.mp4",
	} {
		for _, kind := range []string{"local", "r2"} {
			t.Run(kind+"/"+key, func(t *testing.T) {
				base := "https://cdn.example/media"
				var driver StorageDriver
				if kind == "local" {
					driver = &LocalStorageDriver{publicBaseURL: base}
				} else {
					driver = &R2StorageDriver{publicBaseURL: base}
				}
				parsed, err := url.Parse(driver.GetPublicURL(key))
				require.NoError(t, err)
				assert.Equal(t, "cdn.example", parsed.Host)
				assert.Equal(t, "/media/"+key, parsed.Path)
				assert.Empty(t, parsed.Fragment)
				assert.Empty(t, parsed.RawQuery)
				assert.NotContains(t, parsed.String(), " ")
			})
		}
	}
}
