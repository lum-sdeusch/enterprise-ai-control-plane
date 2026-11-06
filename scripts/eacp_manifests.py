"""Shared loader and validator for eacp manifests.

Used by scripts/validate-manifests.py, scripts/compile-manifests.py and scripts/tool-gate.sh
(through validate). Keeps one definition of "where manifests live" and "what a valid set is".
"""

from __future__ import annotations

import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

import yaml
from jsonschema import Draft202012Validator, FormatChecker

ROOT = Path(__file__).resolve().parent.parent
SCHEMAS = ROOT / "schemas"

KINDS = {
    "tools": ("ToolManifest", "tool-manifest.schema.json"),
    "agents": ("AgentManifest", "agent-manifest.schema.json"),
    "teams": ("TeamManifest", "team-manifest.schema.json"),
}

PLACEHOLDER_DIGEST = "0" * 64
CLASSIFICATION_ORDER = ["public", "internal", "confidential", "restricted"]


@dataclass
class ManifestSet:
    tools: dict[str, dict] = field(default_factory=dict)
    agents: dict[str, dict] = field(default_factory=dict)
    teams: dict[str, dict] = field(default_factory=dict)
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)

    @property
    def ok(self) -> bool:
        return not self.errors


def _load_schema(name: str) -> Draft202012Validator:
    schema = json.loads((SCHEMAS / name).read_text())
    Draft202012Validator.check_schema(schema)
    return Draft202012Validator(schema, format_checker=FormatChecker())


def _rank(classification: str) -> int:
    return CLASSIFICATION_ORDER.index(classification)


def load_all(root: Path = ROOT) -> ManifestSet:
    """Load and validate every manifest. Schema errors and cross-reference errors are collected,
    never raised, so a run reports everything at once."""
    ms = ManifestSet()
    validators = {kind: _load_schema(schema) for kind, (_, schema) in KINDS.items()}

    for kind, (expected_kind, _) in KINDS.items():
        for path in sorted((root / kind).glob("*/manifest.yaml")):
            rel = path.relative_to(root)
            try:
                doc = yaml.safe_load(path.read_text())
            except yaml.YAMLError as exc:
                ms.errors.append(f"{rel}: YAML parse error: {exc}")
                continue
            if not isinstance(doc, dict):
                ms.errors.append(f"{rel}: not a mapping")
                continue
            for err in sorted(validators[kind].iter_errors(doc), key=lambda e: list(e.path)):
                where = "/".join(str(p) for p in err.path) or "<root>"
                ms.errors.append(f"{rel}: {where}: {err.message}")
            if doc.get("kind") != expected_kind:
                continue
            name = doc.get("metadata", {}).get("name")
            if name != path.parent.name:
                ms.errors.append(f"{rel}: metadata.name '{name}' must equal directory name '{path.parent.name}'")
            getattr(ms, kind)[path.parent.name] = doc

    _cross_check(ms)
    return ms


def _cross_check(ms: ManifestSet) -> None:
    for name, tool in ms.tools.items():
        team = tool["metadata"]["team"]
        spec = tool["spec"]
        if team not in ms.teams:
            ms.errors.append(f"tools/{name}: team '{team}' has no teams/{team}/manifest.yaml")
            continue
        tspec = ms.teams[team]["spec"]
        for t in spec["tools"]:
            approval = t.get("approval", {})
            if approval.get("mode") == "required" and approval.get("approvers") not in tspec["approverGroups"]:
                ms.errors.append(
                    f"tools/{name}: tool '{t['name']}' approvers '{approval.get('approvers')}' is not in team {team} approverGroups {tspec['approverGroups']}"
                )
        for g in spec.get("requiredGroups", []):
            if g not in tspec["groups"] and g not in tspec["approverGroups"]:
                ms.warnings.append(f"tools/{name}: requiredGroups entry '{g}' is not one of team {team}'s groups; make sure it exists in the IdP")
        if spec["image"].endswith(PLACEHOLDER_DIGEST):
            ms.warnings.append(f"tools/{name}: image digest is the bootstrap placeholder; the tool gate will reject it")
        for k, v in spec["pins"].items():
            if v.endswith(PLACEHOLDER_DIGEST):
                ms.warnings.append(f"tools/{name}: pins.{k} is the bootstrap placeholder; run scripts/pin-tools.py before merging")
        for rule in spec.get("egress", []):
            if "fqdn" in rule:
                ms.warnings.append(f"tools/{name}: egress fqdn '{rule['fqdn']}' is enforced only with an FQDN-aware CNI (Cilium)")
            if rule.get("cidr") == "0.0.0.0/0":
                ms.warnings.append(f"tools/{name}: egress to 0.0.0.0/0:{rule['port']} is wide open; acceptable only for a fetch-style tool guarded by taint policy")

    for name, agent in ms.agents.items():
        team = agent["metadata"]["team"]
        spec = agent["spec"]
        if team not in ms.teams:
            ms.errors.append(f"agents/{name}: team '{team}' has no teams/{team}/manifest.yaml")
            continue
        tspec = ms.teams[team]["spec"]
        for cls in spec["allowedModelClasses"]:
            if cls not in tspec["allowedModelClasses"]:
                ms.errors.append(f"agents/{name}: model class '{cls}' is not allowed for team {team}")
        for entry in spec["allowedTools"]:
            server, tool = entry.split("/", 1)
            if server not in ms.tools:
                ms.errors.append(f"agents/{name}: allowedTools entry '{entry}' references unknown server '{server}'")
                continue
            declared = {t["name"]: t for t in ms.tools[server]["spec"]["tools"]}
            targets = list(declared.values()) if tool == "*" else [declared.get(tool)]
            if tool != "*" and tool not in declared:
                ms.errors.append(f"agents/{name}: allowedTools entry '{entry}' references a tool the server does not declare")
                continue
            for t in targets:
                if _rank(t["outputClassification"]) > _rank(spec["maxClassification"]):
                    ms.warnings.append(
                        f"agents/{name}: tool {server}/{t['name']} outputs {t['outputClassification']} which exceeds the agent ceiling {spec['maxClassification']}; it will be denied at runtime"
                    )
        if spec["kind"] == "scheduled" and "external" in spec["allowedModelClasses"]:
            ms.warnings.append(f"agents/{name}: a scheduled agent with external model access has no human to notice a leak; reconsider")
        if spec["kind"] == "scheduled" and spec.get("image", "").endswith(PLACEHOLDER_DIGEST):
            ms.warnings.append(f"agents/{name}: image digest is the bootstrap placeholder")


