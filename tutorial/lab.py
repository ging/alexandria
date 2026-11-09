# Self-contained helper library and rich HTML widget renderer for the Alexandria Jupyter notebooks.
# Uses only the Python 3 standard library and emits rich HTML cards in Jupyter/VS Code/JetBrains plus text in CLI.
from __future__ import annotations

import datetime as dt
import html
import json
import os
import re
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

# Hook populated by run.py / build_notebooks.py when capturing display_data outputs into .ipynb files.
_HTML_CAPTURE_LIST: list[tuple[str, str]] | None = None


def set_html_capture(target: list[tuple[str, str]] | None) -> None:
    """Sets or clears the active HTML capture list used by the notebook runner."""
    global _HTML_CAPTURE_LIST
    _HTML_CAPTURE_LIST = target


def emit_widget(html_content: str, plain_fallback: str = "") -> None:
    """Displays an HTML widget in IPython/Jupyter if active and records it for .ipynb serialization."""
    if _HTML_CAPTURE_LIST is not None:
        _HTML_CAPTURE_LIST.append((html_content, plain_fallback))
        return
    try:
        from IPython import get_ipython  # type: ignore
        from IPython.display import HTML, display  # type: ignore

        if get_ipython() is not None:
            display(HTML(html_content))
            return
    except Exception:
        pass
    if plain_fallback:
        print(plain_fallback)


# ==============================================================================
# Syntax Highlighters & HTML Building Blocks
# ==============================================================================

_WIDGET_CSS = """
<style>
.ag-w {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
  background: #0f172a;
  color: #e2e8f0;
  border: 1px solid #1e293b;
  border-radius: 10px;
  margin: 10px 0;
  overflow: hidden;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
  font-size: 13px;
  line-height: 1.5;
}
.ag-w * { box-sizing: border-box; }
.ag-hdr {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px 14px;
  background: #1e293b;
  border-bottom: 1px solid #334155;
}
.ag-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.ag-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 11.5px;
  font-weight: 700;
  letter-spacing: 0.02em;
}
.ag-m-GET { background: rgba(56, 189, 248, 0.16); color: #38bdf8; border: 1px solid rgba(56, 189, 248, 0.35); }
.ag-m-POST { background: rgba(52, 211, 153, 0.16); color: #34d399; border: 1px solid rgba(52, 211, 153, 0.35); }
.ag-m-PUT { background: rgba(251, 191, 36, 0.16); color: #fbbf24; border: 1px solid rgba(251, 191, 36, 0.35); }
.ag-m-DELETE { background: rgba(248, 113, 113, 0.16); color: #f87171; border: 1px solid rgba(248, 113, 113, 0.35); }
.ag-st-2xx { background: rgba(16, 185, 129, 0.2); color: #6ee7b7; border: 1px solid rgba(16, 185, 129, 0.4); }
.ag-st-3xx { background: rgba(129, 140, 248, 0.2); color: #a5b4fc; border: 1px solid rgba(129, 140, 248, 0.4); }
.ag-st-4xx { background: rgba(244, 63, 94, 0.2); color: #fda4af; border: 1px solid rgba(244, 63, 94, 0.45); }
.ag-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  background: #0f172a;
  color: #94a3b8;
  border: 1px solid #334155;
}
.ag-url {
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 12.5px;
  color: #f8fafc;
  word-break: break-all;
}
.ag-url-host { color: #64748b; }
.ag-url-path { color: #e2e8f0; font-weight: 600; }
.ag-banner {
  padding: 8px 14px;
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  border-bottom: 1px solid #1e293b;
}
.ag-banner-warn { background: rgba(245, 158, 11, 0.14); color: #fcd34d; border-left: 4px solid #f59e0b; }
.ag-banner-err { background: rgba(239, 68, 68, 0.14); color: #fca5a5; border-left: 4px solid #ef4444; }
.ag-banner-ok { background: rgba(16, 185, 129, 0.12); color: #6ee7b7; border-left: 4px solid #10b981; }
.ag-pre {
  margin: 0;
  padding: 12px 14px;
  background: #0b1120;
  color: #e2e8f0;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.55;
  overflow-x: auto;
  white-space: pre;
}
.ag-details {
  border-top: 1px solid #1e293b;
  background: #0f172a;
}
.ag-details summary {
  cursor: pointer;
  padding: 7px 14px;
  font-size: 11.5px;
  font-weight: 600;
  color: #94a3b8;
  background: #131c31;
  user-select: none;
  list-style: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.ag-details summary:hover { color: #f8fafc; background: #1e293b; }
.ag-tbl {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.ag-tbl th, .ag-tbl td {
  padding: 6px 12px;
  text-align: left;
  border-bottom: 1px solid #1e293b;
  font-family: 'SFMono-Regular', Consolas, monospace;
}
.ag-tbl th {
  color: #94a3b8;
  font-weight: 600;
  background: #131c31;
  font-family: -apple-system, BlinkMacSystemFont, sans-serif;
}
.ag-k { color: #7dd3fc; }
.ag-s { color: #86efac; }
.ag-n { color: #f9a8d4; }
.ag-b { color: #fcd34d; font-weight: 600; }
.ag-c { color: #64748b; font-style: italic; }
.ag-ni { color: #c084fc; font-weight: 600; }
.ag-ln {
  display: inline-block;
  width: 28px;
  color: #475569;
  user-select: none;
  text-align: right;
  margin-right: 12px;
}
</style>
"""


