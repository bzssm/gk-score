package main

import (
	"bitbucket.org/ai69/popua"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bzssm/goclub/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	log        *zap.SugaredLogger
	debug      = true
	debugCount = 20
	parallel   = 200
	chanBuffer = 500

	schools         []schoolData
	schoolIDNameMap = make(map[string]string, 0)

	schoolInfoFailed = &atomic.Int64{}
	schoolPTBFailed  = &atomic.Int64{}
)

// ptb stands for provice id, type id, batch id
const (
	schoolListURL  = "https://static-data.gaokao.cn/www/2.0/school/name.json"
	schoolListFile = "school_list.json"

	schoolInfoURLFormat = "https://static-data.gaokao.cn/www/2.0/school/%v/info.json"
	schoolInfoDir       = "school_info"
	schoolInfoRawDir    = "RAW_school_info"

	schoolPTBURLFormat = "https://static-data.gaokao.cn/www/2.0/school/%v/dic/provincescore.json"
	schoolPTBRawDir    = "school_ptb"
	schoolPTBFile      = "ptb.txt"

	// year, school id, province id, type, batch, index
	specialDetailURLFormat = "https://static-data.gaokao.cn/www/2.0/schoolspecialindex/%v/%v/%v/%v/%v/%v.json"
)

func main() {
	lgr, _, _ := logger.InitLogger(zapcore.InfoLevel, true, "")
	log = lgr
	// stat
	defer func() {
		log.Infof("school info failed : %v", schoolInfoFailed.Load())
		log.Infof("school ptb failed  : %v", schoolPTBFailed.Load())
	}()
	// 1. get school list from: https://static-data.gaokao.cn/www/2.0/school/name.json
	content, err := request(schoolListURL, true)
	if err != nil {
		os.Exit(1)
	}
	// 1.1 write raw content to file
	if err := os.WriteFile("RAW_"+schoolListFile, content, 0666); err != nil {
		log.Fatalw("write school list raw failed", zap.Error(err))
	}
	// 1.2 load school info
	var schoolJSON school
	if err := json.Unmarshal(content, &schoolJSON); err != nil {
		log.Fatalw("unmarshal school list failed", zap.Error(err))
	}
	schools = schoolJSON.Data
	log.Info(schools[0].Name)
	// 1.3 save to file
	if content, err = json.MarshalIndent(schoolJSON, "", "  "); err != nil {
		log.Fatalw("marshal school list failed", zap.Error(err))
	}
	if err := os.WriteFile(schoolListFile, content, 0666); err != nil {
		log.Fatalw("write school list failed", zap.Error(err))
	}
	// 1.4 load school id -> name map
	for _, sc := range schools {
		schoolIDNameMap[sc.SchoolID] = sc.Name
	}
	log.Infow("school list loaded", zap.Int("school num", len(schools))) // should be 2827

	// 2. parallel get school info
	// 2.0 init
	mkdir(schoolInfoDir)
	mkdir(schoolInfoRawDir)
	schoolInfoIDCh := make(chan string, chanBuffer)
	wg := &sync.WaitGroup{}
	// 2.1 start worker
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go schoolInfoWorker(schoolInfoIDCh, wg)
	}
	// 2.2 producer, send data
	for index, school := range schools {
		if debug && index > debugCount {
			break
		}
		schoolInfoIDCh <- school.SchoolID
		if index%100 == 0 {
			log.Infof("%v/%v school info have been processed", index, len(schools))
		}
	}
	close(schoolInfoIDCh)
	wg.Wait()

	// 3. parallel read province score: get year/type/batch group
	// 3.0 init
	mkdir(schoolPTBRawDir)
	schoolPTBIDCh := make(chan string, chanBuffer)
	schoolPTBCollectorCh := make(chan string, chanBuffer)
	collectorWG := &sync.WaitGroup{}
	// 3.1 start collector
	collectorWG.Add(1)
	go schoolPTBCollector(schoolPTBCollectorCh, collectorWG)
	// 3.2 start worker
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go schoolPTBWorker(schoolPTBIDCh, schoolPTBCollectorCh, wg)
	}
	// 3.3 producer
	for index, school := range schools {
		if debug && index > debugCount {
			break
		}
		schoolPTBIDCh <- school.SchoolID
		if index%100 == 0 {
			log.Infof("%v/%v school ptb have been processed", index, len(schools))
		}
	}
	close(schoolPTBIDCh)
	wg.Wait()
	close(schoolPTBCollectorCh)
	collectorWG.Wait()
	// ptb info may fail: 71

	// 4. detail
	// have to read from file. should be 433381 lines
	// [year,school,province] -> [[type,batch], [type,batch]...]
	detailDistributionMap := make(map[[3]string][][2]string)
	// when generate, generate by year-school-province group
	ptbFile, err := os.Open(schoolPTBFile)
	if err != nil {
		log.Fatalw("open ptb list failed", zap.Error(err))
	}
	defer ptbFile.Close()
	scanner := bufio.NewScanner(ptbFile)
	for scanner.Scan() {
		// year, school, prov, type, batch
		fields := strings.Split(scanner.Text(), ",")
		log.Info(fields)
		key := [3]string{fields[0], fields[1], fields[2]}
		if v, ok := detailDistributionMap[key]; ok {
			v = append(v, [2]string{fields[3], fields[4]})
		} else {
			v = [][2]string{{fields[3], fields[4]}}
			detailDistributionMap[key] = v
		}
	}
	log.Infof("detail group: %v", len(detailDistributionMap))
}

