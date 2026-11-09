// Validation of every document the binaries emitted during F6 against the Appendix A schemas, plus /pub stability.
// Realise T-DOC-001..004, T-SYS-005 and T-SYS-006 on real outputs (DoD item 4).
package conformance

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ging/alexandria/vhub/internal/core/lifecycle"
	"github.com/ging/alexandria/vhub/internal/core/schemas"
	"github.com/ging/alexandria/vhub/internal/core/sign"
)

// fetch performs a GET with an optional bearer token.
func fetch(t *testing.T, u, token string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", u, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// listing returns the hub listing served to a member.
func listing(t *testing.T, hub string) ([]byte, lifecycle.Listing) {
	t.Helper()
	_, b := fetch(t, hub+"/v1/versions", "member-token")
	var env sign.Envelope[lifecycle.Listing]
	_ = json.Unmarshal(b, &env)
	return b, env.Signed
}

func TestT_DOC_001_ListingFromBinaryValidates(t *testing.T) {
	env, _, _ := f6Run(t)
	b, _ := listing(t, env.HubURL)
	if err := schemas.Validate(schemas.Versions, b); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
}

func TestT_DOC_002_RecordsFromBinaryValidate(t *testing.T) {
	env, _, _ := f6Run(t)
	for _, v := range []string{"1.0.0", "2.0.0"} {
		_, b := fetch(t, env.HubURL+"/v1/versions/"+v, "member-token")
		if err := schemas.Validate(schemas.Version, b); err != nil {
			t.Errorf("%s: %v\n%s", v, err, b)
		}
	}
}

func TestT_DOC_003_ConfigurationsValidate(t *testing.T) {
	_, _, work := f6Run(t)
	for _, p := range []string{filepath.Join(work, "config.json"), filepath.Join("..", "deploy", "config.json"), filepath.Join("examples", "config.json")} {
		b, _ := os.ReadFile(p)
		if err := schemas.Validate(schemas.Config, b); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

func TestT_DOC_004_EmittedVerdictEventsValidate(t *testing.T) {
	env, _, _ := f6Run(t)
	f, err := os.Open(env.EventsFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if !strings.Contains(sc.Text(), `"type":"vhub.verdict"`) {
			continue
		}
		n++
		if err := schemas.Validate(schemas.Verdict, sc.Bytes()); err != nil {
			t.Errorf("%v\n%s", err, sc.Text())
		}
	}
	if n < 5 {
		t.Fatalf("only %d vhub.verdict events", n)
	}
}

func TestT_SYS_005_PubBytesNeverChange(t *testing.T) {
	env, _, _ := f6Run(t)
	_, l := listing(t, env.HubURL)
	for _, r := range l.Versions {
		for _, f := range []string{"manifest.json", "manifest.sig", "manifest.receipt", "space-lock.json"} {
			u := env.HubURL + "/pub/packages/" + r.Manifest.Segment() + "/" + f
			_, a := fetch(t, u, "")
			_, b := fetch(t, u, "")
			if !bytes.Equal(a, b) || len(a) == 0 {
				t.Errorf("%s changed between downloads", u)
			}
		}
	}
	_, m := fetch(t, env.HubURL+"/pub/demo/1.0.0/manifest.json", "")
	sum := sha256.Sum256(m)
	_, again := fetch(t, env.HubURL+"/pub/demo/1.0.0/manifest.json", "")
	if hex.EncodeToString(sum[:]) != hex.EncodeToString(func() []byte { s := sha256.Sum256(again); return s[:] }()) {
		t.Fatal("versioned route changed bytes")
	}
}

func TestT_SYS_006_AgreementVersionStaysObtainable(t *testing.T) {
	env, _, work := f6Run(t)
	_, l := listing(t, env.HubURL)
	var v1 lifecycle.Record
	for _, r := range l.Versions {
		if r.Version.String() == "1.0.0" {
			v1 = r
		}
	}
	if v1.RevokedFrom == nil {
		t.Fatal("1.0.0 should be listed as revoked at the end of F6")
	}
	if st, _ := fetch(t, env.HubURL+"/pub/packages/"+v1.Manifest.Segment()+"/manifest.json", ""); st != 200 {
		t.Fatalf("hub no longer serves 1.0.0: %d", st)
	}
	if _, err := os.Stat(filepath.Join(work, "client", "state", "packages", v1.Manifest.Segment(), "manifest.json")); err != nil {
		t.Fatalf("client dropped the package of an agreement version: %v", err)
	}
}
