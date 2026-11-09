// Package f6 is the F6 scenario driver over HTTP: publishes 1.0.0 and 2.0.0, moves the file clock and sends m1–m3, A1 and its transfers.
// Shared by the e2e test (conformance/e2e_f6_test.go) and make demo, so both observe the same rows (Part 4 M8, M9).
package f6

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Env locates the running processes of the scenario.
type Env struct {
	HubURL      string
	ClientDSP   string
	ClientEgres string
	ClientAdmin string
	Connector   string
	ClockFile   string
	EventsFile  string
	Log         io.Writer
}

// Row is one observed (instant, message) outcome.
type Row struct {
	At             string
	Message        string
	Classification string
	Action         string
	Status         int
	Verdict        string
	Code           string
}

// String renders a row for logs and the demo table.
func (r Row) String() string {
	v := r.Verdict
	if v == "" {
		v = "-"
	}
	return fmt.Sprintf("%-22s %-28s %-10s %-15s %d  %s %s", r.At, r.Message, r.Classification, r.Action, r.Status, v, r.Code)
}

// Header of m2 at 2026-10-10 in F6.
const m2Warning = `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";version="1.0.0";sunset=@1793613600`

// Expected returns the F6 table as rows.
func Expected() []Row {
	ok := func(at, m string) Row {
		return Row{At: at, Message: m, Classification: "ok", Action: "forward", Status: 200}
	}
	warn := func(at, m string) Row {
		return Row{At: at, Message: m, Classification: "warning", Action: "forward-notice", Status: 200, Verdict: m2Warning}
	}
	viol := func(at, m, code string) Row {
		return Row{
			At: at, Message: m, Classification: "violation", Action: "reject", Status: 400,
			Verdict: `violation;gate=policy_intake;code="` + code + `"`, Code: code,
		}
	}
	return []Row{
		ok("2026-10-02T12:00:00Z", "m1 odrl:use"), ok("2026-10-02T12:00:00Z", "m2 ex:access"), viol("2026-10-02T12:00:00Z", "m3 odrl:read", "VERDICT_VIOLATION"),
		ok("2026-10-03T09:59:55Z", "m1 odrl:use"), warn("2026-10-03T09:59:55Z", "m2 ex:access"), ok("2026-10-03T09:59:55Z", "m3 odrl:read"),
		ok("2026-10-10T12:00:00Z", "m1 odrl:use"), warn("2026-10-10T12:00:00Z", "m2 ex:access"), ok("2026-10-10T12:00:00Z", "m3 odrl:read"),
		warn("2026-10-10T12:00:00Z", "A1 agreement ex:access"),
		ok("2026-11-02T10:00:05Z", "m1 odrl:use"), warn("2026-11-02T10:00:05Z", "m2 ex:access"), ok("2026-11-02T10:00:05Z", "m3 odrl:read"),
		ok("2026-11-05T12:00:00Z", "m1 odrl:use"), viol("2026-11-05T12:00:00Z", "m2 ex:access", "VERDICT_VIOLATION"), ok("2026-11-05T12:00:00Z", "m3 odrl:read"),
		ok("2026-11-05T12:00:00Z", "A1 transfer"),
		{At: "2026-11-10T00:00:00Z", Message: "revoke 1.0.0", Classification: "event", Action: "AGREEMENT_REVOCATION_SCHEDULED", Status: 200},
		ok("2026-11-12T00:00:05Z", "A1 transfer"),
		{
			At: "2026-11-12T00:00:10Z", Message: "A1 transfer", Classification: "violation", Action: "reject", Status: 400,
			Verdict: `violation;gate=agreement_admission;code="AGREEMENT_VERSION_REVOKED"`, Code: "AGREEMENT_VERSION_REVOKED",
		},
	}
}

// Messages of F6.
var (
	actions = map[string]string{"m1 odrl:use": "odrl:use", "m2 ex:access": "ex:access", "m3 odrl:read": "odrl:read"}
	order   = []string{"m1 odrl:use", "m2 ex:access", "m3 odrl:read"}
)

// Profile is the policy profile every F6 offer and agreement declares (governance rule of the language, §5).
const Profile = "https://example.com/odrl/profile/f6"

// contractRequest builds a ContractRequestMessage whose profiled offer uses action.
func contractRequest(action string) []byte {
	return []byte(`{"@context":["https://w3id.org/dspace/2025/1/context.jsonld","` + Profile + `/context.jsonld"],"@type":"ContractRequestMessage","consumerPid":"urn:uuid:c-1",` +
		`"offer":{"@id":"urn:uuid:offer-1","@type":"Offer","profile":"` + Profile + `","target":"urn:uuid:ds-1","permission":[{"action":"` + action + `"}]},"callbackAddress":"http://consumer/cb"}`)
}