def _highlight_json(text: str) -> str:
    """Applies lightweight HTML syntax highlighting to a formatted JSON string."""
    esc = html.escape(text)

    def _repl(match: re.Match[str]) -> str:
        tok = match.group(0)
        if tok.startswith('"') and tok.endswith('":'):
            return f'<span class="ag-k">{tok[:-1]}</span>:'
        if tok.startswith('"'):
            if "ni:///sha-256;" in tok or "did:web:" in tok:
                return f'<span class="ag-ni">{tok}</span>'
            return f'<span class="ag-s">{tok}</span>'
        if tok in ("true", "false", "null"):
            return f'<span class="ag-b">{tok}</span>'
        return f'<span class="ag-n">{tok}</span>'

    pattern = re.compile(
        r'("(?:\\.|[^"\\])*"\s*:|"(?:\\.|[^"\\])*"|\b(?:true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)'
    )
    return pattern.sub(_repl, esc)


def _highlight_yaml(text: str, with_line_numbers: bool = True) -> str:
    """Applies HTML syntax highlighting and line numbers to a YAML document stream."""
    out_lines: list[str] = []
    for idx, raw_line in enumerate(text.splitlines(), start=1):
        ln_prefix = f'<span class="ag-ln">{idx}</span>' if with_line_numbers else ""
        stripped = raw_line.strip()
        if stripped == "---":
            out_lines.append(f'{ln_prefix}<span class="ag-ni">---</span>')
            continue
        if stripped.startswith("#"):
            out_lines.append(f'{ln_prefix}<span class="ag-c">{html.escape(raw_line)}</span>')
            continue
        # Separate inline comment if present.
        code_part = raw_line
        comment_part = ""
        if " #" in raw_line:
            code_part, comment_part = raw_line.split(" #", 1)
            comment_part = f' <span class="ag-c">#{html.escape(comment_part)}</span>'

        m = re.match(r"^(\s*-?\s*)([A-Za-z0-9_.-]+)(:\s*)(.*)$", code_part)
        if m:
            indent, key, colon, val = m.groups()
            val_esc = html.escape(val)
            if val_esc.startswith(("http://", "https://", "eunomia.", "odrl:", "xsd:")):
                val_html = f'<span class="ag-ni">{val_esc}</span>'
            elif val_esc in ("true", "false", "null", "OdrlProfile", "SpaceProfile", "mandatory", "recommended"):
                val_html = f'<span class="ag-b">{val_esc}</span>'
            elif val_esc:
                val_html = f'<span class="ag-s">{val_esc}</span>'
            else:
                val_html = ""
            out_lines.append(
                f'{ln_prefix}{html.escape(indent)}<span class="ag-k">{html.escape(key)}</span>{html.escape(colon)}{val_html}{comment_part}'
            )
        else:
            out_lines.append(f"{ln_prefix}{html.escape(code_part)}{comment_part}")
    return "\n".join(out_lines)


def _split_url(url: str) -> tuple[str, str]:
    """Splits http://host:port/path into ('http://host:port', '/path')."""
    m = re.match(r"^(https?://[^/]+)(/.*)?$", url)
    if not m:
        return "", url
    return m.group(1), m.group(2) or "/"


# ==============================================================================
# High-Level Notebook Visual Widgets
# ==============================================================================


def show_yaml(title: str, yaml_text: str, subtitle: str = "") -> None:
    """Renders a syntax-highlighted YAML viewer widget for eunomia.profiles/v1 documents."""
    kinds = re.findall(r"^\s*kind:\s*(\S+)", yaml_text, flags=re.MULTILINE)
    versions = re.findall(r"^\s*version:\s*(\S+)", yaml_text, flags=re.MULTILINE)
    line_count = len(yaml_text.strip().splitlines())

    badges_html = "".join(f'<span class="ag-badge ag-m-GET">{html.escape(k)}</span>' for k in kinds)
    if versions:
        badges_html += f'<span class="ag-chip">v{html.escape(versions[-1])}</span>'
    badges_html += f'<span class="ag-chip">{line_count} lines</span>'

    sub_html = f'<span style="color:#94a3b8;font-size:12px;">{html.escape(subtitle)}</span>' if subtitle else ""
    widget = f"""{_WIDGET_CSS}
<div class="ag-w">
  <div class="ag-hdr">
    <div class="ag-row">
      <span class="ag-badge ag-m-PUT">YAML DSL</span>
      <strong style="color:#f8fafc;">{html.escape(title)}</strong>
      {sub_html}
    </div>
    <div class="ag-row">{badges_html}</div>
  </div>
  <pre class="ag-pre">{_highlight_yaml(yaml_text.strip())}</pre>
</div>"""
    plain = f"=== {title} ({line_count} lines) ===\n{yaml_text.strip()}"
    emit_widget(widget, plain)


