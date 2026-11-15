# Helpers for the Flexgrid lab of the manual; source it from tutorial/manual/examples/lab (bash or zsh).
# Every function is a thin wrapper over curl or fake-idp, so nothing here hides an API call.

BIN=../../../../bin
HUB=https://127.0.0.1:18443
IDP=http://127.0.0.1:19400
CONNECTOR=http://127.0.0.1:18282
ADMINAPI=http://127.0.0.1:19090
INGRESS=http://127.0.0.1:19080
NS=flexgrid
NEG=urn:uuid:9c1f7a52-dso-neg-0042

# clock <instant>: moves the file clock shared by every process.
clock() { echo "$1" > clock; }

# tok --sub <name> [--admin]: mints an access token at the file-clock instant, valid 100 days.
tok() { $BIN/fake-idp token --issuer $IDP --key idp.key --clock-file clock --ttl 2400h "$@"; }

# hubdid: prints the DID the hub logged on start.
hubdid() { grep -o 'hub_did=did:key:[A-Za-z0-9]*' server.log | head -1 | cut -d= -f2; }

# syncat <instant>: moves the clock and waits until the client accepts a listing issued at that instant.
syncat() {
  clock "$1"
  for _ in $(seq 1 100); do
    v=$(curl -s $ADMINAPI/status | python3 -c "import json,sys;print(json.load(sys.stdin)['last_accepted_listing']['valid_from'])" 2>/dev/null)
    [ "$v" = "$1" ] && return 0
    sleep 0.2
  done
  echo "the client did not accept a listing issued at $1" >&2
  return 1
}

# publish <bundle.yaml>: creates a draft, validates it and proposes it; leaves the candidate URL in last-candidate.url.
publish() {
  loc=$(curl -sk -i -X POST $HUB/v1/namespaces/$NS/drafts -H "Authorization: Bearer $(cat steward.token)" \
    -H 'Content-Type: application/yaml' --data-binary @"$1" | grep -i '^location' | tr -d '\r' | awk '{print $2}')
  echo "draft:     ${loc##*/}"
  curl -sk -X POST "$loc:validate" -H "Authorization: Bearer $(cat steward.token)"; echo
  curl -sk -X POST "$loc:propose" -H "Authorization: Bearer $(cat steward.token)" > last-candidate.json
  python3 -c "import json;d=json.load(open('last-candidate.json'));print(d.get('id') or json.dumps(d))" > last-candidate.url
  echo "candidate: $(cat last-candidate.url | sed 's|.*/||')"
}

# approve <json-body>: approves the last candidate and prints the status line and the dates of the record.
approve() {
  curl -sk -i -X POST "$(cat last-candidate.url):approve" -H "Authorization: Bearer $(cat steward.token)" -d "$1" | python3 -c "
import sys, json
raw = sys.stdin.read().replace('\r', ''); head, _, body = raw.partition('\n\n'); print(head.split('\n')[0]); d = json.loads(body)
s = d.get('credentialSubject')
print({k: s[k] for k in ('version', 'class', 'published_at', 'new_content_from', 'enforced_from')} if s else json.dumps(d))"
}

# ex <message>: asks /rules:explain what the proxy would do with ../msgs/<message>.json, without side effects.
ex() {
  python3 -c "
import json, sys
print(json.dumps({'listener': 'dsp-ingress', 'method': 'POST', 'path': '/negotiations/$NEG/request', 'body': json.load(open(sys.argv[1]))}))" ../msgs/"$1".json |
    curl -s -X POST "$ADMINAPI/rules:explain" -d @-
}

# short: condenses an :explain answer read from stdin.
short() {
  python3 -c "
import json, sys
d = json.load(sys.stdin)
print('rule=%s state=%s action=%s code=%s' % (d.get('rule'), d.get('state'), d.get('action'), d.get('code')))
for g in d.get('gates', []):
    print('  gate=%-14s class=%-9s codes=%s matched=%s' % (g['gate'], g.get('classification'), g.get('codes'), g.get('matched')))
    for f in g.get('findings', []):
        print('     -', f.get('severity'), f.get('source'), f.get('path', ''), (f.get('message') or '')[:150])
print('  verdict:', d.get('vhub_verdict'))"
}

# post <path> <message> [-b]: sends ../msgs/<message>.json through dsp-ingress and prints status and VHub-Verdict (-b adds the body).
post() {
  curl -s -i -X POST "$INGRESS$1" -H 'Content-Type: application/json' --data-binary @../msgs/"$2".json | python3 -c "
import sys
raw = sys.stdin.read().replace('\r', ''); head, _, body = raw.partition('\n\n'); lines = head.split('\n')
v = [l for l in lines if l.lower().startswith('vhub-verdict')]
print('%-22s %s | %s' % (sys.argv[1], lines[0], v[0] if v else '-'))
if '-b' in sys.argv: print('   ', body)" "$2" "$3"
}

# row <message>: sends a contract request of the running negotiation.
row() { post "/negotiations/$NEG/request" "$1" "$2"; }

# rows: sends m1, m2 and m3 and prints the client state and local versions.
rows() {
  for m in m1-forecast m2-profiling m3-research; do row $m; done
  curl -s $ADMINAPI/status | python3 -c "import json,sys;d=json.load(sys.stdin);print('  ', d['state'], [(v['version'], v['local_role']) for v in d['versions']])"
}

# setrule <id>: replaces a rule with ../rules/<id>.json using the current revision as If-Match.
setrule() {
  rev=$(curl -s -i $ADMINAPI/rules | grep -i '^etag' | tr -d '\r' | awk '{print $2}')
  curl -s -o /dev/null -w "$1 -> %{http_code}\n" -X PUT "$ADMINAPI/rules/$1" -H "If-Match: $rev" --data-binary @../rules/"$1".json
}
