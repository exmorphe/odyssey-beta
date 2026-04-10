# ody — Odyssey gym CLI

A command-line client for the Odyssey DevOps/SRE gym. Fetches exercises,
applies them to a local kind cluster, and submits cluster state for
verification.

## Install

```bash
cd cli/
go build -o ody .
# move ody to somewhere on your PATH
```

## Commands

### `ody login <server-url>`

Authenticate via OAuth device flow. Opens a browser-based authorization
page where you enter a code.

```
$ ody login https://gym.example.com
Visit https://gym.example.com/activate and enter code: ABCD-1234
Logged in to https://gym.example.com
```

Credentials are saved to `~/.odyssey/config.json`.

### `ody start`

Fetch the active exercise, ensure a kind cluster exists, clean up stale
namespaces, and apply the exercise steps.

```
$ ody start
Creating kind cluster "odyssey"
Exercise #42 applied
  Namespaces: exercise
  Resources:  Deployment, Service
Run 'ody verify' when you think you've fixed the faults.
```

### `ody verify`

Capture cluster state and submit it for verification. Displays per-fault
results with masking.

```
$ ody verify
✗ wrong_image/tag_mismatch — FAIL
  symptom: image pull error
✗ missing_labels/no_selector_match — FAIL (masked by wrong_image/tag_mismatch)
  symptom: pods not ready — fix wrong_image/tag_mismatch first

0/2 faults resolved
```

### `ody status`

Show the current exercise state.

```
$ ody status
Exercise #42
  Status:     active
  Created:    09 Apr 2026 14:00 UTC
  Namespaces: exercise
  Resources:  Deployment, Service
```

## Requirements

- Go 1.22+
- `kubectl` on PATH
- `kind` on PATH (for cluster management)
- An Odyssey gym server account
