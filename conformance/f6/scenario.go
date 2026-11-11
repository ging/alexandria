// Package f6 runs the F6 scenario against the real binaries through their APIs and records one row per observable outcome.
// Expected returns the exact F6 tables (F6.1–F6.4); the e2e test compares both and the demo prints them (Part 4 M8, M9).
package f6

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Row is one observable outcome of the scenario.
type Row struct {
	At      string `json:"at"`
	Step    string `json:"step"`
	Status  int    `json:"status"`
	Verdict string `json:"verdict,omitempty"`
	Code    string `json:"code,omitempty"`
}

// String renders a row for logs and diffs.
func (r Row) String() string {
	return fmt.Sprintf("%-20s %-34s %3d %-28s %s", r.At, r.Step, r.Status, r.Code, r.Verdict)
}

// warning is the exact F6 header of m2 during the coexistence.
const warning = `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";rule="contract-request";version="1.0.0";sunset=@1793613600`

// Expected returns the F6 rows in order.
func Expected() []Row {
	ok := func(at, s string) Row { return Row{At: at, Step: s, Status: 200} }
	warn := func(at, s string) Row { return Row{At: at, Step: s, Status: 200, Verdict: warning} }
	viol := func(at, s string) Row {
		return Row{At: at, Step: s, Status: 400, Code: "VERDICT_VIOLATION", Verdict: `violation;gate=policy_intake;code="VERDICT_VIOLATION";rule="contract-request"`}
	}
	return []Row{
		{At: "2026-09-01T09:00:00Z", Step: "F6.1 create namespace demo", Status: 201},
		{At: "2026-09-01T09:00:00Z", Step: "F6.1 assign steward sergio", Status: 201},
		{At: "2026-09-01T09:00:00Z", Step: "F6.1 proxy vocabulary revision", Status: 201, Code: "r/1"},
		{At: "2026-09-01T10:00:00Z", Step: "F6.2 approve 1.0.0", Status: 201, Code: "initial"},
		{At: "2026-10-01T10:00:00Z", Step: "F6.2 approve 2.0.0 grace P3D", Status: 422, Code: "DATES_GRACE_TOO_SHORT"},
		{At: "2026-10-01T10:00:00Z", Step: "F6.2 approve 2.0.0", Status: 201, Code: "breaking 2026-10-03T10:00:00Z 2026-11-02T10:00:00Z"},
		ok("2026-10-02T12:00:00Z", "F6.3 m1 odrl:use"), ok("2026-10-02T12:00:00Z", "F6.3 m2 ex:access"), viol("2026-10-02T12:00:00Z", "F6.3 m3 odrl:read"),
		ok("2026-10-03T09:59:55Z", "F6.3 m1 odrl:use"), warn("2026-10-03T09:59:55Z", "F6.3 m2 ex:access"), ok("2026-10-03T09:59:55Z", "F6.3 m3 odrl:read"),
		ok("2026-10-10T12:00:00Z", "F6.3 m1 odrl:use"), warn("2026-10-10T12:00:00Z", "F6.3 m2 ex:access"), ok("2026-10-10T12:00:00Z", "F6.3 m3 odrl:read"),
		{At: "2026-10-10T12:00:00Z", Step: "F6.3 :explain m2", Status: 200, Code: "forward-notice", Verdict: warning},
		{At: "2026-10-10T12:00:00Z", Step: "F6.4 A1 agreement ex:access", Status: 200, Code: "1.0.0", Verdict: strings.Replace(warning, "contract-request", "contract-agreement-in", 1)},
		ok("2026-11-02T10:00:05Z", "F6.3 m1 odrl:use"), warn("2026-11-02T10:00:05Z", "F6.3 m2 ex:access"), ok("2026-11-02T10:00:05Z", "F6.3 m3 odrl:read"),
		ok("2026-11-05T12:00:00Z", "F6.3 m1 odrl:use"), viol("2026-11-05T12:00:00Z", "F6.3 m2 ex:access"), ok("2026-11-05T12:00:00Z", "F6.3 m3 odrl:read"),
		{At: "2026-11-05T12:00:00Z", Step: "F6.4 1.0.0 package retained", Status: 200, Code: "retained"},
		ok("2026-11-05T12:00:00Z", "F6.4 A1 transfer"),
		{At: "2026-11-10T00:00:00Z", Step: "F6.4 revoke 1.0.0", Status: 200, Code: "2026-11-12T00:00:00Z"},
		{At: "2026-11-10T00:00:00Z", Step: "F6.4 client event", Status: 200, Code: "AGREEMENT_REVOCATION_SCHEDULED"},
		ok("2026-11-12T00:00:05Z", "F6.4 A1 transfer"),
		{At: "2026-11-12T00:00:10Z", Step: "F6.4 A1 transfer", Status: 400, Code: "AGREEMENT_VERSION_REVOKED", Verdict: `violation;gate=agreement_admission;code="AGREEMENT_VERSION_REVOKED";rule="transfer-request"`},
	}
}

