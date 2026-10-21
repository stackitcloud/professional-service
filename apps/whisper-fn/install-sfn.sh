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

set -e

# Check prerequisites:
which curl >/dev/null || (echo "ERROR: curl not found; this script requires curl to run"; exit 1)

# Defaults:
DRY_RUN="${DRY_RUN:-}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_VERSION="${INSTALL_VERSION:-latest}"
INSTALL_GOOS="${INSTALL_GOOS:-autodetect}"
INSTALL_GOARCH="${INSTALL_GOARCH:-autodetect}"

# Parse arguments: # https://stackoverflow.com/a/14203146
while [ $# -gt 0 ]; do
    case "$1" in
        --help)
            echo "Usage: $0 [--path \$HOME/.local/bin] [--version latest] [--os autodetect] [--arch autodetect] [--dry-run]"
            echo "Install the latest STACKIT Function CLI binary from GitHub releases"
            exit 1
        ;;
        --path)    INSTALL_DIR="$2"; shift ;;
        --version) INSTALL_VERSION="$2"; shift;;
        --os)      INSTALL_GOOS="$2"; shift;;
        --arch)    INSTALL_GOARCH="$2"; shift;;
        --dry-run) DRY_RUN='1';;
        --*) echo "ERROR: Unknown option $1" && exit 2;;
    esac
    shift || (echo "ERROR: Expected argument"; exit 2)
done

# Resolve OS and architecture:
if [ "$INSTALL_GOOS" = 'autodetect' ]; then
    case "$(uname -s)" in
        darwin | Darwin) INSTALL_GOOS="darwin";;
        linux | Linux) INSTALL_GOOS="linux";;
        *) echo "ERROR: Unknown operating system: $(uname -s)" && exit 3;;
    esac
fi
if [ "$INSTALL_GOARCH" = 'autodetect' ]; then
    case "$(uname -m)" in
        x86_64 | amd64) INSTALL_GOARCH="amd64";;
        aarch64 | arm64) INSTALL_GOARCH="arm64";;
        *) echo "ERROR: Unknown architecture: $(uname -m)" && exit 3;;
    esac
fi

# Resolve latest version:
if [ "$INSTALL_VERSION" = 'latest' ]; then
    INSTALL_VERSION="$(curl --silent https://api.github.com/repos/stackitcloud/sfn-cli/releases/latest | sed -En 's|.+"tag_name": "v([^"]+)".+|\1|p')"
fi

# Construct download URL:
DOWNLOAD_URL="https://github.com/stackitcloud/sfn-cli/releases/download/v${INSTALL_VERSION}/sfn_${INSTALL_VERSION}_${INSTALL_GOOS}_${INSTALL_GOARCH}.tar.gz"

# Download and install:
echo "INFO: Downloading SFN CLI $INSTALL_VERSION from $DOWNLOAD_URL and installing to $INSTALL_DIR"
if [ -z "$DRY_RUN" ]; then
    mkdir -p "$INSTALL_DIR"
    curl -L "$DOWNLOAD_URL" | tar -xz -C "$INSTALL_DIR" sfn
    echo "INFO: Installed SFN CLI to $INSTALL_DIR/sfn!"
else
    echo "INFO: Dry run, skipping installation"
fi

# Check whether PATH contains install dir: # https://unix.stackexchange.com/a/32054
case ":$PATH:" in
*:$INSTALL_DIR:*);;
*)
echo "WARNING: $INSTALL_DIR is not part of your \$PATH"
echo "To use the \`sfn\` command, add export PATH=\"$INSTALL_DIR:\$PATH\" to your ~/.bashrc or ~/.zshrc (or use fish_add_path $INSTALL_DIR)."
echo "Alternatively, call the sfn CLI directly using \`$INSTALL_DIR/sfn\`"
;;
esac
