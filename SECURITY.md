# Security Policy

## Supported Versions

Only the latest minor line receives security fixes.

| Version | Status                       |
| ------- | ---------------------------- |
| 1.1.x   | :white_check_mark: supported |
| 1.0.x   | :warning: security fixes only |
| < 1.0   | :x: unsupported              |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security reports.**

Report privately to **security@atlasent.io** or via GitHub's
[private vulnerability reporting](https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/security/advisories/new).

Include:

- a minimal reproduction (code, request, or inputs that trigger the issue),
- the SDK version (`atlasent.Version`) and Go version,
- the impact you expect an attacker to achieve,
- any suggested remediation.

We aim to:

- acknowledge receipt within **two business days**,
- confirm or refute the issue within **five business days**,
- ship a fix (or publish a mitigation) within **30 days** for critical
  and high-severity issues, **90 days** for others.

We practice coordinated disclosure: credit is given when a reporter
wants it, and we'll agree an embargo date before going public.

## Scope

In scope:

- The SDK (`atlasent/`), the test fake (`atlasenttest/`), every adapter
  submodule (`grpc/`, `connectrpc/`, `otel/`, `cacheredis/`, `bundle/`,
  `middleware/*`), and the examples.
- Classes of bug that matter for authorization clients: fail-open
  regressions (the SDK must deny on error by default), cache key
  collisions, obligation bypasses, incorrect TLS behavior, information
  leaks in error messages, data races that corrupt Decisions or
  observer events, bundle signature-verification bypasses.

Out of scope:

- Denial-of-service against an operator's own PDP by their own
  application (retry and breaker tuning is a configuration concern).
- Vulnerabilities in transitive dependencies already disclosed upstream —
  `govulncheck` runs in CI to catch these. Report the upstream advisory.
- Policy content: this SDK does not evaluate policies (except through
  a user-supplied `bundle.PolicyEngine`); the PDP server project is a
  separate report surface.

## Hardening Notes for Integrators

- Keep `Client.FailClosed` at its default (`true`) unless availability
  truly outranks correctness for the call-site.
- Install a `Cache` only when stale decisions are acceptable for the
  window; sensitive actions (payment, deletion) should not short-circuit
  through the cache.
- Register every obligation you expect; leave `ObligationRegistry`'s
  unknown-fails-by-default in place so the PDP can't silently demand a
  side-effect your code didn't honor.
- Never disable Ed25519 signature verification on a `bundle.HTTPSyncer`
  — we ship no "skip verify" switch by design; bundle bypass is the bug
  class this submodule exists to prevent.
- Never log full JWTs or raw `Principal.Attributes`; the bundled
  `SlogObserver` intentionally logs only bounded fields.
