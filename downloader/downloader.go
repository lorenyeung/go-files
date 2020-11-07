package main

import (
	"fmt"
	"go-files/auth"
	"go-files/helpers"
	"go-files/rest"
	"os"
	"os/exec"
	"os/user"

	log "github.com/Sirupsen/logrus"
)

func main() {
	flags := helpers.SetFlags()
	helpers.SetLogger(flags.LogLevelVar)

	if flags.FolderVar == "" && flags.ShowDownloadedFoldersVar == false {
		log.Error("y u do dis")
		os.Exit(0)
	}
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	//argsWithoutProg := os.Args[1:]
	configPath := usr.HomeDir + "/.lorenygo/downloader/"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Info("No config folder")
		err = os.MkdirAll(configPath, 0700)
		helpers.Check(err, true, "Generating .downloader directory", helpers.Trace())
	}

	masterKey := auth.VerifyMasterKey(configPath + "master.key")
	creds := auth.GetDownloadJSON(configPath+"download.json", masterKey)
	if !auth.VerifyAPIKey(creds.URL, creds.Username, creds.Apikey) {
		log.Warn("Looks like there's an issue with your credentials.")
		auth.GenerateDownloadJSON(configPath+"download.json", true, masterKey)
		creds = auth.GetDownloadJSON(configPath+"download.json", masterKey)
	}
	if flags.ShowDownloadedFoldersVar == true {
		log.Info("Showing downloaded Folders")
		cmd := exec.Command("ls", creds.DlLocation)
		stdout, err := cmd.Output()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Print(string(stdout))
		os.Exit(0)
	}
	//var download = flags.FolderVar
	sorted := rest.GetFilesDetails(creds.Username, creds.Apikey, creds.URL, creds.Repository, flags.FolderVar)

	rest.DownloadFilesList(sorted, creds, flags)

}
