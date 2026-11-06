#!/usr/bin/env bash

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

set -euo pipefail

# STACKIT-Whisper-functions — First-time local setup
# Usage: ./setup.sh
#
# This script ONLY prepares your local environment: it installs the `sfn`
# CLI and copies the .env template. It does NOT deploy anything and does NOT
# create any billable STACKIT resource. Deploying (./deploy.sh) is a
# separate, explicitly-confirmed step — see GETTING-STARTED.md.

echo "=== STACKIT-Whisper-functions Setup ==="

cd "$(dirname "$0")"

# Check prerequisites
command -v curl >/dev/null 2>&1 || { echo "Error: curl is required."; exit 1; }
command -v docker >/dev/null 2>&1 || echo "Warning: docker not found — needed later to build/run the function locally."

# Install/update the sfn CLI (local, free — no cloud resources created)
echo "Installing the sfn CLI (STACKIT Functions CLI)..."
./install-sfn.sh

# Environment
if [ ! -f whisper-fn/.env ]; then
  cp whisper-fn/.env.example whisper-fn/.env
  echo "Created whisper-fn/.env from whisper-fn/.env.example — edit it with your S3 bucket/credentials"
else
  echo "whisper-fn/.env already exists — leaving it as is"
fi

echo ""
echo "=== Setup complete! ==="
echo ""
echo "Next steps (manual — nothing further was created or deployed):"
echo "  1. Edit whisper-fn/.env with your S3 bucket, credentials, and endpoint"
echo "  2. Create the Object Storage bucket and Container Registry via the"
echo "     STACKIT Portal (one-time, manual — see GETTING-STARTED.md)"
echo "  3. Add \$HOME/.local/bin to your PATH if the sfn CLI install warned about it"
echo "  4. Test locally without Docker/S3: python3 whisper-fn/test_handler_local.py"
echo "  5. When ready to deploy (creates billable STACKIT resources):"
echo "     ./deploy.sh <project-id> <registry-image-ref> <registry-username>"
echo "  6. Using Claude Code? CLAUDE.md has all the context, including the"
echo "     cost-safety rule for this project."
