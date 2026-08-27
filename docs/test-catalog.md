# Test catalog

This file is generated. Run `go generate ./...` after changing cases or profiles.

## Scenarios

| ID | Revision | Layer | Stability | Assertions | Title |
| --- | ---: | --- | --- | ---: | --- |
| `ANT-AUTH-001` | 1 | protocol | stable | 3 | Anthropic authentication boundary |
| `ANT-MODL-001` | 1 | protocol | stable | 4 | Anthropic model discovery |
| `ANT-MSG-001` | 1 | protocol | stable | 5 | Anthropic non-streaming message |
| `ANT-SDK-001` | 1 | sdk | stable | 2 | Anthropic Go SDK interoperability |
| `ANT-TOOL-001` | 1 | protocol | stable | 5 | Anthropic forced tool use |
| `BEH-ANT-001` | 1 | behavioral | stable | 1 | Anthropic exact instruction following |
| `BEH-OAI-001` | 1 | behavioral | stable | 1 | OpenAI exact instruction following |
| `CDX-EXEC-001` | 1 | agent | experimental | 4 | Codex non-interactive coding workflow |
| `CLC-EXEC-001` | 1 | agent | experimental | 4 | Claude Code non-interactive coding workflow |
| `OAI-AUTH-001` | 1 | protocol | stable | 3 | OpenAI authentication boundary |
| `OAI-MODL-001` | 1 | protocol | stable | 4 | OpenAI model discovery |
| `OAI-RESP-001` | 1 | protocol | stable | 6 | OpenAI non-streaming response |
| `OAI-SDK-001` | 1 | sdk | stable | 2 | OpenAI Go SDK interoperability |
| `OAI-TOOL-001` | 1 | protocol | stable | 5 | OpenAI forced function call |

## Profiles

| ID | Version | Included profiles | Required references | Title |
| --- | --- | --- | ---: | --- |
| `anthropic-core` | 1.0.0 |  | 3 | Anthropic Core |
| `anthropic-sdk-go` | 1.0.0 |  | 1 | Anthropic Go SDK |
| `anthropic-tools` | 1.0.0 | anthropic-core | 1 | Anthropic Tool Calling |
| `behavioral-anthropic` | 1.0.0 |  | 1 | Anthropic Behavioral Diagnostics |
| `behavioral-openai` | 1.0.0 |  | 1 | OpenAI Behavioral Diagnostics |
| `claude-code-ready` | 1.0.0 | anthropic-tools, anthropic-sdk-go | 1 | Claude Code Ready |
| `codex-ready` | 1.0.0 | oai-tools, oai-sdk-go | 1 | Codex Ready |
| `oai-core` | 1.0.0 |  | 3 | OpenAI Core |
| `oai-sdk-go` | 1.0.0 |  | 1 | OpenAI Go SDK |
| `oai-tools` | 1.0.0 | oai-core | 1 | OpenAI Tool Calling |
