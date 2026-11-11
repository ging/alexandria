// Package testkit extends the fake IdP with an interactive OAuth 2.0 / OIDC flow: authorization code with PKCE, refresh tokens and logout.
// It lets a browser client such as the console sign in as any subject without passwords; lab and tests only (DP-4, DP-14).
package testkit

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// codeTTL bounds how long an authorization code can be redeemed.
const codeTTL = time.Minute

// grant is what an authorization code or refresh token stands for.
type grant struct {
	Sub, ClientID, RedirectURI, Challenge, Nonce string
	Admin                                        bool
	Expires                                      time.Time
}

// OAuth is the interactive authorization server built on an IDP key.
type OAuth struct {
	IDP *IDP
	Now func() time.Time
	TTL time.Duration

	mu      sync.Mutex
	codes   map[string]grant
	refresh map[string]grant
}

// NewOAuth returns an interactive server; now defaults to the wall clock and ttl to one hour.
func NewOAuth(idp *IDP, now func() time.Time, ttl time.Duration) *OAuth {
	if now == nil {
		now = time.Now
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &OAuth{IDP: idp, Now: now, TTL: ttl, codes: map[string]grant{}, refresh: map[string]grant{}}
}

// Handler serves discovery, JWKS, /authorize, /token and /logout with permissive CORS for browser clients.
func (o *OAuth) Handler() http.Handler {
	base := o.IDP.Handler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", o.discovery)
	mux.HandleFunc("GET /authorize", o.authorizePage)
	mux.HandleFunc("POST /authorize", o.authorize)
	mux.HandleFunc("POST /token", o.token)
	mux.HandleFunc("GET /logout", o.logout)
	mux.Handle("/", base)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// discovery serves provider metadata advertising the interactive endpoints.
func (o *OAuth) discovery(w http.ResponseWriter, _ *http.Request) {
	iss := o.IDP.Issuer
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer": iss, "jwks_uri": iss + "/jwks.json", "authorization_endpoint": iss + "/authorize", "token_endpoint": iss + "/token",
		"end_session_endpoint": iss + "/logout", "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported": []string{"S256"}, "id_token_signing_alg_values_supported": []string{"ES256"},
		"subject_types_supported": []string{"public"}, "token_endpoint_auth_methods_supported": []string{"none"},
	})
}

