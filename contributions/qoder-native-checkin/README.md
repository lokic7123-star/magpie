# Qoder native check-in patch for Magpie #2696

Apply `qoder-native-checkin.patch` to upstream `dee5c9d75f452756752dbbed3b46308f6d8edffc` with `git am`.

This refreshes the previously reviewed source proposal (6c294bd + 6116d3f). It restores the existing Qoder/Qoder CN campaign recheck policy and manual action for inactive results. Other plugins retain daily caching. No new hook field, scheduler, timestamp handling or live account request is introduced.

The patch contains six source/document/test files. The patch-file commit is only a delivery artifact, not a request to merge a patch directory into upstream. Applying the patch produces the actual source change. Source proposal: https://github.com/yetone/magpie/issues/2696 .

Verified on Windows against the exact upstream base above:
- Original source fails the native Qoder/Qoder CN manual/interval and completed-cache regressions. The unmodified complete page has no manual action for an inactive Qoder result.
- Final plugin contract/regression gate and ten Chromium/WebKit complete-page checks pass, with five languages and narrow/wide viewports. Thirty-four neighboring check-in/i18n GUI checks pass.
- All-package Go test compilation, Windows/Linux vet and builds pass. The existing script budgets pass without raising thresholds.
- The full Windows runtime suite is NOT green: TestAsideOfflineDisconnectRestoresWithoutDaemon fails, and the agent package later times out at three minutes. The exact AsideOffline assertion also fails in a clean checkout of the same upstream base. The broad run was then stopped. A later broad check-in repeat hit Windows loopback socket errors; the isolated WorkBuddy contrast passed, so the repeat failure is preserved without claiming its root cause is fixed.

No real inference, reward claim, production deployment or balance verification was performed. Maintainer review and release remain pending. Prepared with AI assistance. No credentials, private accounts, binaries, raw logs or local handoff records are included.
