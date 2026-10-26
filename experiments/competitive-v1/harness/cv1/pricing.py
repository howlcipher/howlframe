"""Cost of one model call from its reported usage, and a safe upper bound on it before the call is sent.

All amounts are USD. Rates come from `prices.json` (USD per 1M tokens). The upper bound is what lets the proxy
promise that spend never exceeds a cap: a call is forwarded only if the cap still holds after charging that
call's worst case, and the actual charge can never be above the worst case.
"""

import json
import os

MTOK = 1_000_000

# Input tokens are bounded by request bytes: a token covers at least one byte of the text it encodes. Message
# framing and template tokens are not in the body, so a fixed allowance is added on top. Opaque fields
# (encrypted reasoning, thinking signatures) may expand when the provider decodes them, so their bytes count
# OPAQUE_EXPANSION times. The proxy checks every response against this bound and stops the experiment if a
# call ever exceeds it (see proxy.py), so a wrong assumption here is detected, not silently absorbed.
TEMPLATE_TOKEN_ALLOWANCE = 8192
OPAQUE_EXPANSION = 4

DEFAULT_PRICES = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "prices.json")


class PricingError(Exception):
    pass


def load_prices(path=DEFAULT_PRICES):
    with open(path) as f:
        return json.load(f)


def model_entry(prices, model):
    try:
        return prices["models"][model]
    except KeyError:
        raise PricingError(f"no price for model {model!r}")


def _anthropic_cost(entry, usage):
    r = entry["rates"]
    cache_write = usage.get("cache_creation_input_tokens") or 0
    split = usage.get("cache_creation") or {}
    w1h = split.get("ephemeral_1h_input_tokens") or 0
    w5m = split.get("ephemeral_5m_input_tokens")
    if w5m is None:
        w5m = max(0, cache_write - w1h)
    cost = ((usage.get("input_tokens") or 0) * r["input"]
            + w5m * r["cache_write_5m"]
            + w1h * r["cache_write_1h"]
            + (usage.get("cache_read_input_tokens") or 0) * r["cache_read"]
            + (usage.get("output_tokens") or 0) * r["output"])
    return cost / MTOK


def _openai_cost(entry, usage):
    total_in = usage.get("input_tokens") or 0
    details = usage.get("input_tokens_details") or {}
    cached = details.get("cached_tokens") or 0
    r = entry["long_context_rates"] if total_in > entry.get("long_context_threshold", float("inf")) else entry["rates"]
    uncached = max(0, total_in - cached)
    if "cache_write_tokens" in details:
        written = min(uncached, details.get("cache_write_tokens") or 0)
        uncached_cost = written * r["cache_write"] + (uncached - written) * r["input"]
    else:
        # The response does not say which uncached tokens were cache writes; charge all of them at the higher rate.
        uncached_cost = uncached * max(r["input"], r["cache_write"])
    cost = uncached_cost + cached * r["cache_read"] + (usage.get("output_tokens") or 0) * r["output"]
    return cost / MTOK


def call_cost(prices, model, usage):
    """Charge for one completed call, from the usage block the provider returned."""
    entry = model_entry(prices, model)
    if entry["provider"] == "anthropic":
        return _anthropic_cost(entry, usage)
    if entry["provider"] == "openai":
        return _openai_cost(entry, usage)
    raise PricingError(f"unknown provider for {model!r}")


def input_token_bound(entry, body_bytes, opaque_bytes=0):
    return min(entry["context_window"], body_bytes + OPAQUE_EXPANSION * opaque_bytes + TEMPLATE_TOKEN_ALLOWANCE)


def worst_call_cost(prices, model, body_bytes, max_output_tokens=None, opaque_bytes=0):
    """Upper bound on what one call can cost: every input token at the dearest input-side rate, plus the
    largest output the request allows."""
    entry = model_entry(prices, model)
    in_bound = input_token_bound(entry, body_bytes, opaque_bytes)
    out_bound = entry["max_output_tokens"]
    if max_output_tokens:
        out_bound = min(out_bound, int(max_output_tokens))
    rates = entry["rates"]
    if entry["provider"] == "openai" and in_bound > entry.get("long_context_threshold", float("inf")):
        rates = entry["long_context_rates"]
    in_rate = max(v for k, v in rates.items() if k != "output")
    return (in_bound * in_rate + out_bound * rates["output"]) / MTOK


def input_tokens_reported(entry, usage):
    """Total input tokens a usage block reports, for checking the bound after the call."""
    if entry["provider"] == "anthropic":
        return ((usage.get("input_tokens") or 0) + (usage.get("cache_creation_input_tokens") or 0)
                + (usage.get("cache_read_input_tokens") or 0))
    return usage.get("input_tokens") or 0
