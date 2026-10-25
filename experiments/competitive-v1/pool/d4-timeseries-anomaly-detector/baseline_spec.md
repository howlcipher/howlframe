# Baseline Specification: Time-Series Window Aggregator (v1)

`timeseries-analyzer` computes moving window statistical summaries over timestamped metric data.

## Invocation

```bash
./run <config.json> <timeseries.json> <analysis.json>
```

## Baseline Input Formats

### `<config.json>`
```json
{
  "window_size": 5
}
```

### `<timeseries.json>`
```json
{
  "metric": "cpu_usage",
  "samples": [
    {"timestamp": 100, "value": 10.0},
    {"timestamp": 110, "value": 12.0},
    {"timestamp": 120, "value": 11.0}
  ]
}
```

## Baseline Behavior

1. For each sample index $i$:
   - Consider the rolling window of samples from $\max(0, i - \text{window\_size} + 1)$ to $i$.
   - Calculate rolling `mean`, `min`, and `max`.
2. Output results to `<analysis.json>`.

## Baseline Output Format (`<analysis.json>`)

```json
{
  "metric": "cpu_usage",
  "total_samples": 3,
  "samples": [
    {
      "timestamp": 100,
      "value": 10.0,
      "window_mean": 10.0,
      "window_min": 10.0,
      "window_max": 10.0
    }
  ]
}
```

## Exit Codes

- `0`: Success.
- `2`: Missing arguments, malformed JSON, or non-positive window_size.
