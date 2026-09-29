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


# The costguard release this Terraform code installs. Written by
# `make pin VERSION=vX.Y.Z` (scripts/pin.py), never by hand. When a commit
# that changes this file is pushed, the release workflow rebuilds the
# binaries, refuses to release if their hashes differ (the build is
# reproducible) and publishes them under the tag apps/costguard/<version>.
# So a checkout of a release tag installs exactly the binary built from that
# tag, and a changed download can't run.
#
# main keeps the latest release's pin until the next release: install from a
# release tag, not from main. A test build sets binary_override instead.

locals {
  release = {
    version = "v0.1.0"
    sha256 = {
      amd64 = "7cff453d69043fba75bb0d105a4112d980dda16f2d482b23be067d78dbb16850"
      arm64 = "09a9d1950529503bf8aa79da2a96886eefb4849d954189561e9c8e345a42f8bb"
    }
    # Where the release workflow publishes the binaries; {version} is
    # filled in, the file name costguard_<version>_linux_<arch> appended.
    download_url = "https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/releases/download/apps%2Fcostguard%2F{version}/"
  }
}
