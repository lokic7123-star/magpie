# DSH compaction output-reservation verification

## Confirmed failure chain

The generated DSH descriptor for Space Bunny listed `contextWindow: 524288` and `maxTokens: 524288`. In the inspected installed DSH, explicitly configured model `maxTokens` becomes the effective request default. Its compaction budget computes `contextWindow - reservedCompletionTokens - headroomTokens`; a full-window reservation leaves no message budget. The automatic pre-step hook warns and continues the turn after this configuration error.

Once a history is oversized, manual compaction still sends that history prefix to the summarizer. A read-only replay of the original failed session, using the installed session fold, message conversion and SDK budget functions, produced:

| Check | Result |
| --- | --- |
| Surface nodes / projected messages | 1737 / 1735 |
| Final node kept by zero-retention manual selection | Projects to no message |
| SDK estimated historical input, excluding the appended summary instruction | 577694 tokens |
| Original model window | 524288 |
| SDK `clampMaxTokensToContext`, requested summary output 65536 | 1 token |
| Same history, changing only target window to 1048576 | 65536 output tokens |
| Original automatic reservation 524288 | `leaving no message budget` |
| Corrected reservation 65536 | threshold 393216; retain 73400 |

The summary instruction can only increase pressure, so omitting it is a conservative bound. The matching gateway record returned HTTP 200 with one output token, and the durable session recorded `summarization truncated at the token cap (incomplete checkpoint)`. The log association is temporal; no cross-log shared request ID was obtained. Private prompts, tools, account data and the original session are not included here.

This explains the observed one-token cap without relying on the separate finding that Magpie's compaction prompt detector does not recognize the DSH instruction. The failing route named a direct model, not a compact routing group.

## Scoped fix

Both DSH descriptor writers use `dshMaxTokens`. Only an unconfigured vendor output ceiling that fills or exceeds its entire known window is trimmed to at most 65536 and one quarter of that window. Ordinary outputs, the true context window and explicit user output settings are preserved.

This repairs the unusable automatic-compaction default. It does not expand the model, shrink an already oversized conversation, alter DSH's compaction policy or overrule an explicitly selected full-window output. A window below DSH's configured headroom still needs a suitable DSH policy.

## Checks

- The old implementation failed the full-window and above-window behavior cases; the failure was an output-budget assertion, not a compilation failure.
- `go test ./internal/agent -run '^TestDsh' -count=1`: passed.
- The patch was applied as an independent delta on upstream `33dbdcc` and all DSH checks passed again.
- The regression covers every generated protocol route, the legacy writer, ordinary ceilings, explicit overrides and unknown capacity metadata.
- An additional whole-agent-package run was interrupted after a prolonged silent run; it is not counted as passing. No full-repository result is claimed.
- Original session replay and installed-budget evaluation made zero inference requests and did not mutate a user session or production configuration.
- A second check loaded the installed DSH modules directly through its Electron Node mode, without copied function bodies or the older source checkout. The original history passed through the real BasicCompactionEngine summarizer, PiAiAdapter and SDK to a local fake upstream. The old window actually sent max output 1 and reproduced the exact MAX_TOKENS / incomplete-checkpoint error; the larger-window control actually sent 65536 and produced a complete summary block. Both outcomes are asserted.
- `internal/agent/testdata/dsh-compaction-replay.mjs` provides this independent installed-DSH check with public synthetic history by default. Set `DSH_APP_ASAR` to the installed app.asar and run the script with that app executable in `ELECTRON_RUN_AS_NODE=1` mode. Optional `--events` consumes an already-decoded event-prefix JSON array on stdin, only in memory; do not publish private event files. The check has no real vendor/model call and does not persist a session.

## Remaining acceptance

The native SDK replay proves that a sufficiently larger target can carry this history and retain the intended output budget. Actual summary generation and durable commit of the user's existing long conversation have not yet been verified. The inference-call budget was exhausted; a separate one-call, free-model-only acceptance request is pending. No extra call, automatic retry or paid fallback was made.

Current user configuration has removed Space Bunny and enables larger free models. This patch did not restore the removed model. It is a source proposal, not an installed EXE update or a claim of upstream acceptance.
