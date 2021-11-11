package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lorenyeung/go-files/auth"
	"github.com/lorenyeung/go-files/helpers"
	"github.com/lorenyeung/go-files/rest"

	log "github.com/sirupsen/logrus"
)

var gitCommit string
var version string

func printVersion() {
	fmt.Println("Current build version:", gitCommit, "Current Version:", version)
}

func main() {
	versionFlag := flag.Bool("v", false, "Print the current version and exit")
	helpers.OrchestrateFlags = helpers.SetFlags()
	switch {
	case *versionFlag:
		printVersion()
		return
	}
	helpers.SetLogger(helpers.OrchestrateFlags.LogLevelVar)

	readmeFileName := ".lorenyfolderReadme"

	if _, err := os.Stat(helpers.OrchestrateFlags.HomeVar); os.IsNotExist(err) {
		log.Info("No config folder")
		err = os.MkdirAll(helpers.OrchestrateFlags.HomeVar, 0700)
		helpers.Check(err, true, "Generating .downloader directory", helpers.Trace())
	}

	masterKey := auth.VerifyMasterKey(helpers.OrchestrateFlags.HomeVar + "master.key")
	creds := auth.GetDownloadJSON(helpers.OrchestrateFlags.HomeVar+"download.json", masterKey)
	log.Debug("verifying API KEY, URL:", creds.URL, " username:", creds.Username)
	if !auth.VerifyAPIKey(creds.URL, creds.Username, creds.Apikey) {
		log.Warn("Looks like there's an issue with your credentials.")
		auth.GenerateDownloadJSON(helpers.OrchestrateFlags.HomeVar+"download.json", true, masterKey)
		creds = auth.GetDownloadJSON(helpers.OrchestrateFlags.HomeVar+"download.json", masterKey)
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

	if helpers.OrchestrateFlags.ManualReadmeVar != "" {
		log.Info("Creating manual readme for:", creds.DlLocation+"/"+helpers.OrchestrateFlags.ManualReadmeVar+"/"+readmeFileName)
		rest.DetectDetailsFile(creds.DlLocation+"/"+helpers.OrchestrateFlags.ManualReadmeVar+"/"+readmeFileName, masterKey)
		os.Exit(0)
	}

	if helpers.OrchestrateFlags.ShowDownloadedFoldersVar {
		log.Info("Showing downloaded Folders")

		for i := 0; i < len(folders); i++ {
			data := showReadmeData(folders, i, creds, readmeFileName, masterKey)
			fmt.Println(folders[i], "\tLast update: ", data.LastModified, "\tTitle: "+data.Title+" - "+data.Description)
		}
		os.Exit(0)
	}
	if helpers.OrchestrateFlags.DeleteVar != "" {
		deletetil, err := time.Parse(time.RFC3339, helpers.OrchestrateFlags.DeleteVar)
		helpers.Check(err, true, "Parse delete time failed", helpers.Trace())
		log.Info("Deleting up to:", deletetil)

		if !helpers.OrchestrateFlags.DeleteVerifyVar {
			log.Info("Just checking files for deletion, will not delete. To delete, add -delv")
		}

		for i := 0; i < len(folders); i++ {
			data := showReadmeData(folders, i, creds, readmeFileName, masterKey)
			shouldDelete := data.LastModified.Before(deletetil)

			if !helpers.OrchestrateFlags.DeleteVerifyVar {
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
	if helpers.OrchestrateFlags.FolderVar == "" {
		log.Error("Please enter a folder")
		os.Exit(0)
	}
	if helpers.OrchestrateFlags.RepoVar != "" {
		log.Debug("Detected repo override:", helpers.OrchestrateFlags.RepoVar)
		creds.Repository = helpers.OrchestrateFlags.RepoVar
	}
	sorted := rest.GetFilesDetails(creds.Username, creds.Apikey, creds.URL, creds.Repository, helpers.OrchestrateFlags.FolderVar)

	rest.DownloadFilesList(sorted, creds, helpers.OrchestrateFlags, masterKey, readmeFileName)

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
