# Mission007 Frame compatibility

The public v0.1.0 compiler writes HFBC1. Plane's current CI-pinned compiler
7631ce5d4008534ef681f35c3436922507d3f441 writes HFBC2, which the published
runtime correctly rejects. Both advertised format1; that was correct for the
published writer but incorrect for the unreleased HFBC2 writer. This change
derives the CLI/interop advertised write format from bytecode.ArtifactVersion;
it does not change the artifact ABI, writer, accepted readers or capabilities.

The pinned runtime also misreads old enum ordinals (legacy CONVERT becomes
CALL). That defect was already fixed upstream by
41d56cecd9e8f936cf9da35240331ab7da0244cd, which resolves known stable names
before unchanged structural validation. No duplicate decoder fix is required.

Consumer options: ship dual artifacts (selection complexity), change to source
interpretation (both runtimes reject evaluator try_let, so not supported), or
compile portable source with the published compiler and retain one HFBC1
artifact. The last option needs three evaluator boolean-symbol comparisons
expressed as equality expressions because v0.1.0 compiles bare true as an
undefined variable. The resulting artifact executes on published v0.1.0 and
existing fixed reader41d56cec/current5d298. No new Frame runtime release is
necessary for that consumer migration; CI must stop pinning the older reader.

Actual probes used temporary synthetic candidate files. Published v0.1.0,
41d56cec and current5d298 each execute all8 status/authority cases for the
portable HFBC1 artifact.41d56cec and current5d298 also execute all8 canonical
HFBC2 cases; published v0.1.0 rejects all8 as expected. Legacy broken artifacts
and intermediate failures remain in the external Mission007 evidence store.
Successful artifact loading is not authorization; finite VM limits and explicit
filesystem grants remain required. No production action was performed.

The format regression fails against unchanged5d298 output (advertised1,
actual header2). Final repository gates and independent review are recorded in
the PR and Mission007 evidence; source unit tests are not evidence of deployed
owner authentication or published release compatibility beyond these probes.
