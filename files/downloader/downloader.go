package main

import (
	"files/auth"
	"files/rest"
	"fmt"
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
	configPath := usr.HomeDir + "/.downloader/"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Println("No config folder")
		err = os.Mkdir(configPath, 0700)
		if err != nil {
			log.Fatal(err)
		}
	}
	creds := auth.GetDownloadJSON(configPath + "download.json")
	if !auth.VerifyAPIKey(creds.URL, creds.Username, creds.Apikey) {
		fmt.Println("Looks like there's an issue with your credentials.")
		auth.GenerateDownloadJSON(configPath+"download.json", true)
	} else {
		encrypted := auth.Encrypt([]byte(creds.Username), "myString")
		fmt.Printf("decrypted %s encrypted %s\n", creds.Username, encrypted)
		plaintext2 := auth.Decrypt(encrypted, "myString")
		fmt.Printf("correctly decrypted %s\n", plaintext2)

		var download = argsWithoutProg[0]
		sorted := rest.GetFilesDetails(creds.Username, creds.Apikey, creds.URL, creds.Repository, download)
		rest.DownloadFilesList(sorted, creds)
	}
}
