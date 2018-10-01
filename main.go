package main

import (
	"os"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Usage = "A cli tool to enjoy uva oj!"
	app.UsageText = "uva [command]"
	app.Version = "0.1.0"

	loadCookies := func(c *cli.Context) error {
		loadLoginInfo()
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:  "user",
			Usage: "manage account",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "l",
					Usage: "user login",
				},
				cli.BoolFlag{
					Name:  "L",
					Usage: "user logout",
				},
			},
			Action: user,
		},
		{
			Name:      "show",
			Usage:     "show problem by id",
			UsageText: "uva show ID",
			Action:    show,
			Before:    loadCookies,
		},
		{
			Name:      "touch",
			Usage:     "create source file",
			UsageText: "uva touch ID",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "lang",
					Value: "cc",
					Usage: "file extension",
				},
			},
			Action: touch,
		},
		{
			Name:      "submit",
			Usage:     "submit code",
			UsageText: "uva submit FILE",
			Action:    submitAndShowResult,
			Before:    loadCookies,
		},
		{
			Name:      "test",
			Usage:     "test code locally",
			UsageText: "uva submit FILE",
			Action:    testProgram,
		},
	}

	defer func() {
		if err := recover(); err != nil {
			cprintf(red, 0, "%s\n", err)
			os.Exit(1)
		}
	}()

	// make data directories
	for _, path := range []string{dataPath, pdfPath, testDataPath} {
		if !exists(path) {
			if err := os.Mkdir(path, 0755); err != nil {
				panic(err)
			}
		}
	}

	app.Run(os.Args)
}
