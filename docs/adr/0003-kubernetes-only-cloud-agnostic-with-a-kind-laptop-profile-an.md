# ADR 0003: Kubernetes only, cloud-agnostic, with a kind laptop profile and an EKS GPU profile

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q3

## Context

docker-compose is easier to try, but the credible parts of the story are Kubernetes-native:
operator-driven onboarding, sandboxed pods, network policies, GitOps, and the strongest
gateway candidates. Two deployment paths would diverge within weeks.

## Decision

Kubernetes only. The laptop profile brings up a kind cluster with one command; the cloud-gpu
profile targets EKS with the NVIDIA GPU Operator. Cloud-specific parts are confined to the
profile overlays.

## Alternatives considered

- **docker-compose as well.** Doubles the maintenance and loses admission control, network policy and the operators.

## Consequences

- Readers need Docker, kind, kubectl and helm; the quick start says so.
- Everything is a Helm chart or a kustomization so Argo CD can own it.
