# ody — Odyssey gym CLI

Odyssey is a gym for practising Kubernetes debugging, hosted at
<https://k8sodyssey.com>. You turn a few dials on the server, generate
an exercise, and `ody` applies it to a local kind cluster for you to
debug. When you think you've fixed it, `ody verify` captures cluster
state and scores it.

This is the closed-beta CLI. Expect rough edges. Feedback and bug reports
are very welcome — see [Feedback & bugs](#feedback--bugs) below.

## Prerequisites

Install these before running `ody`. The CLI doesn't bundle or install them
for you.

- **Docker** — container runtime for kind. The daemon must be running
  before `ody start`; on Linux, your user also needs to be in the `docker`
  group.
  <https://docs.docker.com/engine/install/>
- **kubectl** — the Kubernetes CLI.
  <https://kubernetes.io/docs/tasks/tools/>
- **kind** — provisions the local cluster `ody` targets.
  <https://kind.sigs.k8s.io/docs/user/quick-start/#installation>
- **ody** — this CLI. Download a binary or build from source (below).

## Install

### Download binary

Pre-built binaries will be published on GitHub Releases:
`https://github.com/exmorphe/odyssey-beta/releases` *(not yet available —
build from source until Releases are configured)*.

### Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/exmorphe/odyssey-beta.git
cd odyssey-beta
go build -o ody .
# move ody onto your PATH, e.g.:
#   sudo mv ody /usr/local/bin/
# or:
#   mv ody ~/.local/bin/
```

## Commands

### `ody login <server-url>`

Authenticate via OAuth device flow. Opens a browser-based authorization
page where you enter a code.

```
$ ody login https://k8sodyssey.com
Visit https://k8sodyssey.com/activate and enter code: ABCD-1234
Logged in to https://k8sodyssey.com
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

### `ody feedback "<message>"`

Send a short note about the exercise you just ran. Attaches to the active
exercise, or falls back to the latest verified one.

```
$ ody feedback "HPA fault was unclear — took me ages to find it"
feedback recorded for exercise #42
```

Override the target with `--exercise <id>`:

```
$ ody feedback --exercise 39 "great scenario, harder than it looked"
feedback recorded for exercise #39
```

## Feedback & bugs

- **Quick feedback:** `ody feedback "..."` — goes straight onto the
  exercise record.
- **Bug reports:** <https://github.com/exmorphe/odyssey-beta/issues/new/choose>
  — pick the `client-bug` or `server-bug` template.
- **Discussions:** <https://github.com/exmorphe/odyssey-beta/discussions>
  — categories for what worked, what didn't, ideas, and general chat.
