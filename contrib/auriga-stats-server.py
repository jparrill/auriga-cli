#!/usr/bin/env python3
"""Tiny HTTP server exposing auriga-stats.sh output on port 9100."""

import subprocess
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
import os

STATS_SCRIPT = os.path.expanduser("~/bin/auriga-stats.sh")
PORT = int(os.environ.get("AURIGA_STATS_PORT", "9100"))


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/stats":
            self.send_error(404)
            return
        try:
            result = subprocess.run(
                [STATS_SCRIPT], capture_output=True, text=True, timeout=10
            )
            data = result.stdout.strip()
            json.loads(data)  # validate JSON
        except Exception as e:
            self.send_response(500)
            self.end_headers()
            self.wfile.write(f'{{"error": "{e}"}}'.encode())
            return

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(data.encode())

    def log_message(self, format, *args):
        pass  # silence access logs


if __name__ == "__main__":
    server = HTTPServer(("0.0.0.0", PORT), Handler)
    print(f"Serving stats on :{PORT}/stats")
    server.serve_forever()
