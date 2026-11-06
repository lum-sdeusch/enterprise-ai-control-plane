# Scaffold to POC: tickets

Tracer-bullet tickets that take the scaffold to a running, demonstrable proof of concept and
then to publication. Each ticket is a complete vertical slice that can be verified on its own,
sized for one working session. Blockers are listed per ticket; any ticket whose blockers are
done can start.

```mermaid
flowchart TD
  T01[01 CI green and local reproduce]
  T02[02 Foundation on kind]
  T03[03 First tool through the gateway with OAuth]
  T04[04 Adapter in the pathway]
  T05[05 Real pins, tool gate, rug-pull quarantine]
  T06[06 web-fetch via kmcp, taint exfil block]
  T07[07 LLM path: routing, prompt guard, failover]
  T08[08 Guard model in budget, injection block]
  T09[09 One-human approval end to end]
  T10[10 LibreChat, LLM-path identity, budgets]
  T11[11 Scheduled agent identity path]
  T12[12 Observability stores and content capture]
  T13[13 Six scenarios automated in CI]
  T14[14 Secrets, signing, supply chain]
  T15[15 cloud-gpu profile on EKS]
  T16[16 Publication]
  T02 --> T03 --> T04
  T04 --> T05 --> T06
  T03 --> T07
  T04 --> T07
  T06 --> T08
  T04 --> T09
  T07 --> T10
  T07 --> T11
  T04 --> T12
  T05 --> T13
  T06 --> T13
  T08 --> T13
  T09 --> T13
  T10 --> T13
  T02 --> T14
  T05 --> T14
  T13 --> T15
  T13 --> T16
  T15 --> T16
```

| # | Ticket | Blocked by |
|---|---|---|
| 01 | CI green and reproducible locally | none |
| 02 | Foundation bring-up on kind | none |
| 03 | First tool through the gateway with OAuth, no adapter | 02 |
| 04 | Adapter in the pathway with real request and response context | 03 |
| 05 | Real pins, a working tool gate, and the rug-pull quarantine (scenario 1) | 04 |
| 06 | web-fetch through kmcp and the taint exfiltration block (scenario 3) | 05 |
| 07 | LLM path: routing callout, prompt guard, failover (scenario 4) | 03, 04 |
| 08 | Guard model within the latency budget and the injection block (scenario 2) | 06 |
| 09 | One-human approval end to end (scenario 5) | 04 |
| 10 | LibreChat with Keycloak, identity on the LLM path, budgets (scenario 6) | 07 |
| 11 | Scheduled agent identity path | 07 |
| 12 | Observability stores, redaction and content capture | 04 |
| 13 | The six scenarios automated in CI | 05, 06, 08, 09, 10 |
| 14 | Secrets seeding, bundle signing, admission verification, delegated credentials | 02, 05 |
| 15 | cloud-gpu profile on EKS | 13 |
| 16 | Publication | 13, 15 |

Frontier at the start: 01 and 02 can run in parallel.
