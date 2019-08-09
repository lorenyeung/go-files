package main

import (
	"encoding/json"
	"files/apikey"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

// StorageJSON api/storage call
type StorageJSON struct {
	Children []struct {
		URI    string `json:"uri"`
		Folder string `json:"folder"`
	} `json:"children"`
}

//FileStorageJSON API call
type FileStorageJSON struct {
	LastModified string `json:"lastModified"`
	Path         string `json:"path"`
	Size         string `json:"size"`
	DownloadURI  string `json:"downloadUri"`
}

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
		if !apikey.VerifyAPIKey(username, apiKey, url) {
			fmt.Println("Looks like there's an issue with your credentials.")
			apikey.GenerateDownloadJSON()
		} else {
			var download = argsWithoutProg[0]
			//check for 404, could be its own function
			// fmt.Print("test new func:")
			// fmt.Println(string(GetRestAPI(url+"/"+repo+"/"+download+"/", username, apiKey)))

			var result2 StorageJSON
			var data = GetRestAPI(url+"/api/storage/"+repo+"/"+download+"/", username, apiKey)
			json.Unmarshal([]byte(data), &result2)
			fmt.Println("Found the following files under" + url + "/" + repo + "/" + download + "/\nNumber\tLast Modified\tSize\tPath")
			for i := 0; i < len(result2.Children); i++ {
				//need to deal with index out of range
				//fmt.Println("data:" + result2.Children[i].URI)
				var result3 FileStorageJSON
				var data2 = GetRestAPI(url+"/api/storage/"+repo+"/"+download+result2.Children[i].URI, username, apiKey)
				json.Unmarshal([]byte(data2), &result3)

				size, err := strconv.ParseInt(result3.Size, 10, 64)
				if err != nil {
					fmt.Printf("%d is not of type %T", size, size)
					os.Exit(127)
				}
				fmt.Printf("%d\t%s\t%s\t%s\n", i+1, result3.LastModified, ByteCountDecimal(size), result3.Path)
			}
		}
	}
}

//ByteCountDecimal d
func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

//GetRestAPI blah
func GetRestAPI(url string, username string, apiKey string) []byte {
	client := http.Client{}
	req2, err := http.NewRequest("GET", url, nil)
	req2.SetBasicAuth(username, apiKey)
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
