package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	jsonparser "github.com/knadh/koanf/parsers/json"
	fileprovider "github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/mutantmonkey/golinx/progress"
)

type RespOkJSON struct {
	Filename   string
	Url        string
	Delete_Key string
	Expiry     string
	Size       string
	Sha256sum  string
	Direct_Url string `json:",omitempty"`
}

type RespErrJSON struct {
	Error string
}

var Config struct {
	siteurl string
	logfile string
	apikey  string
}

var keys map[string]string

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func main() {
	var del bool
	var randomize bool
	var overwrite bool
	var expiry int64
	var deleteKey string
	var accessKey string
	var desiredFileName string
	var configPath string
	var noClipboard bool
	var useSelifURL bool
	var cleanLog bool
	var listLog bool

	flag.BoolVar(&del, "d", false,
		"Delete file at url (ex: -d https://linx.example.com/myphoto.jpg")
	flag.BoolVar(&randomize, "r", false,
		"Randomize filename")
	flag.Int64Var(&expiry, "e", 0,
		"Time in seconds until file expires (ex: -e 600)")
	flag.StringVar(&deleteKey, "deletekey", "",
		"Specify your own delete key for the upload(s) (ex: -deletekey mysecret)")
	flag.StringVar(&accessKey, "accesskey", "",
		"Specify an access key to limit access to the file with a password")
	flag.StringVar(&desiredFileName, "f", "",
		"Specify the desired filename if different from the actual filename or if file from stdin")
	flag.StringVar(&configPath, "c", "",
		"Specify a non-default config path")
	flag.BoolVar(&overwrite, "o", false,
		"Overwrite file (assuming you have its delete key")
	flag.BoolVar(&noClipboard, "no-cb", false,
		"Disable automatic insertion into clipboard")
	flag.BoolVar(&useSelifURL, "selif", false,
		"Return selif url")
	flag.BoolVar(&cleanLog, "cleanup", false,
		"Remove dead entries from the logfile")
	flag.BoolVar(&listLog, "ls", false,
		"List entries stored in the logfile")
	flag.Parse()

	parseConfig(configPath)
	getKeys()

	if cleanLog {
		cleanLogfile()
		return
	}

	if listLog {
		listLogEntries()
		return
	}

	if del {
		for _, url := range flag.Args() {
			deleteUrl(url)
		}
	} else {
		for _, fileName := range flag.Args() {
			upload(fileName, deleteKey, accessKey, randomize, expiry, overwrite, desiredFileName, noClipboard, useSelifURL)
		}
	}
}

