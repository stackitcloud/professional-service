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

# Multi-stage build: all dependencies are pure Go, so the final image is a
# static binary on a distroless base with no shell, no package manager,
# and no credentials baked in.
FROM golang:1.27.1 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /costguard ./cmd/costguard

FROM gcr.io/distroless/static

USER nonroot:nonroot
COPY --from=builder /costguard /costguard

LABEL org.opencontainers.image.source="https://professional-service.git.onstackit.cloud/professional-service-best-practices/professional-service/tree/main/apps/costguard"
LABEL org.opencontainers.image.description="STACKIT cloud hygiene bot"
LABEL org.opencontainers.image.licenses="Apache-2.0"

ENTRYPOINT ["/costguard"]
