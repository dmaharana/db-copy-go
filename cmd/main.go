package main

import (
	"fmt"
	"os"

	"db-copy/internal/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
