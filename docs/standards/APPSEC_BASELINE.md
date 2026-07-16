# AppSec Baseline

This document is the single security baseline for prox development and release work. It moves the minimum security checks into the normal definition of done; deeper runtime verification remains part of the release verification phase.

## Ownership And Evidence

- Owner: `quality-guard`
- Entry point: `quality-guard/scripts/appsec-baseline-entry.mjs`
- Evidence: `.context-snapshots/quality-guard.appsec-baseline.latest.{json,md}`
- Additional gates: `secrets-scan`, `security-scan`, dependency review, API tests, and production route smoke tests as applicable

## Control Matrix

| ID | Control | Required implementation behavior | Acceptance evidence |
|---|---|---|---|
| APPSEC-AUTH-01 | Authentication | Declare identity source, credential type, step-up conditions, failure throttling, and account-enumeration behavior. | Tests cover successful, failed, throttled, and enumeration-resistant authentication paths. |
| APPSEC-RBAC-02 | RBAC and resource ownership | Every protected route maps subject, action, resource, and conditions; role checks never replace resource-ownership checks. | Permission tests contain both allow and deny cases, including non-admin paths. |
| APPSEC-SESSION-03 | Session and token lifecycle | Cookies use HttpOnly, Secure, and an explicit SameSite policy; tokens define issuer, audience, expiry, rotation, and revocation. | Tests cover invalid, expired, revoked, and wrong-audience credentials. |
| APPSEC-SECRETS-04 | Secrets and configuration | Secrets stay out of source, frontend bundles, logs, error responses, and examples; sensitive environment variables have a rotation owner. | Secret scans pass and examples use placeholders only. |
| APPSEC-OWASP-05 | OWASP review | API, authorization, and data-flow changes are reviewed for access control, injection, SSRF, XSS, configuration, and vulnerable components. | Verify evidence lists applicable checks, skipped checks with reasons, and command output. |
| APPSEC-SUPPLY-06 | Supply chain | Lockfile or dependency changes require vulnerability, license/source, and artifact-provenance review. | Dependency scan results are recorded; accepted findings have an owner and expiry. |
| APPSEC-SSRF-07 | Outbound requests | URL fetch, proxy, and webhook code restrict protocols, recheck DNS/IP, block private and metadata networks, limit redirects, and enforce time and size bounds. | Tests deny localhost, loopback, metadata IPs, private CIDRs, and `file://`. |
| APPSEC-XSS-08 | Browser rendering | User-controlled HTML, Markdown, URLs, and rich text have an explicit encoding or sanitization boundary; unaudited raw HTML sinks are prohibited. | Tests cover script tags, event handlers, unsafe URL schemes, SVG, and MathML payloads. |
| APPSEC-SQLI-09 | Query and command injection | Database, shell, template, and dynamic-field operations use parameters or an allowlisted builder. | Injection payloads do not alter query semantics; unknown dynamic fields are rejected. |
| APPSEC-PERM-10 | Permission matrix | Each role, tenant/community, and resource-owner combination includes a deny case; horizontal and vertical privilege changes are covered. | API or end-to-end tests prove denied users cannot read or mutate protected resources. |

## Prox Enforcement Map

- Wallet, quota, reward, check-in, subscription, redemption, and transfer mutations must use database transactions, conditional updates, idempotency keys, and immutable audit rows.
- Community Bot, Agent, and game rewards must use a fenced lease or compare-and-swap state transition before external actions are committed.
- Key creation and activation must preserve the activation state and bind code through API, frontend, and batch workflows; every activation route must enforce role and resource ownership.
- Static assets and same-site console APIs must be isolated from upstream model throttling. Public API throttling remains explicit per route and credential.
- Production evidence is taken from the active immutable image, live container identity, health checks, changed API routes, and changed frontend asset markers.

## Minimum Release Flow

1. Record the affected authorization and data-flow boundaries before implementation.
2. Run the AppSec baseline entry point and the repository security and secret scans.
3. Run focused regression tests for the affected allow, deny, idempotency, concurrency, and rollback paths.
4. Rebuild and switch an immutable `new-api` image, then verify the changed live routes and assets.
5. Block release when a high-severity finding lacks a named owner, mitigation, and expiry.

## References

- OWASP ASVS: https://owasp.org/www-project-application-security-verification-standard/
- OWASP Top 10: https://owasp.org/Top10/
- OWASP Cheat Sheet Series: https://cheatsheetseries.owasp.org/
- NIST SP 800-63B: https://pages.nist.gov/800-63-4/sp800-63b.html
- SLSA: https://slsa.dev/spec/latest/
