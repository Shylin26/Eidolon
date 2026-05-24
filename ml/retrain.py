#!/usr/bin/env python3
"""
LoRA fine-tuning trigger. Called by the Go retraining worker.
"""
import json
import os
import subprocess
import sys
import time

MODEL_PATH   = os.path.expanduser("~/eidolon/models/eidolon-base")
DATASET_PATH = os.path.expanduser("~/eidolon/dataset")
ADAPTER_BASE = os.path.expanduser("~/eidolon/adapters")
STATUS_FILE  = os.path.expanduser("~/eidolon/data/training_status.json")

def write_status(status, message, adapter_path=None):
    data = {
        "status": status,
        "message": message,
        "timestamp": time.time(),
        "adapter_path": adapter_path,
    }
    with open(STATUS_FILE, "w") as f:
        json.dump(data, f)
    print(f"[retrain] {status}: {message}", flush=True)

def main():
    # export dataset first
    print("[retrain] exporting dataset...", flush=True)
    result = subprocess.run(
        [sys.executable, os.path.expanduser("~/eidolon/ml/export_dataset.py")],
        capture_output=True, text=True
    )
    print(result.stdout, flush=True)
    if result.returncode != 0:
        write_status("failed", "dataset export failed — not enough accepted completions")
        return 1

    # create adapter output dir
    adapter_name = f"eidolon-v{int(time.time())}"
    adapter_path = os.path.join(ADAPTER_BASE, adapter_name)
    os.makedirs(adapter_path, exist_ok=True)

    write_status("training", f"starting LoRA fine-tune -> {adapter_name}")

    # run mlx_lm lora training
    cmd = [
        sys.executable, "-m", "mlx_lm.lora",
        "--model", MODEL_PATH,
        "--train",
        "--data", DATASET_PATH,
        "--iters", "500",
        "--save-every", "100",
        "--adapter-path", adapter_path,
        "--batch-size", "2",
        "--learning-rate", "2e-5",
    ]

    print(f"[retrain] running: {' '.join(cmd)}", flush=True)
    result = subprocess.run(cmd, capture_output=False, text=True)

    if result.returncode != 0:
        write_status("failed", "mlx_lm.lora training failed")
        return 1

    write_status("complete", f"adapter saved to {adapter_path}", adapter_path)
    print(f"[retrain] done. adapter: {adapter_path}", flush=True)
    return 0

if __name__ == "__main__":
    sys.exit(main())
