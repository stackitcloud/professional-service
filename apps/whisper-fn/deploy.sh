#!/bin/sh

# Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Convenience wrapper around the sfn CLI steps in GETTING-STARTED.md.
# Usage: ./deploy.sh <stackit-project-id> <registry-image-ref> <registry-username>
# Example: ./deploy.sh <your-stackit-project-id> registry.onstackit.cloud/<your-registry-project>/whisper-fn:0.1.1 'robot$<your-registry-project>+<robot-name>'
#
# Requires: Docker running, sfn CLI installed (see install-sfn.sh),
# whisper-fn/.env filled in from whisper-fn/.env.example, and a registry
# account/robot already set up (see GETTING-STARTED.md step 5 -- the
# Container Registry itself has no CLI/Terraform support, so that part
# can't be scripted here).
#
# Note: this builds via the Dockerfile, NOT `sfn functions build`
# (Cloud Native Buildpacks) -- the buildpack output includes a zero-byte
# metadata layer that this project's STACKIT Harbor registry rejects on
# push. See DECISIONS.md.
set -e

PROJECT_ID="$1"
IMAGE_REF="$2"
REGISTRY_USER="$3"
if [ -z "$PROJECT_ID" ] || [ -z "$IMAGE_REF" ] || [ -z "$REGISTRY_USER" ]; then
    echo "Usage: $0 <stackit-project-id> <registry-image-ref> <registry-username>"
    echo "  registry-image-ref must use a SemVer tag, e.g. registry.onstackit.cloud/<project>/whisper-fn:0.1.1"
    echo "  registry-username is the robot account, e.g. 'robot\$<registry-project>+<robot-name>'"
    exit 1
fi

command -v sfn >/dev/null 2>&1 || { echo "ERROR: sfn CLI not found or too old (need >= 1.7.0). Run ./install-sfn.sh first (and add it to PATH)."; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "ERROR: docker not found. Install Docker Desktop (or a docker-socket-compatible alternative)."; exit 1; }
docker info >/dev/null 2>&1 || { echo "ERROR: docker daemon not responding. Start Docker Desktop first."; exit 1; }

cd "$(dirname "$0")/whisper-fn"

if [ ! -f .env ]; then
    echo "ERROR: whisper-fn/.env not found. Copy .env.example to .env and fill in your S3 bucket/credentials first."
    exit 1
fi

echo "== Step 1/4: local build (Dockerfile, linux/amd64) =="
docker build --platform linux/amd64 -t whisper-fn:local .

echo "== Step 2/4: push to registry =="
docker tag whisper-fn:local "$IMAGE_REF"
echo "You will be prompted for the registry password/robot secret."
docker login registry.onstackit.cloud --username "$REGISTRY_USER"
docker push "$IMAGE_REF"

echo "== Step 3/4: auth with STACKIT Functions =="
sfn auth login --project-id "$PROJECT_ID"

echo
echo "This will run 'sfn functions deploy', which creates a billable STACKIT"
echo "function instance in project $PROJECT_ID. Make sure .stackit-functions/revision.yaml's"
echo "'image' field matches $IMAGE_REF and 'plan' has enough memory for your WHISPER_MODEL"
echo "(see GETTING-STARTED.md step 7 -- small needs f5+, large-v3/medium don't fit any plan)."
printf "Continue? [y/N] "
read -r CONFIRM
case "$CONFIRM" in
    y|Y|yes|YES) ;;
    *) echo "Aborted. Nothing was deployed."; exit 0 ;;
esac

echo "== Step 4/4: deploy (no --build/--push, image is already in the registry) =="
sfn functions deploy --project-id "$PROJECT_ID" --env-file .env --no-interactive
