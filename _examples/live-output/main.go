package main

import (
	"fmt"
	"time"

	"github.com/liamg/readline"
	"github.com/liamg/readline/pkg/config"
)

func main() {
	rl, err := readline.New(
		"live-output-example",
		config.WithPrompt(func(_, _ int) string {
			return "agent> "
		}),
	)
	if err != nil {
		panic(err)
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		i := 1
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				_, _ = rl.Write([]byte(fmt.Sprintf("[%s] background event %d\n", t.Format("15:04:05"), i)))
				i++
			}
		}
	}()
	defer close(done)

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