func upload(filePath string, deleteKey string, accessKey string, randomize bool, expiry int64, overwrite bool, desiredFileName string, noClipboard bool, useSelifURL bool) {
	var reader io.Reader
	var fileName string
	var ssum string

	if filePath == "-" {
		byt, err := ioutil.ReadAll(os.Stdin)
		checkErr(err)

		fileName = desiredFileName

		br := bytes.NewReader(byt)

		ssum = sha256sum(br)
		br.Seek(0, 0)

		reader = progress.NewProgressReader(fileName, br, int64(len(byt)))

	} else {
		fileInfo, err := os.Stat(filePath)
		checkErr(err)
		file, err := os.Open(filePath)
		checkErr(err)

		if desiredFileName == "" {
			fileName = path.Base(file.Name())
		} else {
			fileName = desiredFileName
		}

		br := bufio.NewReader(file)
		ssum = sha256sum(br)
		file.Seek(0, 0)

		reader = progress.NewProgressReader(fileName, br, fileInfo.Size())
	}

	escapedFileName := url.QueryEscape(fileName)

	req, err := http.NewRequest("PUT", Config.siteurl+"upload/"+escapedFileName, reader)
	checkErr(err)

	req.Header.Set("User-Agent", "linx-client")
	req.Header.Set("Accept", "application/json")

	if Config.apikey != "" {
		req.Header.Set("Linx-Api-Key", Config.apikey)
	}
	if deleteKey != "" {
		req.Header.Set("Linx-Delete-Key", deleteKey)
	}
	if accessKey != "" {
		req.Header.Set("Linx-Access-Key", accessKey)
	}
	if randomize {
		req.Header.Set("Linx-Randomize", "yes")
	}
	if expiry != 0 {
		req.Header.Set("Linx-Expiry", strconv.FormatInt(expiry, 10))
	}
	if overwrite {
		_, deleteKey, err := findDeleteKeyFor(fileName)
		checkErr(err)

		req.Header.Set("Linx-Delete-Key", deleteKey)
	}

	resp, err := httpClient.Do(req)
	checkErr(err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)

	if resp.StatusCode == 200 {
		var myResp RespOkJSON
		var returnUrl string

		err := json.Unmarshal(body, &myResp)
		checkErr(err)

		if myResp.Sha256sum != ssum {
			fmt.Println("Warning: sha256sum does not match.")
		}

		if useSelifURL && len(myResp.Direct_Url) != 0 {
			returnUrl = myResp.Direct_Url
		} else {
			returnUrl = myResp.Url
		}

		if noClipboard {
			fmt.Println(returnUrl)
		} else {
			fmt.Printf("Copied %s into clipboard!\n", returnUrl)
			clipboard.WriteAll(returnUrl)
		}

		addKey(myResp.Url, myResp.Delete_Key)

	} else if resp.StatusCode == 401 {

		checkErr(errors.New("Incorrect API key"))

	} else {
		var myResp RespErrJSON

		err := json.Unmarshal(body, &myResp)
		checkErr(err)

		fmt.Printf("Could not upload %s: %s\n", fileName, myResp.Error)
	}
}

func deleteUrl(url string) {
	deleteKey, exists := keys[url]
	if !exists {
		checkErr(errors.New("No delete key for " + url))
	}

	req, err := http.NewRequest("DELETE", url, nil)
	checkErr(err)

	req.Header.Set("User-Agent", "linx-client")
	req.Header.Set("Linx-Delete-Key", deleteKey)

	if Config.apikey != "" {
		req.Header.Set("Linx-Api-Key", Config.apikey)
	}

	resp, err := httpClient.Do(req)
	checkErr(err)
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("Deleted " + url)
		delete(keys, url)
		writeKeys()
	} else {
		checkErr(errors.New("Could not delete " + url))
	}

}

func addKey(url string, deleteKey string) {
	keys[url] = deleteKey
	writeKeys()
}

func getKeys() {
	keyFile, err := ioutil.ReadFile(Config.logfile)
	if os.IsNotExist(err) {
		keys = make(map[string]string)
		writeKeys()
		keyFile, err = ioutil.ReadFile(Config.logfile)
		checkErr(err)
	} else {
		checkErr(err)
	}

	err = json.Unmarshal(keyFile, &keys)
	checkErr(err)
}

func writeKeys() {
	if Config.logfile == "" {
		checkErr(errors.New("Logfile path not configured"))
	}

	if dir := filepath.Dir(Config.logfile); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			checkErr(err)
		}
	}

	byt, err := json.Marshal(keys)
	checkErr(err)

	err = ioutil.WriteFile(Config.logfile, byt, 0o600)
	checkErr(err)
}

func cleanLogfile() {
	fmt.Println("Checking logfile for dead entries...")

	foundDead := false
	for url := range keys {
		alive, err := isURLAlive(url)
		if err != nil {
			fmt.Printf("Skipping %s: %v\n", url, err)
			continue
		}

		if !alive {
			fmt.Println("Removing dead entry:", url)
			delete(keys, url)
			foundDead = true
		}
	}

	if foundDead {
		writeKeys()
		fmt.Println("Removed stale entries from", Config.logfile)
	} else {
		fmt.Println("No dead entries found.")
	}
}

func listLogEntries() {
	fmt.Println("Logfile entries:")

	if len(keys) == 0 {
		fmt.Println("  (log is empty)")
		return
	}

	urls := make([]string, 0, len(keys))
	for url := range keys {
		urls = append(urls, url)
	}
	sort.Strings(urls)

	for _, url := range urls {
		fmt.Printf("  %s -> %s\n", url, keys[url])
	}
}

