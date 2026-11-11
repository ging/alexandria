// Tests the vocabulary registry: field validation, revisions, proxy downloads and the D-VOC-1 incorporation into a package.
// Fixtures: the testkit fake fetcher serving a JSON-LD context (T-HUB-026, T-HUB-027, T-HUB-028).
package app_test

import (
	"errors"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// proxyInput is the F6.1 proxied DCAT-AP context.
var proxyInput = app.VocabInput{Name: "dcat-ap-context", Kind: "proxy", MediaType: "application/ld+json", Upstream: "https://semiceu.github.io/dcat-ap.jsonld"}

// hostedInput is a hosted JSON Schema vocabulary.
var hostedInput = app.VocabInput{Name: "offer-schema", Kind: "hosted", MediaType: "application/schema+json"}

func TestT_HUB_026_FieldCodes(t *testing.T) {
	s := newHub(t)
	cases := map[codes.Code]app.VocabInput{
		codes.VocabNameInvalid:       {Name: "Mi Vocab", Kind: "hosted", MediaType: "application/json"},
		codes.VocabKindMismatch:      {Name: "a", Kind: "remote", MediaType: "application/json"},
		codes.VocabMediaTypeMismatch: {Name: "a", Kind: "hosted"},
		codes.VocabUpstreamFailed:    {Name: "a", Kind: "proxy", MediaType: "application/json", Upstream: "ftp://x"},
	}
	for want, in := range cases {
		if _, err := s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, in); codeOf(err) != want {
			t.Errorf("%s: got %v", want, err)
		}
	}
}

func TestT_HUB_026_Exists(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, proxyInput)
	if _, err := s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, proxyInput); codeOf(err) != codes.VocabExists {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_026_RefreshHostedKindMismatch(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, hostedInput)
	if _, _, err := s.Hub.Vocabularies.Refresh(testkit.Sergio, testkit.NS, hostedInput.Name); codeOf(err) != codes.VocabKindMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_026_UploadChecks(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, hostedInput)
	big := make([]byte, 1<<16+1)
	cases := []struct {
		ct   string
		body []byte
		want codes.Code
	}{
		{"text/turtle", []byte("{}"), codes.VocabMediaTypeMismatch},
		{"application/schema+json", []byte("{"), codes.VocabUnparseable},
		{"application/schema+json", big, codes.VocabTooLarge},
	}
	for _, c := range cases {
		if _, _, err := s.Hub.Vocabularies.PutContent(testkit.Sergio, testkit.NS, hostedInput.Name, c.ct, "", c.body); codeOf(err) != c.want {
			t.Errorf("%s: got %v", c.want, err)
		}
	}
}

func TestT_HUB_026_RevisionsAreImmutable(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, hostedInput)
	_, _, _ = s.Hub.Vocabularies.PutContent(testkit.Sergio, testkit.NS, hostedInput.Name, "application/schema+json", "", []byte(`{"a":1}`))
	_, _, _ = s.Hub.Vocabularies.PutContent(testkit.Sergio, testkit.NS, hostedInput.Name, "application/schema+json", "", []byte(`{"a":2}`))
	_, b1, _ := s.Hub.VocabReader.Revision(testkit.NS, hostedInput.Name, 1)
	_, b2, _ := s.Hub.VocabReader.Revision(testkit.NS, hostedInput.Name, 2)
	if string(b1) != `{"a":1}` || string(b2) != `{"a":2}` {
		t.Fatalf("got %s and %s", b1, b2)
	}
}

func TestT_HUB_027_ProxyFetchesOnCreateAndRefreshOnly(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, proxyInput)
	for i := 0; i < 3; i++ {
		_, _, _ = s.Hub.Publication.VocabRevision(testkit.NS, proxyInput.Name, 1)
	}
	if s.Fetcher.Calls != 1 {
		t.Fatalf("%d upstream calls after reads", s.Fetcher.Calls)
	}
}

func TestT_HUB_027_UpstreamDownCreatesNoRevision(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, proxyInput)
	s.Fetcher.Err, s.Fetcher.Body = errors.New("500"), []byte(`{"x":1}`)
	if _, _, err := s.Hub.Vocabularies.Refresh(testkit.Sergio, testkit.NS, proxyInput.Name); codeOf(err) != codes.VocabUpstreamFailed {
		t.Fatalf("got %v", err)
	}
	v, _ := s.Hub.VocabReader.Get(testkit.NS, proxyInput.Name)
	if len(v.Revisions) != 1 {
		t.Fatalf("%d revisions", len(v.Revisions))
	}
}

func TestT_HUB_027_UpstreamMediaTypeMismatch(t *testing.T) {
	s := newHub(t)
	s.Fetcher.ContentType = "text/turtle"
	if _, err := s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, proxyInput); codeOf(err) != codes.VocabMediaTypeMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_028_SameBytesNoNewRevision(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, proxyInput)
	if _, created, err := s.Hub.Vocabularies.Refresh(testkit.Sergio, testkit.NS, proxyInput.Name); err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestDVOC1_SourceIsCopiedIntoThePackage(t *testing.T) {
	s := newHub(t)
	_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, proxyInput)
	src := s.Hub.VocabReader.RevisionURL(testkit.NS, proxyInput.Name, 1)
	doc := []byte(`{"version":"1.0.0","slots":[{"slot":"p","obligation":"optional"}],"files":[{"path":"contexts/dcat-ap.jsonld","media_type":"application/ld+json","role":"context","url":"` + testkit.Base + `/pub/demo/1.0.0/contexts/dcat-ap.jsonld","source":"` + src + `"}]}`)
	s.Publish(t, "2026-09-01T10:00:00Z", doc, app.ApproveOptions{})
	a, err := s.Hub.Publication.VersionFile(testkit.NS, "1.0.0", "contexts/dcat-ap.jsonld")
	if err != nil || string(a.Bytes) != string(s.Fetcher.Body) {
		t.Fatalf("got %q %v", a.Bytes, err)
	}
}
