package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

func main() {
	idCh := make(chan string, 300)
	wg := &sync.WaitGroup{}
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go stockWorker(idCh, wg)
	}
	// producer
	ptbFile, err := os.Open("./test/stock_spider/raw_fund_list.csv")
	if err != nil {
		panic("open ptb list failed")
	}
	defer ptbFile.Close()
	scanner := bufio.NewScanner(ptbFile)
	fundList := make([]string, 0)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")
		fundList = append(fundList, fields[0])
	}
	os.Mkdir("stock", 0777)
	for index, id := range fundList {
		idCh <- id
		if index%100 == 0 {
			fmt.Printf("%v/%v have been processed\n", index, len(fundList))
		}
	}
	close(idCh)
	wg.Wait()
}

func stockWorker(idCh chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for id := range idCh {
		resp, err := http.Get(fmt.Sprintf("https://fund.eastmoney.com/pingzhongdata/%v.js?", id))
		if err != nil {
			fmt.Printf("url: %v, download error: %v\n", id, err)
			continue
		}
		if resp.StatusCode != 200 {
			fmt.Printf("id: %v, status error: %v\n", id, resp.StatusCode)
			continue
		}
		content, _ := io.ReadAll(resp.Body)
		os.WriteFile(fmt.Sprintf("stock/%v.json", id), content, 0777)
	}
}
