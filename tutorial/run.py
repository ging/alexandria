#!/usr/bin/env python3
# CLI runner to execute and verify all tutorial Jupyter notebooks or launch JupyterLab.
# Uses only the Python 3 standard library for headless execution and auto-bootstraps .venv for JupyterLab.
from __future__ import annotations

import argparse
import contextlib
import io
import json
import subprocess
import sys
import traceback
from pathlib import Path
from typing import Any

TUTORIAL_DIR = Path(__file__).resolve().parent
REPO_ROOT = TUTORIAL_DIR.parent
sys.path.insert(0, str(TUTORIAL_DIR))

from lab import ensure_binaries, set_html_capture  # noqa: E402


def _split_ipynb_lines(text: str) -> list[str]:
    """Splits a string into .ipynb line array format (newlines preserved except on the last line)."""
    lines = [line + "\n" for line in text.splitlines()]
    if lines and not text.endswith("\n"):
        lines[-1] = lines[-1].rstrip("\n")
    return lines


def execute_notebook(nb_path: Path, verbose: bool = False) -> int:
    """Executes all code cells in nb_path in order, updating stdout and HTML display_data outputs in place."""
    nb = json.loads(nb_path.read_text(encoding="utf-8"))
    cells: list[dict[str, Any]] = nb.get("cells", [])
    ns: dict[str, Any] = {"__name__": "__main__"}
    exec_count = 0

    print(f"▶ Running {nb_path.name} ...")
    for cell in cells:
        if cell.get("cell_type") != "code":
            continue
        exec_count += 1
        code_str = "".join(cell.get("source", []))
        buf_out = io.StringIO()
        buf_err = io.StringIO()
        html_widgets: list[tuple[str, str]] = []
        set_html_capture(html_widgets)
        try:
            with contextlib.redirect_stdout(buf_out), contextlib.redirect_stderr(buf_err):
                exec(compile(code_str, f"{nb_path.name}#cell-{exec_count}", "exec"), ns)
        except Exception as exc:
            set_html_capture(None)
            if "lab" in ns and hasattr(ns["lab"], "close"):
                ns["lab"].close()
            print(f"\n✖ Error in {nb_path.name} (code cell #{exec_count}):")
            print(code_str)
            if buf_out.getvalue():
                print("--- STDOUT ---\n" + buf_out.getvalue())
            if buf_err.getvalue():
                print("--- STDERR ---\n" + buf_err.getvalue())
            traceback.print_exc()
            raise RuntimeError(f"Failed executing {nb_path.name}") from exc
        finally:
            set_html_capture(None)

        cell["execution_count"] = exec_count
        outputs: list[dict[str, Any]] = []
        out_val = buf_out.getvalue()
        err_val = buf_err.getvalue()
        if verbose and out_val:
            print(out_val, end="" if out_val.endswith("\n") else "\n")
        if out_val:
            outputs.append({"name": "stdout", "output_type": "stream", "text": _split_ipynb_lines(out_val)})
        if err_val:
            outputs.append({"name": "stderr", "output_type": "stream", "text": _split_ipynb_lines(err_val)})
        for widget_html, plain_fallback in html_widgets:
            if verbose and plain_fallback:
                print(plain_fallback)
            outputs.append(
                {
                    "data": {
                        "text/html": _split_ipynb_lines(widget_html),
                        "text/plain": _split_ipynb_lines(plain_fallback or "<HTML Widget>"),
                    },
                    "metadata": {},
                    "output_type": "display_data",
                }
            )
        cell["outputs"] = outputs

    nb_path.write_text(json.dumps(nb, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"✔ {nb_path.name} completed ({exec_count} code cells executed)\n")
    return exec_count


def launch_jupyter(port: int) -> None:
    """Bootstraps tutorial/.venv if needed and launches JupyterLab."""
    ensure_binaries()
    venv_dir = TUTORIAL_DIR / ".venv"
    jupyter_bin = venv_dir / "bin" / "jupyter"
    pip_bin = venv_dir / "bin" / "pip"

    if not jupyter_bin.exists():
        print(f"Creating virtual environment in {venv_dir} and installing JupyterLab...")
        subprocess.run([sys.executable, "-m", "venv", str(venv_dir)], check=True)
        subprocess.run([str(pip_bin), "install", "--upgrade", "pip"], check=True)
        subprocess.run([str(pip_bin), "install", "-r", str(TUTORIAL_DIR / "requirements.txt")], check=True)

    default_nb = "01_dsl_and_alexandriac_compiler.ipynb"
    cmd = [
        str(jupyter_bin),
        "lab",
        f"--port={port}",
        f"--ServerApp.root_dir={TUTORIAL_DIR}",
        f"--ServerApp.default_url=/lab/tree/{default_nb}",
    ]
    print(f"Starting JupyterLab at http://127.0.0.1:{port}/lab/tree/{default_nb}")
    subprocess.run(cmd, cwd=str(TUTORIAL_DIR), check=True)


def main() -> None:
    parser = argparse.ArgumentParser(description="Run or open the Alexandria interactive Jupyter notebooks.")
    parser.add_argument(
        "mode",
        nargs="?",
        default="lab",
        choices=["lab", "exec"],
        help="'lab' opens JupyterLab in the browser (default); 'exec' executes and verifies all notebooks headlessly.",
    )
    parser.add_argument("-f", "--filter", default="", help="Filter notebooks to execute by substring (e.g. '04').")
    parser.add_argument("-v", "--verbose", action="store_true", help="Print full HTTP request/response traces to stdout.")
    parser.add_argument("-p", "--port", type=int, default=8888, help="Port for JupyterLab (default: 8888).")
    args = parser.parse_args()

    if args.mode == "lab":
        launch_jupyter(args.port)
        return

    ensure_binaries()
    notebooks = sorted(TUTORIAL_DIR.glob("*.ipynb"))
    if args.filter:
        notebooks = [nb for nb in notebooks if args.filter in nb.name]
    if not notebooks:
        print(f"No notebooks matched filter '{args.filter}'.", file=sys.stderr)
        sys.exit(1)

    total_cells = 0
    for nb_path in notebooks:
        total_cells += execute_notebook(nb_path, verbose=args.verbose)
    print(f"✔ All notebooks ({len(notebooks)}) and {total_cells} code cells executed successfully.")


if __name__ == "__main__":
    main()
