package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	user := os.Getenv("USER")
	fmt.Println(user)
	fmt.Println(os.Args)
	fmt.Println(runtime.Version())
}
