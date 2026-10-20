# Security Policy

`clix` is pre-1.0 and provided as is, without warranty. Reports are handled on
a best-effort basis, and only the latest release receives fixes.

## Reporting a vulnerability

**Do not open a public GitHub issue for security reports.** Use GitHub's
[private vulnerability reporting][gh-advisories] for this repository.

## In scope

- **Terminal escape injection** through narrated text. Plain and live
  modes print narrated text (lines, labels, outcomes) as is, so a
  program strips control characters from untrusted strings before
  narrating them; JSON mode escapes everything.
- **Unbounded memory growth** from a task writer's buffered output.

## Out of scope

- Vulnerabilities in the calling application's own commands. `clix` only
  renders what the application narrates; report those to the application.
- Vulnerabilities in cobra, pflag or golang.org/x/term themselves. Report
  those upstream; `clix` follows their releases.

## Disclosure

We coordinate disclosure with reporters and credit them in any resulting
advisory unless they prefer otherwise.

[gh-advisories]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability
