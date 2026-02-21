"""
Shared core logic for doc-sync memory system.

Provides content hashing, markdown chunking, and document ingestion
used by both doctool.py (CLI) and mcp_server.py (MCP JSON-RPC server).

No external dependencies (stdlib only).
"""

import hashlib
import sqlite3
from datetime import datetime
from pathlib import Path

# =============================================================================
# CONFIGURATION (shared defaults)
# =============================================================================

DB_PATH = Path("/workspace/_docs/.doc-index.db")
DOCS_ROOT = Path("/workspace/_docs")

# Controlled vocabulary for document genres
VALID_GENRES = [
    "overview", "gotchas", "architecture", "deep-dive",
    "adr", "runbook", "rfc", "guide", "reference",
]

# Tag hints for auto-inferring tags from file paths
TAG_HINTS = ["infra", "agents", "apps", "shared", "pipelines", "ecs", "lambda"]


# =============================================================================
# DATABASE ACCESS
# =============================================================================

def get_connection(db_path: Path = None) -> sqlite3.Connection:
    """Get a database connection with standard pragmas."""
    db = db_path or DB_PATH
    if not db.exists():
        raise RuntimeError(f"Database not found: {db}. Run 'doctool index init' first.")
    conn = sqlite3.connect(db)
    conn.execute("PRAGMA foreign_keys = ON")
    conn.row_factory = sqlite3.Row
    return conn


# =============================================================================
# CONTENT PROCESSING
# =============================================================================

def hash_content(content: str) -> str:
    """Generate SHA256 hash for content deduplication."""
    return hashlib.sha256(content.encode()).hexdigest()


def chunk_document(content: str, source: str) -> list[dict]:
    """
    Chunk a markdown document by headings.

    Splits on heading boundaries, tracking the heading hierarchy to produce
    a breadcrumb-style section label (e.g. "Setup > Prerequisites").

    Returns list of {content, section, source}. Skips sections shorter
    than 50 characters.
    """
    chunks = []
    lines = content.split("\n")

    current_section = []
    current_headings = []

    for line in lines:
        if line.startswith("#"):
            # Save previous section if it has content
            if current_section:
                section_content = "\n".join(current_section).strip()
                if section_content and len(section_content) > 50:
                    chunks.append({
                        "content": section_content,
                        "section": " > ".join(current_headings) if current_headings else "Introduction",
                        "source": source,
                    })
                current_section = []

            # Update heading stack
            level = len(line) - len(line.lstrip("#"))
            heading_text = line.lstrip("#").strip()

            # Trim heading stack to current level
            current_headings = current_headings[:level - 1]
            current_headings.append(heading_text)

        current_section.append(line)

    # Don't forget the last section
    if current_section:
        section_content = "\n".join(current_section).strip()
        if section_content and len(section_content) > 50:
            chunks.append({
                "content": section_content,
                "section": " > ".join(current_headings) if current_headings else "Content",
                "source": source,
            })

    return chunks


def infer_tags(path: str) -> list[str]:
    """Infer tags from a document path using known hints."""
    tags = []
    path_lower = path.lower()
    for hint in TAG_HINTS:
        if hint in path_lower:
            tags.append(hint)
    return tags or ["general"]


# =============================================================================
# DATABASE OPERATIONS
# =============================================================================

def insert_chunk(conn: sqlite3.Connection, content: str, source: str,
                 section: str, tags_csv: str, tags_str: str,
                 chunk_type: str = "doc") -> bool:
    """
    Insert a single chunk into chunk_meta + chunks_fts, with dedup.

    Returns True if inserted, False if duplicate.
    """
    content_hash = hash_content(content)

    existing = conn.execute(
        "SELECT id FROM chunk_meta WHERE content_hash = ?", (content_hash,)
    ).fetchone()

    if existing:
        return False

    now = datetime.now().isoformat()

    cursor = conn.execute("""
        INSERT INTO chunk_meta (source, section, tags, chunk_type, created_at, content_hash)
        VALUES (?, ?, ?, ?, ?, ?)
    """, (source, section, tags_csv, chunk_type, now, content_hash))

    conn.execute("""
        INSERT INTO chunks_fts (rowid, content, source, section, tags)
        VALUES (?, ?, ?, ?, ?)
    """, (cursor.lastrowid, content, source, section, tags_str))

    return True


def ingest_document(conn: sqlite3.Connection, path: str,
                    docs_root: Path = None, tags: list = None) -> int:
    """
    Ingest a single document into the memory system.

    Reads the file, chunks by headings, dedup-inserts each chunk.
    Returns the number of new chunks inserted.
    """
    root = docs_root or DOCS_ROOT
    full_path = root / path
    if not full_path.exists():
        return 0

    content = full_path.read_text()
    chunks = chunk_document(content, path)

    if not chunks:
        return 0

    if not tags:
        tags = infer_tags(path)

    tags_str = " ".join(tags)
    tags_csv = ",".join(tags)

    inserted = 0
    for chunk in chunks:
        if insert_chunk(conn, chunk["content"], chunk["source"],
                        chunk["section"], tags_csv, tags_str, "doc"):
            inserted += 1

    return inserted
