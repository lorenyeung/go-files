package main

import (
	"fmt"
	"go-files/auth"
	"go-files/helpers"
	"go-files/rest"
	"log"
	"os"
	"os/user"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("y u do dis")
		os.Exit(0)
	}
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	argsWithoutProg := os.Args[1:]
	configPath := usr.HomeDir + "/.lorenygo/downloader/"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Println("No config folder")
		err = os.MkdirAll(configPath, 0700)
		helpers.Check(err, true, "Generating .downloader directory")
	}
	masterKey := auth.VerifyMasterKey(configPath + "master.key")
	creds := auth.GetDownloadJSON(configPath+"download.json", masterKey)
	if !auth.VerifyAPIKey(creds.URL, creds.Username, creds.Apikey) {
		fmt.Println("Looks like there's an issue with your credentials.")
		auth.GenerateDownloadJSON(configPath+"download.json", true, masterKey)
		creds = auth.GetDownloadJSON(configPath+"download.json", masterKey)
	}
	var download = argsWithoutProg[0]
	sorted := rest.GetFilesDetails(creds.Username, creds.Apikey, creds.URL, creds.Repository, download)
	rest.DownloadFilesList(sorted, creds)

}
