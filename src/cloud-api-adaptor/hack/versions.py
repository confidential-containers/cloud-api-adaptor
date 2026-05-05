#!/usr/bin/env python3
"""Query and validate versions.yaml."""

import argparse
import json
import sys

import yaml


def lookup(data, path):
    cur = data
    for key in path.split("."):
        if not isinstance(cur, dict) or key not in cur:
            sys.exit(f"error: '{path}' not found")
        cur = cur[key]
    if isinstance(cur, dict):
        sys.exit(f"error: '{path}' is an object, not a scalar")
    return cur


def validate(data, schema_path):
    import jsonschema

    with open(schema_path) as f:
        schema = json.load(f)
    errors = list(jsonschema.Draft7Validator(schema).iter_errors(data))
    if errors:
        for e in errors:
            print(f"  - {e.message}", file=sys.stderr)
        sys.exit(1)
    print("ok")


def main():
    p = argparse.ArgumentParser(description="Query and validate versions.yaml")
    p.add_argument("-f", "--file", default="versions.yaml")
    p.add_argument("-q", "--query", help="Dot path, e.g. tools.golang, oci.pause.tag")
    p.add_argument(
        "-c", "--check", metavar="SCHEMA", help="Validate against JSON schema"
    )
    args = p.parse_args()

    with open(args.file) as f:
        data = yaml.safe_load(f)

    if args.check:
        validate(data, args.check)
    elif args.query:
        print(lookup(data, args.query))
    else:
        p.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
