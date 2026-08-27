# Test case roadmap proposal

Status: proposed  
Last reviewed: 2026-08-27

## Summary

AI Gateway Testkit currently provides 14 scenarios covering authentication, model discovery, minimal non-streaming generation, one forced tool call, official Go SDK deserialization, exact-output diagnostics, and a basic coding-agent workflow.

This proposal reserves a structured backlog of 81 additional scenarios. The first 36 scenarios close core OpenAI and Anthropic protocol and SDK gaps. The remaining scenarios form opt-in resilience, security, coding-agent, multimodal, asynchronous, and behavioral profiles.

The proposal does not add executable scenarios or change an existing compatibility claim. A proposed ID becomes reportable only after its executable definition, deterministic tests, generated JSON and Markdown artifacts, and intended profile change are merged.

## Goals

- Define a shared, reviewable compatibility roadmap before implementation begins.
- Prevent duplicate community work and case-ID collisions.
- Separate core protocol claims from optional, costly, mutating, probabilistic, and operational capabilities.
- Make implementation order follow runner prerequisites instead of endpoint popularity.
- Preserve actionable results through atomic assertions and narrow failure ownership.

## Non-goals

- Requiring every gateway to implement every provider endpoint.
- Treating model quality as protocol compatibility.
- Generating live rate limits by flooding a target.
- Exercising production data, credentials, files, or prompts.
- Adding proposed cases to existing profiles before executable evidence exists.

## ID reservation and lifecycle

Once this proposal is accepted, the IDs below are reserved and must not be reused for a different observable contract. Reservation does not add the ID to the executable catalog.

Implementation of a reserved case must:

1. use exactly one ID-named Go source file;
2. define atomic assertions, stable reason codes, cost, risk, determinism, and dependencies;
3. cover success, incompatibility, and unavailable-evidence paths with local deterministic fixtures;
4. generate `catalog/cases/<ID>.json` and `docs/cases/<ID>.md`;
5. update only the profiles whose compatibility meaning intentionally changes;
6. increment the catalog version according to the release policy.

If review changes the meaning of an unimplemented proposal substantially, allocate a new ID rather than silently repurposing an accepted reservation.

## Current baseline

| Family | Existing scenarios |
| --- | --- |
| OpenAI | `OAI-AUTH-001`, `OAI-MODL-001`, `OAI-RESP-001`, `OAI-TOOL-001`, `OAI-SDK-001` |
| Anthropic | `ANT-AUTH-001`, `ANT-MODL-001`, `ANT-MSG-001`, `ANT-TOOL-001`, `ANT-SDK-001` |
| Behavioral | `BEH-OAI-001`, `BEH-ANT-001` |
| Coding agents | `CDX-EXEC-001`, `CLC-EXEC-001` |

These scenarios remain valid. New scenarios should depend on them rather than broaden their meaning or assertions without a revision.

## P0: OpenAI protocol and SDK baseline

OpenAI Responses and Chat Completions are distinct wire surfaces with different response and streaming representations. A gateway should claim compatibility with them through separate profiles.