def show_artifacts(title: str, base_dir: Path, files: list[str] | None = None) -> None:
    """Renders an interactive multi-tab explorer card for compiled alexandriac artifacts."""
    if files is None:
        all_files = sorted(p for p in base_dir.rglob("*") if p.is_file())
    else:
        all_files = [base_dir / f for f in files if (base_dir / f).exists()]

    cards_html: list[str] = []
    plain_lines = [f"=== {title} ({len(all_files)} artifacts) ==="]
    for idx, fpath in enumerate(all_files):
        rel = fpath.relative_to(base_dir).as_posix()
        content = fpath.read_text(encoding="utf-8")
        size_b = len(content.encode("utf-8"))
        plain_lines.append(f" • {rel} ({size_b} B)")
        preview = content[:900] + ("\n... (truncated)" if len(content) > 900 else "")
        if rel.endswith((".json", ".jsonld")):
            try:
                parsed = json.loads(content)
                preview_fmt = json.dumps(parsed, indent=2, ensure_ascii=False)
                if len(preview_fmt) > 1000:
                    preview_fmt = preview_fmt[:1000] + "\n... (truncated)"
                body_html = _highlight_json(preview_fmt)
            except Exception:
                body_html = html.escape(preview)
        else:
            body_html = html.escape(preview)

        open_attr = " open" if idx < 2 else ""
        cards_html.append(
            f"""<details class="ag-details"{open_attr}>
  <summary>
    <span>📄 <strong style="color:#e2e8f0;font-family:monospace;">{html.escape(rel)}</strong></span>
    <span class="ag-chip">{size_b} bytes</span>
  </summary>
  <pre class="ag-pre">{body_html}</pre>
</details>"""
        )

    widget = f"""{_WIDGET_CSS}
<div class="ag-w">
  <div class="ag-hdr">
    <div class="ag-row">
      <span class="ag-badge ag-m-POST">alexandriac compile</span>
      <strong style="color:#f8fafc;">{html.escape(title)}</strong>
    </div>
    <span class="ag-chip">{len(all_files)} compiled artifacts</span>
  </div>
  {''.join(cards_html)}
</div>"""
    plain = "\n".join(plain_lines)
    emit_widget(widget, plain)


def show_events_table(events: list[dict[str, Any]], title: str = "Client Event Log (events.jsonl)") -> None:
    """Renders client audit, verdict, and agreement events as a visual HTML table."""
    rows_html: list[str] = []
    plain_lines = [f"=== {title} ({len(events)} events) ==="]
    for ev in events:
        etype = ev.get("type", "")
        ts = ev.get("ts", "")
        mode = ev.get("mode") or ev.get("severity") or "-"
        route_or_ver = ev.get("route") or ev.get("version") or ev.get("from", "")
        if ev.get("to"):
            route_or_ver = f"{ev.get('from')} → {ev.get('to')}"
        cls_or_ag = ev.get("classification") or ev.get("agreement") or "-"
        action = ev.get("action") or ev.get("detail") or "-"
        codes_list = ev.get("codes") or ([ev["code"]] if ev.get("code") else [])
        codes_str = ", ".join(codes_list) if codes_list else "OK"

        badge_cls = "ag-st-2xx"
        if "violation" in str(cls_or_ag).lower() or "reject" in str(action).lower() or "REVOKED" in codes_str:
            badge_cls = "ag-st-4xx"
        elif "warning" in str(cls_or_ag).lower() or "SCHEDULED" in codes_str or "Degraded" in str(route_or_ver):
            badge_cls = "ag-m-PUT"

        plain_lines.append(f" • [{etype}] {ts} | {mode} | {route_or_ver} | {cls_or_ag} | {codes_str}")
        rows_html.append(
            f"""<tr>
  <td><span class="ag-chip">{html.escape(str(etype))}</span></td>
  <td style="color:#94a3b8;">{html.escape(str(ts))}</td>
  <td><span class="ag-badge ag-m-GET">{html.escape(str(mode))}</span></td>
  <td style="color:#f8fafc;">{html.escape(str(route_or_ver))}</td>
  <td>{html.escape(str(cls_or_ag))}</td>
  <td><span class="ag-badge {badge_cls}">{html.escape(codes_str)}</span></td>
</tr>"""
        )

    widget = f"""{_WIDGET_CSS}
<div class="ag-w">
  <div class="ag-hdr">
    <div class="ag-row">
      <span class="ag-badge ag-m-GET">JSONL Events</span>
      <strong style="color:#f8fafc;">{html.escape(title)}</strong>
    </div>
    <span class="ag-chip">{len(events)} events</span>
  </div>
  <table class="ag-tbl">
    <thead>
      <tr><th>Type</th><th>Instant (UTC)</th><th>Mode / Sev</th><th>Route / Version</th><th>Classification / Agreement</th><th>Codes</th></tr>
    </thead>
    <tbody>{''.join(rows_html)}</tbody>
  </table>
</div>"""
    plain = "\n".join(plain_lines)
    emit_widget(widget, plain)


