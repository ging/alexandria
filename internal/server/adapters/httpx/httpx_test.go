// Tests of the error mapping: validation findings are 422 whatever compiler produced their codes.
// Regression for language codes such as SCHEMA, which have no catalog prefix (REQ-HUB-001).
package httpx

import (
	"net/http/httptest"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/server/app"
)

func TestFindingsAreUnprocessable(t *testing.T) {
	for _, c := range []codes.Code{"SCHEMA", "YAML", "VERSION_BUMP_TOO_LOW", "LIFECYCLE_LEASE", "POC_N2"} {
		w := httptest.NewRecorder()
		WriteError(w, app.Findings{{Code: c, Detail: "x"}})
		if w.Code != 422 {
			t.Errorf("%s: status %d", c, w.Code)
		}
	}
}
