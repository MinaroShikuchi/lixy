// cmd/cli/main.go
package main

import (
	"fmt"
	"os"

	"github.com/MinaroShikuchi/lixy/cmd/lixy/cli/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
