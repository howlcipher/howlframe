# Venue waiver for the Round 7 isolation rerun (recorded before launch, 2026-10-20 ~01:06 EDT)

The "Isolation rerun terms" in milestone2/MILESTONE2_DECISION.md (main e41b6830) ask for a venue that is
"a non-shared machine with no VM snapshot or fork, no active balloon driver, about 1.5 GB of free disk, and a
declared quiet window". William Elias (Owner) knowingly WAIVED the "non-shared machine with no VM snapshot or fork,
no active balloon driver" part on 2026-10-19 and chose the shared box in an overnight quiet window instead,
because he chose not to spend money on extra compute. All other terms (pins, identical command, fresh artifacts
directory, explicit exit capture, independent non-Claude launcher, one attempt only) are kept unchanged.

The waiver changes no gate: the script's own 120 s idle gate and its noise gates still decide whether the
invocation counts. A VM fork or virtio_balloon kmsg record during the run is still a noise breach.

Known at launch (see venue_evidence.txt): this box is a KVM-style guest (/dev/vdb, virtio), the virtio_balloon
driver is bound (/sys/bus/virtio/drivers/virtio_balloon/virtio0, built in, not a loadable module, so lsmod shows
nothing), and dmesg shows "random: crng reseeded due to virtual machine fork" records at 00:08, 00:23, 00:30,
00:51, 00:57 and 01:04 EDT tonight, the last one about 1 minute before this note.

Declared quiet window: 2026-10-20 from launch until the run ends (or 06:00 EDT at the latest). The operating
session runs no builds, tests, storms or browser work during the run and only waits on the output. It cannot
prove other agents on the shared box stayed idle.