def show_state_machine(current_state: str, subtitle: str = "") -> None:
    """Renders the 5-state operational diagram of alexandria-client highlighting the active state."""
    states = [
        ("Onboarding", "No initial package loaded"),
        ("Active", "1 version in force (t < expires_at)"),
        ("Transition", ">1 version in V_t (grace window)"),
        ("Degraded", "expires_at elapsed (< max_staleness)"),
        ("FailClosed", "max_staleness exceeded (rejects enforce)"),
    ]
    steps_html: list[str] = []
    for name, desc in states:
        active = name.lower() == current_state.lower()
        border = "#38bdf8" if active else "#334155"
        bg = "rgba(56, 189, 248, 0.16)" if active else "#131c31"
        if active and name == "Degraded":
            border, bg = "#f59e0b", "rgba(245, 158, 11, 0.18)"
        elif active and name == "FailClosed":
            border, bg = "#ef4444", "rgba(239, 68, 68, 0.20)"
        elif active and name == "Active":
            border, bg = "#10b981", "rgba(16, 185, 129, 0.18)"
        dot = "● " if active else "○ "
        steps_html.append(
            f"""<div style="flex:1;min-width:135px;padding:10px;border-radius:8px;border:2px solid {border};background:{bg};">
  <div style="font-weight:700;color:#f8fafc;font-size:12.5px;">{dot}{html.escape(name)}</div>
  <div style="color:#94a3b8;font-size:11px;margin-top:3px;">{html.escape(desc)}</div>
</div>"""
        )

    widget = f"""{_WIDGET_CSS}
<div class="ag-w">
  <div class="ag-hdr">
    <div class="ag-row">
      <span class="ag-badge ag-m-GET">State Machine (§4.5)</span>
      <strong style="color:#f8fafc;">Current State: {html.escape(current_state)}</strong>
    </div>
    <span style="color:#94a3b8;font-size:12px;">{html.escape(subtitle)}</span>
  </div>
  <div style="display:flex;gap:8px;padding:12px 14px;flex-wrap:wrap;background:#0b1120;">
    {''.join(steps_html)}
  </div>
</div>"""
    plain = f"🛡️ Client operational state: {current_state} ({subtitle})"
    emit_widget(widget, plain)


def show_f6_matrix(rows: list[dict[str, str]], title: str = "Scenario F6 Verdict Matrix") -> None:
    """Renders a visual summary table of Scenario F6 HTTP evaluations across timestamps."""
    tr_html: list[str] = []
    plain_lines = [f"=== {title} ==="]
    for r in rows:
        instant = r.get("instant", "")
        phase = r.get("phase", "")
        msg = r.get("message", "")
        status = str(r.get("status", ""))
        verdict = r.get("verdict", "-")
        st_cls = "ag-st-2xx" if status.startswith("2") else "ag-st-4xx"
        if "warning" in verdict:
            st_cls = "ag-m-PUT"
        plain_lines.append(f"[{instant}] ({phase}) {msg:18s} -> HTTP {status} | {verdict}")
        tr_html.append(
            f"""<tr>
  <td style="color:#7dd3fc;">{html.escape(instant)}</td>
  <td><span class="ag-chip">{html.escape(phase)}</span></td>
  <td style="color:#f8fafc;font-weight:600;">{html.escape(msg)}</td>
  <td><span class="ag-badge {st_cls}">HTTP {html.escape(status)}</span></td>
  <td style="color:#cbd5e1;">{html.escape(verdict)}</td>
</tr>"""
        )

    widget = f"""{_WIDGET_CSS}
<div class="ag-w">
  <div class="ag-hdr">
    <div class="ag-row">
      <span class="ag-badge ag-m-POST">Scenario F6</span>
      <strong style="color:#f8fafc;">{html.escape(title)}</strong>
    </div>
    <span class="ag-chip">{len(rows)} checks</span>
  </div>
  <table class="ag-tbl">
    <thead>
      <tr><th>Instant (clock_file)</th><th>Lifecycle Phase</th><th>Message / Action</th><th>HTTP</th><th>VHub-Verdict Header / Code</th></tr>
    </thead>
    <tbody>{''.join(tr_html)}</tbody>
  </table>
</div>"""
    plain = "\n".join(plain_lines)
    emit_widget(widget, plain)


# ==============================================================================
# Core Binary Builder, HTTP Client & AlexandriaLab Orchestrator
# ==============================================================================


def repo_root() -> Path:
    """Returns the absolute path to the alexandria repository root."""
    here = Path(__file__).resolve().parent
    if (here.parent / "go.mod").exists():
        return here.parent
    return Path.cwd()


def ensure_binaries() -> Path:
    """Compiles alexandriac, alexandria-server, alexandria-client, and fake-connector into bin/ if needed."""
    root = repo_root()
    bin_dir = root / "bin"
    bin_dir.mkdir(exist_ok=True)
    targets = {
        "alexandriac": "./cmd/alexandriac",
        "alexandria-server": "./cmd/alexandria-server",
        "alexandria-client": "./cmd/alexandria-client",
        "fake-connector": "./deploy/fake-connector",
    }
    for name, pkg in targets.items():
        out = bin_dir / name
        if not out.exists():
            subprocess.run(["go", "build", "-o", str(out), pkg], cwd=str(root), check=True)
    return bin_dir


