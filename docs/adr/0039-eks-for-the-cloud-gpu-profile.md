# ADR 0039: EKS for the cloud-gpu profile

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q39

## Context

The profile is cloud-agnostic through the NVIDIA GPU Operator, but one path must be tested.

## Decision

EKS. The gateway, Keycloak and LibreChat hostnames are the only profile-specific values.
