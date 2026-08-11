# HowlFrame Capability Lab

This demo proves a central architectural idea of HowlFrame: **intent is not authority**. 

Even if an application requests to write a file, it will be deterministically denied by the VM unless the trusted runner explicitly grants the `filesystem` capability at runtime.

## Features Demonstrated

* Capability denial
* Strict security boundaries

## Prerequisites

- Go 1.21 or newer
- A built HowlFrame CLI (`go build -o howlframe howlframe.go`)

## Validate and Compile

```bash
./howlframe -validate examples/capability_lab/capability_lab.howl
./howlframe -compile-bc examples/capability_lab/capability_lab.howl -o capability_lab.hfbc
```

## Run

### 1. Without Capabilities (Denial)

If you run the application without explicitly granting the filesystem capability, the runtime will intercept the intent and crash deterministically, protecting the host system:

```bash
./howlframe -run-bc capability_lab.hfbc
```

**Expected output (exit code 1):**
```
Attempting to write to filesystem...
runtime error: capability denied: filesystem
```

### 2. With Capabilities (Success)

By granting the `filesystem` capability, the runner authorizes the intent:

```bash
./howlframe -run-bc -allow-caps filesystem capability_lab.hfbc
```

**Expected output (exit code 0):**
```
Attempting to write to filesystem...
Write succeeded!
```

*(You can verify that `sensitive_data.txt` was created in your working directory.)*