// agreement builds the ContractAgreementMessage of A1.
func agreement() []byte {
	return []byte(`{"@context":["https://w3id.org/dspace/2025/1/context.jsonld"],"@type":"ContractAgreementMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1",` +
		`"agreement":{"@id":"urn:uuid:A1","@type":"Agreement","profile":"` + Profile + `/1.0.0","target":"urn:uuid:ds-1","permission":[{"action":"ex:access"}]},"callbackAddress":"http://provider/cb"}`)
}

// transfer builds the TransferRequestMessage of A1.
func transfer() []byte {
	return []byte(`{"@context":["https://w3id.org/dspace/2025/1/context.jsonld"],"@type":"TransferRequestMessage","consumerPid":"urn:uuid:t-1","agreementId":"urn:uuid:A1","format":"HTTP_PULL","callbackAddress":"http://consumer/cb"}`)
}

// drafts holds the F6 releases written in the eunomia.profiles/v1 language.
//
//go:embed drafts/*.yaml
var drafts embed.FS

// Draft returns the YAML bundle of an F6 release (v1.0.0 or v2.0.0).
func Draft(v string) []byte {
	b, err := drafts.ReadFile("drafts/v" + v + ".yaml")
	if err != nil {
		panic(err)
	}
	return b
}

// logf writes progress when a log is set.
func (e Env) logf(format string, a ...any) {
	if e.Log != nil {
		_, _ = fmt.Fprintf(e.Log, format+"\n", a...)
	}
}

// setClock writes the file clock read by both binaries.
func (e Env) setClock(t string) error {
	tmp := e.ClockFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(t+"\n"), 0o600); err != nil {
		return err
	}
	e.logf("· clock → %s", t)
	return os.Rename(tmp, e.ClockFile)
}

