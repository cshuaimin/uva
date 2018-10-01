package main

import (
	"net/url"
	"os"
)

var (
	dataPath         = os.Getenv("HOME") + "/.local/share/uva-cli/"
	pdfPath          = dataPath + "pdf/"
	testDataPath     = dataPath + "test-data"
	loginInfoFile    = dataPath + "login-info.gob"
	problemsInfoFile = dataPath + "problems-info.gob"
	uvaURL, _        = url.Parse(baseURL)
)
