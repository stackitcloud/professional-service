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

// costguard is the STACKIT cost-hygiene bot: one image, three
// subcommands (scan, delete, callback). All configuration is supplied
// at runtime via environment variables or the --config YAML file; the
// binary contains no secrets and no STACKIT-specific values.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/app"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to the YAML config file (default: $COSTGUARD_CONFIG)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: costguard [--config path] <scan|delete|callback>\n\nsubcommands:\n  scan     weekly warning run: scan, mark candidates, send report\n  delete weekly execution run: delete due candidates, send confirmation\n  callback protect endpoint for the interactive \"do not delete\" buttons\n")
	}
	flag.Parse()

	subcommand := ""
	if flag.NArg() > 0 {
		subcommand = flag.Arg(0)
	}
	if subcommand == "" {
		flag.Usage()
		os.Exit(app.ExitUsage)
	}

	// Signal-aware root context: SIGTERM/SIGINT cancel the run (CronJob
	// termination, kubectl delete) and stop the callback server.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := app.Run(ctx, subcommand, *configPath, os.Getenv(config.EnvLogLevel))
	os.Exit(code)
}
