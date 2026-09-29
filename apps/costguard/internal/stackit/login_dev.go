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

//go:build dev

package stackit

import (
	"context"
	"time"
)

// newLogin leaves the login to the SDK's default order (workload identity,
// then a key file or a token from the environment), so a developer can run
// costguard locally with a service account key:
//
//	go run -tags dev ./cmd/costguard --config test.yaml report
//
// Release binaries are built without this tag and use the server's
// attached service account only.
func newLogin(func(string) string) (Login, []ConfigurationOption, error) {
	return sdkLogin{}, nil, nil
}

// sdkLogin has nothing to check up front: the SDK reports a missing
// credential when the clients are built.
type sdkLogin struct{}

func (sdkLogin) Ready(context.Context, time.Duration) error { return nil }
