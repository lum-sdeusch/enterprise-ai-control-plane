"""Signing keys and tokens for the pathway test. Stands in for Keycloak: an RSA key, its JWKS,
and tokens shaped like the ones the eacp realm issues (groups, team, clearance, act).

    mint.py keys DIR                      write DIR/key.pem and DIR/jwks.json
    mint.py token DIR --sub alice ...     print a token
"""

from __future__ import annotations

import argparse
import base64
import json
import sys
import time
from pathlib import Path

import jwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

ISSUER = "https://auth.pathway.test/realms/eacp"
KID = "pathway-1"


def _b64(n: int) -> str:
    return base64.urlsafe_b64encode(n.to_bytes((n.bit_length() + 7) // 8, "big")).rstrip(b"=").decode()


def keys(directory: Path) -> None:
    directory.mkdir(parents=True, exist_ok=True)
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    (directory / "key.pem").write_bytes(
        key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption())
    )
    pub = key.public_key().public_numbers()
    jwks = {"keys": [{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": KID, "n": _b64(pub.n), "e": _b64(pub.e)}]}
    (directory / "jwks.json").write_text(json.dumps(jwks))


def token(
    directory: Path,
    sub: str,
    groups: list[str],
    team: str = "support",
    clearance: str | None = "restricted",
    act: str | None = "agent-support-assistant",
    azp: str = "librechat",
    aud: str = "eacp-mcp",
    ttl: int = 600,
) -> str:
    now = int(time.time())
    claims: dict = {
        "iss": ISSUER, "sub": sub, "aud": [aud], "azp": azp, "iat": now, "exp": now + ttl,
        "groups": groups, "team": team, "preferred_username": sub,
    }
    if clearance:
        claims["clearance"] = clearance
    if act:
        claims["act"] = {"client_id": act}
    return jwt.encode(claims, (directory / "key.pem").read_text(), algorithm="RS256", headers={"kid": KID})


def main() -> int:
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    k = sub.add_parser("keys")
    k.add_argument("dir", type=Path)
    t = sub.add_parser("token")
    t.add_argument("dir", type=Path)
    t.add_argument("--sub", required=True)
    t.add_argument("--groups", default="support")
    t.add_argument("--team", default="support")
    t.add_argument("--clearance", default="restricted")
    t.add_argument("--act", default="agent-support-assistant")
    t.add_argument("--azp", default="librechat")
    t.add_argument("--aud", default="eacp-mcp")
    args = ap.parse_args()
    if args.cmd == "keys":
        keys(args.dir)
        return 0
    print(token(args.dir, args.sub, [g for g in args.groups.split(",") if g], args.team, args.clearance or None, args.act or None, args.azp, args.aud))
    return 0


if __name__ == "__main__":
    sys.exit(main())
