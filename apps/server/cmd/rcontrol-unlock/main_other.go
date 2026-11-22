//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("rcontrol-unlock is Windows-only")
}