def free_port() -> int:
    """Allocates and returns an available localhost TCP port."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def http_request(
    method: str,
    url: str,
    *,
    headers: dict[str, str] | None = None,
    body: Any = None,
    token: str | None = None,
    proxy: str | None = None,
    follow_redirects: bool = True,
    verbose: bool = True,
) -> tuple[int, dict[str, str], Any]:
    """Performs an HTTP request, prints CLI summary, and emits an interactive HTML Inspector card."""
    req_headers: dict[str, str] = {}
    if headers:
        req_headers.update(headers)
    if token:
        req_headers["Authorization"] = f"Bearer {token}"

    raw_body: bytes | None = None
    req_body_preview = ""
    if body is not None:
        if isinstance(body, (dict, list)):
            req_body_preview = json.dumps(body, indent=2, ensure_ascii=False)
            raw_body = req_body_preview.encode("utf-8")
            req_headers.setdefault("Content-Type", "application/json")
        elif isinstance(body, str):
            req_body_preview = body
            raw_body = body.encode("utf-8")
            req_headers.setdefault("Content-Type", "application/yaml")
        elif isinstance(body, bytes):
            raw_body = body
            req_body_preview = body.decode("utf-8", errors="replace")

    handlers: list[urllib.request.BaseHandler] = []
    if proxy:
        handlers.append(urllib.request.ProxyHandler({"http": proxy, "https": proxy}))
    if not follow_redirects:

        class _NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, req, fp, code, msg, hdrs, newurl):
                return None

        handlers.append(_NoRedirect())

    opener = urllib.request.build_opener(*handlers)
    req = urllib.request.Request(url, data=raw_body, headers=req_headers, method=method.upper())

    t0 = time.perf_counter()
    try:
        with opener.open(req, timeout=10) as resp:
            status = resp.status
            resp_headers = {k.lower(): v for k, v in resp.headers.items()}
            resp_bytes = resp.read()
    except urllib.error.HTTPError as exc:
        status = exc.code
        resp_headers = {k.lower(): v for k, v in exc.headers.items()}
        resp_bytes = exc.read()
    elapsed_ms = (time.perf_counter() - t0) * 1000.0

    text = resp_bytes.decode("utf-8", errors="replace") if resp_bytes else ""
    parsed: Any = text
    is_json = False
    if text:
        try:
            parsed = json.loads(text)
            is_json = True
        except Exception:
            parsed = text

    if verbose:
        _emit_http_inspector(
            method=method.upper(),
            url=url,
            token=token,
            proxy=proxy,
            req_headers=req_headers,
            req_body_preview=req_body_preview,
            status=status,
            elapsed_ms=elapsed_ms,
            resp_headers=resp_headers,
            parsed=parsed,
            is_json=is_json,
        )

    return status, resp_headers, parsed


def _emit_http_inspector(
    *,
    method: str,
    url: str,
    token: str | None,
    proxy: str | None,
    req_headers: dict[str, str],
    req_body_preview: str,
    status: int,
    elapsed_ms: float,
    resp_headers: dict[str, str],
    parsed: Any,
    is_json: bool,
) -> None:
    """Formats both plain CLI output and the rich HTML HTTP Inspector widget."""
    proxy_note = f" (via proxy {proxy})" if proxy else ""
    plain_lines = [f"▶ {method} {url}{proxy_note}"]
    if token:
        plain_lines.append(f"  Authorization: Bearer {token}")
    for hk in ("If-Match", "Content-Type"):
        if hk in req_headers and hk != "Content-Type":
            plain_lines.append(f"  {hk}: {req_headers[hk]}")
    plain_lines.append(f"◀ HTTP {status} ({elapsed_ms:.1f} ms)")
    for hk in ("etag", "location", "cache-control", "vhub-verdict", "www-authenticate", "content-type", "date"):
        if hk in resp_headers:
            plain_lines.append(f"  {hk}: {resp_headers[hk]}")
    if parsed != "":
        if is_json:
            s = json.dumps(parsed, indent=2, ensure_ascii=False)
            if len(s) > 1400:
                s = s[:1400] + "\n  ... (truncated)"
            plain_lines.append(s)
        else:
            s = str(parsed)
            if len(s) > 900:
                s = s[:900] + "\n... (truncated)"
            plain_lines.append(s)
    plain_lines.append("-" * 72)
    plain_str = "\n".join(plain_lines)

    # Build HTML card.
    host_part, path_part = _split_url(url)
    st_class = "ag-st-2xx" if 200 <= status < 300 else ("ag-st-3xx" if 300 <= status < 400 else "ag-st-4xx")
    m_class = f"ag-m-{method}" if method in ("GET", "POST", "PUT", "DELETE") else "ag-m-GET"

    chips: list[str] = []
    if token:
        chips.append(f'<span class="ag-chip">🔑 {html.escape(token)}</span>')
    if proxy:
        chips.append('<span class="ag-chip">🛡️ via dsp-egress</span>')
    if "If-Match" in req_headers:
        chips.append(f'<span class="ag-chip">If-Match: {html.escape(req_headers["If-Match"])}</span>')
    chips.append(f'<span class="ag-chip">⚡ {elapsed_ms:.1f} ms</span>')

    banners: list[str] = []
    verdict_hdr = resp_headers.get("vhub-verdict", "")
    if verdict_hdr:
        v_cls = "ag-banner-warn" if verdict_hdr.startswith("warning") else "ag-banner-err"
        sunset_note = ""
        m_sun = re.search(r"sunset=@(\d+)", verdict_hdr)
        if m_sun:
            ts_int = int(m_sun.group(1))
            iso_sun = dt.datetime.fromtimestamp(ts_int, tz=dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
            sunset_note = f'<span class="ag-chip">⏳ sunset: {iso_sun}</span>'
        banners.append(
            f'<div class="ag-banner {v_cls}"><span><strong>VHub-Verdict:</strong> {html.escape(verdict_hdr)}</span>{sunset_note}</div>'
        )

    if isinstance(parsed, dict) and parsed.get("code"):
        code_val = str(parsed["code"])
        detail_val = str(parsed.get("detail") or (parsed.get("reason") or [""])[0] or "")
        banners.append(
            f'<div class="ag-banner ag-banner-err"><span>🏷️ Normative Code: <strong>{html.escape(code_val)}</strong></span>'
            f'<span style="color:#fecdd3;">{html.escape(detail_val)}</span></div>'
        )
    elif status == 303 and "location" in resp_headers:
        banners.append(
            f'<div class="ag-banner ag-banner-ok"><span>↪️ Redirect 303 See Other</span>'
            f'<span>Location: <strong>{html.escape(resp_headers["location"])}</strong></span></div>'
        )

    # Response body HTML.
    body_html = ""
    if parsed != "":
        if is_json:
            fmt_json = json.dumps(parsed, indent=2, ensure_ascii=False)
            if len(fmt_json) > 2200:
                fmt_json = fmt_json[:2200] + "\n... (truncated)"
            body_html = f'<pre class="ag-pre">{_highlight_json(fmt_json)}</pre>'
        else:
            s_txt = str(parsed)
            if len(s_txt) > 1200:
                s_txt = s_txt[:1200] + "\n... (truncated)"
            body_html = f'<pre class="ag-pre">{html.escape(s_txt)}</pre>'

    # Headers collapsible section.
    hdr_rows = "".join(
        f"<tr><td class='ag-k'>{html.escape(k)}</td><td>{html.escape(v)}</td></tr>"
        for k, v in sorted(resp_headers.items())
    )
    headers_section = f"""<details class="ag-details">
  <summary><span>📋 Response HTTP Headers ({len(resp_headers)})</span><span>ETag / Cache-Control / Date</span></summary>
  <table class="ag-tbl"><tbody>{hdr_rows}</tbody></table>
