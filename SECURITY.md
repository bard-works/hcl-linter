# Security Policy

## Supported Versions

Only the latest release receives security fixes.

| Version        | Supported          |
| -------------- | ------------------ |
| latest release | :white_check_mark: |
| older          | :x:                |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Instead, report privately via GitHub Security Advisories:

1. Go to https://github.com/bard-works/hcl-linter/security/advisories
2. Click **Report a vulnerability**

You can expect an initial response within 7 days. Please include:

- A description of the issue and its impact
- Steps to reproduce (a minimal HCL file or command invocation)
- The version (`hcl-linter version`) and platform where you observed it

## Verifying releases

Release archives carry build-provenance attestations. Verify a download with:

```bash
gh attestation verify hcl-linter_<version>_<os>_<arch>.tar.gz --repo bard-works/hcl-linter
```

`checksums.txt` in each release lists SHA-256 sums; `scripts/install.sh`
checks them automatically.