type configFileData struct {
	Siteurl string `json:"siteurl"`
	Logfile string `json:"logfile"`
	Apikey  string `json:"apikey"`
}

func loadConfigFile(path string) (configFileData, error) {
	k := koanf.New(".")
	if err := k.Load(fileprovider.Provider(path), jsonparser.Parser()); err != nil {
		return configFileData{}, err
	}

	return configFileData{
		Siteurl: k.String("siteurl"),
		Logfile: k.String("logfile"),
		Apikey:  k.String("apikey"),
	}, nil
}

func writeConfigFile(path string, data configFileData) error {
	byt, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, byt, 0o600)
}

func isURLAlive(u string) (bool, error) {
	req, err := http.NewRequest("HEAD", u, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", "linx-client")
	if Config.apikey != "" {
		req.Header.Set("Linx-Api-Key", Config.apikey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return false, nil
	}

	return true, nil
}

func findDeleteKeyFor(identifier string) (string, string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", "", errors.New("No delete key for empty name")
	}

	if isHTTPURL(identifier) {
		if key, ok := keys[identifier]; ok {
			return identifier, key, nil
		}
		return "", "", errors.New("No delete key for " + identifier)
	}

	candidate := Config.siteurl + strings.TrimPrefix(identifier, "/")
	if key, ok := keys[candidate]; ok {
		return candidate, key, nil
	}

	base := path.Base(identifier)
	for url, key := range keys {
		if path.Base(url) == base {
			return url, key, nil
		}
	}

	return "", "", errors.New("No delete key for " + identifier)
}

func isHTTPURL(u string) bool {
	lower := strings.ToLower(u)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func expandUserPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	raw = os.ExpandEnv(raw)
	if strings.HasPrefix(raw, "~/") {
		homeDir := getHomeDir()
		raw = filepath.Join(homeDir, strings.TrimPrefix(raw, "~/"))
	}

	return raw
}

func ensureTrailingSlash(u string) string {
	if u == "" || strings.HasSuffix(u, "/") {
		return u
	}
	return u + "/"
}

func parseConfig(configPath string) {
	var cfgFilePath string

	if configPath == "" {
		cfgFilePath = filepath.Join(getConfigDir(), "linx-client.conf")
	} else {
		cfgFilePath = configPath
	}

	cfgFilePath = expandUserPath(cfgFilePath)

	if err := os.MkdirAll(filepath.Dir(cfgFilePath), 0o700); err != nil {
		checkErr(err)
	}

	stored := configFileData{}
	if _, err := os.Stat(cfgFilePath); err == nil {
		if fileData, err := loadConfigFile(cfgFilePath); err == nil {
			stored = fileData
		} else {
			fmt.Printf("Warning: could not read config at %s: %v\n", cfgFilePath, err)
		}
	} else if !os.IsNotExist(err) {
		checkErr(err)
	}

	Config.siteurl = ensureTrailingSlash(strings.TrimSpace(stored.Siteurl))
	Config.logfile = expandUserPath(stored.Logfile)
	Config.apikey = strings.TrimSpace(stored.Apikey)

	needsConfig := Config.siteurl == "" || Config.logfile == ""
	if !needsConfig {
		return
	}

	fmt.Println("Configuring linx-client")
	fmt.Println()

	for Config.siteurl == "" {
		Config.siteurl = getInput("Site url (ex: https://linx.example.com/)", false)
		Config.siteurl = ensureTrailingSlash(Config.siteurl)
	}

	for Config.logfile == "" {
		Config.logfile = expandUserPath(getInput("Logfile path (ex: ~/.linxlog)", false))
	}

	if Config.apikey == "" {
		Config.apikey = getInput("API key (leave blank if instance is public)", true)
	}

	stored = configFileData{
		Siteurl: Config.siteurl,
		Logfile: Config.logfile,
		Apikey:  Config.apikey,
	}
	checkErr(writeConfigFile(cfgFilePath, stored))

	fmt.Printf("Configuration written at %s\n", cfgFilePath)
}
