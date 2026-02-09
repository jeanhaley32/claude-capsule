#!/usr/bin/env python3
"""Task manager: wraps bd (beads) with memory integration.

Usage:
    taskctl.py create "description" [--tags t1,t2]
    taskctl.py close <id> --summary "what was done"
    taskctl.py show <id>
    taskctl.py list
    taskctl.py search "query"
    taskctl.py snapshot
    taskctl.py context <id>
"""

import subprocess
import sys
import re
import argparse
from datetime import datetime
from pathlib import Path

# Import DocMemory from doc-sync skill
sys.path.insert(0, str(Path.home() / ".claude/skills/doc-sync"))
from doctool import DocMemory


def run_bd(*args):
    """Run a bd command and return (stdout, stderr, returncode)."""
    result = subprocess.run(["bd", *args], capture_output=True, text=True)
    return result.stdout.strip(), result.stderr.strip(), result.returncode


def get_memory():
    """Get a DocMemory instance."""
    return DocMemory()


def cmd_create(args):
    """Create a task via bd and write a memory entry."""
    stdout, stderr, rc = run_bd("create", args.description)
    if rc != 0:
        print(f"bd create failed: {stderr}", file=sys.stderr)
        sys.exit(1)

    print(stdout)

    # Parse issue ID from bd output (e.g. "Created issue workspace-9j3")
    match = re.search(r'(\S+-\S+)', stdout)
    if not match:
        print("Warning: could not parse issue ID from bd output", file=sys.stderr)
        return

    issue_id = match.group(1)
    now = datetime.now().isoformat()

    tags = ["task"]
    if args.tags:
        tags.extend(args.tags.split(","))

    content = f"Task #{issue_id} created: {args.description}\nTags: {', '.join(tags)}\nDate: {now}"

    memory = get_memory()
    memory.add(
        content=content,
        tags=tags,
        source=f"task:{issue_id}",
        chunk_type="decision",
    )


def cmd_close(args):
    """Close a task via bd and write a closing memory entry."""
    # Get original description from bd show
    show_stdout, _, show_rc = run_bd("show", args.id)
    original_desc = show_stdout if show_rc == 0 else "(could not retrieve)"

    stdout, stderr, rc = run_bd("close", args.id)
    if rc != 0:
        print(f"bd close failed: {stderr}", file=sys.stderr)
        sys.exit(1)

    print(stdout)

    now = datetime.now().isoformat()

    # Try to recover original tags from memory
    memory = get_memory()
    original_tags = ["task", "task-closed"]
    results = memory.search(f"task {args.id}", limit=1)
    if results:
        existing_tags = results[0].get("tags", "")
        if existing_tags:
            for t in existing_tags.split(","):
                t = t.strip()
                if t and t not in original_tags:
                    original_tags.append(t)

    summary = args.summary or "No summary provided"
    content = (
        f"Task #{args.id} closed\n"
        f"Summary: {summary}\n"
        f"Original: {original_desc}\n"
        f"Date: {now}"
    )

    memory.add(
        content=content,
        tags=original_tags,
        source=f"task:{args.id}",
        chunk_type="decision",
    )


def cmd_show(args):
    """Show a task (read-only passthrough to bd)."""
    stdout, stderr, rc = run_bd("show", args.id)
    if rc != 0:
        print(f"bd show failed: {stderr}", file=sys.stderr)
        sys.exit(1)
    print(stdout)


def cmd_list(args):
    """List tasks (read-only passthrough to bd)."""
    stdout, stderr, rc = run_bd("list")
    if rc != 0:
        print(f"bd list failed: {stderr}", file=sys.stderr)
        sys.exit(1)
    print(stdout)


def cmd_search(args):
    """Search tasks (read-only passthrough to bd)."""
    stdout, stderr, rc = run_bd("search", args.query)
    if rc != 0:
        print(f"bd search failed: {stderr}", file=sys.stderr)
        sys.exit(1)
    print(stdout)


def cmd_snapshot(args):
    """Snapshot all open tasks into a single memory entry."""
    stdout, stderr, rc = run_bd("list")
    if rc != 0:
        print(f"bd list failed: {stderr}", file=sys.stderr)
        sys.exit(1)

    now = datetime.now()
    date_str = now.strftime("%Y-%m-%d")

    # Count non-empty lines as open tasks (rough heuristic)
    lines = [line for line in stdout.splitlines() if line.strip()]
    task_count = len(lines)

    content = (
        f"Task Snapshot ({date_str})\n"
        f"Open tasks: {task_count}\n\n"
        f"{stdout}"
    )

    print(f"Snapshot: {task_count} open tasks")

    memory = get_memory()
    memory.add(
        content=content,
        tags=["task", "snapshot"],
        source=f"task-snapshot:{date_str}",
        chunk_type="session",
    )


def cmd_context(args):
    """Show task details + related memory entries."""
    # Get task details from bd
    stdout, stderr, rc = run_bd("show", args.id)
    if rc != 0:
        print(f"bd show failed: {stderr}", file=sys.stderr)
        sys.exit(1)

    print("=== Task Details ===")
    print(stdout)
    print()

    # Search memory for related entries
    memory = get_memory()
    results = memory.search(f"task {args.id}", limit=10)

    if results:
        print(f"=== Memory ({len(results)} entries) ===")
        for i, r in enumerate(results, 1):
            age_days = r.get("age_days", 0)
            age_str = f"{age_days}d ago"
            print(f"--- Entry {i} ({age_str}) ---")
            print(f"Source: {r['source']}")
            print(f"Tags: {r['tags']}")
            print(f"Type: {r['type']}")
            content = r["content"]
            if len(content) > 500:
                content = content[:500] + "..."
            print(content)
            print()
    else:
        print("=== Memory: no related entries ===")


def main():
    parser = argparse.ArgumentParser(
        description="Task manager with memory integration (wraps bd)"
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    # create
    p_create = subparsers.add_parser("create", help="Create a new task")
    p_create.add_argument("description", help="Task description")
    p_create.add_argument("--tags", help="Comma-separated tags")
    p_create.set_defaults(func=cmd_create)

    # close
    p_close = subparsers.add_parser("close", help="Close a task")
    p_close.add_argument("id", help="Issue ID")
    p_close.add_argument("--summary", help="Summary of what was done")
    p_close.set_defaults(func=cmd_close)

    # show
    p_show = subparsers.add_parser("show", help="Show task details")
    p_show.add_argument("id", help="Issue ID")
    p_show.set_defaults(func=cmd_show)

    # list
    p_list = subparsers.add_parser("list", help="List open tasks")
    p_list.set_defaults(func=cmd_list)

    # search
    p_search = subparsers.add_parser("search", help="Search tasks")
    p_search.add_argument("query", help="Search query")
    p_search.set_defaults(func=cmd_search)

    # snapshot
    p_snapshot = subparsers.add_parser("snapshot", help="Snapshot open tasks to memory")
    p_snapshot.set_defaults(func=cmd_snapshot)

    # context
    p_context = subparsers.add_parser("context", help="Show task + related memory")
    p_context.add_argument("id", help="Issue ID")
    p_context.set_defaults(func=cmd_context)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
