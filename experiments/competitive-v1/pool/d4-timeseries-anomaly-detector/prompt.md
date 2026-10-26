# Change Request: Dynamic Z-Score Anomaly Detection, Gap Imputation, and Alert Throttling (v2)

You are updating `timeseries-analyzer` according to `baseline_spec.md`. The observability platform requires automated anomaly detection using rolling standard deviation, gap imputation for missing timestamps, and alert throttling to prevent notification fatigue.

## Updated Configuration (`<config.json>`)

```json
{
  "window_size": 5,
  "z_threshold": 3.0,
  "step_seconds": 10,
  "fill_gaps": true,
  "alert_cooldown_samples": 3
}
```

New fields:
- `window_size` (integer, required): Number of preceding samples in sliding window used to compute baseline statistics. Must be >= 2.
- `z_threshold` (float, optional, default: 3.0): Number of standard deviations away from the rolling mean required to trigger an anomaly.
- `step_seconds` (integer, optional): Expected fixed time step between consecutive samples.
- `fill_gaps` (boolean, optional, default: `false`): If `true` and `step_seconds` is provided, fills gaps in timestamps.
- `alert_cooldown_samples` (integer, optional, default: 0): Minimum number of non-alerting samples required after an alert before a new alert can be emitted.

## Processing Semantics (v2)

1. **Gap Imputation**:
   - Sort input samples by `timestamp` ascending.
   - If `fill_gaps: true` and `step_seconds` > 0:
     - For any two consecutive samples where $t_{k+1} - t_k > \text{step\_seconds}$:
     - Generate synthetic intermediate samples at $t = t_k + \text{step\_seconds}, t_k + 2 \cdot \text{step\_seconds}, \dots < t_{k+1}$.
     - Forward-fill `value` from sample $k$.
     - Mark synthetic samples with `"imputed": true`. Original samples have `"imputed": false`.
2. **Sliding Window Statistics**:
   - For sample at index $i$:
     - The sliding window consists of the previous `window_size` samples (indices $i - \text{window\_size}$ to $i - 1$).
     - If fewer than `window_size` preceding samples exist (warm-up phase $i < \text{window\_size}$):
       - `"is_anomaly": false`, `"alert_emitted": false`, `"z_score": null`.
     - Once at least `window_size` preceding samples exist:
       - Window mean $\mu = \frac{1}{W}\sum_{j=i-W}^{i-1} v_j$.
       - Population standard deviation $\sigma = \sqrt{\frac{1}{W}\sum_{j=i-W}^{i-1} (v_j - \mu)^2}$.
       - If $\sigma > 0$:
         - $z = \frac{|v_i - \mu|}{\sigma}$.
         - Record `"z_score": round(z, 2)`.
         - If $z \ge \text{z\_threshold}$:
           - Flag `"is_anomaly": true`.
         - Else:
           - Flag `"is_anomaly": false`.
       - If $\sigma == 0$:
         - `"z_score": 0.0`, `"is_anomaly": false`.
3. **Alert Throttling (Cooldown)**:
   - When `"is_anomaly": true`:
     - If `last_alert_index` is `None` OR `(i - last_alert_index) > alert_cooldown_samples`:
       - `"alert_emitted": true`, `"suppressed": false`.
       - Update `last_alert_index = i`.
     - Else:
       - `"alert_emitted": false`, `"suppressed": true`.
   - When `"is_anomaly": false`:
     - `"alert_emitted": false`, `"suppressed": false`.

## Updated Output Format (`<analysis.json>`)

```json
{
  "metric": "cpu_usage",
  "total_samples": 8,
  "imputed_count": 1,
  "anomalies_detected": 2,
  "alerts_emitted": 1,
  "alerts_suppressed": 1,
  "samples": [
    {
      "timestamp": 100,
      "value": 10.0,
      "imputed": false,
      "is_anomaly": false,
      "alert_emitted": false,
      "suppressed": false,
      "z_score": null
    },
    {
      "timestamp": 160,
      "value": 50.0,
      "imputed": false,
      "is_anomaly": true,
      "alert_emitted": true,
      "suppressed": false,
      "z_score": 4.12
    }
  ]
}
```

Fields:
- `total_samples`: Count of processed samples (including imputed samples).
- `imputed_count`: Count of imputed samples generated.
- `anomalies_detected`: Count of samples with `"is_anomaly": true`.
- `alerts_emitted`: Count of samples with `"alert_emitted": true`.
- `alerts_suppressed`: Count of samples with `"suppressed": true`.
- `samples`: Array of evaluated sample records in ascending timestamp order.

## Exit Codes

- `0`: Completed successfully and `alerts_emitted == 0`.
- `1`: Completed successfully, but one or more alerts were emitted (`alerts_emitted > 0`).
- `2`: Missing arguments, invalid window size (< 2), negative step_seconds, or malformed JSON.