</details>"""

    req_section = ""
    if req_body_preview:
        preview_trim = req_body_preview[:1200] + ("\n... (truncated)" if len(req_body_preview) > 1200 else "")
        if req_headers.get("Content-Type", "").startswith("application/json"):
            req_inner = _highlight_json(preview_trim)
        else:
            req_inner = _highlight_yaml(preview_trim, with_line_numbers=False)
        req_section = f"""<details class="ag-details">
  <summary><span>📤 Request Payload ({len(req_body_preview)} chars)</span><span>{html.escape(req_headers.get("Content-Type", ""))}</span></summary>
  <pre class="ag-pre">{req_inner}</pre>
</details>"""

    widget = f"""{_WIDGET_CSS}
<div class="ag-w">
  <div class="ag-hdr">
    <div class="ag-row">
      <span class="ag-badge {m_class}">{html.escape(method)}</span>
      <span class="ag-url"><span class="ag-url-host">{html.escape(host_part)}</span><span class="ag-url-path">{html.escape(path_part)}</span></span>
    </div>
    <div class="ag-row">
      {''.join(chips)}
      <span class="ag-badge {st_class}">HTTP {status}</span>
    </div>
  </div>
  {''.join(banners)}
  {body_html}
  {headers_section}
  {req_section}
</div>"""
    emit_widget(widget, plain_str)


class AlexandriaLab:
    """Manages an isolated set of alexandria-server, alexandria-client, and fake-connector processes with a shared clock file."""

    def __init__(
        self,
        *,
        space: str = "https://example.com/spaces/demo",
        compiler: str = "dsl",
        initial_clock: str = "2026-09-01T10:00:00Z",
        routes: list[dict[str, Any]] | None = None,
        degraded_policy_intake: str = "continue",
        unknown_agreement: str = "reject",
    ) -> None:
        self.root = repo_root()
        self.bin_dir = ensure_binaries()
        self.work_dir = Path(tempfile.mkdtemp(prefix="alexandria-notebook-"))
        self.space = space
        self.compiler = compiler
        self.clock_file = self.work_dir / "clock"
        self._current_clock: str | None = None
        self.set_clock(initial_clock, silent=True)

        self.hub_port = free_port()
        self.conn_port = free_port()
        self.dsp_ingress_port = free_port()
        self.mgmt_ingress_port = free_port()
        self.dsp_egress_port = free_port()
        self.admin_port = free_port()

        self.hub_url = f"http://127.0.0.1:{self.hub_port}"
        self.conn_url = f"http://127.0.0.1:{self.conn_port}"
        self.dsp_ingress_url = f"http://127.0.0.1:{self.dsp_ingress_port}"
        self.mgmt_ingress_url = f"http://127.0.0.1:{self.mgmt_ingress_port}"
        self.dsp_egress_url = f"http://127.0.0.1:{self.dsp_egress_port}"
        self.admin_url = f"http://127.0.0.1:{self.admin_port}"
        self.hub_did = f"did:web:127.0.0.1%3A{self.hub_port}"

        self.events_file = self.work_dir / "events.jsonl"
        self.routes = routes if routes is not None else self._default_routes()
        self.degraded_policy_intake = degraded_policy_intake
        self.unknown_agreement = unknown_agreement
        self.procs: list[subprocess.Popen[bytes]] = []

    def _default_routes(self) -> list[dict[str, Any]]:
        return [
            {
                "listener": "dsp-ingress",
                "match": {"method": "POST", "path": "/negotiations/request"},
                "message": "ContractRequestMessage",
                "gates": ["dsp_message", "policy_intake"],
                "mode": "enforce",
                "body": {"select": "$.offer"},
            },
            {
                "listener": "dsp-ingress",
                "match": {"method": "POST", "path": "/transfers/request"},
                "message": "TransferRequestMessage",
                "gates": ["dsp_message", "agreement_admission"],
                "mode": "enforce",
            },
            {
                "listener": "dsp-egress",
                "match": {"method": "POST", "path": "/negotiations/:pid/agreement"},
                "message": "ContractAgreementMessage",
                "gates": ["dsp_message", "policy_intake"],
                "mode": "enforce",
                "body": {"select": "$.agreement"},
            },
        ]

    def set_clock(self, rfc3339_instant: str, *, silent: bool = False) -> None:
        """Advances or sets the shared file clock used by alexandria-server and alexandria-client."""
        prev = self._current_clock
        self._current_clock = rfc3339_instant
        tmp = self.clock_file.with_suffix(".tmp")
        tmp.write_text(rfc3339_instant + "\n", encoding="utf-8")
        os.replace(tmp, self.clock_file)
        if not silent:
            plain = f"🕒 Clock updated → {rfc3339_instant}"
            prev_html = f'<span style="color:#64748b;text-decoration:line-through;margin-right:6px;">{html.escape(prev)}</span> → ' if prev else ""
            widget = f"""{_WIDGET_CSS}
