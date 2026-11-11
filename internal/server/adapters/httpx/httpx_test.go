// Tests of the error mapping: findings are 422, 401 carries WWW-Authenticate and every problem has its @context.
// Fixtures: a recorder and a Problems writer with a fixed context (REQ-HUB-001, REQ-HUB-017, REQ-HUB-029).
package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/server/app"
)

// ps writes problems with a test context.
var ps = Problems{Context: "https://hub.example.com/pub/_vhub/v1/context.jsonld"}

func TestFindingsAreUnprocessable(t *testing.T) {
	for _, c := range []codes.Code{"LIFECYCLE_LEASE", "POC_N2"} {
		w := httptest.NewRecorder()
		ps.Error(w, app.Findings{{Code: c, Detail: "x"}})
		if w.Code != 422 {
			t.Errorf("%s: status %d", c, w.Code)
		}
	}
}

func TestUnauthenticatedHasChallenge(t *testing.T) {
	w := httptest.NewRecorder()
	ps.Error(w, codes.New(codes.APIUnauthenticated, "x"))
	if w.Code != 401 || w.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status %d challenge %q", w.Code, w.Header().Get("WWW-Authenticate"))
	}
}

func TestProblemCarriesContextAndTypeIRI(t *testing.T) {
	w := httptest.NewRecorder()
	ps.Error(w, codes.New(codes.APINotFound, "x"))
	b := w.Body.String()
	if !strings.Contains(b, `"@context":"`+ps.Context+`"`) || !strings.Contains(b, "https://w3id.org/alexandria-vhub/problems#API_NOT_FOUND") {
		t.Fatalf("got %s", b)
	}
}
