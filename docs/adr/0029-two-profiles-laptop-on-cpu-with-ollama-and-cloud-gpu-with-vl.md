# ADR 0029: Two profiles, laptop on CPU with Ollama and cloud-gpu with vLLM

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q29

## Context

The POC must run on a 16 GB laptop without a GPU and also show the full-size models on a
GPU node. Qwen3Guard is not in Ollama's official library, only as community GGUF files.

## Decision

Profile laptop: Ollama serving a small Qwen3 general model and Qwen3Guard-Gen 0.6B from a
community GGUF, judge off. Profile cloud-gpu: vLLM serving Qwen3Guard-Gen 4B, Granite
Guardian 4.1 8B and a larger in-house model, on EKS with the NVIDIA GPU Operator.

## Alternatives considered

- **Single profile.** Either unrunnable on a laptop or unconvincing on stage.

## Consequences

- Model endpoints reach the guardrail service through a per-profile ConfigMap.

## To verify at build time

- The Hugging Face repository and quantisation of a Qwen3Guard-Gen 0.6B GGUF.
- Whether a roughly 200M-parameter DeBERTa prompt-injection classifier (89 percent at 54 ms in Red Hat's October 2026 benchmark) has a permissive license; if so it becomes the laptop default.