// loopback reports whether a redirect target is an absolute http(s) URL on this machine, so the lab IdP is never an open redirector.
func loopback(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

// checkRequest validates the authorization request parameters and returns an error description.
func checkRequest(q url.Values) string {
	switch {
	case !loopback(q.Get("redirect_uri")):
		return "redirect_uri must be an http(s) URL on localhost or a loopback address"
	case q.Get("response_type") != "code":
		return "response_type must be code"
	case q.Get("client_id") == "":
		return "client_id is required"
	case q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256":
		return "PKCE with code_challenge_method=S256 is required"
	}
	return ""
}

var page = template.Must(template.New("authorize").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>fake-idp · sign in</title>
<style>
:root{color-scheme:light dark}
body{font:13px/1.5 Inter,system-ui,sans-serif;margin:0;min-height:100vh;display:grid;place-items:center;background:#f8f8f8;color:#2b2b2b}
main{width:340px;background:#fff;border:1px solid #e5e5e5;border-radius:8px;padding:20px}
h1{font-size:15px;margin:0 0 2px}p{margin:0 0 14px;color:#616161;font-size:12px}
code{font:11px ui-monospace,Menlo,monospace}label{display:block;font-size:11px;font-weight:500;color:#616161;margin:10px 0 4px}
input[type=text]{width:100%;box-sizing:border-box;height:28px;padding:0 8px;border:1px solid #cecece;border-radius:4px;font:12px ui-monospace,Menlo,monospace;background:transparent;color:inherit}
.row{display:flex;align-items:center;gap:6px;margin:10px 0 14px;font-size:12px}
button{height:28px;padding:0 12px;border-radius:4px;border:1px solid #cecece;background:transparent;color:inherit;font:500 12px Inter,system-ui,sans-serif;cursor:pointer}
button.primary{background:#005fb8;border-color:#005fb8;color:#fff;width:100%}
.quick{display:flex;gap:6px;margin-bottom:6px}.quick button{flex:1}
.err{color:#c72e2e}.warn{margin-top:14px;font-size:11px;color:#8a8a8a}
@media(prefers-color-scheme:dark){body{background:#181818;color:#ccc}main{background:#1f1f1f;border-color:#2b2b2b}p,label{color:#9d9d9d}input[type=text],button{border-color:#3c3c3c}button.primary{background:#0078d4;border-color:#0078d4}}
</style></head><body><main>
<h1>fake-idp</h1>
{{if .Error}}<p class="err">{{.Error}}</p>{{else}}
<p>Sign in to <code>{{.Client}}</code> as any subject. No password: this is a laboratory identity provider.</p>
<form method="post" action="authorize">
{{range $k, $v := .Params}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}
<div class="quick"><button name="preset" value="ana:admin">ana · admin</button><button name="preset" value="sergio">sergio · steward</button></div>
<label for="sub">Subject (sub)</label><input id="sub" type="text" name="sub" placeholder="eve" autocomplete="off">
<div class="row"><input id="admin" type="checkbox" name="admin" value="1"><label for="admin" style="margin:0">Grant the <code>vhub-admin</code> role</label></div>
<button class="primary" type="submit">Sign in</button>
</form>{{end}}
<div class="warn">PoC only (DP-14) · issuer <code>{{.Issuer}}</code></div>
</main></body></html>`))

// forwarded lists the authorization parameters echoed through the sign-in form.
var forwarded = []string{"response_type", "client_id", "redirect_uri", "scope", "state", "nonce", "code_challenge", "code_challenge_method", "audience"}

// authorizePage renders the subject picker, or the request error when the parameters are invalid.
func (o *OAuth) authorizePage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := map[string]any{"Issuer": o.IDP.Issuer, "Client": q.Get("client_id"), "Error": checkRequest(q)}
	params := map[string]string{}
	for _, k := range forwarded {
		if v := q.Get(k); v != "" {
			params[k] = v
		}
	}
	data["Params"] = params
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data["Error"] != "" {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = page.Execute(w, data)
}

// authorize issues a single-use code for the chosen subject and redirects back to the client.
func (o *OAuth) authorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	f := r.PostForm
	if msg := checkRequest(f); msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	sub, admin := f.Get("sub"), f.Get("admin") != ""
	switch f.Get("preset") {
	case "ana:admin":
		sub, admin = "ana", true
	case "sergio":
		sub, admin = "sergio", false
	}
	if sub == "" {
		http.Error(w, "sub is required", http.StatusBadRequest)
		return
	}
	code := randomToken()
	o.mu.Lock()
	o.codes[code] = grant{
		Sub: sub, Admin: admin, ClientID: f.Get("client_id"), RedirectURI: f.Get("redirect_uri"),
		Challenge: f.Get("code_challenge"), Nonce: f.Get("nonce"), Expires: o.Now().Add(codeTTL),
	}
	o.mu.Unlock()
	target, _ := url.Parse(f.Get("redirect_uri"))
	q := target.Query()
	q.Set("code", code)
	if s := f.Get("state"); s != "" {
		q.Set("state", s)
	}
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), http.StatusSeeOther) // #nosec G710 -- target is restricted to loopback by checkRequest.
}

// token redeems an authorization code (with PKCE) or a refresh token.
func (o *OAuth) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		oauthError(w, "invalid_request", "unreadable form")
		return
	}
	f := r.PostForm
	var g grant
	var ok bool
	switch f.Get("grant_type") {
	case "authorization_code":
		g, ok = o.take(o.codes, f.Get("code"))
		sum := sha256.Sum256([]byte(f.Get("code_verifier")))
		pkce := subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(g.Challenge)) == 1
		if !ok || !pkce || g.RedirectURI != f.Get("redirect_uri") || g.ClientID != f.Get("client_id") || o.Now().After(g.Expires) {
			oauthError(w, "invalid_grant", "the code is unknown, expired, already used or fails PKCE verification")
			return
		}
	case "refresh_token":
		if g, ok = o.take(o.refresh, f.Get("refresh_token")); !ok || g.ClientID != f.Get("client_id") {
			oauthError(w, "invalid_grant", "the refresh token is unknown or already used")
			return
		}
	default:
		oauthError(w, "unsupported_grant_type", "use authorization_code or refresh_token")
		return
	}
	now := o.Now()
	access, err := o.IDP.Mint(Token{Sub: g.Sub, Admin: g.Admin, Now: now, TTL: o.TTL})
	if err != nil {
		oauthError(w, "server_error", err.Error())
		return
	}
	id, err := o.idToken(g, now)
	if err != nil {
		oauthError(w, "server_error", err.Error())
		return
	}
	next := randomToken()
	o.mu.Lock()
	o.refresh[next] = g
	o.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access, "token_type": "Bearer", "expires_in": int(o.TTL.Seconds()), "refresh_token": next, "id_token": id, "scope": "openid profile offline_access",
	})
}

// take removes and returns a grant, making codes and refresh tokens single-use.
func (o *OAuth) take(m map[string]grant, key string) (grant, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	g, ok := m[key]
	delete(m, key)
	return g, ok && key != ""
}

// idToken signs the OIDC ID token for a grant, bound to the client and the request nonce.
func (o *OAuth) idToken(g grant, now time.Time) (string, error) {
	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", o.IDP.Kid)
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: o.IDP.Key}, opts)
	if err != nil {
		return "", err
	}
	roles := []string{}
	if g.Admin {
		roles = append(roles, "vhub-admin")
	}
	claims := jwt.Claims{Issuer: o.IDP.Issuer, Subject: g.Sub, Audience: jwt.Audience{g.ClientID}, IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(now.Add(o.TTL))}
	extra := map[string]any{"preferred_username": g.Sub, "roles": roles}
	if g.Nonce != "" {
		extra["nonce"] = g.Nonce
	}
	return jwt.Signed(sig).Claims(claims).Claims(extra).Serialize()
}

// logout ends the (stateless) session and returns to a loopback post_logout_redirect_uri when given.
func (o *OAuth) logout(w http.ResponseWriter, r *http.Request) {
	if target := r.URL.Query().Get("post_logout_redirect_uri"); loopback(target) {
		http.Redirect(w, r, target, http.StatusSeeOther) // #nosec G710 -- target is restricted to loopback.
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("signed out\n"))
}

// randomToken returns 32 random bytes as base64url.
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// writeJSON writes a JSON body with its status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// oauthError writes an RFC 6749 §5.2 error response.
func oauthError(w http.ResponseWriter, code, description string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": code, "error_description": description})
}
