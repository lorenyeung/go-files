package rest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go-files/auth"
	"go-files/helpers"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

//GetFilesDetails get file details, sort by date and print
func GetFilesDetails(username, apiKey, url, repo, download string) helpers.TimeSlice {

	//create map of all file details from list of files
	var unsorted = make(map[int]helpers.FileStorageJSON)
	var filesList helpers.StorageJSON
	var data, _ = auth.GetRestAPI(url+"/api/storage/"+repo+"/"+download+"/", username, apiKey, "", "")
	json.Unmarshal([]byte(data), &filesList)
	for len(filesList.Children) == 0 {
		fmt.Println("No files found under " + url + "/" + repo + "/" + download + "/. Enter again, or type n to quit:")
		reader := bufio.NewReader(os.Stdin)
		downloadIn, _ := reader.ReadString('\n')
		download = strings.TrimSuffix(downloadIn, "\n")
		if download == "n" {
			os.Exit(0)
		}
		data, _ = auth.GetRestAPI(url+"/api/storage/"+repo+"/"+download+"/", username, apiKey, "", "")
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
			var data2, _ = auth.GetRestAPI(url+"/api/storage/"+repo+"/"+download+filesList.Children[i].URI, username, apiKey, "", "")
			json.Unmarshal([]byte(data2), &fileDetail)
			log.Debug("Debug before, url details:", fileDetail.DownloadURI, " :", url, " :data:", fileDetail, " download uri:", download+filesList.Children[i].URI)

			if strings.Contains(download+filesList.Children[i].URI, "%") {
				log.Warn("Encoding charactrer % detected in file URL, ", download+filesList.Children[i].URI, ", skipping")
				return
			}
			if !strings.Contains(fileDetail.DownloadURI, url) {
				log.Debug("Debug, url details:", fileDetail.DownloadURI, " :", url, " :data:", fileDetail)
				log.Warn("It looks like your URL context has been updated, as the file URL is different. Please reset your download.json")
				//os.Exit(1)
			}
			time, _ := time.Parse(time.RFC3339, fileDetail.LastModified)
			mutex.Lock()
			unsorted[i+1] = helpers.FileStorageJSON{
				LastModified:  fileDetail.LastModified,
				ConvertedTime: time,
				Size:          fileDetail.Size,
				Path:          fileDetail.Path,
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
func DownloadFilesList(sorted helpers.TimeSlice, creds auth.Creds, flags helpers.Flags, masterkey, readmeFileName string) {
	sortedSize := len(sorted)
	fmt.Println("Which files do you wish to download? Please separate each number by a space. Use a '-' for ranges, like: 1 3-6 11-12:")
	reader := bufio.NewReader(os.Stdin)
	downloadIn, _ := reader.ReadString('\n')
	download := strings.TrimSuffix(downloadIn, "\n")

	words := strings.Fields(download)
	//download all
	if strings.HasPrefix(download, "0 ") || download == "0" || strings.HasSuffix(download, " 0") || strings.Contains(download, " 0 ") {
		log.Info("zero detected, downloading everything")
		words = nil
		for i := 0; i < sortedSize; i++ {
			t := strconv.Itoa(i + 1)
			words = append(words, t)
		}
	} else if strings.Contains(download, "-") {
		//parse ranges
		words = nil
		numbers := strings.Split(download, " ")
		for i := 0; i < len(numbers); i++ {
			if strings.Contains(numbers[i], "-") {
				log.Info("found number with dash ", numbers[i])
				splitNumbers := strings.Split(numbers[i], "-")
				first, err := strconv.Atoi(splitNumbers[0])
				helpers.Check(err, true, "floor check", helpers.Trace())
				second, err := strconv.Atoi(splitNumbers[len(splitNumbers)-1])
				helpers.Check(err, true, "ceiling check", helpers.Trace())
				for j := first; j <= second; j++ {
					log.Debug("adding to download:", j)
					words = append(words, strconv.Itoa(j))
				}
			} else {
				words = append(words, numbers[i])
			}
		}
	}
	log.Debug("downloading the indexes (raw):", words)
	//remove duplicates from list
	check := make(map[string]int)
	for _, val := range words {
		check[val] = 1
	}
	words = nil
	for letter, _ := range check {
		words = append(words, letter)
	}
	log.Info("downloading the indexes (dedup):", words)

	//path := strings.TrimPrefix(sorted[0].DownloadURI, creds.URL+"/"+creds.Repository+"/")
	path := strings.TrimPrefix(sorted[0].Path, "/")
	log.Debug("Path trimmed:" + path)
	path = path[:strings.IndexByte(path, '/')]
	relativePath := creds.DlLocation + "/" + path + "/"
	var filesystemChecksums = make(map[string]string)
	if _, err := os.Stat(relativePath); os.IsNotExist(err) {
		log.Debug("%s does not exist, creating\n", relativePath)
		err2 := os.Mkdir(relativePath, 0700)
		helpers.Check(err2, true, "Creating log folder", helpers.Trace())

	} else {
		log.Info(relativePath, " exists, running checksum validation")
		f, err := os.Open(relativePath)
		helpers.Check(err, true, "Opening download directory", helpers.Trace())
		files, err := f.Readdir(-1)
		f.Close()
		helpers.Check(err, true, "Reading download directory files", helpers.Trace())

		for _, file := range files {
			if file.IsDir() {
				//I guess we could walk the entire tree if we wanted..
				log.Info(file.Name(), " is a directory. skipping\n")
				continue
			}
			//store list of checksums in memory then compare before download
			if flags.SkipDownloadedChecksumCheckVar == false {
				log.Debug("Checksum check not skipped for:", relativePath+file.Name())
				sha2 := helpers.ComputeSha256(relativePath + file.Name())
				filesystemChecksums[sha2] = relativePath + file.Name()
			}
		}
	}
	//create file
	readme := relativePath + "/" + readmeFileName
	log.Debug("Trying to create readme file under ", readme)
	DetectDetailsFile(readme, masterkey)

	log.Debug("size of index", words)
	for key := range words {
		//check if the index is an invalid option, skip if needed
		size := helpers.StringToInt64(words[key])
		if size < 1 || size > int64(sortedSize) {
			log.Warn("Out of bounds number ", words[key], ", skipping")
			continue
		}

		//fileName := strings.TrimPrefix(sorted[size-1].DownloadURI, creds.URL+"/"+creds.Repository+"/"+path+"/")
		fileName := strings.TrimPrefix(sorted[size-1].Path, "/"+path+"/")
		log.Debug("fileName trimmed:", fileName, " path:", path, " sorted[size-1].Path:", sorted[size-1].Path)
		//check shasum of download against in folder
		if filesystemChecksums[sorted[size-1].Checksums.Sha256] != "" {
			log.Info("file ", fileName, " exists, skipping download\n")
			continue
		}

		log.Info("downloading ", words[key], " ", sorted[size-1].DownloadURI)
		log.Debug("sorted:", sorted)
		// do some naive file type detection here
		readableFilesExtensions := []string{"txt", "pdf", "json", "yaml", "yml", "json", "xml"}
		var readableFile bool
		for i := range readableFilesExtensions {
			if strings.HasSuffix(fileName, readableFilesExtensions[i]) {
				log.Info("do not create folder, is readable without unarchiving:", fileName)
				readableFile = true
			}
		}
		oldRelativePath := relativePath
		if !readableFile {
			log.Info("creating folder due to archive:", relativePath+fileName+"-folder")
			err := os.Mkdir(relativePath+fileName+"-folder", 0755)
			helpers.Check(err, false, "Archive folder create", helpers.Trace())
			relativePath = relativePath + fileName + "-folder/"

		}
		_, filepath := auth.GetRestAPI(sorted[size-1].DownloadURI, creds.Username, creds.Apikey, relativePath+fileName, sorted[size-1].Checksums.Sha256)
		if !readableFile {
			log.Debug("creating symlink for file:", fileName)
			os.Symlink(relativePath+fileName, oldRelativePath+"."+fileName)
			//create symlink post download for checksum checker
		}
		log.Info("Successfully finished downloading ", sorted[size-1].DownloadURI)

		//try to unarchive if true
		if flags.UnzipVar {
			//file type detection
			buff := make([]byte, 512)
			file, err := os.Open(filepath)
			helpers.Check(err, true, "File testing failed at open:", helpers.Trace())

			_, err = file.Read(buff)
			helpers.Check(err, true, "File testing failed at read:", helpers.Trace())
			filetype := http.DetectContentType(buff)
			switch filetype {
			case "application/x-gzip", "application/zip":
				log.Info("File is compressed with gzip or zip, attempting to unzip")
				log.Debug("Unzipping ", filepath, " to ", filepath+"-folder")
				err := helpers.Unzip(filepath, filepath+"-folder")
				if err != nil {
					log.Error(err)
				}
			default:
				log.Info("File is not compressed")

			}
		}
		//reset relative path
		relativePath = oldRelativePath
	}
	//update readme last modified
	log.Info("Updating readme last modified to ", time.Now())
	readmeData := ReadDetailsFile(readme, masterkey)
	readmeData.LastModified = time.Now()
	readmeData.Title = auth.Encrypt(readmeData.Title, masterkey)
	readmeData.Description = auth.Encrypt(readmeData.Description, masterkey)
	fileData, err := json.Marshal(readmeData)
	helpers.Check(err, true, "The JSON marshal", helpers.Trace())

	err = ioutil.WriteFile(readme, fileData, 0644)
	helpers.Check(err, false, "Readme File update", helpers.Trace())

	log.Info("all requested files downloaded to " + relativePath + ". Safe travels!")
}

func DetectDetailsFile(readme, masterKey string) {
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		log.Info("Readme does not exist, creating")

		fmt.Println("Readme Title:")
		reader := bufio.NewReader(os.Stdin)
		readmeTitleIn, _ := reader.ReadString('\n')
		readmeTitle := strings.TrimSuffix(readmeTitleIn, "\n")

		fmt.Println("Readme Description:")
		reader = bufio.NewReader(os.Stdin)
		readmeDescIn, _ := reader.ReadString('\n')
		readmeDesc := strings.TrimSuffix(readmeDescIn, "\n")

		data := helpers.FolderDetailsJSON{
			Title:        auth.Encrypt(readmeTitle, masterKey),
			Description:  auth.Encrypt(readmeDesc, masterKey),
			LastModified: time.Now(),
		}
		fileData, err := json.Marshal(data)
		helpers.Check(err, true, "The JSON marshal", helpers.Trace())

		err = ioutil.WriteFile(readme, fileData, 0644)
		helpers.Check(err, false, "File create", helpers.Trace())

	} else {
		data := ReadDetailsFile(readme, masterKey)
		fmt.Println(data.Title, data.Description, data.LastModified)
	}
}

//ReadDetailsFile reads in hidden metadata file about folder
func ReadDetailsFile(readme, masterKey string) helpers.FolderDetailsJSON {
	log.Debug("Readme exists")
	var resultData helpers.FolderDetailsJSON
	file, err := os.Open(readme)
	helpers.Check(err, true, "Reading readme", helpers.Trace())
	byteValue, _ := ioutil.ReadAll(file)
	json.Unmarshal([]byte(byteValue), &resultData)
	//TODO need to validate some of these fields
	var data helpers.FolderDetailsJSON
	data.Title = auth.Decrypt(resultData.Title, masterKey)
	data.Description = auth.Decrypt(resultData.Description, masterKey)
	data.LastModified = resultData.LastModified
	//TODO need to account for file sha later
	return data
}

/*
*.tar.bz2)   tar xvjf $1    ;;
*.tar.gz)    tar xvzf $1    ;;
*.bz2)       bunzip2 $1     ;;
*.rar)       unrar x $1     ;;
*.gz)        gunzip $1      ;;
*.tar)       tar xvf $1     ;;
*.tbz2)      tar xvjf $1    ;;
*.tgz)       tar xvzf $1    ;;
*.zip)       unzip $1       ;;
*.Z)         uncompress $1  ;;
*.7z)        7z x $1        ;;
*)           echo "don't know how to extract '$1'..." ;;
 */