<div class="ag-w" style="border-color:#0284c7;">
  <div class="ag-hdr" style="background:rgba(2,132,199,0.15);border-bottom:none;">
    <div class="ag-row">
      <span class="ag-badge ag-m-GET">🕒 Injected Clock (DP-9)</span>
      <span style="font-family:monospace;font-size:12.5px;color:#f8fafc;">{prev_html}<strong style="color:#38bdf8;">{html.escape(rfc3339_instant)}</strong></span>
    </div>
    <span class="ag-chip">clock_file synchronized</span>
  </div>
</div>"""
            emit_widget(widget, plain)

    def start_connector(self) -> None:
        """Starts the fake-connector process."""
        p = subprocess.Popen(
            [str(self.bin_dir / "fake-connector"), "-listen", f"127.0.0.1:{self.conn_port}"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        self.procs.append(p)
        self._wait_url(f"{self.conn_url}/healthz")

    def start_server(self) -> None:
        """Starts the alexandria-server process."""
        srv_dir = self.work_dir / "server"
        srv_dir.mkdir(parents=True, exist_ok=True)
        tokens_path = srv_dir / "tokens.json"
        tokens_path.write_text(
            json.dumps(
                {
                    "editor-token": {"principal": "steward", "roles": ["editor"]},
                    "reviewer-token": {"principal": "reviewer", "roles": ["reviewer"]},
                    "releaser-token": {"principal": "releaser", "roles": ["releaser"]},
                    "member-token": {"principal": "alexandria-client", "roles": ["member"]},
                },
                indent=2,
            ),
            encoding="utf-8",
        )
        srv_cfg_path = srv_dir / "server.json"
        srv_cfg_path.write_text(
            json.dumps(
                {
                    "listen": f"127.0.0.1:{self.hub_port}",
                    "public_base_url": self.hub_url,
                    "did": self.hub_did,
                    "sqlite_path": str(srv_dir / "hub.db"),
                    "blob_dir": str(srv_dir / "blobs"),
                    "signing_key_path": str(srv_dir / "hub.key"),
                    "tokens_path": str(tokens_path),
                    "compiler": self.compiler,
                },
                indent=2,
            ),
            encoding="utf-8",
        )
        p = subprocess.Popen(
            [
                str(self.bin_dir / "alexandria-server"),
                "-config",
                str(srv_cfg_path),
                "-clock-file",
                str(self.clock_file),
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        self.procs.append(p)
        self._wait_url(f"{self.hub_url}/.well-known/did.json")
        plain = f"✔ alexandria-server listening at {self.hub_url} (DID: {self.hub_did})"
        widget = f"""{_WIDGET_CSS}
