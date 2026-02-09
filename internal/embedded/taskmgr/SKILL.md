---
name: task-mgr
description: This skill should be used when the user asks to create tasks, manage issues, track work items, close tasks, take a task snapshot, or discusses task/issue management with memory integration.
allowed-tools: Bash, Glob, Grep, Read, Edit, Write
---

# Task Manager: Issue Tracking with Memory

Wraps the **beads** (`bd`) issue tracker with automatic memory integration.
Every create and close writes a decision record to the collaboration memory system,
so task history is searchable across sessions.

---

## Commands

| Command | What it does | Memory side-effect |
|---------|-------------|-------------------|
| `create "desc" --tags t1,t2` | Creates issue via `bd` | Writes "Task created" decision |
| `close <id> --summary "..."` | Closes issue via `bd` | Writes "Task closed" decision |
| `show <id>` | Shows issue details | None (read-only) |
| `list` | Lists open issues | None (read-only) |
| `search "query"` | Searches issues | None (read-only) |
| `snapshot` | Snapshots all open tasks | Writes snapshot to memory |
| `context <id>` | Shows issue + memory history | None (read-only) |

All commands use `~/.claude/skills/task-mgr/taskctl.py`.

---

## Quick Reference

```bash
TASK="python3 ~/.claude/skills/task-mgr/taskctl.py"

# Create a task
$TASK create "Fix auth token refresh" --tags auth,api

# List open tasks
$TASK list

# Show task details
$TASK show workspace-9j3

# Close with summary
$TASK close workspace-9j3 --summary "Added retry logic to token refresh"

# Snapshot all open tasks to memory
$TASK snapshot

# Get full context (task details + memory history)
$TASK context workspace-9j3

# Search tasks
$TASK search "auth"
```

---

## Workflow

### 1. Create tasks as you discover work

```bash
python3 ~/.claude/skills/task-mgr/taskctl.py create "Refactor DB connection pooling" --tags db,perf
```

### 2. Work on the task, making decisions along the way

Use `memory add` from doc-sync to record decisions tagged with the task ID.

### 3. Close with a summary when done

```bash
python3 ~/.claude/skills/task-mgr/taskctl.py close workspace-a1b --summary "Switched to pgbouncer, 3x throughput improvement"
```

### 4. Before context rolls, snapshot open tasks

```bash
python3 ~/.claude/skills/task-mgr/taskctl.py snapshot
```

### 5. In a new session, recover context

```bash
python3 ~/.claude/skills/task-mgr/taskctl.py context workspace-a1b
```

---

## Direct bd Access

For operations not covered by taskctl.py, use `bd` directly:

```bash
bd list          # List open issues
bd show <id>     # Show issue details
bd create "..."  # Create (no memory integration)
bd close <id>    # Close (no memory integration)
bd search "..."  # Search issues
```

Note: Direct `bd` commands skip memory integration.

---

## References

- Task tool: `~/.claude/skills/task-mgr/taskctl.py`
- Memory system: `~/.claude/skills/doc-sync/doctool.py`
- Issue tracker: `bd` (beads)
