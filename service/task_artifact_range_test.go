package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTaskArtifactRange(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		size    int64
		start   int64
		length  int64
		partial bool
		wantErr bool
	}{
		{name: "no header serves the whole object", size: 100},
		{name: "unsupported unit serves the whole object", header: "items=0-1", size: 100},
		{name: "multiple ranges serve the whole object", header: "bytes=0-1,4-5", size: 100},
		{name: "open ended range", header: "bytes=10-", size: 100, start: 10, length: 90, partial: true},
		{name: "closed range", header: "bytes=10-19", size: 100, start: 10, length: 10, partial: true},
		{name: "range past the end is clamped", header: "bytes=10-999", size: 100, start: 10, length: 90, partial: true},
		{name: "suffix range", header: "bytes=-10", size: 100, start: 90, length: 10, partial: true},
		{name: "suffix longer than the object", header: "bytes=-999", size: 100, start: 0, length: 100, partial: true},
		{name: "start past the end is unsatisfiable", header: "bytes=100-", size: 100, wantErr: true},
		{name: "backwards range serves the whole object", header: "bytes=20-10", size: 100},
		{name: "garbage range serves the whole object", header: "bytes=abc-def", size: 100},
		{name: "zero sized object ignores the range", header: "bytes=0-1", size: 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			start, length, partial, err := parseTaskArtifactRange(testCase.header, testCase.size)
			if testCase.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, testCase.partial, partial)
			assert.Equal(t, testCase.start, start)
			assert.Equal(t, testCase.length, length)
		})
	}
}

func TestVideoPosterExtractorRejectsEmptyPayload(t *testing.T) {
	_, err := extractVideoPosterFrame(t.Context(), nil)
	assert.True(t, errors.Is(err, errTaskArtifactPosterEmpty))
}