<div class="ag-w" style="border-color:#10b981;">
  <div class="ag-hdr" style="background:rgba(16,185,129,0.12);border-bottom:none;">
    <div class="ag-row">
      <span class="ag-badge ag-m-POST">alexandria-server (C-HUB)</span>
      <span style="font-family:monospace;color:#f8fafc;">{html.escape(self.hub_url)}</span>
    </div>
    <div class="ag-row">
      <span class="ag-chip">compiler={html.escape(self.compiler)}</span>
      <span class="ag-chip">{html.escape(self.hub_did)}</span>
    </div>
  </div>
</div>"""
        emit_widget(widget, plain)

    def start_client(self) -> None:
        """Writes client-config.json and starts the alexandria-client sidecar process."""
        cred_path = self.work_dir / "credentials.json"
        cred_path.write_text(json.dumps({"token": "member-token"}), encoding="utf-8")
        cfg = {
            "space": self.space,
            "trust": {
                "hub": f"https://127.0.0.1:{self.hub_port}",
                "hub_did": self.hub_did,
                "credentials": f"file:{cred_path}",
            },
            "listeners": {
                "dsp-ingress": {"bind": f"127.0.0.1:{self.dsp_ingress_port}"},
                "mgmt-ingress": {"bind": f"127.0.0.1:{self.mgmt_ingress_port}"},
                "dsp-egress": {"bind": f"127.0.0.1:{self.dsp_egress_port}"},
                "admin": {"bind": f"127.0.0.1:{self.admin_port}"},
            },
            "upstreams": {"dsp": self.conn_url, "management": self.conn_url},
            "routes": self.routes,
            "default_action": "forward",
            "unknown_agreement": self.unknown_agreement,
            "events": {"sink": f"file:{self.events_file}"},
        }
        cfg_path = self.work_dir / "client-config.json"
        cfg_path.write_text(json.dumps(cfg, indent=2), encoding="utf-8")
        p = subprocess.Popen(
            [
                str(self.bin_dir / "alexandria-client"),
                "-config",
                str(cfg_path),
                "-state-dir",
                str(self.work_dir / "client-state"),
                "-clock-file",
                str(self.clock_file),
                "-did.allow_insecure_localhost",
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        self.procs.append(p)
        self._wait_url(f"{self.admin_url}/healthz")
        plain = (
            f"✔ alexandria-client active | dsp-ingress: {self.dsp_ingress_url} | "
            f"dsp-egress: {self.dsp_egress_url} | admin: {self.admin_url}"
        )
        widget = f"""{_WIDGET_CSS}
<div class="ag-w" style="border-color:#38bdf8;">
  <div class="ag-hdr" style="background:rgba(56,189,248,0.12);border-bottom:none;">
    <div class="ag-row">
      <span class="ag-badge ag-m-GET">alexandria-client (C-CLI)</span>
      <span class="ag-chip">dsp-ingress: {html.escape(self.dsp_ingress_url)}</span>
      <span class="ag-chip">dsp-egress: {html.escape(self.dsp_egress_url)}</span>
      <span class="ag-chip">admin: {html.escape(self.admin_url)}</span>
    </div>
  </div>
</div>"""
        emit_widget(widget, plain)

    def start_all(self) -> None:
        """Starts fake-connector, alexandria-server, and alexandria-client."""
        self.start_connector()
        self.start_server()
        self.start_client()

    def wait_synced(self, expected_issued_at: str | None = None, timeout: float = 8.0) -> dict[str, Any]:
        """Polls GET /status on the client admin listener until a listing is accepted."""
        deadline = time.time() + timeout
        last_status: dict[str, Any] = {}
        while time.time() < deadline:
            code, _, data = http_request("GET", f"{self.admin_url}/status", verbose=False)
            if code == 200 and isinstance(data, dict):
                last_status = data
                listing = data.get("last_accepted_listing")
                if listing:
                    if expected_issued_at is None or listing.get("issued_at") == expected_issued_at:
                        return data
            time.sleep(0.15)
        raise TimeoutError(f"alexandria-client did not sync listing={expected_issued_at!r}; last /status={last_status}")

    def read_events(self) -> list[dict[str, Any]]:
        """Reads all JSONL events emitted by alexandria-client so far."""
        if not self.events_file.exists():
            return []
        lines = self.events_file.read_text(encoding="utf-8").strip().splitlines()
        return [json.loads(line) for line in lines if line.strip()]

    def _wait_url(self, url: str, timeout: float = 8.0) -> None:
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                code, _, _ = http_request("GET", url, verbose=False)
                if code in (200, 401, 404, 503):
                    return
            except Exception:
                pass
            time.sleep(0.05)
        raise TimeoutError(f"Service at {url} did not become ready within {timeout}s")

    def close(self) -> None:
        """Stops all started processes and removes the temporary directory."""
        for p in reversed(self.procs):
            if p.poll() is None:
                p.terminate()
                try:
                    p.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    p.kill()
        self.procs.clear()
        shutil.rmtree(self.work_dir, ignore_errors=True)


# Backwards-compatible alias.
VHubLab = AlexandriaLab