// Draft builds the Part 5.1 draft of an F6 version; 1.0.0 admits odrl:use and ex:access, 2.0.0 odrl:use and odrl:read.
func Draft(hub, v string) []byte {
	enum := `["odrl:use","ex:access"]`
	if v == "2.0.0" {
		enum = `["odrl:use","odrl:read"]`
	}
	return []byte(`{"version":"` + v + `","profiles":["` + hub + `/pub/demo/profiles/odrl/` + v + `"],
"slots":[{"slot":"policy","obligation":"mandatory","applies_to":["policy_intake"]}],
"files":[{"path":"schemas/policy.schema.json","media_type":"application/schema+json","role":"json-schema","slot":"policy","applies_to":["policy_intake"],
"content":{"type":"object","properties":{"action":{"enum":` + enum + `}},"required":["action"]}}]}`)
}

// Messages of F6: a contract request with an action, A1 and its transfer.
func contractRequest(action string) []byte {
	return []byte(`{"@type":"ContractRequestMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1","action":"` + action + `"}`)
}

// agreement is the ContractAgreementMessage of A1.
func agreement() []byte {
	return []byte(`{"@type":"ContractAgreementMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1","action":"ex:access","agreement":{"@id":"urn:uuid:A1"}}`)
}

// transfer is the TransferRequestMessage of A1.
func transfer() []byte {
	return []byte(`{"@type":"TransferRequestMessage","consumerPid":"urn:uuid:t-1","agreementId":"urn:uuid:A1"}`)
}

