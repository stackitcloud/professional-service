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

//go:build !dev

package stackit

import coreconfig "github.com/stackitcloud/stackit-sdk-go/core/config"

// newLogin makes every client log in with the service account attached to
// the server and nothing else: no key file, no token, no workload
// identity. The SDK's custom login wins over all of its own, even when the
// environment offers them. Local runs use the dev build tag (login_dev.go).
func newLogin(getenv func(string) string) (Login, []ConfigurationOption, error) {
	login, err := NewMetadataLogin(getenv(EnvServiceAccountEmail))
	if err != nil {
		return nil, nil, err
	}
	return login, []ConfigurationOption{coreconfig.WithCustomAuth(login)}, nil
}
