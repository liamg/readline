package main

import (
	"fmt"

	"github.com/liamg/readline"
	"github.com/liamg/readline/pkg/config"
)

func main() {
	rl, err := readline.New(
		"example",
		config.WithPrompt(func(w, h int) string {
			return "> "
		}),
	)
	if err != nil {
		panic(err)
	}
	for {
		line, err := rl.Readline()
		if err != nil {
			panic(err)
		}
		if line == "exit" {
			break
		}
		fmt.Printf("you entered %q\n", line)
	}
}
