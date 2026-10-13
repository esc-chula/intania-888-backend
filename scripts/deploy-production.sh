#!/usr/bin/env bash
set -euo pipefail

# Both the migration and API revision use the same immutable container image.
readonly project_id=intania-888
readonly region=asia-southeast1
readonly service_name=intania888-api
readonly image_repository=asia-southeast1-docker.pkg.dev/intania-888/intania888/intania-888-backend

if [[ $# -ne 1 || "$1" != "${image_repository}@sha256:"* || ! "${1##*@}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "Usage: $0 ${image_repository}@sha256:<digest>" >&2
  exit 1
fi

if [[ ! "${GITHUB_RUN_ID:-}" =~ ^[0-9]+$ || ! "${GITHUB_RUN_ATTEMPT:-}" =~ ^[0-9]+$ ]]; then
  echo 'GITHUB_RUN_ID and GITHUB_RUN_ATTEMPT must identify this deployment.' >&2
  exit 1
fi

readonly image="$1"
readonly revision_suffix="gh-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
readonly revision_name="${service_name}-${revision_suffix}"

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

echo 'Applying pending database migrations.'
gcloud run jobs deploy intania888-migrate \
  --project="$project_id" \
  --region="$region" \
  --image="$image" \
  --service-account=intania888-migrator@intania-888.iam.gserviceaccount.com \
  --env-vars-file=config/database-jobs.production.yaml \
  --set-secrets=DB_PASS=intania888-db-migrator-password:2 \
  --network=intania888-network \
  --subnet=intania888-run-sg \
  --vpc-egress=private-ranges-only \
  --tasks=1 \
  --parallelism=1 \
  --max-retries=1 \
  --task-timeout=10m \
  --cpu=1 \
  --memory=512Mi \
  --command=/bin/migrate \
  --args=up \
  --execute-now \
  --wait \
  --quiet

secret_refs=(
  'DB_PASS=intania888-db-app-password:1'
  'CACHE_PASS=intania888-redis-auth:2'
  'JWT_ACCESS_TOKEN_SECRET=intania888-jwt-signing:1'
  'OAUTH_CLIENT_ID=intania888-google-client-id:1'
  'OAUTH_CLIENT_SECRET=intania888-google-client-secret:1'
  'INTANIA_GAMES_CLIENT_SECRET=intania888-games-client-secret:1'
  '/secrets/redis/ca.pem=intania888-redis-ca:2'
  '/secrets/auth/registry.yaml=intania888-auth-registry:1'
)
secrets_argument=$(IFS=,; printf '%s' "${secret_refs[*]}")

echo "Deploying revision ${revision_name} with zero production traffic."
# Public access is already configured on this service; preserve its IAM policy.
gcloud run deploy "$service_name" \
  --project="$project_id" \
  --region="$region" \
  --image="$image" \
  --service-account=intania888-runtime@intania-888.iam.gserviceaccount.com \
  --env-vars-file=config/api.production.yaml \
  --set-secrets="$secrets_argument" \
  --network=intania888-network \
  --subnet=intania888-run-sg \
  --vpc-egress=private-ranges-only \
  --execution-environment=gen2 \
  --port=8080 \
  --cpu=1 \
  --memory=512Mi \
  --min=0 \
  --max=6 \
  --concurrency=40 \
  --timeout=60s \
  --startup-probe='httpGet.path=/readyz,httpGet.port=8080,initialDelaySeconds=0,periodSeconds=10,timeoutSeconds=3,failureThreshold=24' \
  --liveness-probe='httpGet.path=/healthz,httpGet.port=8080,periodSeconds=30,timeoutSeconds=2,failureThreshold=3' \
  --ingress=all \
  --revision-suffix="$revision_suffix" \
  --tag=candidate \
  --no-traffic \
  --quiet

candidate_url=$(gcloud run services describe "$service_name" \
  --project="$project_id" \
  --region="$region" \
  --format='json(status.traffic)' |
  python3 -c '
import json
import sys

revision_name = sys.argv[1]
routes = json.load(sys.stdin)["status"]["traffic"]
urls = [
    route["url"] for route in routes
    if route.get("tag") == "candidate" and route.get("revisionName") == revision_name
]
if len(urls) != 1 or not urls[0].startswith("https://"):
    raise SystemExit("Could not identify the candidate revision URL.")
print(urls[0])
' "$revision_name")

echo "Checking readiness at ${candidate_url}/readyz."
readiness=$(curl --fail --silent --show-error \
  --retry 6 \
  --retry-all-errors \
  --retry-delay 5 \
  --retry-max-time 180 \
  --max-time 30 \
  "${candidate_url}/readyz")

python3 -c '
import json
import sys

if json.loads(sys.argv[1]).get("status") != "ready":
    raise SystemExit("The candidate revision is not ready; traffic was not changed.")
' "$readiness"

echo "Routing production traffic to ${revision_name}."
gcloud run services update-traffic "$service_name" \
  --project="$project_id" \
  --region="$region" \
  --to-revisions="${revision_name}=100" \
  --remove-tags=candidate \
  --quiet

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    printf '## Production deployment\n\n'
    printf -- '- Commit: `%s`\n' "${DEPLOY_COMMIT:-unknown}"
    printf -- '- Image: `%s`\n' "$image"
    printf -- '- Revision: `%s`\n' "$revision_name"
    printf -- '- API: https://888-api.intania.org\n'
    printf -- '- Database migrations and candidate readiness completed.\n'
  } >> "$GITHUB_STEP_SUMMARY"
fi