// call performs an HTTP request and returns status, headers and body.
func call(c *http.Client, method, u, token string, body []byte) (int, http.Header, []byte, error) {
	req, _ := http.NewRequestWithContext(context.Background(), method, u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b, nil
}

// publish creates, proposes and approves a draft as editor and releaser.
func (e Env) publish(doc []byte) error {
	c := http.DefaultClient
	st, _, b, err := call(c, "POST", e.HubURL+"/v1/drafts", "editor-token", doc)
	if err != nil || st != 201 {
		return fmt.Errorf("create draft: %d %s %w", st, b, err)
	}
	var d struct{ ID string }
	_ = json.Unmarshal(b, &d)
	st, _, b, err = call(c, "POST", e.HubURL+"/v1/drafts/"+d.ID+":propose", "editor-token", nil)
	if err != nil || st != 201 {
		return fmt.Errorf("propose: %d %s %w", st, b, err)
	}
	var cand struct{ ID string }
	_ = json.Unmarshal(b, &cand)
	st, _, b, err = call(c, "POST", e.HubURL+"/v1/candidates/"+cand.ID+":approve", "releaser-token", []byte(`{}`))
	if err != nil || st != 201 {
		return fmt.Errorf("approve: %d %s %w", st, b, err)
	}
	e.logf("· released %s", b)
	return nil
}

// waitSynced polls /status until the client accepted a listing issued at t.
func (e Env) waitSynced(t string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_, _, b, err := call(http.DefaultClient, "GET", e.ClientAdmin+"/status", "", nil)
		var s struct {
			LastAcceptedListing *struct {
				IssuedAt string `json:"issued_at"`
			} `json:"last_accepted_listing"`
		}
		if err == nil && json.Unmarshal(b, &s) == nil && s.LastAcceptedListing != nil && s.LastAcceptedListing.IssuedAt == t {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("client did not accept a listing issued at %s", t)
}

// row converts a client response into an observed row.
func row(at, msg string, st int, hdr http.Header, body []byte) Row {
	r := Row{At: at, Message: msg, Status: st, Verdict: hdr.Get("VHub-Verdict")}
	var b struct{ Code string }
	_ = json.Unmarshal(body, &b)
	switch {
	case st >= 400:
		r.Classification, r.Action, r.Code = "violation", "reject", b.Code
	case r.Verdict != "":
		r.Classification, r.Action = "warning", "forward-notice"
	default:
		r.Classification, r.Action = "ok", "forward"
	}
	return r
}

// step moves the clock, optionally waits for the sync of a listing issued at that instant, and sends messages.
func (e Env) step(at string, sync bool, send func() ([]Row, error)) ([]Row, error) {
	if err := e.setClock(at); err != nil {
		return nil, err
	}
	if sync {
		if err := e.waitSynced(at); err != nil {
			return nil, err
		}
	}
	return send()
}

// sendM sends m1–m3 at at.
func (e Env) sendM(at string) func() ([]Row, error) {
	return func() ([]Row, error) {
		var out []Row
		for _, m := range order {
			st, hdr, b, err := call(http.DefaultClient, "POST", e.ClientDSP+"/negotiations/request", "", contractRequest(actions[m]))
			if err != nil {
				return nil, err
			}
			out = append(out, row(at, m, st, hdr, b))
		}
		return out, nil
	}
}

// sendTransfer sends the A1 transfer at at.
func (e Env) sendTransfer(at string) func() ([]Row, error) {
	return func() ([]Row, error) {
		st, hdr, b, err := call(http.DefaultClient, "POST", e.ClientDSP+"/transfers/request", "", transfer())
		if err != nil {
			return nil, err
		}
		return []Row{row(at, "A1 transfer", st, hdr, b)}, nil
	}
}

// sendAgreement sends A1 through the egress forward proxy towards the connector.
func (e Env) sendAgreement(at string) func() ([]Row, error) {
	return func() ([]Row, error) {
		proxy, _ := url.Parse(e.ClientEgres)
		c := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
		st, hdr, b, err := call(c, "POST", e.Connector+"/negotiations/c-1/agreement", "", agreement())
		if err != nil {
			return nil, err
		}
		return []Row{row(at, "A1 agreement ex:access", st, hdr, b)}, nil
	}
}

// revoke revokes 1.0.0 and reports whether the client announced the revocation for A1.
func (e Env) revoke(at string) func() ([]Row, error) {
	return func() ([]Row, error) {
		st, _, b, err := call(http.DefaultClient, "POST", e.HubURL+"/v1/versions/1.0.0:revoke", "releaser-token", []byte(`{"justification":"superseded by 2.0.0"}`))
		if err != nil || st != 200 {
			return nil, fmt.Errorf("revoke: %d %s %w", st, b, err)
		}
		if err := e.waitSynced(at); err != nil {
			return nil, err
		}
		action := "missing"
		if e.eventSeen("AGREEMENT_REVOCATION_SCHEDULED", "urn:uuid:A1") {
			action = "AGREEMENT_REVOCATION_SCHEDULED"
		}
		return []Row{{At: at, Message: "revoke 1.0.0", Classification: "event", Action: action, Status: st}}, nil
	}
}

// eventSeen reports whether the events file holds an event with code about the agreement.
func (e Env) eventSeen(code, agreement string) bool {
	for i := 0; i < 50; i++ {
		f, err := os.Open(e.EventsFile)
		if err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if strings.Contains(sc.Text(), `"code":"`+code+`"`) && strings.Contains(sc.Text(), agreement) {
					_ = f.Close()
					return true
				}
			}
			_ = f.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// Run executes the scenario and returns the observed rows.
func Run(e Env) ([]Row, error) {
	if err := e.setClock("2026-09-01T10:00:00Z"); err != nil {
		return nil, err
	}
	if err := e.publish(Draft("1.0.0")); err != nil {
		return nil, err
	}
	if err := e.setClock("2026-10-01T10:00:00Z"); err != nil {
		return nil, err
	}
	if err := e.publish(Draft("2.0.0")); err != nil {
		return nil, err
	}
	steps := []struct {
		at   string
		sync bool
		send func(string) func() ([]Row, error)
	}{
		{"2026-10-02T12:00:00Z", true, e.sendM},
		{"2026-10-03T09:59:55Z", true, e.sendM},
		{"2026-10-10T12:00:00Z", true, e.sendM},
		{"2026-10-10T12:00:00Z", false, e.sendAgreement},
		{"2026-11-02T06:00:00Z", true, func(string) func() ([]Row, error) { return func() ([]Row, error) { return nil, nil } }},
		{"2026-11-02T10:00:05Z", false, e.sendM},
		{"2026-11-05T12:00:00Z", true, e.sendM},
		{"2026-11-05T12:00:00Z", false, e.sendTransfer},
		{"2026-11-10T00:00:00Z", false, e.revoke},
		{"2026-11-12T00:00:05Z", true, e.sendTransfer},
		{"2026-11-12T00:00:10Z", false, e.sendTransfer},
	}
	var rows []Row
	for _, s := range steps {
		got, err := e.step(s.at, s.sync, s.send(s.at))
		if err != nil {
			return rows, err
		}
		for _, r := range got {
			e.logf("%s", r)
		}
		rows = append(rows, got...)
	}
	return rows, nil
}
