package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

type schoolList struct {
	Data []struct {
		ID   string `json:"school_id"`
		Name string `json:"name"`
	} `json:"data"`
}

var schoolIdNameMap = make(map[string]string)

func main() {
	idCh := make(chan string, 300)
	wg := &sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go worker(idCh, wg)
	}
	// producer
	var sl schoolList
	content, err := os.ReadFile("./school_list_all.json")
	if err != nil {
		fmt.Printf("read file failed: %v", err)
	}
	if err := json.Unmarshal(content, &sl); err != nil {
		fmt.Printf("unmarshal failed: %v", err)
	}
	// get id -> name map
	for _, school := range sl.Data {
		schoolIdNameMap[school.ID] = school.Name
	}
	os.Mkdir("school_province_score", 0777)
	for index, school := range sl.Data {
		idCh <- school.ID
		if index%100 == 0 {
			fmt.Printf("%v/%v have been processed\n", index, len(sl.Data))
		}
	}
	close(idCh)
	wg.Wait()
}

func worker(idCh chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for id := range idCh {
		resp, err := http.Get(fmt.Sprintf("https://static-data.gaokao.cn/www/2.0/school/%v/dic/provincescore.json", id))
		if err != nil {
			fmt.Printf("url: %v, download error: %v\n", id, err)
			continue
		}
		if resp.StatusCode != 200 {
			fmt.Printf("id: %v, name: %v, status error: %v\n", id, schoolIdNameMap[id], resp.StatusCode)
			continue
		}
		fileContent := &bytes.Buffer{}
		content, _ := io.ReadAll(resp.Body)
		if err := json.Indent(fileContent, content, "", "  "); err != nil {
			fmt.Println(err)
		}
		os.WriteFile(fmt.Sprintf("school_province_score/%v_%v.json", id, schoolIdNameMap[id]), fileContent.Bytes(), 0777)
	}
}