def report(ms: ManifestSet, *, quiet: bool = False, warn: bool = True) -> int:
    if warn:
        for w in ms.warnings:
            print(f"warning: {w}")
    for e in ms.errors:
        print(f"error: {e}", file=sys.stderr)
    if not quiet:
        print(
            f"manifests: {len(ms.tools)} tools, {len(ms.agents)} agents, {len(ms.teams)} teams; "
            f"{len(ms.errors)} errors, {len(ms.warnings)} warnings"
        )
    return 1 if ms.errors else 0


# --- normalisation for OPA data -------------------------------------------------------------

_CAMEL = re.compile(r"(?<!^)(?=[A-Z])")


def snake(s: str) -> str:
    return _CAMEL.sub("_", s).lower()


def to_opa_data(ms: ManifestSet) -> dict:
    """Shape the manifests the way the Rego expects them. Keys are snake_case; lookups are by
    name so policies can write data.eacp.tools[server].tools[tool]."""
    tools: dict[str, dict] = {}
    for name, m in sorted(ms.tools.items()):
        spec = m["spec"]
        tools[name] = {
            "team": m["metadata"]["team"],
            "owner": m["metadata"]["owner"],
            "transport": spec["transport"],
            "required_groups": spec.get("requiredGroups", []),
            "output_trust": spec.get("outputTrust", "trusted"),
            "credentials_mode": spec["credentials"]["mode"],
            "pins": spec["pins"],
            "tools": {
                t["name"]: {
                    "risk_tier": t["riskTier"],
                    "capability": t["capability"],
                    "output_classification": t["outputClassification"],
                    "approval": {
                        "mode": t.get("approval", {}).get("mode", "none"),
                        "approvers": t.get("approval", {}).get("approvers"),
                        "allow_self": t.get("approval", {}).get("allowSelf", False),
                    },
                }
                for t in spec["tools"]
            },
        }
    agents: dict[str, dict] = {}
    for name, m in sorted(ms.agents.items()):
        spec = m["spec"]
        agents[name] = {
            "team": m["metadata"]["team"],
            "owner": m["metadata"]["owner"],
            "kind": spec["kind"],
            "client_id": spec["identity"]["clientId"],
            "risk_tier": spec["riskTier"],
            "allowed_tools": spec["allowedTools"],
            "allowed_model_classes": spec["allowedModelClasses"],
            "max_classification": spec["maxClassification"],
        }
    teams: dict[str, dict] = {}
    for name, m in sorted(ms.teams.items()):
        spec = m["spec"]
        teams[name] = {
            "owner": m["metadata"]["owner"],
            "groups": spec["groups"],
            "approver_groups": spec["approverGroups"],
            "allowed_model_classes": spec["allowedModelClasses"],
            "budgets": {snake(k): v for k, v in spec["budgets"].items()},
            "default_clearance": spec["defaultClearance"],
            "namespace": spec.get("namespace", f"team-{name}"),
        }
    # client_id -> agent name, so the adapter can resolve the actor chain cheaply.
    clients = {a["client_id"]: n for n, a in agents.items()}
    return {"tools": tools, "agents": agents, "teams": teams, "agent_clients": clients}
