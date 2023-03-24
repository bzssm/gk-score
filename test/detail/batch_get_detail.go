package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// year, school id, province id, type, batch, index
const urlFormat = "https://static-data.gaokao.cn/www/2.0/schoolspecialindex/%v/%v/%v/%v/%v/%v.json"

func main() {
	idCh := make(chan string, 300)
	wg := &sync.WaitGroup{}
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go worker(idCh, wg)
	}
	// producer
	os.Mkdir("school_detail", 0777)
	content, _ := os.ReadFile("school_batch_type_comb.txt")
	ids := strings.Split(string(content), "\n")
	for index, id := range ids {
		idCh <- id
		if index%100 == 0 {
			fmt.Printf("%v/%v have been processed\n", index, len(ids))
		}
	}
	close(idCh)
	wg.Wait()
}

func worker(idCh chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for id := range idCh {
		fields := strings.Split(id, ",")
		url := fmt.Sprintf(urlFormat, fields[0], fields[1], fields[2], fields[3], fields[4], 1)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("url: %v, download error: %v\n", id, err)
			continue
		}
		if resp.StatusCode != 200 {
			fmt.Printf("id: %v, status error: %v\n", id, resp.StatusCode)
			continue
		}
		fileContent := &bytes.Buffer{}
		content, _ := io.ReadAll(resp.Body)
		if err := json.Indent(fileContent, content, "", "  "); err != nil {
			fmt.Println(err)
		}
		os.WriteFile(fmt.Sprintf("school_detail/%v.json", id), fileContent.Bytes(), 0777)
	}
}