// httpClient trusts the lab certificate and never follows redirects.
func httpClient(caFile string) *http.Client {
	pool := x509.NewCertPool()
	if pem, err := os.ReadFile(caFile); err == nil { // #nosec G304 -- lab CA written by Launch.
		pool.AppendCertsFromPEM(pem)
	}
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// call sends a request and returns status, headers and body.
func (e Env) call(method, url, token, ifMatch string, body []byte) (int, http.Header, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := e.HTTP().Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b, nil
}

// field extracts a string member at a dotted path of a JSON body.
func field(b []byte, path string) string {
	var v any
	_ = json.Unmarshal(b, &v)
	for _, k := range strings.Split(path, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = m[k]
	}
	s, _ := v.(string)
	return s
}

// last returns the last path segment of an IRI.
func last(iri string) string { return iri[strings.LastIndex(iri, "/")+1:] }

// Runner drives the scenario and collects rows.
type Runner struct {
	E           Env
	Log         io.Writer
	Rows        []Row
	ana, sergio string
	now         string
}

// fail wraps an error with the step.
func fail(step string, err error) error { return fmt.Errorf("%s: %w", step, err) }

// at moves the file clock.
func (r *Runner) at(ts string) error {
	r.now = ts
	if r.Log != nil {
		_, _ = fmt.Fprintf(r.Log, "· clock → %s\n", ts)
	}
	return r.E.SetClock(ts)
}

// add records a row.
func (r *Runner) add(step string, status int, code, verdict string) {
	row := Row{At: r.now, Step: step, Status: status, Code: code, Verdict: verdict}
	r.Rows = append(r.Rows, row)
	if r.Log != nil {
		_, _ = fmt.Fprintln(r.Log, row.String())
	}
}

// publish creates and proposes a draft.
func (r *Runner) publish(v string) (string, error) {
	st, hdr, b, err := r.E.call("POST", r.E.HubURL+"/v1/namespaces/demo/drafts", r.sergio, "", Draft(r.E.HubURL, v))
	if err != nil || st != 201 {
		return "", fmt.Errorf("draft %s: %d %s %w", v, st, b, err)
	}
	st, _, b, err = r.E.call("POST", r.E.HubURL+"/v1/namespaces/demo/drafts/"+last(hdr.Get("Location"))+":propose", r.sergio, "", nil)
	if err != nil || st != 201 {
		return "", fmt.Errorf("propose %s: %d %s %w", v, st, b, err)
	}
	return last(field(b, "id")), nil
}

// approve approves a candidate and records the row.
func (r *Runner) approve(step, id, body string) error {
	st, _, b, err := r.E.call("POST", r.E.HubURL+"/v1/namespaces/demo/candidates/"+id+":approve", r.sergio, "", []byte(body))
	if err != nil {
		return fail(step, err)
	}
	code := field(b, "code")
	if st == 201 {
		code = field(b, "credentialSubject.class")
		if code == "breaking" {
			code += " " + field(b, "credentialSubject.new_content_from") + " " + field(b, "credentialSubject.enforced_from")
		}
	}
	r.add(step, st, code, "")
	return nil
}

// synced waits until the client accepted a listing issued at the current instant, or the last one when issuedAt is empty.
func (r *Runner) synced(issuedAt string) error {
	for i := 0; i < 150; i++ {
		_, _, b, err := r.E.call("GET", r.E.Admin+"/status", "", "", nil)
		if err == nil && field(b, "last_accepted_listing.valid_from") == issuedAt {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("client did not accept the listing of %s", issuedAt)
}

// send posts a message on the ingress listener and records the row.
func (r *Runner) send(step, path string, body []byte) error {
	st, hdr, b, err := r.E.call("POST", r.E.Ingress+path, "", "", body)
	if err != nil {
		return fail(step, err)
	}
	code := ""
	if st >= 400 {
		code = field(b, "code")
	}
	r.add(step, st, code, hdr.Get("VHub-Verdict"))
	return nil
}

// messages sends m1–m3.
func (r *Runner) messages() error {
	for _, m := range []struct{ step, action string }{{"F6.3 m1 odrl:use", "odrl:use"}, {"F6.3 m2 ex:access", "ex:access"}, {"F6.3 m3 odrl:read", "odrl:read"}} {
		if err := r.send(m.step, "/negotiations/urn:uuid:p-1/request", contractRequest(m.action)); err != nil {
			return err
		}
	}
	return nil
}

// setMode switches a client rule to enforce through the admin API.
func (r *Runner) setMode(id string) error {
	_, hdr, b, err := r.E.call("GET", r.E.Admin+"/rules/"+id, "", "", nil)
	if err != nil {
		return err
	}
	var rule map[string]any
	_ = json.Unmarshal(b, &rule)
	rule["mode"] = "enforce"
	nb, _ := json.Marshal(rule)
	st, _, out, err := r.E.call("PUT", r.E.Admin+"/rules/"+id, "", hdr.Get("ETag"), nb)
	if err != nil || st != 200 {
		return fmt.Errorf("rule %s: %d %s %w", id, st, out, err)
	}
	return nil
}

// hasEvent reports whether the client events file contains code.
func (r *Runner) hasEvent(code string) bool {
	f, err := os.Open(r.E.EventsFile)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		if strings.Contains(s.Text(), `"`+code+`"`) {
			return true
		}
	}
	return false
}

// Run executes F6.1–F6.4; startClient launches the client once the namespace exists.
func (r *Runner) Run(startClient func() error) error {
	var err error
	if err = r.at("2026-09-01T09:00:00Z"); err != nil {
		return err
	}
	if r.ana, err = r.E.Token("ana", true); err != nil {
		return fail("token", err)
	}
	if r.sergio, err = r.E.Token("sergio", false); err != nil {
		return fail("token", err)
	}
	st, _, _, err := r.E.call("POST", r.E.HubURL+"/v1/namespaces", r.ana, "", []byte(`{"slug":"demo"}`))
	if err != nil {
		return fail("namespace", err)
	}
	r.add("F6.1 create namespace demo", st, "", "")
	st, _, _, err = r.E.call("POST", r.E.HubURL+"/v1/namespaces/demo/stewards", r.ana, "", []byte(`{"iss":"`+r.E.IDPURL+`","sub":"sergio"}`))
	if err != nil {
		return fail("steward", err)
	}
	r.add("F6.1 assign steward sergio", st, "", "")
	voc := `{"name":"dcat-ap-context","kind":"proxy","media_type":"application/ld+json","upstream":"` + r.E.ConnURL + `/dcat-ap.jsonld"}`
	st, _, b, err := r.E.call("POST", r.E.HubURL+"/v1/namespaces/demo/vocabularies", r.sergio, "", []byte(voc))
	if err != nil {
		return fail("vocabulary", err)
	}
	var v struct {
		Revisions []struct {
			ID string `json:"id"`
		} `json:"revisions"`
	}
	_ = json.Unmarshal(b, &v)
	rev := ""
	if len(v.Revisions) == 1 {
		rev = "r/" + last(v.Revisions[0].ID)
	}
	r.add("F6.1 proxy vocabulary revision", st, rev, "")
	if err = r.at("2026-09-01T10:00:00Z"); err != nil {
		return err
	}
	id, err := r.publish("1.0.0")
	if err != nil {
		return err
	}
	if err := r.approve("F6.2 approve 1.0.0", id, ""); err != nil {
		return err
	}
	if err := startClient(); err != nil {
		return fail("client", err)
	}
	if err := r.synced("2026-09-01T10:00:00Z"); err != nil {
		return err
	}
	for _, rule := range []string{"contract-agreement-in", "transfer-request"} {
		if err := r.setMode(rule); err != nil {
			return err
		}
	}
	if err = r.at("2026-10-01T10:00:00Z"); err != nil {
		return err
	}
	if id, err = r.publish("2.0.0"); err != nil {
		return err
	}
	if err := r.approve("F6.2 approve 2.0.0 grace P3D", id, `{"grace_window":"P3D"}`); err != nil {
		return err
	}
	if err := r.approve("F6.2 approve 2.0.0", id, ""); err != nil {
		return err
	}
	return r.rows()
}

// rows runs F6.3 and F6.4; before each barrier the client syncs, as F6 assumes (see OPEN_QUESTIONS).
func (r *Runner) rows() error {
	for _, step := range []struct{ sync, at string }{
		{"2026-10-02T12:00:00Z", "2026-10-02T12:00:00Z"}, {"2026-10-03T06:00:00Z", "2026-10-03T09:59:55Z"}, {"2026-10-10T12:00:00Z", "2026-10-10T12:00:00Z"},
	} {
		if err := r.syncAt(step.sync); err != nil {
			return err
		}
		if err := r.at(step.at); err != nil {
			return err
		}
		if err := r.messages(); err != nil {
			return err
		}
	}
	explain := `{"listener":"dsp-ingress","method":"POST","path":"/negotiations/urn:uuid:p-1/request","body":` + string(contractRequest("ex:access")) + `}`
	st, _, b, err := r.E.call("POST", r.E.Admin+"/rules:explain", "", "", []byte(explain))
	if err != nil {
		return fail("explain", err)
	}
	r.add("F6.3 :explain m2", st, field(b, "action"), field(b, "vhub_verdict"))
	st, hdr, _, err := r.E.call("POST", r.E.Ingress+"/negotiations/urn:uuid:c-1/agreement", "", "", agreement())
	if err != nil {
		return fail("agreement", err)
	}
	_, _, ab, _ := r.E.call("GET", r.E.Admin+"/agreements/urn:uuid:A1", "", "", nil)
	r.add("F6.4 A1 agreement ex:access", st, field(ab, "version"), hdr.Get("VHub-Verdict"))
	if err := r.syncAt("2026-11-02T06:00:00Z"); err != nil {
		return err
	}
	if err := r.at("2026-11-02T10:00:05Z"); err != nil {
		return err
	}
	if err := r.messages(); err != nil {
		return err
	}
	if err := r.syncAt("2026-11-05T12:00:00Z"); err != nil {
		return err
	}
	if err := r.messages(); err != nil {
		return err
	}
	role := ""
	for i := 0; i < 50 && role != "retained"; i++ {
		_, _, sb, _ := r.E.call("GET", r.E.Admin+"/status", "", "", nil)
		var s struct {
			Versions []struct {
				Version   string `json:"version"`
				LocalRole string `json:"local_role"`
			} `json:"versions"`
		}
		_ = json.Unmarshal(sb, &s)
		for _, v := range s.Versions {
			if v.Version == "1.0.0" {
				role = v.LocalRole
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.add("F6.4 1.0.0 package retained", 200, role, "")
	if err := r.send("F6.4 A1 transfer", "/transfers/request", transfer()); err != nil {
		return err
	}
	return r.revocation()
}

// revocation runs the F6.4 revocation and the transfers around revoked_from + L.
func (r *Runner) revocation() error {
	if err := r.at("2026-11-10T00:00:00Z"); err != nil {
		return err
	}
	st, _, b, err := r.E.call("POST", r.E.HubURL+"/v1/namespaces/demo/versions/1.0.0:revoke", r.sergio, "", []byte(`{"justification":"ex:access is unsafe"}`))
	if err != nil {
		return fail("revoke", err)
	}
	r.add("F6.4 revoke 1.0.0", st, field(b, "credentialSubject.revoked_from"), "")
	if err := r.synced("2026-11-10T00:00:00Z"); err != nil {
		return err
	}
	ev := ""
	for i := 0; i < 50 && ev == ""; i++ {
		if r.hasEvent("AGREEMENT_REVOCATION_SCHEDULED") {
			ev = "AGREEMENT_REVOCATION_SCHEDULED"
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.add("F6.4 client event", 200, ev, "")
	for _, ts := range []string{"2026-11-12T00:00:05Z", "2026-11-12T00:00:10Z"} {
		if err := r.at(ts); err != nil {
			return err
		}
		if err := r.send("F6.4 A1 transfer", "/transfers/request", transfer()); err != nil {
			return err
		}
	}
	return nil
}

// syncAt moves the clock and waits for the hub to issue and the client to accept a listing at that instant.
func (r *Runner) syncAt(ts string) error {
	if err := r.at(ts); err != nil {
		return err
	}
	return r.synced(ts)
}
