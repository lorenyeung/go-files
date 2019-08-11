package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

//Creds struct for creating download.json
type Creds struct {
	URL        string
	Username   string
	Apikey     string
	DlLocation string
	Repository string
}

// VerifyAPIKey for errors
func VerifyAPIKey(urlInput, userName, apiKey string) bool {
	log.Print("starting VerifyAPIkey request")
	fmt.Println("Testing " + userName)
	data := GetRestAPI(urlInput+"/api/system/ping", userName, apiKey)
	if string(data) == "OK" {
		fmt.Println("Credentials are good to go.")
		log.Print("finished VerifyAPIkey request")
		return true
	}
	fmt.Println(string(data))
	log.Print("finished VerifyAPIkey request")
	return false
}

// GenerateDownloadJSON (re)generate download JSON. Tested.
func GenerateDownloadJSON() {
	var urlInput, userName, apiKey string
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your url: ")
	for {
		urlInput, _ = reader.ReadString('\n')
		urlInput = strings.TrimSuffix(urlInput, "\n")
		fmt.Print("Enter your username: ")
		userName, _ = reader.ReadString('\n')
		userName = strings.TrimSuffix(userName, "\n")
		fmt.Print("Enter your API key: ")
		apiKey, _ = reader.ReadString('\n')
		apiKey = strings.TrimSuffix(apiKey, "\n")
		if VerifyAPIKey(urlInput+"/api/system/ping", userName, apiKey) {
			break
		} else {
			fmt.Print("Something seems wrong, please try again. Enter your url: ")
		}
	}
	fmt.Println("username:" + userName)
	//TODO need to check if directory exists and/or valid directory
	fmt.Print("Enter your Download location: ")
	dlLocationInput, _ := reader.ReadString('\n')
	dlLocationInput = strings.TrimSuffix(dlLocationInput, "\n")

	//TODO need to check if repo exists
	fmt.Print("Enter your repository: ")
	repoInput, _ := reader.ReadString('\n')
	repoInput = strings.TrimSuffix(repoInput, "\n")

	data := &Creds{
		URL:        urlInput,
		Username:   userName,
		Apikey:     apiKey,
		DlLocation: dlLocationInput,
		Repository: repoInput,
	}
	file, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("The JSON write failed with error %s\n", err)
	}
	_ = ioutil.WriteFile("test.json", file, 0644)
}

//GetDownloadJSON get data from DownloadJSON
func GetDownloadJSON(fileLocation string) Creds {
	var result map[string]interface{}
	var resultData Creds
	file, err := os.Open(fileLocation)
	if err != nil {
		log.Fatal("error:", err)
	} else {
		defer file.Close()
		byteValue, _ := ioutil.ReadAll(file)
		json.Unmarshal([]byte(byteValue), &result)
		//TODO need to validate some of these fields
		resultData.URL = result["URL"].(string)
		resultData.Username = result["Username"].(string)
		resultData.Apikey = result["Apikey"].(string)
		resultData.DlLocation = result["DlLocation"].(string)
		resultData.Repository = result["Repository"].(string)
	}
	return resultData
}

//GetRestAPI GET rest APIs response with error handling
func GetRestAPI(urlInput, userName, apiKey string) []byte {
	client := http.Client{}
	req2, err := http.NewRequest("GET", urlInput, nil)
	req2.SetBasicAuth(userName, apiKey)
	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
	} else {
		resp, err := client.Do(req2)
		if err != nil {
			fmt.Printf("The HTTP response failed with error %s\n", err)
		}
		data, _ := ioutil.ReadAll(resp.Body)
		return data
	}
	return nil
}
