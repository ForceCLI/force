package sf

import (
	"os"
	"testing"
	"time"
)

// sf shows some times in the local time zone.  The expected output in the
// tests uses UTC.  time.Local is set here, before any test starts a goroutine,
// because assigning it while an httptest server is running is a data race.
func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}
