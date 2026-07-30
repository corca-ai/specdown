package main

import (
	_ "embed"
	"os"

	"github.com/corca-ai/specdown/internal/specdown/cli"
)

var version = "dev"

//go:embed skills/specdown/SKILL.md
var skillSpecdown string

func main() {
	cli.PrependExecutableToPath()
	os.Exit(cli.Execute(
		os.Args[1:],
		os.Stdin,
		os.Stdout,
		os.Stderr,
		cli.Options{Version: version, SkillSpecdown: skillSpecdown},
	))
}
