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

Binaries for macOS and Linux are published on the Releases page:
<https://github.com/exmorphe/odyssey-beta/releases>. Each Release
includes a `checksums.txt` (SHA256) alongside the archives.

**Linux amd64** (most common):

```bash
mkdir -p ~/.local/bin
curl -L https://github.com/exmorphe/odyssey-beta/releases/latest/download/ody_linux_amd64.tar.gz \
  | tar -xz -C ~/.local/bin ody
chmod +x ~/.local/bin/ody
ody --version
```

Make sure `~/.local/bin` is on your `PATH`.

**Linux arm64:** replace `linux_amd64` with `linux_arm64`.

**macOS amd64 (Intel) / arm64 (Apple Silicon):** replace `linux_amd64`
with `darwin_amd64` or `darwin_arm64`. The first run triggers a
Gatekeeper warning because the binary is unsigned. Right-click the
binary in Finder → Open once, or clear the quarantine attribute:

```bash
xattr -d com.apple.quarantine ~/.local/bin/ody
```

### Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/exmorphe/odyssey-beta.git
cd odyssey-beta
go build -o ody .
mkdir -p ~/.local/bin && mv ody ~/.local/bin/
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
