# Security Policy

## Scope

units is a units-of-measure library with no I/O, no network surface, and no
dependencies outside the standard library. The plausible security concern is
**crafted input** reaching the public API — a hostile symbol or magnitude that
causes a reachable panic or unbounded resource use, or a conversion that
silently returns a wrong quantity. Bugs of that kind are in scope; a silent
wrong-unit conversion is treated as a security-grade bug, because callers use
this library precisely to keep that from happening.

Issues that only affect test code or internal tooling are welcome as ordinary
bug reports rather than security reports.

## Supported Versions

No release is tagged yet; until one is, `main` is the reference and only line
that receives fixes.

| Version  | Supported                    |
| -------- | ---------------------------- |
| v0.x.x   | :white_check_mark: (pre-1.0) |
| < v0.1.0 | :x: (unreleased)             |

## Reporting a Vulnerability

If you think you found a vulnerability, please report it via [GitHub Security Advisory](https://github.com/lestrrat-3d/units/security/advisories/new).
Please include explicit steps to reproduce the security issue — a minimal
reproducer or failing test, and the commit SHA you tested against, are ideal.

We will do our best to respond in a timely manner, but please also be aware that
this project is maintained by a very limited number of people. Please help us
with test code and such.
