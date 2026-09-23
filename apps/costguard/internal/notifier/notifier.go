// Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package notifier defines the common output interface and the
// signed "Do not delete" button URLs. The orchestration layer in
// internal/app picks one adapter per run (COSTGUARD_OUTPUT) and never sees
// the backend's payload format.
package notifier

import (
	"context"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// Notifier is implemented by every webhook adapter.
type Notifier interface {
	// SendReport sends the full scan report.
	SendReport(ctx context.Context, rep *report.Report) error
	// SendDeletionSummary sends the post-deletion confirmation
	//. It is sent after every execution run — even when all
	// deletions failed.
	SendDeletionSummary(ctx context.Context, summary *report.DeletionSummary) error
}
