// Tests namespaces, stewards, authorization and isolation of the hub use cases.
// Fixtures come from testkit: Ana (admin), Sergio (steward of demo) and Eve (no permission) (T-HUB-017, 019, 023–025, T-SYS-010).
package app_test

import (
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
	"github.com/ging/alexandria/internal/testkit"
)

func TestT_HUB_025_SlugInvalid(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "Demo_1"}); codeOf(err) != codes.NSSlugInvalid {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_025_SlugExists(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "demo"}); codeOf(err) != codes.NSExists {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_025_SlugReserved(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "_vhub"}); codeOf(err) != codes.NSExists {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_017_CreateNamespaceWithoutToken(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(nil, app.NamespaceInput{Slug: "x1"}); codeOf(err) != codes.APIUnauthenticated {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_017_StewardCannotCreateNamespace(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(testkit.Sergio, app.NamespaceInput{Slug: "x1"}); codeOf(err) != codes.APIForbiddenRole {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_017_AdminIsNotStewardByDefault(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Authoring.CreateDraft(testkit.Ana, testkit.NS, testkit.DraftV1); codeOf(err) != codes.APIForbiddenRole {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_017_ListStewardsStewardOrAdmin(t *testing.T) {
	s := newHub(t)
	cases := []struct {
		who  *domain.Principal
		want codes.Code
	}{{testkit.Ana, ""}, {testkit.Sergio, ""}, {testkit.Eve, codes.APIForbiddenRole}, {nil, codes.APIUnauthenticated}}
	for _, c := range cases {
		if _, err := s.Hub.Namespaces.ListStewards(c.who, testkit.NS); codeOf(err) != c.want {
			t.Errorf("%v: got %v", c.who, err)
		}
	}
}

func TestT_HUB_023_StewardExists(t *testing.T) {
	s := newHub(t)
	_, err := s.Hub.Namespaces.AssignSteward(testkit.Ana, testkit.NS, app.StewardInput{Iss: testkit.Sergio.Iss, Sub: testkit.Sergio.Sub})
	if codeOf(err) != codes.StewardExists {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_023_RemovedStewardLosesAccess(t *testing.T) {
	s := newHub(t)
	st, _ := s.Hub.Namespaces.ListStewards(testkit.Ana, testkit.NS)
	if err := s.Hub.Namespaces.RemoveSteward(testkit.Ana, testkit.NS, st[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftV1); codeOf(err) != codes.APIForbiddenRole {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_019_NamespaceNotFound(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Get("no-existe"); codeOf(err) != codes.APINotFound {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_019_DraftNotFound(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Authoring.GetDraft(testkit.Sergio, testkit.NS, "nope"); codeOf(err) != codes.APINotFound {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_024_StewardOfOtherNamespaceForbidden(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "other"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Authoring.CreateDraft(testkit.Sergio, "other", testkit.DraftV1); codeOf(err) != codes.APIForbiddenRole {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_024_SequencesAreIndependent(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "other"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Namespaces.AssignSteward(testkit.Ana, "other", app.StewardInput{Iss: testkit.Eve.Iss, Sub: testkit.Eve.Sub}); err != nil {
		t.Fatal(err)
	}
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Eve, "other", testkit.DraftV1)
	c, _ := s.Hub.Authoring.Propose(testkit.Eve, "other", d.ID)
	if _, err := s.Hub.Release.Approve(testkit.Eve, "other", c.ID, app.ApproveOptions{}); err != nil {
		t.Fatal(err)
	}
	if a, b := len(s.DB.Listings("demo")), len(s.DB.Listings("other")); a != 1 || b != 1 || s.DB.Listings("other")[0].Sequence != 1 {
		t.Fatalf("listings demo=%d other=%d", a, b)
	}
}

func TestT_SYS_010_EventsAreScopedToTheirNamespace(t *testing.T) {
	s := newHub(t)
	if _, err := s.Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "other"}); err != nil {
		t.Fatal(err)
	}
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	_, _ = s.Hub.Namespaces.AssignSteward(testkit.Ana, "other", app.StewardInput{Iss: testkit.Sergio.Iss, Sub: testkit.Sergio.Sub})
	ev, err := s.Hub.Query.Events(testkit.Sergio, "other", 0, 100)
	if err != nil || len(ev) != 0 {
		t.Fatalf("other sees %d events of demo: %v", len(ev), err)
	}
}
