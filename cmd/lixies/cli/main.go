package main

import (
	"fmt"
	"os"

	"github.com/MinaroShikuchi/lixy/cmd/lixies/cli/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}
