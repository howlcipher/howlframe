"""Validate run and score records against the JSON schemas in ../schema.

Only the keywords those schemas use are implemented (type, const, enum, required, properties, items), so the
harness needs no third-party package. A record that fails validation is a harness bug and raises.
"""

import json
import os

SCHEMA_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "schema")
_TYPES = {"object": dict, "array": list, "string": str, "boolean": bool, "null": type(None)}


class SchemaError(ValueError):
    pass


def _load(name):
    with open(os.path.join(SCHEMA_DIR, name)) as f:
        return json.load(f)


def _type_ok(value, t):
    if t == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if t == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    return isinstance(value, _TYPES[t])


def validate(value, schema, where="$"):
    if "const" in schema and value != schema["const"]:
        raise SchemaError(f"{where}: expected {schema['const']!r}, got {value!r}")
    if "enum" in schema and value not in schema["enum"]:
        raise SchemaError(f"{where}: {value!r} is not one of {schema['enum']}")
    if "type" in schema:
        types = schema["type"] if isinstance(schema["type"], list) else [schema["type"]]
        if not any(_type_ok(value, t) for t in types):
            raise SchemaError(f"{where}: {value!r} is not of type {types}")
    if isinstance(value, dict):
        for key in schema.get("required", []):
            if key not in value:
                raise SchemaError(f"{where}: missing required {key!r}")
        for key, sub in schema.get("properties", {}).items():
            if key in value:
                validate(value[key], sub, f"{where}.{key}")
    if isinstance(value, list) and "items" in schema:
        for i, item in enumerate(value):
            validate(item, schema["items"], f"{where}[{i}]")


def validate_result(record):
    validate(record, _load("run_result.schema.json"))


def validate_score(record):
    validate(record, _load("score_result.schema.json"))
