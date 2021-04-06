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
	"time"

	log "github.com/Sirupsen/logrus"
)

func main() {
	flags := helpers.SetFlags()
	helpers.SetLogger(flags.LogLevelVar)

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

	//get ls of directory
	cmd := exec.Command("ls", creds.DlLocation)
	stdout, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	folders := strings.Split(string(stdout), "\n")
	if folders[len(folders)-1] == "" {
		log.Debug("last one is empty, removing")
		folders = folders[:len(folders)-1]
	}

	if flags.ManualReadmeVar != "" {
		log.Info("Creating manual readme for:", creds.DlLocation+"/"+flags.ManualReadmeVar+"/"+readmeFileName)
		rest.DetectDetailsFile(creds.DlLocation+"/"+flags.ManualReadmeVar+"/"+readmeFileName, masterKey)
		os.Exit(0)
	}

	if flags.ShowDownloadedFoldersVar {
		log.Info("Showing downloaded Folders")

		for i := 0; i < len(folders); i++ {
			data := showReadmeData(folders, i, creds, readmeFileName, masterKey)
			fmt.Println(folders[i], "\tLast update: ", data.LastModified, "\tTitle: "+data.Title+" - "+data.Description)
		}
		os.Exit(0)
	}
	if flags.DeleteVar != "" {
		deletetil, err := time.Parse(time.RFC3339, flags.DeleteVar)
		helpers.Check(err, true, "Parse delete time failed", helpers.Trace())
		log.Info("Deleting up to:", deletetil)

		if !flags.DeleteVerifyVar {
			log.Info("Just checking files for deletion, will not delete. To delete, add -delv")
		}

		for i := 0; i < len(folders); i++ {
			data := showReadmeData(folders, i, creds, readmeFileName, masterKey)
			shouldDelete := data.LastModified.Before(deletetil)

			if !flags.DeleteVerifyVar {
				fmt.Println(folders[i], "delete:", shouldDelete, "\tLast update: ", data.LastModified, " Title: "+data.Title+" - "+data.Description)

			} else {

				if shouldDelete {
					log.Info("Do actual delete of:", creds.DlLocation+"/"+folders[i]+"/")
					err := os.RemoveAll(creds.DlLocation + "/" + folders[i] + "/")
					helpers.Check(err, false, "Delete failed", helpers.Trace())
				} else {
					log.Debug("Skipping delete of:", creds.DlLocation+"/"+folders[i]+"/")
				}
			}
		}
		os.Exit(0)
	}
	//var download = flags.FolderVar
	if flags.FolderVar == "" {
		log.Error("Please enter a folder")
		os.Exit(0)
	}
	sorted := rest.GetFilesDetails(creds.Username, creds.Apikey, creds.URL, creds.Repository, flags.FolderVar)

	rest.DownloadFilesList(sorted, creds, flags, masterKey, readmeFileName)

}

func showReadmeData(folders []string, i int, creds auth.Creds, readmeFileName string, masterKey string) helpers.FolderDetailsJSON {
	fi, err := os.Stat(creds.DlLocation + "/" + folders[i])
	helpers.Check(err, false, "Checking if file is folder", helpers.Trace())
	if fi.Mode().IsDir() {
		if _, err := os.Stat(creds.DlLocation + "/" + folders[i] + "/" + readmeFileName); os.IsNotExist(err) {
			log.Debug("No readme")
		} else {
			data := rest.ReadDetailsFile(creds.DlLocation+"/"+folders[i]+"/"+readmeFileName, masterKey)
			return data
		}
	}
	return helpers.FolderDetailsJSON{}
}
