package main

import (
	"files/auth"
	"files/rest"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("y u do dis")
		os.Exit(0)
	}
	argsWithoutProg := os.Args[1:]
	creds := auth.GetDownloadJSON("download.json")
	if !auth.VerifyAPIKey(creds.URL, creds.Username, creds.Apikey) {
		fmt.Println("Looks like there's an issue with your credentials.")
		auth.GenerateDownloadJSON()
	} else {
		var download = argsWithoutProg[0]
		sorted := rest.GetFilesDetails(creds.Username, creds.Apikey, creds.URL, creds.Repository, download)
		rest.DownloadFiles(sorted, creds)
	}
}
