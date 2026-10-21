#!/usr/bin/env python3
"""Convert legacy Ticketdesk state to v2 without changing the source file.

This optional offline maintenance utility is not an application runtime.
Stop the legacy server first; the destination must not already exist.
"""

import argparse
import json
import os
from pathlib import Path
import re
import tempfile


def positive_integer(value):
    return type(value) is int and value > 0


def convert(source):
    if not isinstance(source, dict):
        raise ValueError("legacy state must be an object")
    if not source:
        source = {"meta": {"next_ticket": 1, "next_token": 1}}
    meta = source.get("meta")
    if not isinstance(meta, dict) or not all(
        positive_integer(meta.get(key))
        for key in ("next_ticket", "next_token")
    ):
        raise ValueError("legacy state has invalid counters")
    result = {"header": {"version": 2, "next": meta["next_ticket"]}}
    token_ordinals = set()
    for key, record in sorted(source.items()):
        if key == "meta" or key.startswith(("b:", "q:", "h:")):
            continue
        if key.startswith("t:"):
            if not isinstance(record, dict):
                raise ValueError("invalid ticket record")
            ticket_id = record.get("id", "")
            if not isinstance(ticket_id, str) or not re.fullmatch(
                r"t[1-9][0-9]{0,14}", ticket_id
            ):
                raise ValueError("invalid ticket id")
            sequence = int(ticket_id[1:])
            if key != f"t:{sequence:010d}" or sequence >= meta["next_ticket"]:
                raise ValueError("ticket key or counter does not match")
            if set(record) != {
                "id",
                "title",
                "body",
                "priority",
                "status",
                "owner",
                "created_at",
                "updated_at",
            }:
                raise ValueError("invalid ticket fields")
            if not isinstance(record["title"], str) or not (
                1 <= len(record["title"]) <= 120
            ):
                raise ValueError("invalid ticket title")
            if not isinstance(record["body"], str) or (
                len(record["body"]) > 4000
            ):
                raise ValueError("invalid ticket body")
            if record["priority"] not in ("low", "normal", "high") or (
                record["status"] not in ("open", "in_progress", "done")
            ):
                raise ValueError("invalid ticket status or priority")
            if not isinstance(record["owner"], str) or not record["owner"]:
                raise ValueError("invalid ticket owner")
            if not positive_integer(record["created_at"]) or not (
                positive_integer(record["updated_at"])
                and record["updated_at"] >= record["created_at"]
            ):
                raise ValueError("invalid ticket timestamps")
            group = result.setdefault(
                f"bucket:{sequence // 64:012d}", {"order": [], "records": {}}
            )
            group["order"].append(ticket_id)
            group["records"][ticket_id] = record
        elif key.startswith("k:"):
            if not isinstance(record, dict):
                raise ValueError("invalid token record")
            token_id, digest = record.get("id"), record.get("hash")
            if (
                not isinstance(token_id, str)
                or not re.fullmatch(r"tok_[0-9a-f]{16}", token_id)
                or key != "k:" + token_id
            ):
                raise ValueError("invalid token id")
            if (
                not isinstance(digest, str)
                or not re.fullmatch(r"[0-9a-f]{64}", digest)
                or "digest:" + digest in result
            ):
                raise ValueError("invalid or duplicate token digest")
            ordinal = record.get("seq")
            if not positive_integer(ordinal) or ordinal in token_ordinals:
                raise ValueError("invalid or duplicate token order")
            token_ordinals.add(ordinal)
            if (
                not isinstance(record.get("name"), str)
                or not (1 <= len(record["name"]) <= 64)
                or record.get("role") not in ("admin", "member", "viewer")
            ):
                raise ValueError("invalid token name or role")
            if not positive_integer(record.get("created_at")) or (
                type(record.get("revoked")) is not int
                or record["revoked"] not in (0, 1)
            ):
                raise ValueError("invalid token time or revoked flag")
            result["token:" + token_id] = {
                "id": token_id,
                "name": record["name"],
                "role": record["role"],
                "created_at": record["created_at"],
                "revoked": bool(record["revoked"]),
                "digest": digest,
                "ordinal": ordinal,
            }
            result["digest:" + digest] = {"id": token_id}
        else:
            raise ValueError("unrecognized legacy state key")
    result["token_counter"] = {
        "next": max(meta["next_token"], max(token_ordinals, default=0) + 1)
    }
    # Numeric ordering is explicit, independent of legacy key serialization.
    for key, group in result.items():
        if key.startswith("bucket:"):
            group["order"].sort(key=lambda item: int(item[1:]))
    tokens = sorted(
        (value for key, value in result.items() if key.startswith("token:")),
        key=lambda record: record["ordinal"],
    )
    result = {
        key: value
        for key, value in result.items()
        if key == "header" or key.startswith("bucket:")
    }
    result["tokens"] = {
        "order": [record["id"] for record in tokens],
        "records": {
            record["id"]: {
                key: value for key, value in record.items() if key != "ordinal"
            }
            for record in tokens
        },
    }
    return result


def migrate(source, destination):
    raw = source.read_bytes()
    document = (
        json.loads(raw)
        if raw.strip()
        else {"meta": {"next_ticket": 1, "next_token": 1}}
    )
    # Validate and prepare the entire result before any output file is created.
    encoded = (
        json.dumps(convert(document), ensure_ascii=False) + "\n"
    ).encode()
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(
            dir=destination.parent, prefix=".ticketdesk-migrate-", delete=False
        ) as stream:
            temporary = Path(stream.name)
            stream.write(encoded)
            stream.flush()
            os.fsync(stream.fileno())
        # link is an atomic no-overwrite publish; input and an existing output
        # cannot be clobbered, even if a destination appears during validation.
        os.link(temporary, destination)
        directory = os.open(destination.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("destination", type=Path)
    args = parser.parse_args()
    try:
        migrate(args.source, args.destination)
    except (OSError, ValueError, TypeError, KeyError) as error:
        parser.exit(1, f"ticketdesk-migrate: {error}\n")


if __name__ == "__main__":
    main()
