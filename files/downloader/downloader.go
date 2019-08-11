package main

import (
	"encoding/json"
	"files/auth"
	"files/rest"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("y u do dis")
		os.Exit(0)
	}
	argsWithoutProg := os.Args[1:]
	var result map[string]interface{}
	var url, username, apiKey, dlLocation, repo string
	file, err := os.Open("download.json")

	if err != nil {
		log.Fatal("error:", err)
	} else {
		defer file.Close()
		byteValue, _ := ioutil.ReadAll(file)
		json.Unmarshal([]byte(byteValue), &result)
		url = result["URL"].(string)
		username = result["Username"].(string)
		apiKey = result["Apikey"].(string)
		dlLocation = result["DlLocation"].(string)
		repo = result["Repository"].(string)
		fmt.Println(dlLocation) //go requires every declared var to be used remove later when working TODO
		if !auth.VerifyAPIKey(username, apiKey, url) {
			fmt.Println("Looks like there's an issue with your credentials.")
			auth.GenerateDownloadJSON()
		} else {
			var download = argsWithoutProg[0]
			//check for 404, could be its own function
			// fmt.Print("test new func:")
			// fmt.Println(string(GetRestAPI(url+"/"+repo+"/"+download+"/", username, apiKey)))

			//If there's files, then get details
			rest.GetFilesDetails(username, apiKey, url, repo, download)
		}
	}
}
