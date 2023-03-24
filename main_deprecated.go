//go:build deprecated
// +build deprecated

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type schoolInfo struct {
	Data map[string]schoolInfoData `json:"data"`
}

type schoolInfoData struct {
	Name string `json:"school_name"`
}

// specific schema

type specificInfo struct {
	Data *specificInfoData `json:"data"`
}

type specificInfoData struct {
	Num  int             `json:"numFound"`
	Item []*specificItem `json:"item"`
}

type specificItem struct {
	Name       string `json:"spname"`
	SpeType    string `json:"zslx_name"`
	Batch      string `json:"local_batch_name"`
	Min        string `json:"min"`
	MinSection string `json:"min_section"`
	Max        string `json:"max"`
	Avg        string `json:"average"`
}

// specific result schema

type spResult struct {
	School   schoolInfoData
	Year     int
	Specific *specificInfo
}

// year, school id, province id, type, batch, index
const specificURLPattern = "https://static-data.gaokao.cn/www/2.0/schoolplanindex/%v/%v/45/1/7/%v.json"

//const specificURLPattern = "https://static-data.gaokao.cn/www/2.0/schoolspecialindex/%v/%v/45/1/7/%v.json"

var (
	concurrency = flag.Int("f", 20, "concurrency")
)

func main() {
	flag.Parse()
	// get school list first
	url := "https://static-gkcx.gaokao.cn/www/2.0/json/live/v2/schoolnum.json"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("get school list failed, err is :%v", err)
	}
	var schools schoolInfo
	info, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(info, &schools)
	if err != nil {
		log.Fatalf("unmarshal school info failed, err is :%v", err)
	}

	// get special info consumer, generate every school's all specific list for all years
	// send school id only
	urlSendCh := make(chan int, *concurrency)
	// res coll ch
	resCh := make(chan spResult, 20)

	wg := sync.WaitGroup{}
	wg.Add(*concurrency)
	for i := 0; i < *concurrency; i++ {
		go func() {
			defer wg.Done()
			for schoolID := range urlSendCh {
				// get 1st page
			INNER:
				for year := 2017; year <= 2022; year++ {
					sURL := fmt.Sprintf(specificURLPattern, year, schoolID, 1)
					specInfo, err := getSchoolSpecInfo(sURL)
					if err != nil || specInfo == nil {
						//log.Printf("get spec info failed, url: %v, err: %v", sURL, err)
						if err != nil {
							log.Printf("err get spec info, school id: %v, year: %v, err: %v", schoolID, year, err)
						}
						continue INNER
					}
					// add to result
					res := spResult{
						School:   schools.Data[strconv.Itoa(schoolID)],
						Year:     year,
						Specific: &specificInfo{Data: &specificInfoData{Item: make([]*specificItem, 0)}},
					}
					// use second way to get school name
					if res.School.Name == "" {
						sn, err := getSchoolInfo(schoolID)
						if err != nil {
							log.Printf("get school name failed, err : %v, id: %v", err, schoolID)
						}
						res.School.Name = sn
					}
					if res.School.Name == "" {
						res.School.Name = fmt.Sprintf("%v", schoolID)
					}
					res.Year = year
					res.Specific.Data.Item = append(res.Specific.Data.Item, specInfo.Data.Item...)
					if pages := int(math.Ceil(float64(specInfo.Data.Num) / 10)); pages > 1 {
						for page := 2; page <= pages; page++ {
							sURL := fmt.Sprintf(specificURLPattern, year, schoolID, page)
							specInfo, err := getSchoolSpecInfo(sURL)
							if err != nil {
								log.Printf("get spec info failed, url: %v, err: %v", sURL, err)
								continue INNER
							}
							res.Specific.Data.Item = append(res.Specific.Data.Item, specInfo.Data.Item...)
						}
					}
					// send year data to res ch
					resCh <- res
				}
			}
		}()
	}

	// result ch
	resWg := sync.WaitGroup{}
	resWg.Add(1)
	go func() {
		defer resWg.Done()
		f, _ := os.OpenFile("./result.json", os.O_CREATE|os.O_RDWR, 0666)
		w := bufio.NewWriter(f)
		defer w.Flush()
		for res := range resCh {
			byt, err := json.Marshal(res)
			if err != nil {
				log.Fatal(err)
			}
			_, err = w.Write(byt)
			if err != nil {
				log.Fatal(err)
			}
			w.Write([]byte("\n"))
		}
	}()
	// producer ch
	log.Printf("total count: %v", 10000)
	i := 0
	for k := 0; k <= 5000; k++ {
		urlSendCh <- k
		i++
		log.Printf("%v schools produced", i)
	}
	close(urlSendCh)
	wg.Wait()
	close(resCh)
	resWg.Wait()
}

func getSchoolSpecInfo(url string) (*specificInfo, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	var specific specificInfo
	info, _ := ioutil.ReadAll(resp.Body)
	if len(info) < 10 {
		return nil, nil
	}
	err = json.Unmarshal(info, &specific)
	if err != nil {
		return nil, err
	}
	return &specific, nil
}

func getSchoolInfo(id int) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://static-data.gaokao.cn/www/2.0/school/%v/info.json", id))
	if err != nil {
		return "", err
	}
	var specific struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	info, _ := ioutil.ReadAll(resp.Body)
	if len(info) < 10 {
		return "", nil
	}
	err = json.Unmarshal(info, &specific)
	if err != nil {
		return "", err
	}
	return specific.Data.Name, nil
}
