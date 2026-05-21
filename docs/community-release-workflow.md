# Community Release Workflow

eyeVesa v1 stays private and controlled by Hafizal. The public Community repository must be created from a sanitized export, not from a fork that carries private git history.

## Private v1 Repository

- Keep production operations, GCP deployment, and International Airport control in the private repo.
- Do not commit live env files, Terraform state, Terraform plans, private keys, or issued API keys.
- Issue one named, revocable International Airport key per developer or gateway.
- Deliver keys through a password manager or Secret Manager, never through git.

## Public Community Repository

- Create a new empty public repository when ready.
- Export a clean working tree with no `.git` directory or private history.
- Include source code, SDKs, CLI, local Docker setup, migrations, tests, and placeholder docs.
- Exclude production ops files, real domains if private, env files, Terraform state, and generated plans.
- Public users run a local Airport by default. Official International Airport write access remains invite/API-key controlled.

## Ongoing Sync

Use the private repo for normal work. Push public-safe commits through a clean sync branch:

```bash
git checkout community-sync
git pull community main
git cherry-pick <safe-private-commit-sha>
git push community community-sync:main
git checkout main
```

Golden rule: work on private `main`, publish only from `community-sync`.
