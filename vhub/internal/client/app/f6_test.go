// F6 replay at application level: classification and actions of m1–m3 at the five instants, agreement A1 and its revocation.
// Realises T-SYS-001-style determinism inputs and T-CLI-011 on the F6 corpus (§4.7, F6).
package app_test

import (
	"net/http"
	"testing"

	"github.com/ging/alexandria/vhub/internal/client/domain"
	"github.com/ging/alexandria/vhub/internal/server/app"
	"github.com/ging/alexandria/vhub/internal/testkit"
)

// f6Client publishes F6 on a fresh hub and returns a synced client.
func f6Client(t *testing.T) (*testkit.Server, *testkit.Client) {
	t.Helper()
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(s, testkit.F6Routes())
	return s, c
}

// at moves the shared clock and syncs the client.
func at(t *testing.T, s *testkit.Server, c *testkit.Client, ts string) {
	t.Helper()
	s.Clock.Set(ts)
	if err := c.Sync.Run(); err != nil {
		t.Fatalf("sync at %s: %v", ts, err)
	}
}

// expect sends a ContractRequestMessage and checks status and header.
func expect(t *testing.T, c *testkit.Client, action string, status int, header string) {
	t.Helper()
	r := c.Send(domain.DSPIngress, "POST", "/negotiations/request", testkit.ContractRequest(action))
	if r.Status != status || testkit.VerdictOf(r) != header {
		t.Errorf("%s: got %d %q %s want %d %q", action, r.Status, testkit.VerdictOf(r), r.Body, status, header)
	}
}

func TestT_CLI_011_F6Table(t *testing.T) {
	s, c := f6Client(t)
	at(t, s, c, "2026-10-02T12:00:00Z")
	expect(t, c, "odrl:use", http.StatusOK, "")
	expect(t, c, "ex:access", http.StatusOK, "")
	expect(t, c, "odrl:read", http.StatusBadRequest, `violation;gate=policy_intake;code="VERDICT_VIOLATION"`)
	at(t, s, c, "2026-10-03T09:59:55Z")
	expect(t, c, "odrl:use", http.StatusOK, "")
	expect(t, c, "ex:access", http.StatusOK, `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";version="1.0.0";sunset=@1793613600`)
	expect(t, c, "odrl:read", http.StatusOK, "")
	at(t, s, c, "2026-10-10T12:00:00Z")
	expect(t, c, "ex:access", http.StatusOK, `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";version="1.0.0";sunset=@1793613600`)
	at(t, s, c, "2026-11-02T06:00:00Z")
	s.Clock.Set("2026-11-02T10:00:05Z")
	expect(t, c, "odrl:use", http.StatusOK, "")
	expect(t, c, "ex:access", http.StatusOK, `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";version="1.0.0";sunset=@1793613600`)
	expect(t, c, "odrl:read", http.StatusOK, "")
	at(t, s, c, "2026-11-05T12:00:00Z")
	expect(t, c, "odrl:use", http.StatusOK, "")
	expect(t, c, "ex:access", http.StatusBadRequest, `violation;gate=policy_intake;code="VERDICT_VIOLATION"`)
	expect(t, c, "odrl:read", http.StatusOK, "")
}

func TestT_CLI_016_F6AgreementAndRevocation(t *testing.T) {
	s, c := f6Client(t)
	at(t, s, c, "2026-10-10T12:00:00Z")
	r := c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A1", "ex:access", ""))
	if r.Status != http.StatusOK || !testkit.Contains(testkit.VerdictOf(r), "warning") {
		t.Fatalf("agreement: %d %q", r.Status, testkit.VerdictOf(r))
	}
	if e, ok, _ := c.Repo.Get("urn:uuid:A1"); !ok || e.Version != "1.0.0" || e.Role != "provider" {
		t.Fatalf("registry: %+v", e)
	}
	transfer := func(ts string, want int) {
		t.Helper()
		s.Clock.Set(ts)
		_ = c.Sync.Run()
		if r := c.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:uuid:A1")); r.Status != want {
			t.Errorf("transfer at %s: %d %s", ts, r.Status, r.Body)
		}
	}
	transfer("2026-11-05T12:00:00Z", http.StatusOK)
	s.Clock.Set("2026-11-10T00:00:00Z")
	if _, err := s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "superseded"}); err != nil {
		t.Fatal(err)
	}
	at(t, s, c, "2026-11-10T00:00:00Z")
	if !c.Events.Has("AGREEMENT_REVOCATION_SCHEDULED") {
		t.Fatal("AGREEMENT_REVOCATION_SCHEDULED not emitted")
	}
	transfer("2026-11-12T00:00:05Z", http.StatusOK)
	s.Clock.Set("2026-11-12T00:00:10Z")
	r = c.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:uuid:A1"))
	if r.Status != http.StatusBadRequest || testkit.BodyCode(r.Body) != "AGREEMENT_VERSION_REVOKED" {
		t.Fatalf("transfer after rf+L: %d %s", r.Status, r.Body)
	}
}
