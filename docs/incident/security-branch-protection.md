# Security Branch Protection Setup

Configure branch protection for `main` in GitHub repository settings.

## Required status checks
Require these checks to pass before merging:
- `CI`
- `Secret Scan (Gitleaks)`
- `Go Vulnerability Scan (High/Critical Gate)`
- `IaC and Config Scan (Trivy)`
- `Auth and Policy Regression Gate`
- `Container Image Vulnerability Gate (High/Critical)`
- `post-deploy-smoke`

## Additional protection
1. Require pull request before merge.
2. Require approvals (recommended: at least 1 code owner or security reviewer).
3. Dismiss stale approvals when new commits are pushed.
4. Restrict force pushes.
5. Restrict deletions of `main`.

## Notes
- GitHub required checks must match workflow/job names exactly.
- If you rename workflows/jobs, update required checks accordingly.
