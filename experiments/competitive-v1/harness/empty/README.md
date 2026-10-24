# Empty programs

One do-nothing program per arm, built and run the same way agents are asked to structure their work (`./build`
if present, then `./run`). The scorer times each one in the scoring jail and subtracts its median from the
task's runtime (protocol §6, "minus a measured empty-program startup for that arm").
