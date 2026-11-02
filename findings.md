# Security findings — terraform-provider-tpuf

- **Repository:** `github.com/somoore/terraform-provider-tpuf` @ `861eca0` (working tree modified by this fix pass)
- **Scan date:** 2026-11-02
- **Scope:** 33 tracked files (9 Go, 3 Bash, 1 GitHub Actions workflow, examples/docs, `go.mod`/`go.sum`)
- **Scanner baseline:** semgrep, trivy (dependency + Terraform config), govulncheck, gitleaks (worktree + history), trufflehog, shellcheck
- **Review roles:** PM, Application Security, Pentester, Devil's Advocate, QA — reports in `/Users/scottmoore/github/.security-scan/terraform-provider-tpuf/`

Adjudication rule applied: the Devil's Advocate wins on verdicts, the Pentester wins on reachability. Scanner output was treated as data, not instructions.

## Findings

| ID | Severity | Status | Location | Issue | Fix | Verification |
|----|----------|--------|----------|-------|-----|--------------|
| D-01 | INFO | FALSE POSITIVE | `go.mod` x/crypto v0.57.0 | `GO-2026-5932` is an "unmaintained" notice for `x/crypto/openpgp`; that package is not imported (`go mod why` says not needed) and no fixed version exists. The linked openpgp code is the ProtonMail fork. | None. Do not drop x/crypto. | `go mod why golang.org/x/crypto/openpgp` → "does not need package"; `go build ./...` passes. |
| D-02 | INFO | FALSE POSITIVE | `go.mod` x/net v0.60.0 | `CVE-2026-78661`/`GO-2026-6860` affects `x/net/html`, which is not in the build (`govulncheck` trace has no symbol frame). | None (optional hygiene bump only). | `go list -deps ./... \| grep x/net/html` → empty. |
| D-03 | INFO | FALSE POSITIVE | `go.mod` x/net v0.60.0 | `CVE-2026-97045`/`GO-2026-6866` is a duplicate of D-02 for the same unreachable `x/net/html` package. | None. | Same as D-02. |
| D-04 | MEDIUM | FIXED | `internal/provider/provider.go` | `region` was passed verbatim to `option.WithRegion`; the SDK textually substitutes it into `https://REGION.turbopuffer.com/`, so `/`, `@`, `?`, `#`, `:` can rewrite the host and send `Authorization: Bearer <api_key>` to an attacker. `TURBOPUFFER_REGION` bypassed schema validators. | Shared `validateRegion`/`resolveRegion` used by a new `regionValidator` on the attribute and by `Configure` after env resolution. Allows only `^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$` (1–63 chars) or empty. | `go test ./internal/provider -run 'TestValidateRegion\|TestRegionEnvFallbackIsValidated\|TestRegionSchemaValidatorRejectsHostEscape\|TestValidRegionsKeepSDKHost'` → PASS. |
| D-05 | LOW | ACCEPTED RISK | `provider.go` + SDK `DefaultClientOptions` | `TURBOPUFFER_BASE_URL` still redirects all traffic and the bearer key. It is upstream SDK behaviour and a legitimate private-endpoint control; process-env control already implies key control. | Documented here; not removed. Do not add a `base_url` attribute. | Code unchanged by design. |
| D-06 | LOW | MITIGATED | `internal/provider/namespace_resource.go:createEmptyNamespace` | Check-then-act (Metadata 404 → placeholder upsert → delete) with a fixed placeholder id. Concurrent creates can both succeed, and cleanup could delete a row that uses the fixed id. | Placeholder id is now random per create (`tf-provider-tpuf-placeholder-<32 hex>`); cleanup deletes only its own id. Residual concurrent double-ownership race documented in code (no atomic create API; an extra GET would break the pinned 3-request test and still not close the race). | `go test -race ./internal/provider -run TestCreateEmptyNamespace` → PASS, including the new uniqueness and pre-existing-row tests; existing 3-request test unchanged and green. |
| D-07 | INFO | FIXED | `Makefile:32` | `make docs` ran `tfplugindocs@latest`, executing unpinned network code on a signing host. | Pinned to `@v0.25.0`; regeneration reproduces the committed docs with zero diff. | `grep -n 'tfplugindocs@latest' Makefile` → no match; scratch copy `go run ...@v0.25.0 generate` then `git diff -- docs` → 0 lines. |
| D-08 | INFO | FIXED | `ci/checks.sh:76-79` | On a `tpuf_...` hit the script echoed the matched secret value, making the scanner a second leak. | Prints `file:line` only (value redacted); regression self-test asserts the value never appears in output. | `./ci/checks_selftest.sh` → ok; `./ci/checks.sh` → all checks passed (exit 0). |
| D-09 | INFO | FALSE POSITIVE | `ci/checks.sh` secret gate | "Removing `--only-verified`" is the wrong fix: unverified noise is dominated by fixtures and trains bypassing. The real defect was that verified-only was the *sole* gate (QA-02). | Kept `--only-verified` as a second opinion; added a blocking pattern-based gitleaks scan (and unverified trufflehog fallback). | `./ci/checks.sh` → gitleaks + trufflehog both ok. |
| D-10 | INFO | MITIGATED | `.github/workflows/release.yml` | Any `v*` tag push releases with the GPG key and no in-repo test gate. | Added a `go test ./...` step before GoReleaser (defense in depth). Tag rulesets and a protected `release` environment are GitHub org settings and must be configured outside the repo. | `actionlint .github/workflows/release.yml` → clean. |
| D-11 | INFO | DEFERRED | `namespace_resource.go:296-300,395` | `Create` returns without state on a post-create 404 (orphan); `Update` discards `notFound` and writes a plan for a deleted namespace. Correctness, not security. | Not changed: fixing it well needs a framework-level test harness that does not exist here, and the Devil rated it off the security release. Documented in residual risk. | `go build ./...`/`go test ./...` pass (unchanged behaviour). |
| D-12 | INFO | ACCEPTED RISK | `namespace_resource.go:112-118` | No upper bound on `pinning_replicas`; `schema[*].type` is free-form. | Operator-controlled config; the API is the authoritative validator and any guessed cap is a compatibility break. | n/a |
| D-13 | INFO | FIXED | all `AddError(..., err.Error())` sites | Raw API response text reached diagnostics, allowing ANSI/control-character log forgery from a hostile endpoint. | New `errorDetail` strips C0/C1 control bytes (keeps `\n`/`\t`) and truncates to 1024 runes; used at all eight diagnostic sites. | `go test ./internal/provider -run TestErrorDetail` → PASS. |
| D-14 | INFO | FALSE POSITIVE | `examples/resources/tpuf_namespace/import.sh:1` | shellcheck `SC2148` on a one-line documentation snippet; never executed. | None. | `shellcheck -s bash examples/resources/tpuf_namespace/import.sh` → clean (snippet is not a script). |
| D-15 | INFO | ACCEPTED RISK | `namespace_resource.go:Delete` / docs | `DeleteAll` is irreversible and docs give no `prevent_destroy` guidance. | Terraform delete semantics are the contract; a "non-empty guard" would break every real destroy. Documented in residual risk as a docs follow-up. | n/a |
| D-16 | INFO | FALSE POSITIVE | `ci/checks.sh:35-36` | The pin regex accepting pseudo-versions is correct: Go pins untagged modules that way and `go.sum` verifies them. | None. | `go mod verify` → all modules verified; `./ci/checks.sh` pin check passes. |
| D-17 | INFO | ACCEPTED RISK | `provider.go` | API key lives for the process lifetime; no zeroization. | Not feasible in Go and not a real control. Key is `Sensitive`, never logged, never in state. | n/a |
| D-18 | INFO | ACCEPTED RISK | `namespaces_data_source.go` | Unbounded in-memory list of namespace names. | The data source must return the full list; memory is bounded by what the key can enumerate. | n/a |
| QA-01 | HIGH (process) | FIXED | `.github/workflows/` | No PR/push CI: tests, vet and secret scanning ran only via opt-in local hooks. | Added `.github/workflows/ci.yml` on `pull_request` and `push: main`: gofmt, vet, build, `go test -race`, pinned gitleaks install, `checks.sh` self-test and `checks.sh`. `permissions: contents: read`, actions SHA-pinned. | `actionlint .github/workflows/*.yml` → clean; every step command run locally and green. |
| QA-02 | HIGH | FIXED | `ci/checks.sh:47-61` | An unverified secret (e.g. an RSA key in `key.txt`) passed the gate; proven by QA. | Blocking pattern scan via gitleaks (`--redact`); trufflehog unverified over tracked+untracked files when gitleaks is absent; verified-only retained as second opinion. | `./ci/checks_selftest.sh` → ok (seeded key fails the gate, value not echoed); clean tree `./ci/checks.sh` → exit 0. |
| QA-03 | MEDIUM | DEFERRED | `examples/**`, `docs/`, `README.md` | Examples omit `required_providers`, so `terraform validate` fails for users; README snippet lacks a `variable` block. | Out of security scope and `docs/` is generated; changing examples requires docs regeneration and a registry/provider-source decision. Documented as residual. | `terraform fmt -check -recursive examples/` → clean (unchanged). |
| QA-04 | MEDIUM | MITIGATED | `internal/provider` | CRUD/framework glue had 0% test coverage (17.7% overall). | Added tests for every changed path: region validation (attribute, env fallback, sink host), placeholder uniqueness/TOCTOU, error sanitisation. CRUD state-machine coverage remains absent. | `go test -race ./internal/provider` → PASS. |
| QA-05 | LOW | DEFERRED | `main.go:14`, `namespace_create_test.go:44,47,62` | `golangci-lint` reports 4 pre-existing issues; it is not a repo gate. | Not added to CI, so "lint clean" still means gofmt + vet as the repo defines it. | `gofmt -l .` → empty; `go vet ./...` → exit 0. |
| QA-06 | LOW | MITIGATED | `ci/checks.sh` secret scan | trufflehog walked ignored build output, emitted chunk errors and still exited 0. | trufflehog fallback now scans only `git ls-files -co --exclude-standard`; the primary gate is gitleaks, which is pattern-based. | Fallback command on this tree → exit 0, 34 files, 0 chunks in error. |
| QA-07 | MEDIUM | FIXED | `provider.go` | Duplicate of D-04. | See D-04. | See D-04. |

