// Command toy mirrors the in-process toy in golden_test.go for exec mode.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := append(os.Args[1:], "")
	switch args[0] {
	case "--stdin":
		data, _ := io.ReadAll(os.Stdin)
		fmt.Fprintf(os.Stderr, "no match in %d bytes\n", len(data))
		os.Exit(1)
	case "--env":
		wd, _ := os.Getwd()
		fmt.Printf("COLUMNS=%s NO_COLOR=%s cwd=%s\n", os.Getenv("COLUMNS"), os.Getenv("NO_COLOR"), filepath.Base(wd))
		return
	}
	fmt.Println(strings.Join(args[:len(args)-1], " "))
}
