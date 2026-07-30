//line test_swarm_js.zero:2
;(async () => {
  console.log(`[Swarm JS] Spawning agent ${"JS_Researcher"} for task: ${"find sources"}`);
  await new Promise(r => setTimeout(r, 100));
  console.log(`[Swarm JS] Agent ${"JS_Researcher"} completed task: ${"find sources"}`);
})();
//line test_swarm_js.zero:3
;(async () => {
  console.log(`[Swarm JS] Spawning agent ${"JS_Writer"} for task: ${"summarize"}`);
  await new Promise(r => setTimeout(r, 100));
  console.log(`[Swarm JS] Agent ${"JS_Writer"} completed task: ${"summarize"}`);
})();