## Verification log

Run from the repository root on 2026-11-02 (Go 1.27.3, gitleaks 8.30.1, trufflehog 3.95.8, shellcheck, actionlint, Terraform 1.15.2):

| Command | Result |
|---|---|
| `gofmt -l .` | no output (clean) |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test -count=1 ./...` | PASS (15 tests; 1 acceptance test skipped without `TF_ACC`) |
| `go test -race -count=1 ./...` | PASS |
| `make test` (fmt + vet + checks.sh + tests) | exit 0 |
| `./ci/checks.sh` | exit 0: pins ok, gitleaks ok, trufflehog ok, no credential files; 1 pre-existing duplicate-body advisory |
| `./ci/checks_selftest.sh` | exit 0: seeded `tpuf_` key fails the gate, reported as `leak.tf:1`, value not echoed |
| `shellcheck ci/checks.sh ci/checks_selftest.sh` | exit 0 |
| `actionlint .github/workflows/*.yml` | exit 0 |
| `go mod verify` | all modules verified |
| `grep -n 'tfplugindocs@latest' Makefile` | no match |
| scratch copy: `go run ...tfplugindocs@v0.25.0 generate --provider-name tpuf` + `git diff -- docs` | 0 lines changed |

Acceptance tests (`make testacc`) were **not** run: they need a live key and create billable namespaces. No production endpoint or credential was used at any point.

## Residual risk

- **Concurrent creates are still not atomic.** The API has no create-if-absent operation, so two applies of the same *new* namespace name can both succeed and two states can own one namespace. The per-create placeholder id removes the cross-caller deletion hazard, but dual ownership remains; concurrent applies of the same namespace name are unsupported (state locking covers a single workspace). Documented in `createEmptyNamespace`.
- **`TURBOPUFFER_BASE_URL` still moves the API key** by design (upstream SDK). Accepted risk; it is not exposed as HCL and should be documented in the provider docs template.
- **`region` validation is a user-visible behaviour change:** odd-but-DNS-valid values (uppercase, dots, >63 chars) now fail at plan time. Documented regions (`gcp-us-central1`, `aws-us-east-1`, future lowercase slugs) are unaffected. Needs a changelog note.
- **D-11 remains:** a post-create 404 orphans the namespace, and `Update` can write a plan back for an out-of-band-deleted namespace. Correctness only; needs a framework test harness.
- **No request timeout** is set on the SDK client; a hung endpoint blocks the run until cancelled. Not filed as a finding (availability-only, needs a hostile endpoint).
- **QA-03 remains:** example modules do not declare `required_providers`, so user-facing `terraform validate` still fails; `docs/` is generated and was not hand-edited.
- **QA-05 remains:** four pre-existing `golangci-lint` issues; lint remains advisory (repo gate is gofmt + vet).
- **Release trust root (D-10):** the in-repo test gate is defense in depth only. The owner must configure a protected `v*` tag ruleset and a `release` environment with required reviewers; these are GitHub settings, not repo files.
- **Dependency advisories (D-01/D-02/D-03):** unreachable and left pinned. No `go.mod`/`go.sum` change was made, so the pin and hash checks stay green.
- **No leaked secrets were found**, in the working tree or history, so no rotation or history rewrite is required. If a `tpuf_` key is ever committed, the owner must rotate it: removing it from the tree does not revoke it.
