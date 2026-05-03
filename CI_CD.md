# CI/CD Pipeline

<div align="center">

[![CI/CD Pipeline](https://github.com/ayush1452/CLIverse/actions/workflows/ci.yml/badge.svg)](https://github.com/ayush1452/CLIverse/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/ayush1452/CLIverse/branch/main/graph/badge.svg?token=YOUR_TOKEN)](https://codecov.io/gh/ayush1452/CLIverse)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Security](https://img.shields.io/badge/Security-gosec%20%7C%20govulncheck-red?style=flat&logo=shield)](https://github.com/ayush1452/CLIverse/security)
[![Release](https://img.shields.io/github/v/release/ayush1452/CLIverse?style=flat&logo=github)](https://github.com/ayush1452/CLIverse/releases)
[![Platforms](https://img.shields.io/badge/Builds-linux%20%7C%20darwin%20%7C%20windows-0f172a?style=flat)](https://github.com/ayush1452/CLIverse/releases)

**Five-stage automated pipeline · Code quality → Security → Test → Build → Release**

</div>

---

## Pipeline at a Glance

```mermaid
flowchart TD
    TRIGGER["🔁 Trigger\n─────────────────\npush → main / dev\npull_request → main\ntag → v*.*.*"]

    subgraph S1["Stage 1 · Code Quality  (parallel)"]
        FMT["📐 gofmt\nformat check"]
        VET["🔬 go vet\nstatic analysis"]
        LINT["🔍 golangci-lint\n20+ linters"]
        TIDY["📦 go mod tidy\nmodule sync check"]
    end

    subgraph S2["Stage 2 · Security  (parallel, independent)"]
        VULN["🛡️ govulncheck\nGo vuln database"]
        GOSEC["🔐 gosec\nSAST → SARIF"]
    end

    subgraph S3["Stage 3 · Test  (matrix)"]
        TUB["🐧 ubuntu-latest\n+ coverage upload"]
        TMAC["🍎 macos-latest"]
        TWIN["🪟 windows-latest"]
    end

    subgraph S4["Stage 4 · Build  (matrix, needs S1+S3)"]
        BLA["linux/amd64"]
        BLARM["linux/arm64"]
        BDA["darwin/amd64"]
        BDARM["darwin/arm64"]
        BWA["windows/amd64"]
    end

    subgraph S5["Stage 5 · Release  (needs S4+S2, tags only)"]
        CHECKSUMS["🔏 SHA-256 checksums"]
        GHR["🚀 GitHub Release\nauto release notes"]
    end

    CODECOV[("☂️ Codecov\ncoverage report")]

    TRIGGER --> S1
    TRIGGER --> S2
    TRIGGER --> S3

    S1 --> S4
    S3 --> S4

    S4 --> S5
    S2 --> S5

    TUB -->|coverage.out| CODECOV

    style S1 fill:#0f2027,stroke:#00ADD8,color:#e2e8f0
    style S2 fill:#1a0a0a,stroke:#ef4444,color:#e2e8f0
    style S3 fill:#0a1a0a,stroke:#22c55e,color:#e2e8f0
    style S4 fill:#0a0a1a,stroke:#8b5cf6,color:#e2e8f0
    style S5 fill:#1a1400,stroke:#f59e0b,color:#e2e8f0
    style CODECOV fill:#f01f7a,stroke:#f01f7a,color:#fff
```

---

## Trigger Conditions

```mermaid
flowchart LR
    subgraph EVENTS["Events that fire the pipeline"]
        direction TB
        P1["git push\n→ main"]
        P2["git push\n→ dev"]
        P3["git push\n→ v*.*.* tag"]
        P4["pull_request\n→ main"]
    end

    subgraph JOBS["Jobs that run"]
        direction TB
        J1["✅ Quality\n(all 4 checks)"]
        J2["✅ Security\n(both scanners)"]
        J3["✅ Test\n(all 3 OSes)"]
        J4["✅ Build\n(all 5 targets)"]
        J5["🔒 Release\n(tags only)"]
    end

    P1 --> J1 & J2 & J3 & J4
    P2 --> J1 & J2 & J3 & J4
    P4 --> J1 & J2 & J3 & J4
    P3 --> J1 & J2 & J3 & J4 & J5

    style J5 fill:#1a1400,stroke:#f59e0b,color:#e2e8f0
```

> Concurrent runs on the same branch are cancelled automatically (except `main`, where all runs complete).

---

## Stage 1 · Code Quality

Four checks run in **parallel** — each is a separate job so a failure is pinpointed precisely.

```mermaid
flowchart LR
    SRC["📁 Source code\n+ go.mod / go.sum"]

    subgraph Q["Quality jobs  (fail-fast: false)"]
        FMT["📐 fmt\n─────────────\ngofmt -l .\nFails & shows diff\nif any file needs\nformatting"]
        VET["🔬 vet\n─────────────\ngo vet ./...\nChecks for common\nmistakes the\ncompiler misses"]
        LINT["🔍 lint\n─────────────\ngolangci-lint\n20+ linters via\n.golangci.yml\n5 min timeout"]
        TIDY["📦 tidy\n─────────────\ngo mod tidy\nDiffs go.mod and\ngo.sum — fails if\nout of sync"]
    end

    SRC --> FMT & VET & LINT & TIDY

    FMT --> R1["✅ / ❌"]
    VET --> R2["✅ / ❌"]
    LINT --> R3["✅ / ❌"]
    TIDY --> R4["✅ / ❌"]
```

**Linters enabled in `.golangci.yml`**

| Linter | What it catches |
|---|---|
| `errcheck` | Ignored errors |
| `staticcheck` | Bugs, performance, style |
| `gosimple` | Simplifiable code |
| `unused` | Dead code |
| `gofmt` / `goimports` | Formatting and import order |
| `misspell` | Typos in comments and strings |
| `bodyclose` | Unclosed HTTP response bodies |
| `prealloc` | Slice pre-allocation opportunities |
| `unconvert` | Unnecessary type conversions |
| `noctx` | HTTP requests missing context |

---

## Stage 2 · Security

Two scanners run **independently and in parallel** — a failure in either blocks the release stage.

```mermaid
flowchart TD
    SRC["📁 Source code"]

    subgraph SEC["Security jobs  (fail-fast: false)"]
        direction LR

        subgraph GOV["govulncheck"]
            GV1["Fetches Go\nvulnerability DB"]
            GV2["Matches deps in\ngo.mod against CVEs"]
            GV3["Reports call-graph\nreachable vulns only"]
            GV1 --> GV2 --> GV3
        end

        subgraph GOS["gosec"]
            GS1["Scans AST for\ncommon security\nmistakes"]
            GS2["Emits SARIF report\n(gosec.sarif)"]
            GS3["Uploads to GitHub\nSecurity tab"]
            GS1 --> GS2 --> GS3
        end
    end

    SRC --> GOV & GOS

    GV3 --> VR["✅ No known vulns\n❌ Blocks release"]
    GS3 --> SR["✅ SARIF uploaded\n🔍 Visible in\nSecurity → Code\nScanning Alerts"]
```

> SARIF results appear in **GitHub → Security → Code Scanning Alerts** even when the job passes.

---

## Stage 3 · Test

Tests run across all three target operating systems **in parallel**, with race detection enabled.

```mermaid
flowchart TD
    SRC["📁 Source code"]

    subgraph MATRIX["Test matrix  (fail-fast: false)"]
        direction LR

        subgraph LX["🐧 ubuntu-latest"]
            LT["go test -v -race\n-coverprofile=coverage.out\n-covermode=atomic ./..."]
            LC["codecov/codecov-action\nuploads coverage.out\nwith CODECOV_TOKEN"]
            LA["upload-artifact\ncoverage-report\n(7 day retention)"]
            LT --> LC --> LA
        end

        subgraph MC["🍎 macos-latest"]
            MT["go test -v -race\n-coverprofile=coverage.out\n-covermode=atomic ./..."]
        end

        subgraph WN["🪟 windows-latest"]
            WT["go test -v -race\n-coverprofile=coverage.out\n-covermode=atomic ./..."]
        end
    end

    SRC --> LX & MC & WN

    LC -->|"coverage.out"| COV[("☂️ Codecov\nTracks history\nFlags: unittests\nFails CI on error")]
```

**Flags used**

| Flag | Purpose |
|---|---|
| `-v` | Verbose output — every test name visible in the log |
| `-race` | Enables Go race detector — catches concurrent memory access bugs |
| `-covermode=atomic` | Thread-safe coverage counters (required with `-race`) |
| `-coverprofile` | Writes coverage data consumed by Codecov |

---

## Stage 4 · Build

Cross-compilation of **five release targets**, gated on Stage 1 and Stage 3 both passing.

```mermaid
flowchart TD
    GATE["🚦 needs: quality + test"]

    META["🏷️ Build metadata\n───────────────────────\nversion  = git ref name\ncommit   = SHA[:8]\ndate     = UTC RFC3339\ninjected via -ldflags"]

    subgraph TARGETS["Build matrix  (CGO_ENABLED=0, -trimpath)"]
        direction LR
        LA["🐧 linux\namd64"]
        LARM["🐧 linux\narm64"]
        DA["🍎 darwin\namd64"]
        DARM["🍎 darwin\narm64 (Apple Silicon)"]
        WA["🪟 windows\namd64 (.exe)"]
    end

    ARTS["📦 Artifacts uploaded\ncliverse-{os}-{arch}\n7 day retention"]

    GATE --> META --> TARGETS --> ARTS
```

**ldflags injected into every binary**

```
-s -w                              strip debug symbols + DWARF → smaller binary
-trimpath                          remove build machine paths from stack traces
-X main.version=<tag or branch>   embedded version string
-X main.commit=<sha[:8]>          short commit hash
-X main.date=<UTC RFC3339>        build timestamp
```

> To expose these in the CLI, add `var version, commit, date string` to [main.go](main.go).

---

## Stage 5 · Release

Runs **only on `v*.*.*` tags**, requires both the Build and Security stages to pass.

```mermaid
flowchart TD
    TAG["🏷️ git tag v1.2.3\ngit push origin v1.2.3"]

    DL["⬇️ Download all\n5 build artifacts"]

    CS["🔏 Generate checksums\nsha256sum * > checksums.txt"]

    PRE{"Pre-release?\n(alpha / beta / rc\nin tag name)"}

    STABLE["🚀 GitHub Release\n• Full release\n• Auto release notes\n• All 5 binaries\n• checksums.txt"]

    PREVIEW["🧪 GitHub Release\n• Marked pre-release\n• Auto release notes\n• All 5 binaries\n• checksums.txt"]

    TAG --> DL --> CS --> PRE
    PRE -->|"v1.2.3"| STABLE
    PRE -->|"v1.2.3-rc.1\nv2.0.0-beta\nv3.0.0-alpha"| PREVIEW
```

**Published release assets**

```
cliverse-linux-amd64
cliverse-linux-arm64
cliverse-darwin-amd64
cliverse-darwin-arm64
cliverse-windows-amd64.exe
checksums.txt
```

---

## Job Dependency Graph

```mermaid
flowchart LR
    subgraph PARALLEL_1["Runs immediately on trigger"]
        Q["Quality\n4 checks"]
        SEC["Security\n2 scanners"]
        T["Test\n3 OSes"]
    end

    subgraph GATE_1["Needs: quality + test"]
        B["Build\n5 targets"]
    end

    subgraph GATE_2["Needs: build + security\nOnly on v*.*.* tags"]
        R["Release"]
    end

    Q --> B
    T --> B
    B --> R
    SEC --> R
```

---

## Configuration

### Required secret

| Secret | Where to set | Description |
|---|---|---|
| `CODECOV_TOKEN` | Repository → Settings → Secrets and Variables → Actions | Codecov upload token from [codecov.io](https://codecov.io) |

### Required GitHub permissions

The Release job uses `permissions: contents: write` to create GitHub Releases. This is scoped to the job only — all other jobs run with default read permissions.

No additional repository settings are needed beyond the Codecov token.

---

## How to Ship a Release

```bash
# Stable release
git tag v1.0.0
git push origin v1.0.0

# Release candidate (marked pre-release automatically)
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1

# Beta (marked pre-release automatically)
git tag v2.0.0-beta
git push origin v2.0.0-beta
```

The pipeline will:

1. Run all Quality and Security checks
2. Test across Linux, macOS, Windows
3. Compile all 5 binaries with version metadata embedded
4. Publish a GitHub Release with checksums

---

## Local Equivalents

Every pipeline step has a local `make` counterpart.

| Pipeline stage | Local command |
|---|---|
| Format check | `make fmt` then `git diff` |
| Static analysis | `go vet ./...` |
| Lint | `make lint` |
| Module tidy | `go mod tidy && git diff go.mod go.sum` |
| Vulnerability scan | `make vuln` |
| Tests | `make test` |
| Coverage report | `make coverage` → opens `coverage.html` |
| Build | `make build` |

---

## Pipeline Files

```text
CLIverse/
├── .github/
│   └── workflows/
│       └── ci.yml          ← full pipeline definition
├── .golangci.yml           ← linter configuration
└── CI_CD.md                ← this document
```

---

<div align="center">

Built for [CLIverse](README.md) · Powered by [GitHub Actions](https://github.com/features/actions)

</div>
