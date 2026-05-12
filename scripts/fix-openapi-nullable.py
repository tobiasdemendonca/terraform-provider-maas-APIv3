#!/usr/bin/env python3
"""Convert OpenAPI 3.1 nullable syntax to 3.0 nullable: true form.

anyOf: [{type: X, ...}, {type: null}]  ->  {type: X, ..., nullable: true}
"""

import json
import sys


def fix_nullable(obj):
    if isinstance(obj, dict):
        if "anyOf" in obj:
            null_items = [i for i in obj["anyOf"] if i == {"type": "null"}]
            non_null = [i for i in obj["anyOf"] if i != {"type": "null"}]
            if null_items:
                result = {k: v for k, v in obj.items() if k != "anyOf"}
                if len(non_null) == 1:
                    result.update(non_null[0])
                else:
                    result["anyOf"] = non_null
                result["nullable"] = True
                return fix_nullable(result)
        return {k: fix_nullable(v) for k, v in obj.items()}
    if isinstance(obj, list):
        return [fix_nullable(i) for i in obj]
    return obj


def main():
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <input> <output>", file=sys.stderr)
        sys.exit(1)

    with open(sys.argv[1]) as f:
        spec = json.load(f)

    fixed = fix_nullable(spec)

    with open(sys.argv[2], "w") as f:
        json.dump(fixed, f, indent=2)
        f.write("\n")


if __name__ == "__main__":
    main()
