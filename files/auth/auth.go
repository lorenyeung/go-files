package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
}

// VerifyAPIKey for errors
func VerifyAPIKey(user string, apiKey string, url string) bool {
	fmt.Println("Testing " + user)
	client := http.Client{}
	req, err := http.NewRequest("GET", url+"/api/system/ping", nil)
	req.SetBasicAuth(user, apiKey)
	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
	} else {
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("The HTTP response failed with error %s\n", err)
		}
		data, _ := ioutil.ReadAll(resp.Body)
		if string(data) == "OK" {
			fmt.Println("Credentials are good to go.")
			return true
		}
		fmt.Println(string(data))
	}
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
		if VerifyAPIKey(userName, apiKey, urlInput) {
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

	data := &Creds{
		URL:        urlInput,
		Username:   userName,
		Apikey:     apiKey,
		DlLocation: dlLocationInput,
	}
	file, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
	}
	_ = ioutil.WriteFile("test.json", file, 0644)
}
