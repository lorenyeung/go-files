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
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/lorenyeung/go-files/helpers"

	log "github.com/sirupsen/logrus"
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
		//strip for URL safe characters
		reg, err := regexp.Compile("[^a-zA-Z0-9:/.%]+")
		if err != nil {
			log.Fatal(err)
		}
		urlInput, _ = reader.ReadString('\n')

		urlInput = strings.TrimSuffix(urlInput, "\n")
		urlInput = strings.TrimSuffix(urlInput, "/")
		if urlInput == "" {
			log.Debug("url input is empty")
			urlInput = creds.URL
		}
		urlInput = reg.ReplaceAllString(urlInput, "")
		log.Debug("stripping url to final string:", urlInput)

		fmt.Printf("Enter your username (leave blank for token) [%s]: ", creds.Username)
		userName, _ = reader.ReadString('\n')
		userName = strings.TrimSuffix(userName, "\n")
		if userName == "" {
			userName = creds.Username
		}
		fmt.Print("Enter your API key/token: ")
		apiKey, _ = reader.ReadString('\n')
		apiKey = strings.TrimSuffix(apiKey, "\n")

		log.Error("URL:", urlInput)
		if VerifyAPIKey(urlInput, userName, apiKey) {
			break
		} else {
			fmt.Print("Something seems wrong, please try again. Enter your url: ")
		}
	}
	log.Error("URL 2:", urlInput)
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
	log.Debug("before first write:", urlInput)
	data := Creds{
		URL:        Encrypt(urlInput, masterKey),
		Username:   Encrypt(userName, masterKey),
		Apikey:     Encrypt(apiKey, masterKey),
		DlLocation: Encrypt(dlLocationInput, masterKey),
		Repository: Encrypt(repoInput, masterKey),
	}
	//should probably encrypt data here
	fileData, err := json.MarshalIndent(data, "", "    ")
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

func overwritePropertiesJSON(cred Creds, filePath string) {
	fileData, err := json.MarshalIndent(cred, "", "    ")
	helpers.Check(err, true, "The JSON marshal", helpers.Trace())
	err2 := ioutil.WriteFile(filePath, fileData, 0600)
	helpers.Check(err2, true, "The JSON write", helpers.Trace())
}

//GetDownloadJSON get data from DownloadJSON
func GetDownloadJSON(fileLocation string, masterKey string) Creds {
	var result map[string]interface{}
	var resultData Creds
	toEncrypt := resultData
	file, err := os.Open(fileLocation)
	if err != nil {
		log.Print("error:", err)
		resultData = GenerateDownloadJSON(fileLocation, false, masterKey)
	} else {
		//should decrypt here
		defer file.Close()
		byteValue, _ := ioutil.ReadAll(file)
		json.Unmarshal([]byte(byteValue), &result)

		toEncrypt.Apikey = result["Apikey"].(string)
		toEncrypt.DlLocation = result["DlLocation"].(string)
		toEncrypt.Repository = result["Repository"].(string)
		toEncrypt.URL = result["URL"].(string)
		toEncrypt.Username = result["Username"].(string)

		var modified bool
		//TODO need to validate some of these fields
		decUrl, err := Decrypt(result["URL"].(string), masterKey, false)
		log.Debug(decUrl)
		if err != nil {
			modified = true
			log.Warn("re-encrypting url")
			toEncrypt.URL = Encrypt(result["URL"].(string), masterKey)
			resultData.URL = result["URL"].(string)
		} else {
			resultData.URL = decUrl
		}

		decUser, err := Decrypt(result["Username"].(string), masterKey, false)
		if err != nil {
			modified = true
			log.Warn("re-encrypting username")
			toEncrypt.Username = Encrypt(result["Username"].(string), masterKey)
			resultData.Username = result["Username"].(string)
		} else {
			resultData.Username = decUser
		}

		decApikey, err := Decrypt(result["Apikey"].(string), masterKey, false)
		if err != nil {
			modified = true
			log.Warn("re-encrypting Apikey")
			toEncrypt.Apikey = Encrypt(result["Apikey"].(string), masterKey)
			resultData.Apikey = result["Apikey"].(string)
		} else {
			resultData.Apikey = decApikey
		}

		decDllocation, err := Decrypt(result["DlLocation"].(string), masterKey, false)
		if err != nil {
			modified = true
			log.Warn("re-encrypting DlLocation")
			toEncrypt.DlLocation = Encrypt(result["DlLocation"].(string), masterKey)
			resultData.DlLocation = result["DlLocation"].(string)
		} else {
			resultData.DlLocation = decDllocation
		}

		decRepo, err := Decrypt(result["Repository"].(string), masterKey, false)
		if err != nil {
			modified = true
			log.Warn("re-encrypting Repository")
			toEncrypt.Repository = Encrypt(result["Repository"].(string), masterKey)
			resultData.Repository = result["Repository"].(string)
		} else {
			resultData.Repository = decRepo
		}

		if modified {
			log.Debug("writing new file to:", helpers.OrchestrateFlags.HomeVar+"download.json")
			overwritePropertiesJSON(toEncrypt, helpers.OrchestrateFlags.HomeVar+"download.json")
		}
		return resultData
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
func Encrypt(stringToEncrypt string, keyString string) string {
	//Since the key is in string, we need to convert decode it to bytes
	key, err := hex.DecodeString(keyString)
	if err != nil {
		log.Warn(err)
	}
	plaintext := []byte(stringToEncrypt)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	helpers.Check(err, true, "new Cipher", helpers.Trace())

	//Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	//https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	helpers.Check(err, true, "new GCM", helpers.Trace())

	//Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		helpers.Check(err, true, "new nonce", helpers.Trace())
	}

	//Encrypt the data using aesGCM.Seal
	//Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return fmt.Sprintf("%x", ciphertext)
}

//Decrypt self explanatory
func Decrypt(encryptedString string, keyString string, hideLog bool) (string, error) {
	key, err := hex.DecodeString(keyString)
	if err != nil {
		log.Warn(err)
	}
	enc, err := hex.DecodeString(encryptedString)
	if err != nil {
		if !hideLog {
			log.Warn(err, " string is not decode-able and likely is decrypted")
		}
		//try to reencrypt
		return "", err
	}

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	helpers.Check(err, true, "new Cipher", helpers.Trace())

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	helpers.Check(err, true, "new GCM", helpers.Trace())

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data. If the key is too short it will fail here with an ugly panic. If the field is empty it'll also panic.
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	//Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		if !hideLog {
			log.Warn(err, " string is not decryptable and likely is decrypted")
		}
		//try to reencrypt
		return "", err
	}

	return fmt.Sprintf("%s", plaintext), nil
}

//VerifyMasterKey self explanatory
func VerifyMasterKey(configPath string) string {
	_, err := os.Open(configPath)
	var token string
	if err != nil {
		log.Warn("Finding master key failed with error:", err)
		data, err := RandomHex(32)
		helpers.Check(err, true, "Generating new master key", helpers.Trace())
		err2 := ioutil.WriteFile(configPath, []byte(data), 0600)
		helpers.Check(err2, true, "Master key write", helpers.Trace())
		log.Info("Successfully generated master key")
		//token = base64.URLEncoding.EncodeToString([]byte(data))
		token = data
	} else {
		dat, err := ioutil.ReadFile(configPath)
		helpers.Check(err, true, "Reading master key", helpers.Trace())
		token = string(dat)
	}
	return token
}

func RandomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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
