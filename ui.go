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
)

func colored(s string, color int, highlight int) string {
	return fmt.Sprintf("\033[%d;%dm%s\033[0m", highlight, color, s)
}

func cprintf(color int, highlight int, format string, a ...interface{}) {
	fmt.Printf(colored(format, color, highlight), a...)
}

func warning(format string, a ...interface{}) {
	cprintf(magenta, 1, "✘ "+format, a...)
}

func failed(format string, a ...interface{}) {
	cprintf(red, 1, "✘ "+format, a...)
}

func success(format string, a ...interface{}) {
	cprintf(cyan, 1, "✔ "+format, a...)
}

func spin(text string) func() {
	dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < len(dots); i++ {
		dots[i] = colored(dots[i], green, 0)
	}
	overlay := strings.Repeat(" ", len(text)+2)
	text = colored(text, black, 1)
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
