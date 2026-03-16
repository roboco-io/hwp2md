#!/usr/bin/env python3
"""md2hwp-ui server — HWPX template viewer with real-time fill preview.

Usage:
    python3 server.py [--port 8080]

Browse to http://localhost:8080 after starting.
"""

import argparse
import json
import os
import re
import shutil
import tempfile
import time
import threading
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from urllib.parse import urlparse, parse_qs

# Add renderer to path
import sys
sys.path.insert(0, str(Path(__file__).parent))
from renderer import render_hwpx_to_html

# Session state
STATE = {
    "upload_dir": None,
    "template_path": None,
    "template_html": None,
    "text_count": 0,
    "output_path": None,
    "event_file": None,
}

EVENT_FILE_PATH = "/tmp/md2hwp-events.jsonl"


def _parse_multipart(body: bytes, boundary: bytes) -> tuple:
    """Parse multipart form data, return (filename, file_bytes) or (None, None)."""
    delimiter = b"--" + boundary
    parts = body.split(delimiter)

    for part in parts:
        if b"Content-Disposition" not in part:
            continue
        # Split headers from body at double newline
        header_end = part.find(b"\r\n\r\n")
        if header_end == -1:
            continue
        headers_raw = part[:header_end].decode("utf-8", errors="replace")
        file_body = part[header_end + 4:]
        # Remove trailing \r\n-- if present
        if file_body.endswith(b"\r\n"):
            file_body = file_body[:-2]

        # Extract filename from Content-Disposition
        fn_match = re.search(r'filename="([^"]+)"', headers_raw)
        if fn_match:
            return fn_match.group(1), file_body

    return None, None


