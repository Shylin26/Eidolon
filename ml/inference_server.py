import json
import os
import signal
import socket
import sys
import threading

from mlx_lm import load, stream_generate
from mlx_lm.sample_utils import make_sampler

MODEL_PATH = os.path.expanduser("~/eidolon/models/eidolon-base")
SOCKET_PATH = "/tmp/eidolon.sock"

print(f"[eidolon] loading model from {MODEL_PATH}...", flush=True)
model, tokenizer = load(MODEL_PATH)
print("[eidolon] model ready", flush=True)

def handle_request(conn):
    try:
        data = b""
        while b"\r\n\r\n" not in data:
            data += conn.recv(4096)

        header_part, body_start = data.split(b"\r\n\r\n", 1)
        headers = header_part.decode()

        content_length = 0
        for line in headers.split("\r\n"):
            if line.lower().startswith("content-length:"):
                content_length = int(line.split(":")[1].strip())

        body = body_start
        while len(body) < content_length:
            body += conn.recv(4096)

        req = json.loads(body)

        if req.get("path") == "/health" or "GET" in headers.split("\r\n")[0]:
            response_body = json.dumps({"status": "ready"})
            conn.sendall(
                f"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {len(response_body)}\r\nConnection: close\r\n\r\n{response_body}".encode()
            )
            return

        prefix = req.get("prefix", "")
        suffix = req.get("suffix", "")
        request_id = req.get("request_id", "unknown")
        max_tokens = req.get("max_tokens", 256)
        temperature = req.get("temperature", 0.2)
        top_p = req.get("top_p", 0.95)

        if suffix:
            prompt = f"<PRE> {prefix} <SUF> {suffix} <MID>"
        else:
            prompt = prefix

        sampler = make_sampler(temp=temperature, top_p=top_p)

        conn.sendall(
            b"HTTP/1.1 200 OK\r\n"
            b"Content-Type: application/x-ndjson\r\n"
            b"Transfer-Encoding: chunked\r\n"
            b"Connection: close\r\n"
            b"\r\n"
        )

        for response in stream_generate(
            model,
            tokenizer,
            prompt=prompt,
            max_tokens=max_tokens,
            sampler=sampler,
        ):
            done = response.finish_reason is not None
            chunk = json.dumps({
                "request_id": request_id,
                "token": response.text,
                "done": done,
            }) + "\n"
            chunk_bytes = chunk.encode()
            conn.sendall(f"{len(chunk_bytes):x}\r\n".encode() + chunk_bytes + b"\r\n")
            if done:
                break

        conn.sendall(b"0\r\n\r\n")

    except Exception as e:
        print(f"[eidolon] error: {e}", flush=True)
    finally:
        conn.close()

def serve():
    if os.path.exists(SOCKET_PATH):
        os.unlink(SOCKET_PATH)

    server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    server.bind(SOCKET_PATH)
    server.listen(5)
    os.chmod(SOCKET_PATH, 0o666)

    print(f"[eidolon] listening on {SOCKET_PATH}", flush=True)

    def shutdown(sig, frame):
        print("\n[eidolon] shutting down...", flush=True)
        server.close()
        if os.path.exists(SOCKET_PATH):
            os.unlink(SOCKET_PATH)
        sys.exit(0)

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    while True:
        conn, _ = server.accept()
        threading.Thread(target=handle_request, args=(conn,), daemon=True).start()

if __name__ == "__main__":
    serve()
