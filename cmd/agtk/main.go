package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/trungdlp/ai-gateway-testkit/internal/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, cli.OSEnvironment, resolvedBuildInfo()))
}

func resolvedBuildInfo() cli.BuildInfo {
	settings := map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			settings[setting.Key] = setting.Value
		}
		if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	return resolveBuildInfo(version, commit, date, settings)
}

func resolveBuildInfo(buildVersion, buildCommit, buildDate string, settings map[string]string) cli.BuildInfo {
	if (buildCommit == "" || buildCommit == "unknown") && settings["vcs.modified"] != "true" && settings["vcs.revision"] != "" {
		buildCommit = settings["vcs.revision"]
	}
	if (buildDate == "" || buildDate == "unknown") && settings["vcs.time"] != "" {
		buildDate = settings["vcs.time"]
	}
	return cli.BuildInfo{Version: buildVersion, Commit: buildCommit, Date: buildDate}
}
