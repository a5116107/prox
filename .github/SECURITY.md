# Security Policy

## Supported Version

Security fixes are applied to the latest `main` release. Older images should
be upgraded before a report is reproduced.

## Reporting

Do not publish credentials, production data, or vulnerability details in a
public issue. Submit a private report through
[GitHub Security Advisories](https://github.com/a5116107/prox/security/advisories/new).

Include the affected commit or image marker, component and route, reproduction
steps, observed impact, relevant logs with secrets removed, and a proposed fix
when available.

## Deployment Baseline

- Expose only the reverse proxy on ports 80/443.
- Keep PostgreSQL, Redis, new-api, and Hermes Adapter on private or loopback
  interfaces.
- Set unique values for `SESSION_SECRET`, `CRYPTO_SECRET`, database and Redis
  passwords, OAuth state signing secret, and the Hermes shared key.
- Store `.env.deploy` and `/etc/prox/hermes.env` with mode `0600`.
- Build releases from a clean `main` checkout and verify the active image,
  `/api/status`, and `/release-marker.txt` after every switch.
- Rotate any credential that appears in logs, screenshots, commits, or issue
  attachments.
