package rest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//FileStorageJSON file details call
type FileStorageJSON struct {
	LastModified  string `json:"lastModified"`
	ConvertedTime time.Time
	Size          string `json:"size"`
	DownloadURI   string `json:"downloadUri"`
}

// StorageJSON file list call
type StorageJSON struct {
	Children []struct {
		URI    string `json:"uri"`
		Folder string `json:"folder"`
	} `json:"children"`
}

//TimeSlice sorted data structure
type TimeSlice []FileStorageJSON

func (p TimeSlice) Len() int {
	return len(p)
}

func (p TimeSlice) Less(i, j int) bool {
	return p[i].ConvertedTime.Before(p[j].ConvertedTime)
}

func (p TimeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

//GetFilesDetails get file details, sort by date and print
func GetFilesDetails(username, apiKey, url, repo, download string) TimeSlice {

	//create map of all file details from list of files
	var unsorted = make(map[int]FileStorageJSON)
	var filesList StorageJSON
	var data = GetRestAPI(url+"/api/storage/"+repo+"/"+download+"/", username, apiKey)
	json.Unmarshal([]byte(data), &filesList)
	for len(filesList.Children) == 0 {
		fmt.Println("No files found under " + url + "/" + repo + "/" + download + "/. Enter again, or type n to quit:")
		reader := bufio.NewReader(os.Stdin)
		downloadIn, _ := reader.ReadString('\n')
		download = strings.TrimSuffix(downloadIn, "\n")
		if download == "n" {
			os.Exit(0)
		}
		data = GetRestAPI(url+"/api/storage/"+repo+"/"+download+"/", username, apiKey)
		json.Unmarshal([]byte(data), &filesList)
	}
	fmt.Println("Found the following files under " + url + "/" + repo + "/" + download + "/\nNumber\tLast Modified\t\tSize\tPath")

	var wg sync.WaitGroup //multi threading the GET details request. currently not concurrent write safe
	wg.Add(len(filesList.Children))
	for i := 0; i < len(filesList.Children); i++ {
		go func(i int) {
			defer wg.Done()
			var fileDetail FileStorageJSON
			var data2 = GetRestAPI(url+"/api/storage/"+repo+"/"+download+filesList.Children[i].URI, username, apiKey)
			json.Unmarshal([]byte(data2), &fileDetail)
			time, _ := time.Parse(time.RFC3339, fileDetail.LastModified)
			unsorted[i+1] = FileStorageJSON{fileDetail.LastModified, time, fileDetail.Size, fileDetail.DownloadURI}
		}(i)
	}
	wg.Wait()

	//get unsorted data and sort it
	sorted := make(TimeSlice, 0, len(unsorted))
	for _, d := range unsorted {
		sorted = append(sorted, d)
	}
	sort.Sort(sorted)
	printSorted(sorted, url, repo, download)
	return sorted
}

//PrintSorted print data in human readable format
func printSorted(sorted TimeSlice, url, repo, download string) {
	for key, value := range sorted {
		size, err := strconv.ParseInt(value.Size, 10, 64)
		if err != nil {
			fmt.Printf("%d is not of type %T", size, size)
			os.Exit(127)
		}
		fmt.Printf("%d\t%s\t%s\t%s\n", key+1, value.ConvertedTime.Format("2006-01-02 15:04:05"), ByteCountDecimal(size), strings.TrimPrefix(value.DownloadURI, url+"/"+repo+"/"+download+"/"))
	}
}

//GetRestAPI GET rest APIs response with error handling
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

//ByteCountDecimal convert bytes to human readable data size
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
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "kMGTPE"[exp])
}
