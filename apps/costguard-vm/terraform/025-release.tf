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


# The costguard-vm release this Terraform code installs. `make pin
# VERSION=vX.Y.Z` writes this file right before the release is tagged; the
# release CI rebuilds the binaries and fails if its hashes differ (the build
# is reproducible). So a checkout of a release tag always installs exactly
# the binary built from that tag, and a changed download can't run.
#
# Between releases the version is empty: then only binary_override (a test
# build with its own hash) can be applied.
locals {
  release = {
    version = ""
    sha256  = {}
    # Where the release pipeline publishes the binaries; empty until the first
    # release, so a test build must set download_url.
    download_url = ""
  }
}
