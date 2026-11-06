#!/usr/bin/env python3
"""Compile manifests into the artefacts derived from them.

Outputs, written deterministically so CI can check they are committed and current:
  policies/eacp/manifests.json         OPA data: data.eacp.tools / agents / teams / agent_clients
  platform/keycloak/realm-import.yaml  KeycloakRealmImport with groups, clients, scopes and mappers

The bundle revision is not written here; scripts/build-bundle.sh stamps it into the bundle
manifest, and policies read it from data.system.bundles.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import yaml

sys.path.insert(0, str(Path(__file__).resolve().parent))

from eacp_manifests import ROOT, ManifestSet, load_all, report, to_opa_data  # noqa: E402

REALM = "eacp"
MCP_AUDIENCE = "eacp-mcp"
LLM_AUDIENCE = "eacp-llm"


def _attr_mapper(name: str, attribute: str, claim: str) -> dict:
    return {
        "name": name,
        "protocol": "openid-connect",
        "protocolMapper": "oidc-usermodel-attribute-mapper",
        "consentRequired": False,
        "config": {
            "user.attribute": attribute,
            "claim.name": claim,
            "jsonType.label": "String",
            "id.token.claim": "true",
            "access.token.claim": "true",
            "userinfo.token.claim": "true",
        },
    }


def _audience_mapper(name: str, audience: str) -> dict:
    return {
        "name": name,
        "protocol": "openid-connect",
        "protocolMapper": "oidc-audience-mapper",
        "consentRequired": False,
        "config": {"included.client.audience": audience, "id.token.claim": "false", "access.token.claim": "true"},
    }


def realm_import(ms: ManifestSet) -> dict:
    groups = []
    for team, m in sorted(ms.teams.items()):
        for g in m["spec"]["groups"]:
            groups.append({"name": g, "attributes": {"team": [team], "eacp.role": ["member"]}})
        for g in m["spec"]["approverGroups"]:
            groups.append({"name": g, "attributes": {"team": [team], "eacp.role": ["approver"]}})

    client_scopes = [
        {
            "name": "eacp",
            "description": "Claims the control plane needs: groups, team, clearance, data_label.",
            "protocol": "openid-connect",
            "attributes": {"include.in.token.scope": "true", "display.on.consent.screen": "false"},
            "protocolMappers": [
                {
                    "name": "groups",
                    "protocol": "openid-connect",
                    "protocolMapper": "oidc-group-membership-mapper",
                    "consentRequired": False,
                    "config": {
                        "claim.name": "groups",
                        "full.path": "false",
                        "id.token.claim": "true",
                        "access.token.claim": "true",
                        "userinfo.token.claim": "true",
                    },
                },
                _attr_mapper("team", "team", "team"),
                _attr_mapper("clearance", "clearance", "clearance"),
                _attr_mapper("data_label", "data_label", "data_label"),
            ],
        },
        {
            "name": "eacp-mcp-audience",
            "protocol": "openid-connect",
            "attributes": {"include.in.token.scope": "false", "display.on.consent.screen": "false"},
            "protocolMappers": [_audience_mapper("aud-eacp-mcp", MCP_AUDIENCE)],
        },
        {
            "name": "eacp-llm-audience",
            "protocol": "openid-connect",
            "attributes": {"include.in.token.scope": "false", "display.on.consent.screen": "false"},
            "protocolMappers": [_audience_mapper("aud-eacp-llm", LLM_AUDIENCE)],
        },
    ]

    clients = [
        # Resource audiences. bearer-only clients exist so audience mappers and token exchange
        # have a client to point at; nobody logs in to them.
        {"clientId": MCP_AUDIENCE, "name": "eacp MCP gateway resource", "bearerOnly": True, "enabled": True},
        {"clientId": LLM_AUDIENCE, "name": "eacp LLM gateway resource", "bearerOnly": True, "enabled": True},
        {
            "clientId": "librechat",
            "name": "LibreChat",
            "enabled": True,
            "publicClient": False,
            "standardFlowEnabled": True,
            "directAccessGrantsEnabled": False,
            "serviceAccountsEnabled": False,
            # TODO(deploy): set to the LibreChat URL of the profile in use.
            "redirectUris": ["http://chat.127.0.0.1.sslip.io:3080/oauth/openid/callback"],
            "webOrigins": ["+"],
            "defaultClientScopes": ["openid", "profile", "email", "eacp", "eacp-mcp-audience", "eacp-llm-audience"],
            "attributes": {"eacp.kind": "ui"},
        },
        {
            # The adapter's approval page logs approvers in through OIDC.
            "clientId": "eacp-adapter",
            "name": "eacp adapter approval page",
            "enabled": True,
            "publicClient": False,
            "standardFlowEnabled": True,
            "directAccessGrantsEnabled": False,
            "serviceAccountsEnabled": False,
            # TODO(deploy): the gateway public URL of the profile in use plus /approvals/callback.
            "redirectUris": ["http://gw.127.0.0.1.sslip.io:8080/approvals/callback"],
            "defaultClientScopes": ["openid", "profile", "email", "eacp"],
            "attributes": {"eacp.kind": "ui"},
        },
        {
            # Langfuse single sign-on (platform/telemetry/langfuse-values.yaml).
            "clientId": "langfuse",
            "name": "Langfuse",
            "enabled": True,
            "publicClient": False,
            "standardFlowEnabled": True,
            "directAccessGrantsEnabled": False,
            "serviceAccountsEnabled": False,
            "redirectUris": ["http://langfuse.127.0.0.1.sslip.io:8080/api/auth/callback/keycloak"],
            "defaultClientScopes": ["openid", "profile", "email", "eacp"],
            "attributes": {"eacp.kind": "ui"},
        },
        {
            # The gateway's own identity for RFC 8693 token exchange toward tool backends.
            "clientId": "eacp-gateway",
            "name": "agentgateway token-exchange requester",
            "enabled": True,
            "publicClient": False,
            "standardFlowEnabled": False,
            "serviceAccountsEnabled": True,
            "defaultClientScopes": ["eacp"],
            "attributes": {"eacp.kind": "gateway"},
        },
        {
            # demo/scenarios/run.py obtains user tokens with the password grant on this client.
            # Laptop profile only; direct grants are off for every other client.
            "clientId": "demo-runner",
            "name": "demo scenario runner",
            "enabled": True,
            "publicClient": False,
            "standardFlowEnabled": False,
            "directAccessGrantsEnabled": True,
            "serviceAccountsEnabled": False,
            "defaultClientScopes": ["openid", "eacp", "eacp-mcp-audience", "eacp-llm-audience"],
            "attributes": {"eacp.kind": "demo"},
        },
        {
            # Nightly red-team identity used by .github/workflows/nightly-redteam.yaml.
            "clientId": "agent-redteam",
            "name": "promptfoo red-team runner",
            "enabled": True,
            "publicClient": False,
            "standardFlowEnabled": False,
            "serviceAccountsEnabled": True,
            "defaultClientScopes": ["eacp", "eacp-mcp-audience", "eacp-llm-audience"],
            "attributes": {"eacp.kind": "scheduled", "eacp.team": "security", "eacp.owner": "security@example.com"},
        },
    ]
    for name, m in sorted(ms.agents.items()):
        spec = m["spec"]
        clients.append(
            {
                "clientId": spec["identity"]["clientId"],
                "name": f"agent {name}",
                "description": m["metadata"].get("description", ""),
                "enabled": True,
                "publicClient": False,
                "standardFlowEnabled": False,
                "directAccessGrantsEnabled": False,
                # Service accounts give every agent a client-credentials identity: scheduled agents
                # use it directly (after Kubernetes service-account federation); interactive agents
                # use it as the actor token in the delegation exchange (ADR 0023).
                "serviceAccountsEnabled": True,
                "defaultClientScopes": ["eacp", "eacp-mcp-audience", "eacp-llm-audience"],
                "attributes": {
                    "eacp.kind": spec["kind"],
                    "eacp.team": m["metadata"]["team"],
                    "eacp.owner": m["metadata"]["owner"],
                    "eacp.risk_tier": spec["riskTier"],
                    "eacp.agent": name,
                },
            }
        )

    # Demo users for the laptop profile only. Temporary passwords; Keycloak forces a change.
    users = [
        {
            "username": "alice",
            "email": "alice@example.com",
            "firstName": "Alice",
            "lastName": "Support",
            "enabled": True,
            "emailVerified": True,
            "groups": ["/support"],
            "attributes": {"team": ["support"], "clearance": ["restricted"]},
            "credentials": [{"type": "password", "value": "changeme-alice", "temporary": True}],
        },
        {
            "username": "bob",
            "email": "bob@example.com",
            "firstName": "Bob",
            "lastName": "Approver",
            "enabled": True,
            "emailVerified": True,
            "groups": ["/support", "/support-approvers"],
            "attributes": {"team": ["support"], "clearance": ["confidential"]},
            "credentials": [{"type": "password", "value": "changeme-bob", "temporary": True}],
        },
        {
            # Low-budget user for demo scenario 6 (budget exhaustion).
            "username": "carol",
            "email": "carol@example.com",
            "firstName": "Carol",
            "lastName": "Budget",
            "enabled": True,
            "emailVerified": True,
            "groups": ["/support"],
            "attributes": {"team": ["support"], "clearance": ["internal"]},
            "credentials": [{"type": "password", "value": "changeme-carol", "temporary": True}],
        },
    ]

    return {
        "apiVersion": "k8s.keycloak.org/v2alpha1",
        "kind": "KeycloakRealmImport",
        "metadata": {"name": REALM, "namespace": "keycloak", "labels": {"app.kubernetes.io/part-of": "eacp"}},
        "spec": {
            "keycloakCRName": "keycloak",
            "realm": {
                "realm": REALM,
                "displayName": "Enterprise AI Control Plane",
                "enabled": True,
                "sslRequired": "external",
                "accessTokenLifespan": 900,
                "groups": groups,
                "clientScopes": client_scopes,
                "clients": clients,
                "users": users,
                # TODO(verify): Keycloak 26.8 feature flags for the delegation (actor token) preview and
                # the identity-assertion-jwt experimental feature are set on the Keycloak CR, see
                # platform/keycloak/keycloak.yaml, not in the realm.
            },
        },
    }


def team_namespaces(ms: ManifestSet) -> list[dict]:
    """One Namespace per team plus the ReferenceGrants that let the team's gateway policies
    reach the adapter and Keycloak services across namespaces."""
    docs: list[dict] = []
    for name, m in sorted(ms.teams.items()):
        ns = m["spec"].get("namespace", f"team-{name}")
        docs.append(
            {
                "apiVersion": "v1",
                "kind": "Namespace",
                "metadata": {
                    "name": ns,
                    "labels": {
                        "app.kubernetes.io/part-of": "eacp",
                        "eacp.dev/team": name,
                        "eacp.dev/routes": "true",
                        "pod-security.kubernetes.io/enforce": "restricted",
                        "pod-security.kubernetes.io/enforce-version": "latest",
                    },
                    "annotations": {"eacp.dev/owner": m["metadata"]["owner"]},
                },
            }
        )
    froms = [
        {"group": "agentgateway.dev", "kind": "AgentgatewayPolicy", "namespace": m["spec"].get("namespace", f"team-{n}")}
        for n, m in sorted(ms.teams.items())
    ]
    # TODO(verify): whether agentgateway requires ReferenceGrant for cross-namespace backendRefs
    # in policies. Harmless if it does not.
    docs.append(
        {
            "apiVersion": "gateway.networking.k8s.io/v1beta1",
            "kind": "ReferenceGrant",
            "metadata": {"name": "teams-to-eacp-adapter", "namespace": "eacp-system"},
            "spec": {"from": froms, "to": [{"group": "", "kind": "Service", "name": "eacp-adapter"}]},
        }
    )
    docs.append(
        {
            "apiVersion": "gateway.networking.k8s.io/v1beta1",
            "kind": "ReferenceGrant",
            "metadata": {"name": "teams-to-keycloak", "namespace": "keycloak"},
            "spec": {"from": froms, "to": [{"group": "", "kind": "Service", "name": "keycloak-service"}]},
        }
    )
    return docs


def main() -> int:
    ms = load_all()
    rc = report(ms, quiet=True, warn=False)
    if rc:
        return rc

    gen_dir = ROOT / "platform" / "generated"
    gen_dir.mkdir(parents=True, exist_ok=True)
    header = "# GENERATED by scripts/compile-manifests.py from teams/*/manifest.yaml. Do not edit.\n"
    (gen_dir / "teams.yaml").write_text(header + yaml.safe_dump_all(team_namespaces(ms), sort_keys=False))
    (gen_dir / "kustomization.yaml").write_text(
        "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - teams.yaml\n"
    )

    data_path = ROOT / "policies" / "eacp" / "manifests.json"
    data_path.parent.mkdir(parents=True, exist_ok=True)
    data_path.write_text(json.dumps(to_opa_data(ms), indent=2, sort_keys=True) + "\n")

    realm_path = ROOT / "platform" / "keycloak" / "realm-import.yaml"
    realm_path.parent.mkdir(parents=True, exist_ok=True)
    header = "# GENERATED by scripts/compile-manifests.py from teams/, agents/ and tools/. Do not edit.\n"
    realm_path.write_text(header + yaml.safe_dump(realm_import(ms), sort_keys=False, width=120))

    print(f"wrote {data_path.relative_to(ROOT)}, {realm_path.relative_to(ROOT)} and {gen_dir.relative_to(ROOT)}/teams.yaml")
    return 0


if __name__ == "__main__":
    sys.exit(main())
