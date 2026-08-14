import os, sys, json
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import xml_lint
try:
    payload = json.loads(sys.stdin.read())
except Exception as e:  # malformed driver input -> structured error, still exit 0
    sys.stdout.write(json.dumps({"driver_error": "invalid batch payload: %s" % e}))
    sys.exit(0)
xmls = payload if isinstance(payload, list) else payload.get("slides", [])
out = [xml_lint.lint_xml(x, "INPUT") for x in xmls]
sys.stdout.write(json.dumps(out, ensure_ascii=False))
# NOTE: never sys.exit(nonzero) — wazero finalize on nonzero exit costs ~2s.
