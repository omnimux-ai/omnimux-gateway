package controller

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestMain(m *testing.M) {
	// Handler fixtures replace process-wide storage and database state between
	// tests. Exercise asynchronous admission/retries with isolated queue instances
	// instead of allowing production workers to outlive those fixtures.
	taskImageThumbnailSchedule = func(*model.Task, string) bool { return false }
	os.Exit(m.Run())
}
