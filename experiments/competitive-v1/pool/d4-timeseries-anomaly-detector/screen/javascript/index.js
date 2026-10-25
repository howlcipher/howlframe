#!/usr/bin/env node
const fs = require('fs');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function round2(num) {
  return Math.round(num * 100) / 100;
}

function main() {
  const args = process.argv.slice(2);
  if (args.length !== 3) {
    fail('Usage: ./run <config.json> <timeseries.json> <analysis.json>', 2);
  }

  const [cfgPath, tsPath, anaPath] = args;

  let cfg, tsData;
  try {
    cfg = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse config JSON: ${err.message}`, 2);
  }

  try {
    tsData = JSON.parse(fs.readFileSync(tsPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse timeseries JSON: ${err.message}`, 2);
  }

  if (!cfg || typeof cfg !== 'object') {
    fail('Config must be an object', 2);
  }
  if (!tsData || typeof tsData !== 'object' || !Array.isArray(tsData.samples)) {
    fail("Timeseries must contain 'samples' array", 2);
  }

  const windowSize = cfg.window_size !== undefined ? cfg.window_size : 5;
  if (typeof windowSize !== 'number' || !Number.isInteger(windowSize) || windowSize < 2) {
    fail('Invalid window_size: must be integer >= 2', 2);
  }

  const zThreshold = typeof cfg.z_threshold === 'number' ? cfg.z_threshold : 3.0;
  const stepSeconds = typeof cfg.step_seconds === 'number' ? cfg.step_seconds : 0;
  if (stepSeconds < 0 || !Number.isInteger(stepSeconds)) {
    fail('Invalid step_seconds', 2);
  }

  const fillGaps = Boolean(cfg.fill_gaps);
  const alertCooldown = typeof cfg.alert_cooldown_samples === 'number' ? cfg.alert_cooldown_samples : 0;
  if (alertCooldown < 0 || !Number.isInteger(alertCooldown)) {
    fail('Invalid alert_cooldown_samples', 2);
  }

  const cleanedSamples = [];
  for (const s of tsData.samples) {
    if (!s || typeof s !== 'object' || typeof s.timestamp !== 'number' || typeof s.value !== 'number') {
      fail("Sample must be an object with 'timestamp' and 'value'", 2);
    }
    cleanedSamples.push({
      timestamp: s.timestamp,
      value: s.value,
      imputed: false
    });
  }

  cleanedSamples.sort((a, b) => a.timestamp - b.timestamp);

  const processedSamples = [];
  let imputedCount = 0;

  if (fillGaps && stepSeconds > 0 && cleanedSamples.length > 1) {
    for (let idx = 0; idx < cleanedSamples.length; idx++) {
      const curr = cleanedSamples[idx];
      if (idx > 0) {
        const prev = processedSamples[processedSamples.length - 1];
        const diff = curr.timestamp - prev.timestamp;
        if (diff > stepSeconds) {
          let t = prev.timestamp + stepSeconds;
          while (t < curr.timestamp) {
            processedSamples.push({
              timestamp: t,
              value: prev.value,
              imputed: true
            });
            imputedCount++;
            t += stepSeconds;
          }
        }
      }
      processedSamples.push(curr);
    }
  } else {
    for (const s of cleanedSamples) {
      processedSamples.push(s);
    }
  }

  const outputSamples = [];
  let anomaliesDetected = 0;
  let alertsEmitted = 0;
  let alertsSuppressed = 0;
  let lastAlertIndex = null;

  for (let i = 0; i < processedSamples.length; i++) {
    const s = processedSamples[i];
    const val = s.value;
    let isAnomaly = false;
    let alertFlag = false;
    let suppressedFlag = false;
    let zScore = null;

    if (i >= windowSize) {
      let sum = 0;
      for (let j = i - windowSize; j < i; j++) {
        sum += processedSamples[j].value;
      }
      const mu = sum / windowSize;

      let varianceSum = 0;
      for (let j = i - windowSize; j < i; j++) {
        const diff = processedSamples[j].value - mu;
        varianceSum += diff * diff;
      }
      const sigma = Math.sqrt(varianceSum / windowSize);

      if (sigma > 0) {
        const z = Math.abs(val - mu) / sigma;
        zScore = round2(z);
        if (z >= zThreshold) {
          isAnomaly = true;
        }
      } else {
        zScore = 0.0;
        isAnomaly = false;
      }
    }

    if (isAnomaly) {
      anomaliesDetected++;
      if (lastAlertIndex === null || (i - lastAlertIndex) > alertCooldown) {
        alertFlag = true;
        suppressedFlag = false;
        alertsEmitted++;
        lastAlertIndex = i;
      } else {
        alertFlag = false;
        suppressedFlag = true;
        alertsSuppressed++;
      }
    }

    outputSamples.push({
      timestamp: s.timestamp,
      value: s.value,
      imputed: s.imputed,
      is_anomaly: isAnomaly,
      alert_emitted: alertFlag,
      suppressed: suppressedFlag,
      z_score: zScore
    });
  }

  const report = {
    metric: tsData.metric || 'metric',
    total_samples: outputSamples.length,
    imputed_count: imputedCount,
    anomalies_detected: anomaliesDetected,
    alerts_emitted: alertsEmitted,
    alerts_suppressed: alertsSuppressed,
    samples: outputSamples
  };

  try {
    fs.writeFileSync(anaPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write analysis JSON: ${err.message}`, 2);
  }

  process.exit(alertsEmitted > 0 ? 1 : 0);
}

main();
