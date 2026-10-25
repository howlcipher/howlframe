"""Task draw from the NIST Randomness Beacon (protocol §3.2 step 4, as pinned by Revision 3 G3).

Pinned rules, so that any two implementers derive the same draw:

1. **Pulse.** `parameters.json` records `beacon_pulse_not_before_utc`, a whole minute at least 24 hours after the
   manifest commit. Fetch `https://beacon.nist.gov/beacon/2.0/pulse/time/<ms>` for that minute. If the returned
   pulse's `timeStamp` equals the minute, use it. Otherwise use `.../pulse/time/next/<ms>`, which must be later.
   Archive the full pulse JSON with the draw.
2. **Seed.** `seed = SHA-256(bytes.fromhex(outputValue) || bytes.fromhex(manifest_sha256))`: raw bytes, both hex
   strings decoded case-insensitively, output value first.
3. **Shuffle.** For each domain, in the order the manifest lists domains, take the candidate IDs sorted by
   Unicode code point and apply Fisher-Yates from the last index down: for i = n-1 .. 1, j = U(i) mod (i+1),
   where U(i) = int.from_bytes(SHA-256(seed || domain || ":" || str(i)), "big") (UTF-8). The modulo bias of a
   256-bit value is negligible. No language's library RNG is involved.
4. **Selection.** Walk the domains in manifest order; in each, select the first candidate in shuffled order that
   keeps the authority-centred count within the cap. The full shuffled order is published; a §3.2 step 5
   replacement takes the next unselected candidate of the same domain in that order.
"""

import argparse
import hashlib
import json
import sys
import urllib.request
from datetime import datetime, timezone

BEACON = "https://beacon.nist.gov/beacon/2.0/pulse"


def minute_ms(utc_text):
    t = datetime.strptime(utc_text, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    if t.second or t.microsecond:
        raise ValueError("beacon_pulse_not_before_utc must be a whole minute")
    return int(t.timestamp() * 1000)


def _pulse_ms(pulse):
    t = datetime.strptime(pulse["timeStamp"], "%Y-%m-%dT%H:%M:%S.%fZ").replace(tzinfo=timezone.utc)
    return int(t.timestamp() * 1000)


def http_fetch(url):
    req = urllib.request.Request(url, headers={"Accept": "application/json", "User-Agent": "howlframe-cv1/1"})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())


def select_pulse(not_before_utc, fetch=http_fetch):
    """Apply rule 1. Returns (pulse, url, archived responses)."""
    ms = minute_ms(not_before_utc)
    archive = []
    url = f"{BEACON}/time/{ms}"
    doc = fetch(url)
    archive.append({"url": url, "response": doc})
    pulse = doc["pulse"]
    if _pulse_ms(pulse) == ms:
        return pulse, url, archive
    url = f"{BEACON}/time/next/{ms}"
    doc = fetch(url)
    archive.append({"url": url, "response": doc})
    pulse = doc["pulse"]
    if _pulse_ms(pulse) <= ms:
        raise ValueError(f"time/next returned pulse {pulse['pulseIndex']} at {pulse['timeStamp']}, "
                         "not after the minute")
    return pulse, url, archive


def seed_from(output_value_hex, manifest_sha256_hex):
    return hashlib.sha256(bytes.fromhex(output_value_hex) + bytes.fromhex(manifest_sha256_hex)).digest()


def shuffle(ids, seed, domain):
    order = sorted(ids)
    for i in range(len(order) - 1, 0, -1):
        u = int.from_bytes(hashlib.sha256(seed + domain.encode() + b":" + str(i).encode()).digest(), "big")
        j = u % (i + 1)
        order[i], order[j] = order[j], order[i]
    return order


def draw(manifest, seed, per_domain=1, max_authority=1):
    """manifest: {"domains": [{"id": "D1", "candidates": [{"id":..., "authority_centred": bool}, ...]}, ...]}"""
    selected, orders, authority = [], {}, 0
    for dom in manifest["domains"]:
        by_id = {c["id"]: c for c in dom["candidates"]}
        order = shuffle(list(by_id), seed, dom["id"])
        orders[dom["id"]] = order
        taken = 0
        for cid in order:
            if taken == per_domain:
                break
            if by_id[cid].get("authority_centred"):
                if authority >= max_authority:
                    continue
                authority += 1
            selected.append({"domain": dom["id"], "task": cid})
            taken += 1
        if taken < per_domain:
            raise ValueError(f"domain {dom['id']} cannot supply {per_domain} task(s) under the authority cap")
    return {"selected": selected, "shuffled_order": orders}


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--manifest", required=True, help="frozen pool manifest JSON")
    p.add_argument("--manifest-sha256", required=True, help="the SHA-256 committed at the manifest freeze")
    p.add_argument("--not-before", required=True, help="beacon_pulse_not_before_utc, e.g. 2026-11-02T00:00:00Z")
    p.add_argument("--out", required=True)
    p.add_argument("--per-domain", type=int, default=1)
    p.add_argument("--max-authority", type=int, default=1)
    args = p.parse_args(argv)
    with open(args.manifest, "rb") as f:
        raw = f.read()
    got = hashlib.sha256(raw).hexdigest()
    if got != args.manifest_sha256.lower():
        raise SystemExit(f"manifest SHA-256 {got} does not match the committed {args.manifest_sha256}")
    pulse, url, archive = select_pulse(args.not_before)
    seed = seed_from(pulse["outputValue"], got)
    result = draw(json.loads(raw), seed, args.per_domain, args.max_authority)
    result.update(pulse_url=url, pulse_index=pulse["pulseIndex"], pulse_time=pulse["timeStamp"],
                  output_value=pulse["outputValue"], manifest_sha256=got, seed_sha256_hex=seed.hex(),
                  beacon_responses=archive)
    with open(args.out, "w") as f:
        json.dump(result, f, indent=2)
    print(json.dumps({k: v for k, v in result.items() if k != "beacon_responses"}, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