| Proposed ID | Observable contract | Prerequisite |
| --- | --- | --- |
| `OAI-HDR-001` | Successful responses use an appropriate media type, expose a non-empty request identifier when supported, and do not reflect credentials. | `OAI-AUTH-001` |
| `OAI-ERR-001` | Malformed JSON, missing required fields, unknown models, and unsupported parameters produce a bounded JSON error envelope with a stable HTTP classification. | `OAI-AUTH-001` |
| `OAI-MODL-002` | Retrieving the configured model by ID returns a correctly discriminated model object. | `OAI-MODL-001` |
| `OAI-RESP-002` | A completed response exposes valid identifiers, timestamps, status, model attribution, and non-negative usage counters. | `OAI-RESP-001` |
| `OAI-RESP-003` | A deliberately constrained output terminates as incomplete with a machine-readable reason rather than a malformed completed response. | `OAI-RESP-001` |
| `OAI-STRM-001` | A text Responses stream uses SSE, follows a valid event lifecycle, reconstructs non-empty text, and terminates cleanly. | streaming transport, `OAI-RESP-001` |
| `OAI-STRM-002` | Streaming function-call argument deltas reconstruct the requested call and valid JSON arguments. | `OAI-STRM-001`, `OAI-TOOL-001` |
| `OAI-STRC-001` | Structured output conforms to the supplied JSON Schema, including required fields and additional-property policy. | `OAI-RESP-001` |
| `OAI-TOOL-002` | A function call, client `function_call_output`, and final assistant response form a complete correlated tool loop. | `OAI-TOOL-001` |
| `OAI-TOOL-003` | `none`, `auto`, `required`, and named function choices are honored with the correct output representation. | `OAI-TOOL-001` |
| `OAI-STATE-001` | `previous_response_id` carries conversation state into a second response without corrupting instructions or output; any stored response is cleaned up. | `OAI-RESP-001`, resource tracker |
| `OAI-STATE-002` | A stateless continuation can replay required output items with storage disabled. | `OAI-RESP-001` |
| `OAI-CHAT-001` | A non-streaming Chat Completions request returns the expected envelope, choice index, assistant role, finish reason, model, and usage. | `OAI-AUTH-001` |
| `OAI-CHAT-002` | Chat Completions SSE chunks reconstruct the message and expose a terminal finish reason and usage when requested. | streaming transport, `OAI-CHAT-001` |
| `OAI-CHAT-003` | A Chat Completions function call and tool message are correlated by tool-call ID through a final response. | `OAI-CHAT-001` |
| `OAI-CHAT-004` | Chat Completions structured output conforms to the supplied JSON Schema. | `OAI-CHAT-001` |
| `OAI-SDK-002` | The official OpenAI Go SDK consumes and accumulates a Responses stream through the gateway. | `OAI-STRM-001` |
| `OAI-SDK-003` | The official SDK exposes a typed API error, status code, bounded body, and request metadata without credential leakage. | `OAI-ERR-001` |
| `OAI-SDK-004` | The official SDK completes a function-call round trip through the gateway. | `OAI-TOOL-002` |

References:

