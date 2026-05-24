import json
import os
import signal
import socket
import sys
import threading
import numpy as np

# Use a lightweight sentence transformer for embeddings
from sentence_transformers import SentenceTransformer

MODEL_NAME = "all-MiniLM-L6-v2"
SOCKET_PATH = "/tmp/eidolon-embed.sock"

print(f"[eidolon-embed] loading {MODEL_NAME}...", flush=True)
model = SentenceTransformer(MODEL_NAME)
print("[eidolon-embed] embedding model ready", flush=True)

def handle(conn):
    try:
        data = b""
        while True:
            chunk = conn.recv(4096)
            if not chunk:
                break
            data += chunk
            try:
                req = json.loads(data.decode())
                break
            except:
                continue

        req = json.loads(data.decode())
        texts = req.get("texts", [])

        if not texts:
            conn.sendall(json.dumps({"embeddings": []}).encode())
            return

        embeddings = model.encode(texts, normalize_embeddings=True)
        result = {"embeddings": embeddings.tolist()}
        conn.sendall(json.dumps(result).encode())
    except Exception as e:
        print(f"[eidolon-embed] error: {e}", flush=True)
        try:
            conn.sendall(json.dumps({"error": str(e)}).encode())
        except:
            pass
    finally:
        conn.close()

def serve():
    if os.path.exists(SOCKET_PATH):
        os.unlink(SOCKET_PATH)

    server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    server.bind(SOCKET_PATH)
    server.listen(5)
    os.chmod(SOCKET_PATH, 0o666)

    def shutdown(sig, frame):
        print("\n[eidolon-embed] shutting down...", flush=True)
        server.close()
        if os.path.exists(SOCKET_PATH):
            os.unlink(SOCKET_PATH)
        sys.exit(0)

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    print(f"[eidolon-embed] listening on {SOCKET_PATH}", flush=True)
    while True:
        conn, _ = server.accept()
        threading.Thread(target=handle, args=(conn,), daemon=True).start()

if __name__ == "__main__":
    serve()
