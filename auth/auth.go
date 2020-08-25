package auth

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-files/helpers"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
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
	// username password validation
	data, _ := GetRestAPI(urlInput+"/api/system/ping", userName, apiKey, "", "")
	if string(data) == "OK" {
		log.Printf("finished VerifyAPIkey request. Credentials are good to go.")
		return true
	}
	log.Printf("finished VerifyAPIkey request: %s\n", string(data))
	return false
}

// GenerateDownloadJSON (re)generate download JSON. Tested.
func GenerateDownloadJSON(configPath string, regen bool, masterKey string) Creds {
	var creds Creds
	if regen {
		creds = GetDownloadJSON(configPath, masterKey)
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
		fmt.Printf("Enter your username (leave blank for token) [%s]: ", creds.Username)
		userName, _ = reader.ReadString('\n')
		userName = strings.TrimSuffix(userName, "\n")
		if userName == "" {
			userName = creds.Username
		}
		fmt.Print("Enter your API key/token: ")
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
	return writeFileDownloadJSON(configPath, urlInput, userName, apiKey, dlLocationInput, repoInput, masterKey)
}

func writeFileDownloadJSON(configPath, urlInput, userName, apiKey, dlLocationInput, repoInput, masterKey string) Creds {
	data := Creds{
		URL:        Encrypt(urlInput, masterKey),
		Username:   Encrypt(userName, masterKey),
		Apikey:     Encrypt(apiKey, masterKey),
		DlLocation: Encrypt(dlLocationInput, masterKey),
		Repository: Encrypt(repoInput, masterKey),
	}
	//should probably encrypt data here
	fileData, err := json.Marshal(data)
	helpers.Check(err, true, "The JSON marshal", helpers.Trace())
	err2 := ioutil.WriteFile(configPath, fileData, 0600)
	helpers.Check(err2, true, "The JSON write", helpers.Trace())

	data2 := Creds{
		URL:        urlInput,
		Username:   userName,
		Apikey:     apiKey,
		DlLocation: dlLocationInput,
		Repository: repoInput,
	}

	return data2
}

//GetDownloadJSON get data from DownloadJSON
func GetDownloadJSON(fileLocation string, masterKey string) Creds {
	var result map[string]interface{}
	var resultData Creds
	file, err := os.Open(fileLocation)
	if err != nil {
		log.Print("error:", err)
		resultData = GenerateDownloadJSON(fileLocation, false, masterKey)
	} else {
		//should decrypt here
		defer file.Close()
		byteValue, _ := ioutil.ReadAll(file)
		json.Unmarshal([]byte(byteValue), &result)
		//TODO need to validate some of these fields
		resultData.URL = Decrypt(result["URL"].(string), masterKey)
		resultData.Username = Decrypt(result["Username"].(string), masterKey)
		resultData.Apikey = Decrypt(result["Apikey"].(string), masterKey)
		resultData.DlLocation = Decrypt(result["DlLocation"].(string), masterKey)
		resultData.Repository = Decrypt(result["Repository"].(string), masterKey)
	}
	return resultData
}

//GetRestAPI GET rest APIs response with error handling
func GetRestAPI(urlInput, userName, apiKey, filepath, backupSha256 string) ([]byte, string) {
	log.Debug("URL:", urlInput, " to directory:", filepath, " with cred:", userName)
	client := http.Client{}
	req, err := http.NewRequest("GET", urlInput, nil)
	if userName != "" {
		log.Debug("Using Basic Auth")
		req.SetBasicAuth(userName, apiKey)
	} else {
		log.Debug("Using Token Auth")
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if err != nil {
		log.Warn("The HTTP request failed with error %s\n", err)
	} else {

		resp, err := client.Do(req)
		helpers.Check(err, false, "The HTTP response", helpers.Trace())
		//defer resp.Body.Close()

		if filepath != "" {
			//download percent logger
			log.Debug("logging Header map:", resp.Header)
			log.Debug("logging content-disposition:", resp.Header["Content-Disposition"])
			log.Debug("logging checksum:", resp.Header["X-Checksum-Sha256"])
			var sourceSha256 string
			if len(resp.Header["X-Checksum-Sha256"]) > 0 {
				sourceSha256 = string(resp.Header["X-Checksum-Sha256"][0])
			} else {
				//redirect doesn't return checksum headers
				log.Warn("Unable to retrieve SHA256 checksum header, trying backup SHA256")
				if backupSha256 != "" {
					sourceSha256 = backupSha256
				} else {
					log.Warn("Backup SHA256 is empty")
					sourceSha256 = ""
				}
			}
			// Create the file
			out, err := os.Create(filepath)
			helpers.Check(err, false, "File create", helpers.Trace())
			defer out.Close()

			done := make(chan int64)
			go helpers.PrintDownloadPercent(done, filepath, int64(resp.ContentLength))
			_, err = io.Copy(out, resp.Body)
			helpers.Check(err, true, "The file copy", helpers.Trace())
			log.Info("\nChecking downloaded Shasum's match")
			fileSha256 := helpers.ComputeSha256(filepath)
			if sourceSha256 != fileSha256 {
				log.Warn("Shasums do not match. Source:", sourceSha256, " filesystem:", fileSha256)
			}
			log.Debug("Shasums match.")

		} else {
			data, _ := ioutil.ReadAll(resp.Body)
			return data, ""
		}
	}
	return nil, filepath
}

//CreateHash self explanatory
func CreateHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

//Encrypt self explanatory
func Encrypt(dataString string, passphrase string) string {
	data := []byte(dataString)
	block, _ := aes.NewCipher([]byte(CreateHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	helpers.Check(err, true, "Cipher", helpers.Trace())
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.RawURLEncoding.EncodeToString([]byte(ciphertext))
}

//Decrypt self explanatory
func Decrypt(dataString string, passphrase string) string {
	data, _ := base64.RawURLEncoding.DecodeString(dataString)

	key := []byte(CreateHash(passphrase))
	block, err := aes.NewCipher(key)
	helpers.Check(err, true, "Cipher", helpers.Trace())
	gcm, err := cipher.NewGCM(block)
	helpers.Check(err, true, "Cipher GCM", helpers.Trace())
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	helpers.Check(err, true, "GCM open", helpers.Trace())
	return string(plaintext)
}

//VerifyMasterKey self explanatory
func VerifyMasterKey(configPath string) string {
	_, err := os.Open(configPath)
	var token string
	if err != nil {
		log.Warn("Finding master key failed with error %s\n", err)
		data, err := generateRandomBytes(32)
		helpers.Check(err, true, "Generating new master key", helpers.Trace())
		err2 := ioutil.WriteFile(configPath, []byte(base64.URLEncoding.EncodeToString(data)), 0600)
		helpers.Check(err2, true, "Master key write", helpers.Trace())
		log.Info("Successfully generated master key")
		token = base64.URLEncoding.EncodeToString(data)
	} else {
		dat, err := ioutil.ReadFile(configPath)
		helpers.Check(err, true, "Reading master key", helpers.Trace())
		token = string(dat)
	}
	return token
}

func generateRandomString(s int) (string, error) {
	b, err := generateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}
	return b, nil
}
