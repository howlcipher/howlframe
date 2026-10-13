# Network/env follow-ups and runner-owned adapter deadline (#177, #175)

Consumer-driven findings from the #176 reviews (Lain, Motoko) and the #173
review (Motoko). Each item has a negative test that was mutation-checked:
the fix was reverted or weakened locally and the named test failed.

| Finding | Fix | Negative test |
| --- | --- | --- |
| env refusal list missed `GIT_*`, `GCONV_PATH`, `PYTHONHOME`, `PERL5DB`, `JAVA_TOOL_OPTIONS`, `SHELLOPTS`/`BASHOPTS`/`PS4`, `HOME`/`XDG_CONFIG_HOME`, `PATH`, and was duplicated in three files | One table, `capability.CodeLoadingEnvNames` + `CodeLoadingEnvPrefixes`, rendered into generated Go and JS | `nettest.EnvRefused`/`EnvAllowed` run on core (`TestEnvGrantRefusesCodeLoading`), generated Go (`TestCodeLoadingEnv`) and Node (`TestJSCodeLoadingEnv`); `TestSharedTables` / `TestJSSharedTables` compare the rendered tables |
| `Proxy-Authorization` and custom auth headers survived an origin change | Allow-list `capability.RedirectSafeHeaders`; every other header is dropped | `CrossOriginHeaders` (core, generated Go), `TestJSCrossOriginHeadersAndUserinfo` |
| userinfo in redirect targets was followed | Refused in `CheckRedirect` and the Node hop loop | `RedirectUserinfo`, `TestJSCrossOriginHeadersAndUserinfo` |
| Node pinned only the first resolved address | The whole vetted set is pinned; `lookup` honors `all` and `family` | `TestJSLocalhostLoopbackAndPinAll` |
| core and Node disagreed on `[::ffff:a.b.c.d]` | Node canonicalizes mapped literals like `canonicalIP` | `nettest.MappedGrantCases` on all three targets |
| `::ffff:0:0/96` not decoded; `3fff::/20`, `5f00::/16` missing | Decode and add rows | new `BlockedAddresses` / `PublicAddresses` rows; `TestAddressLists` now runs them on generated Go |
| `localhost` opt-out could reach any address | Every resolved address must be loopback | `LocalhostLoopbackOnly`, `TestJSLocalhostLoopbackAndPinAll` |
| #175: adapter budget fixed at 1s | `ExecutionPolicy.AdapterDeadline`, clamped at 30s; `AdapterDeadlineFlag` rejects over-cap values | `TestAdapterDeadlineRunnerOwned`, `TestAdapterDeadlineFlag`, `TestAdapterDeadlineIgnoredForArtifacts` |

Choice: the env list stays a deny-list (single-sourced) rather than an
allow-list, because existing grants for ordinary variables (`LANG`, `TZ`,
app tokens) must keep working; `HOME` and `PATH` are now refused, which is
the one consumer-visible break. Exec children are already granted by
absolute path and start with an empty environment.
