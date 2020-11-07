package main

import (
	"fmt"
	"go-files/auth"
	"go-files/helpers"
	"go-files/rest"
	"os"
	"os/exec"
	"os/user"
	"strings"

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

	//custom file/folder names
	configPath := usr.HomeDir + "/.lorenygo/downloader/"
	readmeFileName := ".lorenyfolderReadme"

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
		folders := strings.Split(string(stdout), "\n")
		for i := 0; i < len(folders); i++ {
			fmt.Print(folders[i])
			fi, err := os.Stat(creds.DlLocation + "/" + folders[i])
			helpers.Check(err, false, "Checking if file is folder", helpers.Trace())
			if fi.Mode().IsDir() {
				if _, err := os.Stat(creds.DlLocation + "/" + folders[i] + "/" + readmeFileName); os.IsNotExist(err) {
					log.Debug("No readme")

				} else {
					data := rest.ReadDetailsFile(creds.DlLocation+"/"+folders[i]+"/"+readmeFileName, masterKey)
					fmt.Print(" Last update: ", data.LastModified, " Title: "+data.Title+" - "+data.Description)
				}
			}
			fmt.Println("")
		}
		os.Exit(0)
	}
	//var download = flags.FolderVar
	sorted := rest.GetFilesDetails(creds.Username, creds.Apikey, creds.URL, creds.Repository, flags.FolderVar)

	rest.DownloadFilesList(sorted, creds, flags, masterKey, readmeFileName)

}
