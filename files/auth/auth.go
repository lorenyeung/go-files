package auth

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"files/helpers"
	"fmt"
	"io"
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
	log.Printf("starting VerifyAPIkey request. Testing: %s\n", userName)
	data := GetRestAPI(urlInput+"/api/system/ping", userName, apiKey, "")
	if string(data) == "OK" {
		log.Print("finished VerifyAPIkey request. Credentials are good to go.")
		return true
	}
	log.Printf("finished VerifyAPIkey request: %s\n", string(data))
	return false
}

// GenerateDownloadJSON (re)generate download JSON. Tested.
func GenerateDownloadJSON(configPath string, regen bool) Creds {
	var creds Creds
	//TODO optionally read in existing files in case of updated creds/url
	if regen {
		creds = GetDownloadJSON(configPath)
	}
	var urlInput, userName, apiKey string
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter your url [%s]: ", creds.URL)
	for {
		urlInput, _ = reader.ReadString('\n')
		urlInput = strings.TrimSuffix(urlInput, "\n")
		if urlInput == "" {
			urlInput = creds.URL
		}
		fmt.Printf("Enter your username [%s]: ", creds.Username)
		userName, _ = reader.ReadString('\n')
		userName = strings.TrimSuffix(userName, "\n")
		if userName == "" {
			userName = creds.Username
		}
		fmt.Print("Enter your API key: ")
		apiKey, _ = reader.ReadString('\n')
		apiKey = strings.TrimSuffix(apiKey, "\n")
		if VerifyAPIKey(urlInput, userName, apiKey) {
			break
		} else {
			fmt.Print("Something seems wrong, please try again. Enter your url: ")
		}
	}
	//TODO need to check if directory exists and/or valid directory. trim trailing /
	fmt.Printf("Enter your Download location [%s]: ", creds.DlLocation)
	dlLocationInput, _ := reader.ReadString('\n')
	dlLocationInput = strings.TrimSuffix(dlLocationInput, "\n")
	if dlLocationInput == "" {
		dlLocationInput = creds.DlLocation
	}

	//TODO need to check if repo exists. trim trailing /
	fmt.Printf("Enter your repository [%s]: ", creds.Repository)
	repoInput, _ := reader.ReadString('\n')
	repoInput = strings.TrimSuffix(repoInput, "\n")
	if repoInput == "" {
		repoInput = creds.Repository
	}
	return writeFileDownloadJSON(configPath, urlInput, userName, apiKey, dlLocationInput, repoInput)
}

func writeFileDownloadJSON(configPath, urlInput, userName, apiKey, dlLocationInput, repoInput string) Creds {
	data := Creds{
		URL:        urlInput,
		Username:   userName,
		Apikey:     apiKey,
		DlLocation: dlLocationInput,
		Repository: repoInput,
	}
	//should probably encrypt data here
	fileData, err := json.Marshal(data)
	if err != nil {
		log.Panicf("The JSON form failed with error %s\n", err)

	}
	err2 := ioutil.WriteFile(configPath, fileData, 0600)
	if err2 != nil {
		log.Panicf("The JSON write failed with error %s\n", err2)
	}
	return data
}

//GetDownloadJSON get data from DownloadJSON
func GetDownloadJSON(fileLocation string) Creds {
	var result map[string]interface{}
	var resultData Creds
	file, err := os.Open(fileLocation)
	if err != nil {
		log.Print("error:", err)
		resultData = GenerateDownloadJSON(fileLocation, false)
	} else {
		//should decrypt here
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
func GetRestAPI(urlInput, userName, apiKey, filepath string) []byte {
	client := http.Client{}
	req, err := http.NewRequest("GET", urlInput, nil)
	req.SetBasicAuth(userName, apiKey)
	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
	} else {

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("The HTTP response failed with error %s\n", err)
		}
		//defer resp.Body.Close()

		if filepath != "" {
			//download percent logger
			sourceSha256 := string(resp.Header["X-Checksum-Sha256"][0])
			fmt.Println(resp.Header["Content-Disposition"][0])
			// Create the file
			out, err := os.Create(filepath)
			if err != nil {
				fmt.Printf("File create failed with %s\n", err)
			}
			defer out.Close()

			done := make(chan int64)
			go helpers.PrintDownloadPercent(done, filepath, int64(resp.ContentLength))
			_, err = io.Copy(out, resp.Body)
			if err != nil {
				log.Printf("File copy failed with %s\n", err)
			}
			log.Println("Checking downloaded Shasum's match")
			fileSha256 := helpers.ComputeSha256(filepath)
			if sourceSha256 != fileSha256 {
				fmt.Printf("Shasums do not match. Source: %s filesystem %s\n", sourceSha256, fileSha256)
			}
			log.Println("Shasums match.")

		} else {
			data, _ := ioutil.ReadAll(resp.Body)
			return data
		}
	}
	return nil
}

func CreateHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

func Encrypt(data []byte, passphrase string) []byte {
	block, _ := aes.NewCipher([]byte(CreateHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext
}

func Decrypt(data []byte, passphrase string) []byte {
	key := []byte(CreateHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}
	return plaintext
}
