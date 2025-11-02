package main

import (
	"fmt"
	"os"

	"messages/pkg/msgclient"
)

func main() {
	if err := msgclient.RunCLI(os.Args[0], os.Args[1:], os.Stderr); err != nil {
		if usage, ok := err.(msgclient.UsageError); ok {
			fmt.Fprintln(os.Stderr, usage.Error())
			for _, line := range usage.UsageLines() {
				fmt.Fprintln(os.Stderr, line)
			}
			os.Exit(2)
		}
		os.Exit(1)
	}
}