def init_session():
    """Initialize temp directory for uploads."""
    if STATE["upload_dir"] and os.path.exists(STATE["upload_dir"]):
        shutil.rmtree(STATE["upload_dir"])
    STATE["upload_dir"] = tempfile.mkdtemp(prefix="md2hwp-ui-")
    STATE["event_file"] = EVENT_FILE_PATH
    # Clear event file
    with open(EVENT_FILE_PATH, "w") as f:
        f.write("")


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass  # Suppress default logging

    def do_GET(self):
        path = urlparse(self.path).path

        if path == "/":
            self._serve_html()
        elif path == "/api/events":
            self._serve_sse()
        elif path.startswith("/api/download/"):
            self._serve_download()
        else:
            self._respond(404, "Not found")

    def do_POST(self):
        path = urlparse(self.path).path

        if path == "/api/upload":
            self._handle_upload()
        elif path == "/api/fill":
            self._handle_fill()
        else:
            self._respond(404, "Not found")

    # --- Handlers ---

    def _serve_html(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(HTML_PAGE.encode("utf-8"))

    def _serve_sse(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()

        event_file = EVENT_FILE_PATH
        last_pos = 0

        # Start from end of file
        if os.path.exists(event_file):
            last_pos = os.path.getsize(event_file)

        try:
            while True:
                if os.path.exists(event_file):
                    size = os.path.getsize(event_file)
                    if size > last_pos:
                        with open(event_file, "r", encoding="utf-8") as f:
                            f.seek(last_pos)
                            new_lines = f.read()
                            last_pos = f.tell()

                        for line in new_lines.strip().split("\n"):
                            if line.strip():
                                self.wfile.write(f"data: {line}\n\n".encode())
                                self.wfile.flush()

                # Heartbeat
                self.wfile.write(b": heartbeat\n\n")
                self.wfile.flush()
                time.sleep(0.3)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def _handle_upload(self):
        content_type = self.headers.get("Content-Type", "")
        if "multipart/form-data" not in content_type:
            self._respond_json(400, {"error": "multipart/form-data required"})
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        # Extract boundary from Content-Type
        boundary_match = re.search(r"boundary=(.+)", content_type)
        if not boundary_match:
            self._respond_json(400, {"error": "No boundary in Content-Type"})
            return
        boundary = boundary_match.group(1).strip().encode()

        # Parse multipart: split by boundary, find file part
        filename, file_data = _parse_multipart(body, boundary)
        if not filename or file_data is None:
            self._respond_json(400, {"error": "No file uploaded"})
            return

        init_session()

        # Save uploaded file
        filename = os.path.basename(filename)
        save_path = os.path.join(STATE["upload_dir"], filename)
        with open(save_path, "wb") as f:
            f.write(file_data)

        STATE["template_path"] = save_path

        # Render to HTML
        try:
            html, count = render_hwpx_to_html(save_path)
            STATE["template_html"] = html
            STATE["text_count"] = count
            self._respond_json(200, {
                "html": html,
                "text_count": count,
                "filename": filename,
                "event_file": EVENT_FILE_PATH,
            })
        except (BrokenPipeError, ConnectionResetError):
            pass
        except Exception as e:
            try:
                self._respond_json(500, {"error": str(e)})
            except (BrokenPipeError, ConnectionResetError):
                pass

    def _handle_fill(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        try:
            plan = json.loads(body)
        except json.JSONDecodeError:
            self._respond_json(400, {"error": "Invalid JSON"})
            return

        if not STATE["template_path"]:
            self._respond_json(400, {"error": "No template uploaded"})
            return

        # Set template and output in plan
        output_name = "result_" + os.path.basename(STATE["template_path"])
        output_path = os.path.join(STATE["upload_dir"], output_name)
        plan["template_file"] = STATE["template_path"]
        plan["output_file"] = output_path

        # Save plan
        plan_path = os.path.join(STATE["upload_dir"], "fill_plan.json")
        with open(plan_path, "w", encoding="utf-8") as f:
            json.dump(plan, f, ensure_ascii=False, indent=2)

        # Run fill_hwpx.py in background thread
        def run_fill():
            fill_script = str(Path.home() / ".claude/skills/md2hwp/scripts/fill_hwpx.py")
            env = os.environ.copy()
            env["MD2HWP_EVENT_FILE"] = EVENT_FILE_PATH
            import subprocess
            result = subprocess.run(
                [sys.executable, fill_script, plan_path],
                env=env, capture_output=True, text=True,
            )
            # Write done event
            done_event = {"type": "done", "output": output_name, "log": result.stdout + result.stderr}
            with open(EVENT_FILE_PATH, "a", encoding="utf-8") as f:
                f.write(json.dumps(done_event, ensure_ascii=False) + "\n")
            STATE["output_path"] = output_path

        threading.Thread(target=run_fill, daemon=True).start()
        self._respond_json(200, {"status": "started", "plan_path": plan_path})

    def _serve_download(self):
        filename = urlparse(self.path).path.split("/api/download/", 1)[-1]
        filepath = os.path.join(STATE["upload_dir"] or "", filename)

        if not os.path.exists(filepath):
            self._respond(404, "File not found")
            return

        self.send_response(200)
        self.send_header("Content-Type", "application/octet-stream")
        self.send_header("Content-Disposition", f'attachment; filename="{filename}"')
        self.send_header("Content-Length", str(os.path.getsize(filepath)))
        self.end_headers()
        with open(filepath, "rb") as f:
            shutil.copyfileobj(f, self.wfile)

    # --- Helpers ---

    def _respond(self, code, text):
        self.send_response(code)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.end_headers()
        self.wfile.write(text.encode("utf-8"))

    def _respond_json(self, code, data):
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.end_headers()
        self.wfile.write(json.dumps(data, ensure_ascii=False).encode("utf-8"))


# ===== Inline HTML/CSS/JS =====

HTML_PAGE = """<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>md2hwp Viewer</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: 'Malgun Gothic', 'Apple SD Gothic Neo', sans-serif; background: #f5f5f5; color: #333; }

  .header { background: #1a1a2e; color: white; padding: 16px 24px; display: flex; justify-content: space-between; align-items: center; }
  .header h1 { font-size: 18px; font-weight: 600; }
  .header .actions { display: flex; gap: 8px; }

  .toolbar { background: white; border-bottom: 1px solid #ddd; padding: 12px 24px; display: flex; gap: 12px; align-items: center; }

  .btn { padding: 8px 16px; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; font-weight: 500; transition: all 0.2s; }
  .btn-primary { background: #4361ee; color: white; }
  .btn-primary:hover { background: #3a56d4; }
  .btn-primary:disabled { background: #aaa; cursor: not-allowed; }
  .btn-secondary { background: #e9ecef; color: #495057; }
  .btn-secondary:hover { background: #dee2e6; }
  .btn-download { background: #2d6a4f; color: white; }
  .btn-download:hover { background: #245a3f; }
  .btn-download:disabled { background: #aaa; cursor: not-allowed; }

  .upload-area { border: 2px dashed #ccc; border-radius: 8px; padding: 24px; text-align: center; background: #fafafa; cursor: pointer; transition: border-color 0.2s; }
  .upload-area:hover { border-color: #4361ee; }
  .upload-area.dragover { border-color: #4361ee; background: #eef; }
  .upload-area input { display: none; }

  .viewer { padding: 24px; max-width: 900px; margin: 0 auto; }
  .viewer-empty { text-align: center; color: #999; padding: 80px 0; }

  /* HWPX table styles */
  .hwpx-table { width: 100%; border-collapse: collapse; margin: 12px 0; font-size: 13px; }
  .hwpx-table td, .hwpx-table th { border: 1px solid #999; padding: 6px 8px; vertical-align: top; }
  .hwpx-table th { background: #f0f0f0; font-weight: 600; }
  .hwpx-p { margin: 2px 0; line-height: 1.5; }

  /* Highlight animation */
  .hwpx-t.highlight {
    background: #ffe066;
    outline: 2px solid #f59f00;
    border-radius: 2px;
    padding: 0 2px;
  }
  .hwpx-t.highlight-settle {
    background: #fff9db;
    outline: none;
    transition: background 4s ease-out;
    border-radius: 2px;
    padding: 0 2px;
  }

  /* Status bar */
  .statusbar { background: #1a1a2e; color: #aaa; padding: 8px 24px; font-size: 12px; display: flex; justify-content: space-between; position: fixed; bottom: 0; left: 0; right: 0; }
  .statusbar .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 6px; }
  .dot-connected { background: #2d6a4f; }
  .dot-disconnected { background: #e63946; }
  .dot-idle { background: #666; }

  /* Log panel */
  .log-panel { background: #1e1e2e; color: #ccc; padding: 12px 24px; font-family: monospace; font-size: 12px; max-height: 150px; overflow-y: auto; display: none; }
  .log-panel.visible { display: block; }
  .log-entry { margin: 2px 0; }
  .log-entry .find { color: #e63946; text-decoration: line-through; }
  .log-entry .replace { color: #2d6a4f; font-weight: bold; }
</style>
</head>
<body>

<div class="header">
  <h1>md2hwp Viewer</h1>
  <div class="actions">
    <button class="btn btn-secondary" id="btnLog" onclick="toggleLog()">Log</button>
    <button class="btn btn-download" id="btnDownload" disabled onclick="downloadResult()">Download .hwpx</button>
  </div>
</div>

<div class="toolbar">
  <div class="upload-area" id="uploadArea">
    <input type="file" id="fileInput" accept=".hwpx">
    <span id="uploadLabel">HWPX 파일을 드래그하거나 클릭하여 업로드</span>
  </div>
</div>

<div class="log-panel" id="logPanel"></div>

<div class="viewer" id="viewer">
  <div class="viewer-empty">HWPX 파일을 업로드하면 미리보기가 표시됩니다</div>
</div>

<div class="statusbar">
  <span><span class="dot dot-idle" id="statusDot"></span><span id="statusText">대기 중</span></span>
  <span id="statusCount"></span>
</div>

<script>
const viewer = document.getElementById('viewer');
const fileInput = document.getElementById('fileInput');
const uploadArea = document.getElementById('uploadArea');
const uploadLabel = document.getElementById('uploadLabel');
const btnDownload = document.getElementById('btnDownload');
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');
const statusCount = document.getElementById('statusCount');
const logPanel = document.getElementById('logPanel');

let eventSource = null;
let totalReplaced = 0;
let outputFilename = null;

// Upload area interactions
uploadArea.addEventListener('click', () => fileInput.click());
uploadArea.addEventListener('dragover', (e) => { e.preventDefault(); uploadArea.classList.add('dragover'); });
uploadArea.addEventListener('dragleave', () => uploadArea.classList.remove('dragover'));
uploadArea.addEventListener('drop', (e) => {
  e.preventDefault();
  uploadArea.classList.remove('dragover');
  if (e.dataTransfer.files.length > 0) uploadFile(e.dataTransfer.files[0]);
});
fileInput.addEventListener('change', () => { if (fileInput.files.length > 0) uploadFile(fileInput.files[0]); });

function uploadFile(file) {
  uploadLabel.textContent = file.name + ' 업로드 중...';
  const formData = new FormData();
  formData.append('file', file);

  fetch('/api/upload', { method: 'POST', body: formData })
    .then(r => r.json())
    .then(data => {
      if (data.error) { alert(data.error); return; }
      viewer.innerHTML = data.html;
      uploadLabel.textContent = file.name + ' (' + data.text_count + ' text elements)';
      statusText.textContent = '템플릿 로드 완료';
      totalReplaced = 0;
      statusCount.textContent = '';
      btnDownload.disabled = true;
      outputFilename = null;
      connectSSE();
    })
    .catch(err => { alert('Upload failed: ' + err); uploadLabel.textContent = 'HWPX 파일을 드래그하거나 클릭하여 업로드'; });
}

function connectSSE() {
  if (eventSource) eventSource.close();

  eventSource = new EventSource('/api/events');
  statusDot.className = 'dot dot-connected';
  statusText.textContent = '연결됨 - 대기 중';

  eventSource.onmessage = (e) => {
    const event = JSON.parse(e.data);

    if (event.type === 'replace') {
      totalReplaced++;
      const el = document.querySelector('[data-idx="' + event.idx + '"]');
      if (el) {
        el.textContent = event.replace;
        el.classList.remove('highlight-settle');
        el.classList.add('highlight');
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
        setTimeout(() => { el.classList.remove('highlight'); el.classList.add('highlight-settle'); }, 2000);
        setTimeout(() => el.classList.remove('highlight-settle'), 6000);
      }
      statusText.textContent = '편집 중';
      statusCount.textContent = totalReplaced + '건 교체';
      addLog(event);
    }

    if (event.type === 'done') {
      outputFilename = event.output;
      btnDownload.disabled = false;
      statusText.textContent = '완료';
      statusDot.className = 'dot dot-connected';
      addLog({ type: 'done', msg: '완료 - ' + totalReplaced + '건 교체' });
    }
  };

  eventSource.onerror = () => {
    statusDot.className = 'dot dot-disconnected';
    statusText.textContent = '연결 끊김 - 재연결 중';
  };
}

function downloadResult() {
  if (outputFilename) {
    window.location.href = '/api/download/' + encodeURIComponent(outputFilename);
  }
}

function toggleLog() {
  logPanel.classList.toggle('visible');
}

function addLog(event) {
  const div = document.createElement('div');
  div.className = 'log-entry';
  if (event.type === 'replace') {
    div.innerHTML = '[' + event.idx + '] <span class="find">' + escapeHtml(event.find || '') + '</span> → <span class="replace">' + escapeHtml(event.replace || '') + '</span>';
  } else {
    div.textContent = event.msg || JSON.stringify(event);
  }
  logPanel.appendChild(div);
  logPanel.scrollTop = logPanel.scrollHeight;
}

function escapeHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
</script>

</body>
</html>
"""


def main():
    parser = argparse.ArgumentParser(description="md2hwp Viewer Server")
    parser.add_argument("--port", type=int, default=8080, help="Port (default: 8080)")
    args = parser.parse_args()

    init_session()

    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.daemon_threads = True
    print(f"md2hwp Viewer running at http://localhost:{args.port}")
    print(f"Event file: {EVENT_FILE_PATH}")
    print("Press Ctrl+C to stop")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping...")
        server.server_close()
        if STATE["upload_dir"] and os.path.exists(STATE["upload_dir"]):
            shutil.rmtree(STATE["upload_dir"])


if __name__ == "__main__":
    main()
