package main

import (
	"os"

	"github.com/one710/codeye/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args))
}
