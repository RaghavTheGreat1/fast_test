package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"sync"
	"time"
)

var count = 0
var lastCount = 0
var mu sync.Mutex

func main() {

	// Get https://fast.com html response
	res, err := http.Get("https://fast.com")
	if err != nil {
		log.Fatal("Unable to access fast.com", err)
	}

	defer res.Body.Close()

	// Parse the html
	body, err := io.ReadAll(res.Body)

	if err != nil {
		log.Fatal("Unable to read from fast.com", err)
	}

	// RegExp for extracting the path to the script.js file, usually found in <script> HTML tag
	rForScriptFileId := regexp.MustCompile(`src\=\"\/app-(.*?)\.js\"`)

	// Find all matches
	scriptFileIdMatch := rForScriptFileId.FindStringSubmatch(string(body))
	if len(scriptFileIdMatch) == 0 {
		log.Fatal("Unable to find js file. May be renamed from app-*.js pattern.")
	}

	scriptFileId := scriptFileIdMatch[1]

	// For getting the JS file
	res, err = http.Get("https://fast.com/app-" + scriptFileId + ".js")
	if err != nil {
		log.Fatal("Unable to access fast.com", err)
	}

	h, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal("Unable to read from fast.com", err)
	}

	// To extract the token that is present in script.js file
	r := regexp.MustCompile(`token\:\"(.*?)\"`)
	tokenMatch := r.FindStringSubmatch(string(h))
	if len(tokenMatch) == 0 {
		log.Fatal("Unable to find token in js file. May be renamed.")
	}
	token := tokenMatch[1]

	// Hitting the https://fast.com api to get the download URLs
	res, err = http.Get("https://api.fast.com/netflix/speedtest?https=true&token=" + token)
	if err != nil {
		log.Fatal("Unable to access api.fast.com", err)
	}

	j, err := io.ReadAll(res.Body)
	fmt.Println(string(j))
	if err != nil {
		log.Fatal("Unable to read from api.fast.com", err)
	}

	// URLs to download files
	var urls []map[string]string
	json.Unmarshal(j, &urls)

	ticker := time.NewTicker(500 * time.Millisecond)
	done := make(chan bool)

	// Starting new GoRouting
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				diff := count - lastCount
				lastCount = count
				mu.Unlock()

				log.Println("Speed", prettyByteSize(diff*2*8)+"bps")
			}
		}
	}()

	//Start timer
	start := time.Now()
	var wg sync.WaitGroup
	for _, v := range urls {
		wg.Add(1)

		url := v["url"]

		// Run concurrently
		go func() {
			defer wg.Done()
			downloadUrl(url)
		}()
	}
	wg.Wait()
	end := time.Since(start)

	log.Println("Downloaded data size:", prettyByteSize(count)+"B")
	log.Println("Average speed: ", prettyByteSize(int(float64(count*8)/end.Seconds()))+"bps")
	ticker.Stop()

	done <- true
}

func downloadUrl(url string) {
	log.Println("Downloading", url)

	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, 1024*1024*10)
	for {
		n, err := res.Body.Read(buf)
		mu.Lock()
		count = count + n
		mu.Unlock()

		if err != nil {
			break
		}
	}
}

// ref: https://gist.github.com/anikitenko/b41206a49727b83a530142c76b1cb82d?permalink_comment_id=4467913#gistcomment-4467913
func prettyByteSize(b int) string {
	bf := float64(b)
	for _, unit := range []string{"", "K", "M", "G", "T", "P", "E", "Z"} {
		if math.Abs(bf) < 1024.0 {
			return fmt.Sprintf("%3.1f %s", bf, unit)
		}
		bf /= 1024.0
	}
	return fmt.Sprintf("%.1f Y", bf)
}