- [OpenAI Responses API](https://developers.openai.com/api/reference/cli/resources/responses/methods/create)
- [OpenAI Chat Completions API](https://developers.openai.com/api/reference/cli/resources/chat/subresources/completions)

## P0: Anthropic protocol and SDK baseline

| Proposed ID | Observable contract | Prerequisite |
| --- | --- | --- |
| `ANT-HDR-001` | The configured Anthropic version is accepted; a missing or invalid version receives a bounded, correctly classified error. | `ANT-AUTH-001` |
| `ANT-ERR-001` | Invalid requests return the Anthropic error discriminator, typed error object, message, and request identifier. | `ANT-AUTH-001` |
| `ANT-MODL-002` | Retrieving the configured model by ID returns a correctly discriminated model object. | `ANT-MODL-001` |
| `ANT-MSG-002` | A message exposes valid identifiers, assistant role, model, stop reason, and non-negative usage counters. | `ANT-MSG-001` |
| `ANT-MSG-003` | A deliberately constrained response terminates with `max_tokens` rather than an invalid success shape. | `ANT-MSG-001` |
| `ANT-MSG-004` | A top-level system prompt and stateless message roles are accepted with the documented message representation. | `ANT-MSG-001` |
| `ANT-STRM-001` | A text stream follows the message-start, content, delta, and message-stop SSE lifecycle and reconstructs non-empty text. | streaming transport, `ANT-MSG-001` |
| `ANT-STRM-002` | Fine-grained tool-input deltas reconstruct a valid `tool_use` block and structured input. | `ANT-STRM-001`, `ANT-TOOL-001` |
| `ANT-STRM-003` | Ping and forward-compatible unknown events do not corrupt the stream; final usage remains valid. | `ANT-STRM-001` |
| `ANT-STRC-001` | `output_config.format` produces JSON conforming to the supplied schema. | `ANT-MSG-001` |
| `ANT-TOOL-002` | A `tool_use` block, correlated `tool_result`, and final assistant response form a complete tool loop. | `ANT-TOOL-001` |
| `ANT-TOOL-003` | Auto, any, named-tool, and none choices produce the documented content blocks and stop reasons. | `ANT-TOOL-001` |
| `ANT-STATE-001` | Replaying stateless conversation history produces a valid second assistant turn without role corruption. | `ANT-MSG-001` |
| `ANT-TOKN-001` | Token counting accepts the same core message shape and returns a non-negative input-token count without generation. | `ANT-MSG-001` |
| `ANT-SDK-002` | The official Anthropic Go SDK consumes and accumulates a Messages stream through the gateway. | `ANT-STRM-001` |
| `ANT-SDK-003` | The official SDK exposes `*anthropic.Error`, status code, bounded body, and raw response metadata without credential leakage. | `ANT-ERR-001` |
| `ANT-SDK-004` | The official SDK completes a tool-result round trip through the gateway. | `ANT-TOOL-002` |

References:

- [Anthropic Messages API](https://platform.claude.com/docs/en/api/http/messages/create)
- [Anthropic streaming](https://platform.claude.com/docs/en/build-with-claude/streaming)
- [Anthropic structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
- [Anthropic API errors](https://platform.claude.com/docs/en/api/errors)

## P1: deterministic resilience and security

These scenarios require a controlled fault-injection proxy or local fixture. They must never manufacture rate limits or overload by sending abusive traffic to a live gateway.

| Proposed ID | Observable contract | Execution class |
| --- | --- | --- |
| `GWY-RTRY-001` | A synthetic `429` with `Retry-After` is retried within the configured bound. | resilience, deterministic fixture |
| `GWY-RTRY-002` | A transient `500`, `502`, `503`, `504`, or Anthropic `529` can recover on a later attempt. | resilience, deterministic fixture |
| `GWY-RTRY-003` | Exhausted transient attempts become unavailable evidence rather than incompatibility. | resilience, deterministic fixture |
| `GWY-RTRY-004` | Stable `400`, `401`, `403`, and `404` responses are not retried. | resilience, deterministic fixture |
| `GWY-TIME-001` | Deadline or cancellation interrupts an in-flight request and releases resources. | resilience, deterministic fixture |
| `GWY-STRM-001` | A mid-stream disconnect produces a bounded stream error and never a partial pass. | resilience, deterministic fixture |
| `GWY-HTTP-001` | Chunked and gzip-encoded responses, header casing, and connection reuse preserve the logical protocol result. | transport, deterministic fixture |
| `SEC-CRED-001` | Valid and invalid credentials do not appear in response evidence, diagnostics, logs, or canonical reports. | security |
| `SEC-REDR-001` | A cross-host redirect cannot receive the original Authorization or API-key header. | security, deterministic fixture |
| `SEC-TLS-001` | Certificate and hostname verification remain enabled for remote targets. | security, deterministic fixture |
| `SEC-BODY-001` | Oversized, truncated, or malformed bodies are bounded, classified, and redacted. | security, deterministic fixture |

## P1: coding-agent readiness

The existing agent scenarios prove only a minimal filesystem and shell effect. Readiness version 2 should require representative coding work plus isolation guarantees.

| Codex ID | Claude Code ID | Observable contract |
| --- | --- | --- |
| `CDX-EXEC-002` | `CLC-EXEC-002` | Read multiple files, make a scoped code edit, and pass a deterministic unit test. |
| `CDX-EXEC-003` | `CLC-EXEC-003` | Observe an initial command or test failure, diagnose it, and recover successfully. |
| `CDX-EXEC-004` | `CLC-EXEC-004` | Complete multiple correlated tool calls while handling bounded or truncated tool output. |
| `CDX-SECU-001` | `CLC-SECU-001` | The raw gateway credential is unavailable to workspace files, commands, and agent output. |
| `CDX-SECU-002` | `CLC-SECU-002` | Network access outside the configured gateway host remains denied. |
| `CDX-CLNP-001` | `CLC-CLNP-001` | Timeout, cancellation, or non-zero exit still removes the named sandbox and temporary files. |

Agent scenarios remain operational and experimental. They must use disposable fixtures, never mount the testkit repository, and never infer protocol failure solely from model behavior.

## P2: optional OpenAI capabilities

These scenarios require separate opt-in profiles and, where applicable, dedicated target model fields.

| Proposed ID | Observable contract | Additional requirement |
| --- | --- | --- |
| `OAI-EMBD-001` | Single and batch embeddings return ordered finite vectors, requested dimensions or encoding, and valid usage. | embedding model |
| `OAI-VISN-001` | A synthetic image input is accepted and produces a non-empty text response. | vision-capable model |
| `OAI-FILE-001` | File upload, metadata retrieval, content retrieval, and deletion form a cleaned-up lifecycle. | raw multipart transport |
| `OAI-BTCH-001` | Batch creation, polling, result correlation, cancellation, and cleanup preserve custom request IDs. | polling and resource tracker |
| `OAI-BGND-001` | A background response can be created, polled to a terminal state, and cancelled when still active. | polling and resource tracker |
| `OAI-MODR-001` | Moderation returns a correctly discriminated result with bounded category data. | moderation model or endpoint |
| `OAI-AUDO-001` | Synthetic speech or transcription data follows the documented binary and metadata contract. | audio model, binary transport |
| `OAI-IMAG-001` | Image generation or editing returns a valid URL or bounded encoded payload. | image model, binary transport |
| `OAI-REAL-001` | A Realtime WebSocket session completes authentication, session setup, one response, and clean close. | WebSocket transport |
| `OAI-MCP-001` | A controlled MCP server can be discovered and called with correctly correlated results. | isolated local MCP fixture |

References:

- [OpenAI embeddings](https://developers.openai.com/api/reference/ruby/resources/embeddings/methods/create)
- [OpenAI files](https://developers.openai.com/api/reference/cli/resources/files)

## P2: optional Anthropic capabilities

| Proposed ID | Observable contract | Additional requirement |
| --- | --- | --- |
| `ANT-VISN-001` | A synthetic image content block is accepted and produces a non-empty text block. | vision-capable model |
| `ANT-DOCS-001` | A synthetic document or PDF input returns valid text and bounded citation metadata when requested. | document transport |
| `ANT-CACH-001` | Cache creation followed by cache reuse produces coherent creation, read, and uncached usage counters. | sufficiently large synthetic prefix |
| `ANT-BTCH-001` | Message Batch create, poll, results, cancel, and delete preserve unique `custom_id` correlation. | polling and resource tracker |
| `ANT-FILE-001` | File upload, metadata, download, and deletion form a cleaned-up lifecycle. | raw multipart and binary transport |
| `ANT-CITE-001` | Non-streaming citations and streaming citation deltas reference the intended synthetic source. | document-capable model |
| `ANT-THNK-001` | Thinking blocks and signatures can be replayed without corruption when the capability is enabled. | thinking-capable model |
| `ANT-MCP-001` | A controlled MCP connector can discover and call a local synthetic tool. | isolated local MCP fixture |

References:

- [Anthropic prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)
- [Anthropic Message Batches](https://platform.claude.com/docs/en/api/http/messages/batches/create)

## Behavioral diagnostics

Behavioral scenarios are probabilistic diagnostics and must never be included in protocol compatibility claims.

| Proposed ID | Observable contract |
| --- | --- |
| `BEH-OAI-002` | A developer or system instruction wins over a conflicting user instruction. |
| `BEH-ANT-002` | A top-level system instruction wins over a conflicting user instruction. |
| `BEH-OAI-003` | Repeated exact-output trials report sample count and pass rate without converting variance into protocol failure. |
| `BEH-ANT-003` | Repeated exact-output trials report sample count and pass rate without converting variance into protocol failure. |

## Required runner capabilities

Implementation should begin with shared primitives, not case-specific transport code.

### Streaming

- Add a protocol-neutral streaming request contract alongside `DoJSON`.
- Parse SSE with byte, event-count, line-length, and duration limits.
- Preserve event order and unknown event types for forward-compatible case logic.
- Distinguish a clean terminal event, caller cancellation, timeout, and interrupted transport.
- Never retry after observable stream output has been delivered.

### Raw request and response modes

- Support raw, multipart, binary, SSE, and WebSocket modes without weakening existing JSON limits.
- Allow a case to deliberately omit or replace protocol-version headers while keeping normal credential injection safe.
- Keep credentials and sensitive headers out of evidence.
- Track content type and a small allowlist of diagnostic response headers.

### Target capabilities

Generation, embedding, image, audio, and realtime workloads may require different model IDs. The target manifest should add explicit optional capability fields rather than overload the current generation model.

Unsupported optional capabilities should be `not_applicable` only when the target manifest did not claim them. A configured capability that violates its contract is a failure.

### Resource lifecycle

Mutating cases need a per-run resource tracker with bounded polling, cancellation, and reverse-order cleanup. Cleanup failure must be visible in evidence and affect the case result without hiding the original failure.

### Fault injection

Retry, timeout, redirect, malformed-body, and interrupted-stream cases need a deterministic local or explicitly configured proxy. The default live run must not attempt to provoke rate limits, overload, or large-request failures.

## Profile plan

Profiles should be introduced only as their executable cases land.

| Profile | Intended claim |
| --- | --- |
| `oai-responses-core` | Authentication, discovery, Responses metadata, errors, and non-streaming generation without required server storage. |
| `oai-responses-state` | Stateful `previous_response_id` and storage-free replay compatibility. |
| `oai-responses-streaming` | Responses text and tool-call SSE compatibility. |
| `oai-chat-core` | Non-streaming Chat Completions compatibility. |
| `oai-chat-streaming` | Chat Completions SSE compatibility. |
| `oai-chat-tools` | Chat Completions function-tool round trips. |
| `oai-structured` | Responses and Chat Completions JSON Schema output. |
| `anthropic-messages-core` | Headers, errors, discovery, Messages metadata, state, and token counting. |
| `anthropic-streaming` | Text and tool-input Messages SSE compatibility. |
| `anthropic-structured` | Anthropic structured JSON output. |
| `gateway-resilience` | Deterministic retry, timeout, and interrupted-transport behavior. |
| `gateway-security` | Credential, redirect, TLS, and bounded-body protections. |
| `codex-ready-v2` | Representative coding workflow plus sandbox isolation and cleanup. |
| `claude-code-ready-v2` | Representative coding workflow plus sandbox isolation and cleanup. |

Embedding, files, batch, background, vision, audio, image, realtime, MCP, caching, citations, and thinking should each remain narrow opt-in profiles or optional references. They should not be folded into provider core profiles.

## Delivery order

1. Streaming transport and deterministic SSE fixtures.
2. Provider header, error, metadata, and usage scenarios.
3. Streaming text, streaming tools, and full tool-result loops.
4. Structured output and multi-turn state.
5. Chat Completions and official SDK streaming/error paths.
6. Deterministic resilience and security fixtures.
7. Coding-agent readiness version 2.
8. Optional model-, storage-, and asynchronous capability packs.

Each implementation pull request should normally add one scenario. Closely coupled pairs may share a pull request only when they use the same new runner primitive and remain independently identifiable.

## Acceptance criteria for this proposal

- Proposed IDs match the repository ID grammar and do not collide with the executable catalog.
- Core, optional, probabilistic, mutating, and operational scopes are explicitly separated.
- Live tests remain bounded and safe for third-party gateways.
- Streaming, raw transport, fault injection, target capability, and cleanup prerequisites are identified before case implementation.
- Existing compatibility profiles remain unchanged until executable evidence lands.