func specialDetailWorker(idCh chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for id := range idCh {
		fields := strings.Split(id, ",")
		// year, school id, province id, type, batch, index
		url := fmt.Sprintf(specialDetailURLFormat, fields[0], fields[1], fields[2], fields[3], fields[4], 1)
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

func schoolPTBCollector(collectorCh chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	f, err := os.OpenFile(schoolPTBFile, os.O_CREATE|os.O_RDWR|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer func() {
		must(f.Close())
	}()
	writer := bufio.NewWriter(f)
	defer func() {
		writer.Flush()
	}()
	for record := range collectorCh {
		if len(record) <= 10 {
			panic(record)
		}
		_, err := writer.WriteString(record + "\n")
		if err != nil {
			panic(err)
		}
	}
}

func schoolPTBWorker(idCh, collectorCh chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for id := range idCh {
		content, err := request(fmt.Sprintf(schoolPTBURLFormat, id), false)
		if err != nil {
			schoolPTBFailed.Add(1)
			continue
		}
		// write raw content
		if err := os.WriteFile(path.Join(schoolPTBRawDir, fmt.Sprintf("%v_%v.json", id, schoolIDNameMap[id])), content, 0666); err != nil {
			log.Fatalw("write school ptb raw failed", zap.Error(err))
		}
		// load school info
		var schoolPTB ptb
		if err := json.Unmarshal(content, &schoolPTB); err != nil {
			log.Fatalw("unmarshal school ptb failed", zap.Error(err), zap.String("id", id))
		}
		// generate year, school, province,
		for _, yearData := range schoolPTB.Data.Data {
			for _, provinceData := range yearData.Province {
				for _, tb := range combination(provinceData.Type, provinceData.Batch) {
					collectorCh <- fmt.Sprintf("%v,%v,%v,%v,%v", yearData.Year, mustToInt(id), provinceData.Pid, tb[0], tb[1])
				}
			}
		}
	}
}

func schoolInfoWorker(idCh chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for id := range idCh {
		content, err := request(fmt.Sprintf(schoolInfoURLFormat, id), true)
		if err != nil {
			schoolInfoFailed.Add(1)
			continue
		}
		// write raw content
		if err := os.WriteFile(path.Join(schoolInfoRawDir, fmt.Sprintf("%v_%v.json", id, schoolIDNameMap[id])), content, 0666); err != nil {
			log.Fatalw("write school list raw failed", zap.Error(err))
		}
		// load school info
		var schoolInfoJSON info
		if err := json.Unmarshal(content, &schoolInfoJSON); err != nil {
			log.Fatalw("unmarshal school info failed", zap.Error(err), zap.String("id", id))
		}
		// write to file
		if content, err = json.MarshalIndent(schoolInfoJSON, "", "  "); err != nil {
			log.Fatalw("marshal school info failed", zap.Error(err), zap.String("id", id))
		}
		if err := os.WriteFile(path.Join(schoolInfoDir, fmt.Sprintf("%v_%v.json", id, schoolIDNameMap[id])), content, 0666); err != nil {
			log.Fatalw("write school info failed", zap.Error(err), zap.String("id", id))
		}
	}
}

func request(url string, checkStatus bool) ([]byte, error) {
	client := &http.Client{}
	ua := popua.GetWeightedRandom()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Errorw("http request build failed", zap.Error(err), zap.String("url", url))
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		log.Errorw("http request send failed", zap.Error(err), zap.String("url", url))
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if checkStatus {
			log.Errorw("http response code check failed", zap.Int("status", resp.StatusCode), zap.String("url", url))
		}
		return nil, errors.New("check http status code failed")
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorw("load resp body failed", zap.Error(err), zap.String("url", url))
		return nil, err
	}
	return content, err
}

func combination(a, b []int) [][2]int {
	res := make([][2]int, 0, len(a)*len(b))
	for _, a1 := range a {
		for _, b1 := range b {
			res = append(res, [2]int{a1, b1})
		}
	}
	return res
}

func mustToInt(s string) int {
	i, e := strconv.Atoi(s)
	if e != nil {
		panic(e)
	}
	return i
}

func must(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func mkdir(path string) {
	_, err := os.Stat(path)
	if err == nil {
		// already exists
		return
	}
	if os.IsNotExist(err) {
		must(os.Mkdir(path, 0777))
		return
	}
	panic(err)
}
