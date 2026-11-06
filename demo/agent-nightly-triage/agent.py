"""nightly-triage: the scheduled agent (Pydantic AI, MIT).

What it demonstrates: an agent with no human in the loop still has a verifiable identity
(Kubernetes service-account token federated into Keycloak), a named owner, a tool allow-list,
no external model, and a confidential ceiling, all enforced by the control plane rather than by
this code. The agent uses the gateway's virtual model and the single MCP endpoint like any
other client.

Token flow (ADR 0023):
  1. Read the projected service-account token (audience: keycloak).
  2. Exchange it at Keycloak for an access token for client agent-nightly-triage.
     TODO(verify): Keycloak 26.8 Kubernetes service-account federation parameters; the laptop
     profile falls back to client credentials with the client secret from the secrets manager.
"""

from __future__ import annotations

import os
import sys
import uuid

import httpx
from pydantic_ai import Agent
from pydantic_ai.mcp import MCPServerStreamableHTTP
from pydantic_ai.models.openai import OpenAIChatModel
from pydantic_ai.providers.openai import OpenAIProvider

GATEWAY = os.environ["EACP_GATEWAY_URL"].rstrip("/")
TOKEN_URL = os.environ["EACP_KEYCLOAK_TOKEN_URL"]
CLIENT_ID = os.environ.get("EACP_CLIENT_ID", "agent-nightly-triage")
CONVERSATION_ID = f"nightly-triage:{uuid.uuid4()}"


def obtain_token() -> str:
    sa_token_file = os.environ.get("EACP_SA_TOKEN_FILE")
    if sa_token_file and os.path.exists(sa_token_file):
        subject_token = open(sa_token_file, encoding="utf-8").read().strip()
        data = {
            "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
            "client_id": CLIENT_ID,
            "subject_token": subject_token,
            "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
            "audience": "eacp-llm",
        }
    else:  # laptop fallback
        data = {"grant_type": "client_credentials", "client_id": CLIENT_ID, "client_secret": os.environ["EACP_CLIENT_SECRET"]}
    resp = httpx.post(TOKEN_URL, data=data, timeout=10)
    resp.raise_for_status()
    return resp.json()["access_token"]


def main() -> int:
    token = obtain_token()
    headers = {"Authorization": f"Bearer {token}", "x-eacp-conversation-id": CONVERSATION_ID}

    # Every call carries the conversation id so taint and traces line up (ADR 0027, 0043).
    model = OpenAIChatModel(
        "eacp-auto",
        provider=OpenAIProvider(base_url=f"{GATEWAY}/v1", api_key=token, http_client=httpx.AsyncClient(headers=headers)),
    )
    tools = MCPServerStreamableHTTP(f"{GATEWAY}/mcp", headers=headers)
    agent = Agent(
        model,
        toolsets=[tools],
        system_prompt=(
            "You are the nightly support triage agent. List the open tickets, group them by "
            "theme, and create one ticket titled 'Nightly triage summary' with a short summary. "
            "Never include customer contact details in the summary."
        ),
    )
    Agent.instrument_all()  # OpenTelemetry GenAI spans to OTEL_EXPORTER_OTLP_ENDPOINT

    result = agent.run_sync("Run the nightly triage.")
    print(result.output)
    return 0


if __name__ == "__main__":
    sys.exit(main())
