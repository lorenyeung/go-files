package rest

import (
	"bufio"
	"encoding/json"
	"files/auth"
	"files/helpers"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//GetFilesDetails get file details, sort by date and print
func GetFilesDetails(username, apiKey, url, repo, download string) helpers.TimeSlice {

	//create map of all file details from list of files
	var unsorted = make(map[int]helpers.FileStorageJSON)
	var filesList helpers.StorageJSON
	var data = auth.GetRestAPI(url+"/api/storage/"+repo+"/"+download+"/", username, apiKey, "")
	json.Unmarshal([]byte(data), &filesList)
	for len(filesList.Children) == 0 {
		fmt.Println("No files found under " + url + "/" + repo + "/" + download + "/. Enter again, or type n to quit:")
		reader := bufio.NewReader(os.Stdin)
		downloadIn, _ := reader.ReadString('\n')
		download = strings.TrimSuffix(downloadIn, "\n")
		if download == "n" {
			os.Exit(0)
		}
		data = auth.GetRestAPI(url+"/api/storage/"+repo+"/"+download+"/", username, apiKey, "")
		json.Unmarshal([]byte(data), &filesList)
	}
	fmt.Println("Found the following files under " + url + "/" + repo + "/" + download + "/\nNumber\tLast Modified\t\tSize\tPath")
	var mutex = &sync.Mutex{} //should help with the concurrent map writes issue
	var wg sync.WaitGroup     //multi threading the GET details request
	wg.Add(len(filesList.Children))
	for i := 0; i < len(filesList.Children); i++ {
		go func(i int) {
			defer wg.Done()
			var fileDetail helpers.FileStorageJSON
			var data2 = auth.GetRestAPI(url+"/api/storage/"+repo+"/"+download+filesList.Children[i].URI, username, apiKey, "")
			json.Unmarshal([]byte(data2), &fileDetail)
			time, _ := time.Parse(time.RFC3339, fileDetail.LastModified)
			mutex.Lock()
			unsorted[i+1] = helpers.FileStorageJSON{
				LastModified:  fileDetail.LastModified,
				ConvertedTime: time,
				Size:          fileDetail.Size,
				DownloadURI:   fileDetail.DownloadURI,
				Checksums:     fileDetail.Checksums,
			}
			mutex.Unlock()
		}(i)
	}
	wg.Wait()

	//get unsorted data and sort it
	sorted := make(helpers.TimeSlice, 0, len(unsorted))
	for _, d := range unsorted {
		sorted = append(sorted, d)
	}
	sort.Sort(sorted)
	helpers.PrintSorted(sorted, url, repo, download)
	return sorted
}

//DownloadFilesList download files selected
func DownloadFilesList(sorted helpers.TimeSlice, creds auth.Creds) {
	sortedSize := len(sorted)
	fmt.Println("Which files do you wish to download? Please separate each number by a space:")
	reader := bufio.NewReader(os.Stdin)
	downloadIn, _ := reader.ReadString('\n')
	download := strings.TrimSuffix(downloadIn, "\n")

	//TODO need number check
	words := strings.Fields(download)
	if strings.HasPrefix(download, "0 ") || download == "0" || strings.HasSuffix(download, " 0") || strings.Contains(download, " 0 ") {
		words = nil
		for i := 0; i < sortedSize; i++ {
			t := strconv.Itoa(i + 1)
			words = append(words, t)
		}
	}
	path := strings.TrimPrefix(sorted[0].DownloadURI, creds.URL+"/"+creds.Repository+"/")
	path = path[:strings.IndexByte(path, '/')]
	relativePath := creds.DlLocation + "/" + path + "/"
	var filesystemChecksums = make(map[string]string)
	if _, err := os.Stat(relativePath); os.IsNotExist(err) {
		log.Printf("%s does not exist, creating\n", relativePath)
		//TODO create folder
		_ = os.Mkdir(relativePath, 0700)

	} else {
		log.Printf("%s exists, running checksum validation\n", relativePath)
		f, err := os.Open(relativePath)
		if err != nil {
			log.Fatal(err)
		}
		files, err := f.Readdir(-1)
		f.Close()
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if file.IsDir() {
				//I guess we could walk the entire tree if we wanted..
				fmt.Printf("%s is a directory. skipping\n", file.Name())
				continue
			}
			//store list of checksums in memory then compare before download. maybe this would be better as a hashmap?
			sha2 := helpers.ComputeSha256(relativePath + file.Name())
			filesystemChecksums[sha2] = relativePath + file.Name()
		}
	}

	for key := range words {
		//check if the index is an invalid option, skip if needed
		size := helpers.StringToInt64(words[key])
		if size > int64(sortedSize) || size < 1 {
			log.Printf("Out of bounds number %d, skipping", size)
			continue
		}

		fileName := strings.TrimPrefix(sorted[size-1].DownloadURI, creds.URL+"/"+creds.Repository+"/"+path+"/")
		//check shasum of dowload against in folder
		if filesystemChecksums[sorted[size-1].Checksums.Sha256] != "" {
			log.Printf("file %s exists, skipping download\n", fileName)
			continue
		}

		log.Printf("downloading %s %s\n", words[key], sorted[size-1].DownloadURI)
		auth.GetRestAPI(sorted[size-1].DownloadURI, creds.Username, creds.Apikey, relativePath+fileName)
		log.Printf("Successfully finished downloading %s\n", sorted[size-1].DownloadURI)
	}
}
