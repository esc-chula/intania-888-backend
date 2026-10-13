# Continuous deployment to Cloud Run

## What happens after a change reaches main

1. The existing **Quality** workflow runs for the push to `main`.
2. After Quality succeeds, **Deploy to Cloud Run** checks out that exact commit.
   A completed Quality run for an older, superseded main commit is skipped.
3. GitHub builds a Linux AMD64 image and pushes it to the Singapore Artifact Registry.
4. The Singapore migration job applies pending Goose migrations using that image.
5. Cloud Run creates a new `intania888-api` revision with zero production traffic.
6. The workflow requests that revision's `/readyz`, which checks PostgreSQL and Redis.
7. Only after readiness succeeds does the workflow move all production traffic to
   that specific revision. The production URL remains `https://888-api.intania.org`.

Deployment runs are serialized. A running migration/release is not cancelled when
another commit arrives. GitHub concurrency can replace an older pending run with a
newer pending run; intermediate commits do not each need a separate deployment.
Pull requests, `development`, feature branches, and tags do not deploy to production.

The catalogue seed is a separate operation and does not run on every release.

## One-time Google Cloud setup

From the repo directory, using the host `gcloud` account that manages `intania-888`:

```sh
bash scripts/setup-github-cd.sh
```

This command is also valid in Fish: `bash` runs the script with Bash.
The script stops if any command fails, and it can be rerun after resolving an error.
It changes only the APIs and IAM resources listed below; it does not run a deployment.

The script enables IAM, IAM Credentials, Security Token Service, and Resource
Manager APIs, then creates:

- Service account: `intania888-deployer@intania-888.iam.gserviceaccount.com`
- Workload Identity Pool: `intania888-github`, location `global`
- OIDC provider: `backend-production`

The provider accepts only this GitHub repository's numeric ID (`855932190`) and
owner ID (`66741385`), branch `main`, workflow `.github/workflows/deploy.yml`, and
the `workflow_run` or `workflow_dispatch` events. The principal binding also selects
the repository's numeric ID.

### Permissions granted

| Identity | Scope | Role |
| --- | --- | --- |
| Deployment service account | Project `intania-888` | Cloud Run Developer |
| Deployment service account | Project `intania-888` | Service Usage Consumer |
| Deployment service account | Singapore repository `intania888` | Artifact Registry Writer |
| Deployment service account | Runtime and migrator service accounts | Service Account User |
| Restricted GitHub principal | Deployment service account | Workload Identity User |

Cloud Run Developer allows updating services/jobs and running the migration job.
The script preserves the existing public API IAM policy. It does not grant Owner,
Editor, Cloud SQL Admin, Secret Manager access, or service account key permissions
to the deployment service account. The existing Cloud Run service agent handles
Direct VPC access; the runtime and migrator identities retain their existing secret
permissions.

GitHub exchanges its OIDC identity for a temporary impersonated OAuth access token.
The one-hour token is obtained after the Docker build and used for the publish,
migration, and rollout steps, within the workflow's 45-minute timeout. No Google
Cloud JSON key or application password needs to be copied into GitHub settings.

Wait at least **five minutes** after setup for IAM changes to propagate.

## Activate the workflow

Commit and merge these files into `main` through the normal repo workflow:

- `.github/workflows/deploy.yml`
- `scripts/deploy-production.sh`
- `scripts/setup-github-cd.sh`
- `config/api.production.yaml`
- `config/database-jobs.production.yaml`
- `.dockerignore`, `.gitignore`, and this deployment guide

The production YAML files contain public configuration. Credentials stay in Google
Secret Manager. Local credential JSON files and private environment files must not
be included in the commit.

To start a deployment manually after the workflow reaches `main`:

1. Open the repository on GitHub.
2. Open **Actions** > **Deploy to Cloud Run**.
3. Select **Run workflow**, choose `main`, and click **Run workflow**.
4. Open the run to follow the build, migration, readiness check, and traffic switch.

Manual runs deploy the selected `main` commit immediately and do not wait for a new
Quality run. Automatic runs require the existing Quality workflow to pass. Review
the run summary for the deployed commit, image digest, and revision.

To require passing Quality before changes can merge into `main`, a repository admin
can configure branch protection or a ruleset with the existing `checks` and
`integration` checks. Branch protection was not enabled when this setup was prepared.

## Configuration and secret rotation

Public runtime settings are in `config/api.production.yaml`; database job settings
are in `config/database-jobs.production.yaml`. Both target `asia-southeast1` private
endpoints and keep the application's `Asia/Bangkok` business timezone.

The production API pool allows six open and three idle database connections per
instance. The deployment script sets the Cloud Run service maximum to six
instances, for a steady-state budget of 36 API connections. Database jobs retain
two open and one idle connection per task. Rollouts, scaling transitions, other
clients, and additional job tasks can increase the total connection count.
These limits do not establish peak workload capacity; the Cloud SQL machine tier
is configured separately from application deployment.

`scripts/deploy-production.sh` pins these Secret Manager versions:

| Reference | Version |
| --- | --- |
| `intania888-db-app-password` | 1 |
| `intania888-db-migrator-password` | 2 |
| `intania888-redis-auth` | 2 |
| `intania888-redis-ca` | 2 |
| `intania888-jwt-signing` | 1 |
| `intania888-google-client-id` | 1 |
| `intania888-google-client-secret` | 1 |
| `intania888-games-client-secret` | 1 |
| `intania888-auth-registry` | 1 |

When rotating a secret, update the appropriate version reference in the deployment
script and merge the change. Editing `config/auth.production.yaml` alone does not
update the mounted auth registry: publish the intended registry as a new Secret
Manager version and update `intania888-auth-registry` in the script too.

## Failed deployment and rollback

If migration, revision creation, or candidate readiness fails, the workflow stops
before switching traffic. The existing API revision continues serving requests.
An unsuccessful candidate might retain the `candidate` tag until the next release.
Migration changes already applied to the shared database remain applied.

Migrations run while the previous API is still serving requests. Keep new migrations
compatible with that API. For a destructive schema change, first deploy code that
stops depending on the old schema and remove the old schema in a later release.

To roll back the API, find a known working revision in **Cloud Run** >
**intania888-api** (Singapore) > **Revisions**, then run:

```sh
gcloud run services update-traffic intania888-api \
  --project=intania-888 \
  --region=asia-southeast1 \
  --to-revisions=KNOWN_WORKING_REVISION=100
```

Replace `KNOWN_WORKING_REVISION` with the actual revision name. This changes API
traffic only; it does not undo database migrations. Do not automatically run Goose
`down` against the production database.

## Verification boundary

Local YAML and shell syntax checks can confirm that the files parse. The first
GitHub Actions run is still needed to confirm federation, IAM, image publishing,
migration execution, candidate routing, and the final production traffic switch.
Readiness does not verify browser OAuth, student admission policies, or peak capacity.

## References

- [Google GitHub Actions authentication](https://github.com/google-github-actions/auth)
- [Workload Identity Federation for deployment pipelines](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines)
- [Cloud Run deployment permissions](https://docs.cloud.google.com/run/docs/deploying)
- [Cloud Run traffic migration](https://docs.cloud.google.com/run/docs/rollouts-rollbacks-traffic-migration)
- [gcloud authentication with an access token](https://docs.cloud.google.com/sdk/docs/authenticate#use_the_gcloud_cli_with_an_access_token)
