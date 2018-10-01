package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	baseURL  = "https://uva.onlinejudge.org"
	red      = "\033[0;31m"
	green    = "\033[0;32m"
	cyan     = "\033[1;36m"
	yellow   = "\033[0;33m"
	gray     = "\033[1;30m"
	hiyellow = "\033[1;33m"
	hiwhite  = "\033[1;37m"
	magenta  = "\033[1;35m"
	end      = "\033[0m"
)

func spin(text string) func() {
	dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < len(dots); i++ {
		dots[i] = fmt.Sprintf("%s%s%s", green, dots[i], end)
	}
	text = fmt.Sprintf("%s%s%s", gray, text, end)
	stop := make(chan struct{})
	done := make(chan struct{})
	fmt.Printf("%s %s", dots[0], text)
	go func() {
		for i := 1; ; i++ {
			select {
			case <-time.After(100 * time.Millisecond):
				fmt.Printf("\r%s %s", dots[i%len(dots)], text)
			case <-stop:
				fmt.Printf("\r%s\r", strings.Repeat(" ", len(text)+2))
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
