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

// costguard-vm cleans up idle and labelled resources in a STACKIT
// organization, run by systemd timers on a server that Terraform sets up.
// See the README for the features and labels.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	// The messages show times in the configured zone; the binary must not
	// depend on the server's zoneinfo.
	_ "time/tzdata"

	"github.com/stackitcloud/professional-service/apps/costguard-vm/internal/app"
	"github.com/stackitcloud/professional-service/apps/costguard-vm/internal/config"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = ""

func main() {
	configPath := flag.String("config", "", "path to the YAML config (default: $COSTGUARD_CONFIG, then "+config.DefaultConfigPath+")")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s\n\n"+
			"  report  scan and post a report; changes nothing (the report run while delete is off)\n"+
			"  flag    post, then label new cleanup candidates delete=true (the report run while delete is on)\n"+
			"  delete  delete everything labelled delete=true that may go (needs delete on)\n"+
			"  boot    wait for the login, then post a report; changes nothing (once after every server start)\n\n", app.Usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(buildVersion())
		return
	}
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(app.ExitUsage)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := app.Run(ctx, app.Options{
		Subcommand: flag.Arg(0),
		ConfigPath: *configPath,
		LogLevel:   os.Getenv(config.EnvLogLevel),
		Version:    buildVersion(),
	})
	stop()
	os.Exit(code)
}

// buildVersion prefers the linker-set version (release images), then the
// module version (set by "go install ...@vX.Y.Z").
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
