package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	black = iota + 30
	red
	green
	yellow
	blue
	magenta
	cyan
	white

	bold      = 1
	underline = 4

	yes = "✔"
	no  = "✘"
)

func colored(s string, color int, attr int) string {
	return fmt.Sprintf("\033[%d;%dm%s\033[0m", attr, color, s)
}

func cprintf(color int, attr int, format string, a ...interface{}) {
	fmt.Printf(colored(format, color, attr), a...)
}

func spin(text string) func() {
	dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < len(dots); i++ {
		dots[i] = colored(dots[i], blue, 0)
	}
	overlay := strings.Repeat(" ", len(text)+2)
	text = colored(text, blue, 0)
	stop := make(chan struct{})
	done := make(chan struct{})

	fmt.Printf("%s %s", dots[0], text)
	go func() {
		for i := 1; ; i++ {
			select {
			case <-time.After(100 * time.Millisecond):
				fmt.Printf("\r%s %s", dots[i%len(dots)], text)
			case <-stop:
				fmt.Printf("\r%s\r", overlay)
				done <- struct{}{}
				return
			}
		}
	}()
	return func() {
		stop <- struct{}{}
		// Wait and make sure the spinner is erased.
		<-done
	}
}
