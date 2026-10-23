#!/usr/bin/env python3
"""Black-box smoke test for any level. Same checks for every level and both databases.

Usage: python3 scripts/smoke.py http://localhost:8081

Reads are polled for a few seconds so eventually consistent levels (CQRS, modular monolith) pass too.
"""
import json
import sys
import time
import uuid
import urllib.error
import urllib.request

BASE = sys.argv[1].rstrip("/") if len(sys.argv) > 1 else "http://localhost:8080"
failures = []


def call(method, path, body=None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            raw = resp.read()
            return resp.status, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, (json.loads(raw) if raw else None)
        except ValueError:
            return e.code, raw.decode(errors="replace")


def check(name, cond, detail=""):
    print(("  ok   " if cond else "  FAIL ") + name + ("" if cond else f"  -> {detail}"))
    if not cond:
        failures.append(name)


def poll(fn, want, timeout=5.0):
    """Repeat fn() until want(result) is true or timeout; return the last result."""
    deadline = time.time() + timeout
    while True:
        result = fn()
        if want(result) or time.time() > deadline:
            return result
        time.sleep(0.2)


def has_error(body):
    return isinstance(body, dict) and isinstance(body.get("error"), str) and body["error"] != ""


def actions(todo_id):
    status, body = call("GET", f"/todos/{todo_id}/activities")
    if status != 200 or not isinstance(body, dict):
        return status, None
    return status, [a.get("action") for a in body.get("items", [])]


print(f"smoke test against {BASE}")

status, body = call("GET", "/health")
check("GET /health -> 200", status == 200, (status, body))

status, body = call("POST", "/todos", {"title": "   "})
check("POST blank title -> 400 + error", status == 400 and has_error(body), (status, body))

status, body = call("POST", "/todos", {"title": "x" * 201})
check("POST 201-char title -> 400", status == 400 and has_error(body), (status, body))

title = f"smoke {uuid.uuid4().hex[:8]}"
status, todo = call("POST", "/todos", {"title": f"  {title}  "})
check("POST /todos -> 201", status == 201, (status, todo))
if status != 201:
    print("cannot continue without a created todo")
    sys.exit(1)
tid = todo.get("id")
check("created todo has trimmed title", todo.get("title") == title, todo)
check("created todo is not completed", todo.get("completed") is False, todo)
check("created todo has timestamps", bool(todo.get("created_at")) and bool(todo.get("updated_at")), todo)

status, got = poll(lambda: call("GET", f"/todos/{tid}"), lambda r: r[0] == 200)
check("GET /todos/{id} -> 200", status == 200 and got.get("id") == tid, (status, got))

status, page = poll(lambda: call("GET", "/todos?limit=100"),
                    lambda r: r[0] == 200 and any(t.get("id") == tid for t in r[1].get("items", [])))
check("GET /todos lists the todo", status == 200 and any(t.get("id") == tid for t in page.get("items", [])), (status, page))
check("GET /todos echoes limit/offset", page.get("limit") == 100 and page.get("offset") == 0, page)

status, page = call("GET", "/todos?limit=1")
check("GET /todos?limit=1 returns <= 1 item", status == 200 and len(page.get("items", [])) <= 1, (status, page))
for bad in ("limit=0", "limit=101", "limit=abc", "offset=-1"):
    status, body = call("GET", f"/todos?{bad}")
    check(f"GET /todos?{bad} -> 400", status == 400 and has_error(body), (status, body))

status, body = call("PATCH", f"/todos/{tid}", {})
check("PATCH empty body -> 400", status == 400 and has_error(body), (status, body))

status, upd = call("PATCH", f"/todos/{tid}", {"title": title + " v2", "completed": True})
check("PATCH title+completed -> 200", status == 200, (status, upd))
check("PATCH applied both fields", upd.get("title") == title + " v2" and upd.get("completed") is True, upd)

status, upd = call("PATCH", f"/todos/{tid}", {"completed": False})
check("PATCH reopen -> 200", status == 200 and upd.get("completed") is False, (status, upd))

want = ["created", "renamed", "completed", "reopened"]
status, acts = poll(lambda: actions(tid), lambda r: r[1] == want)
check("activities = created, renamed, completed, reopened", acts == want, (status, acts))

status, body = call("DELETE", f"/todos/{tid}")
check("DELETE /todos/{id} -> 204", status == 204, (status, body))

status, body = poll(lambda: call("GET", f"/todos/{tid}"), lambda r: r[0] == 404)
check("GET deleted todo -> 404 + error", status == 404 and has_error(body), (status, body))

status, body = call("DELETE", f"/todos/{tid}")
check("DELETE again -> 404", status == 404 and has_error(body), (status, body))

status, acts = poll(lambda: actions(tid), lambda r: r[1] == want + ["deleted"])
check("activities end with deleted", acts == want + ["deleted"], (status, acts))

missing = str(uuid.uuid4())
status, body = call("GET", f"/todos/{missing}")
check("GET unknown id -> 404", status == 404 and has_error(body), (status, body))
status, body = call("GET", "/todos/not-a-uuid")
check("GET malformed id -> 404", status == 404 and has_error(body), (status, body))
status, body = call("PATCH", f"/todos/{missing}", {"completed": True})
check("PATCH unknown id -> 404", status == 404 and has_error(body), (status, body))

print(f"\n{'PASS' if not failures else 'FAIL'}: {len(failures)} failure(s)")
sys.exit(1 if failures else 0)
