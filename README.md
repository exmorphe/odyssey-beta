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

After installing the CLI (next section), run `ody doctor` to verify your
local setup before continuing.

## Install

### Quick install

```bash
curl -fsSL https://raw.githubusercontent.com/exmorphe/odyssey-beta/master/scripts/install.sh | sh
```

Detects your OS and CPU architecture, downloads the matching release
from <https://github.com/exmorphe/odyssey-beta/releases>, verifies its
SHA256 against the published `checksums.txt`, and installs to
`~/.local/bin/ody`. Override with `ODY_VERSION=vX.Y.Z` to pin a release
or `ODY_INSTALL_DIR=/some/path` to install elsewhere.

Make sure `~/.local/bin` (or your override) is on your `PATH`. Then:

```bash
ody --version
ody login https://k8sodyssey.com
```

### Manual install

Prefer not to pipe a script to `sh`? Download the tarball directly from
<https://github.com/exmorphe/odyssey-beta/releases>. Each release ships
a `checksums.txt` (SHA256) you can verify against.

**Linux amd64:**

```bash
mkdir -p ~/.local/bin
curl -L https://github.com/exmorphe/odyssey-beta/releases/latest/download/ody_linux_amd64.tar.gz \
  | tar -xz -C ~/.local/bin ody
chmod +x ~/.local/bin/ody
```

For other platforms, replace `linux_amd64` with `linux_arm64`,
`darwin_amd64`, or `darwin_arm64`.

**macOS quarantine note.** If you download the tarball through a
browser (Safari, Chrome) instead of `curl`, macOS tags the binary with
the `com.apple.quarantine` extended attribute and Gatekeeper blocks
first run. The Quick install path avoids this — `curl` doesn't set the
attribute. If you hit the dialog after a manual download, clear it:

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

### `ody doctor`

Check that local prerequisites are installed and the Docker daemon is
reachable. Run this after installing the CLI, or any time `ody start`
complains about something missing.

```
$ ody doctor
docker     ✓  Docker 25.0.3
kind       ✓  v0.22.0
kubectl    ✓  v1.29.1 (client)
group      ✓  user is in 'docker' group
memory     ✓  4.0 GiB allocated to Docker

All checks passed. Ready for 'ody start'.
```

`ody start` runs the same checks automatically and bails before doing any
work if something's wrong — `doctor` is for verifying ahead of time or
debugging a failed `start`.

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
  Local cluster: running — run 'ody down' to tear down
```

### `ody down`

Tear down the local kind cluster. Prompts for confirmation; the
cluster and its kubeconfig entries are removed on `y` or `yes`.
Safe to run when no cluster exists.

```
$ ody down
Delete kind cluster "odyssey"? [y/N]: y
Deleted kind cluster "odyssey".
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
