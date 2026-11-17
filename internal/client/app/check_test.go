// Tests Edge.Check, the hot path of validator mode: the same decisions as Handle, without ever forwarding.
// Fixtures: the F6 hub and client of sync_test.go and the testkit F6 messages (§4.7, §4.8, §4.9).
package app_test

import (
	"net/http"
	"testing"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/testkit"
)

// check asks for a decision on an F6 contract request with action.
func check(c *testkit.Client, action string) *app.Response {
	return checkAt(c, testkit.ContractRequestPath, testkit.ContractRequest(action))
}

// checkAt asks for a decision on a dsp-ingress message.
func checkAt(c *testkit.Client, path string, body []byte) *app.Response {
	return c.Edge.Check(&app.Request{Listener: domain.DSPIngress, Method: http.MethodPost, Path: path, Body: body})
}

func TestCheckAllowsWithoutForwarding(t *testing.T) {
	_, c := f6(t)
	r := check(c, "odrl:use")
	if r.Status != http.StatusOK || len(r.Body) != 0 || testkit.VerdictOf(r) != "" || c.Upstream.Count() != 0 {
		t.Fatalf("got %d %q %q with %d forwards", r.Status, r.Body, testkit.VerdictOf(r), c.Upstream.Count())
	}
}

func TestCheckAllowsWarningWithNoticeHeader(t *testing.T) {
	_, c := f6(t)
	r := check(c, "ex:access")
	if r.Status != http.StatusOK || testkit.VerdictOf(r) != m2Header {
		t.Fatalf("got %d %q", r.Status, testkit.VerdictOf(r))
	}
}

func TestCheckRejectsLikeHandle(t *testing.T) {
	s, c := f6(t)
	at(t, s, c, "2026-11-05T12:00:00Z")
	got, want := check(c, "ex:access"), request(c, "ex:access")
	if got.Status != http.StatusBadRequest || string(got.Body) != string(want.Body) || testkit.VerdictOf(got) != testkit.VerdictOf(want) {
		t.Fatalf("check %d %s, handle %d %s", got.Status, got.Body, want.Status, want.Body)
	}
}

func TestCheckWorksWithoutUpstream(t *testing.T) {
	_, c := f6(t)
	c.Edge.Upstream = nil
	if r := check(c, "odrl:use"); r.Status != http.StatusOK {
		t.Fatalf("got %d", r.Status)
	}
}

func TestCheckCapturesAllowedAgreement(t *testing.T) {
	_, c := f6(t)
	c.SetMode(t, "contract-agreement-in", domain.Enforce)
	checkAt(c, testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", ""))
	e, ok, _ := c.Repo.Get("urn:A1")
	if !ok || e.Version != "1.0.0" || e.Role != "consumer" || e.Source != "dsp-ingress" {
		t.Fatalf("got %+v", e)
	}
}

func TestCheckDoesNotCaptureRejectedAgreement(t *testing.T) {
	_, c := f6(t)
	c.SetMode(t, "contract-agreement-in", domain.Enforce)
	r := checkAt(c, testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "odrl:nope", ""))
	if _, ok, _ := c.Repo.Get("urn:A1"); ok || r.Status != http.StatusBadRequest {
		t.Fatalf("status %d captured %v", r.Status, ok)
	}
}

func TestCheckEmitsVerdictEvents(t *testing.T) {
	_, c := f6(t)
	c.Events.Reset()
	check(c, "ex:access")
	if !c.Events.Has("VERDICT_NOT_IN_HEAD") {
		t.Fatalf("got %v", c.Events.Codes())
	}
}
