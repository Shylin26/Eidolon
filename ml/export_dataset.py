#!/usr/bin/env python3
"""
Export accepted completions from SQLite as FIM training data.
Run automatically by the retraining trigger.
"""
import json
import os
import sqlite3
import sys

DB_PATH = os.path.expanduser("~/eidolon/data/eidolon.db")
OUTPUT_PATH = os.path.expanduser("~/eidolon/dataset/feedback_train.jsonl")

def export():
    os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    cursor.execute("""
        SELECT c.completion_text, c.file_path, c.language_id
        FROM completions c
        WHERE c.accepted = 1
          AND c.completion_text IS NOT NULL
          AND length(c.completion_text) > 10
        ORDER BY c.created_at DESC
        LIMIT 10000
    """)

    rows = cursor.fetchall()
    conn.close()

    if not rows:
        print("[export] no accepted completions found", flush=True)
        return 0

    written = 0
    with open(OUTPUT_PATH, "w") as f:
        for completion_text, file_path, language_id in rows:
            # create FIM training example
            example = {
                "text": f"<PRE> <SUF> <MID>{completion_text}"
            }
            f.write(json.dumps(example) + "\n")
            written += 1

    print(f"[export] wrote {written} examples to {OUTPUT_PATH}", flush=True)
    return written

if __name__ == "__main__":
    count = export()
    sys.exit(0 if count > 0 else 1)
