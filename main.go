package main

import (
	"bitbucket.org/ai69/popua"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bzssm/goclub/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"math"
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
	debugCount = 3000
	parallel   = 200
	chanBuffer = 500

	schools         []schoolData
	schoolIDNameMap = make(map[string]string, 0)

	schoolInfoFailed    = &atomic.Int64{}
	schoolPTBFailed     = &atomic.Int64{}
	specialDetailFailed = &atomic.Int64{}
	specialDetailTotal  = &atomic.Int64{}
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
	specialDetailDir       = "special_detail"
)

func main() {
	lgr, _, _ := logger.InitLogger(zapcore.InfoLevel, true, "")
	log = lgr
	// stat
	defer func() {
		log.Infof("school info failed    : %v", schoolInfoFailed.Load())
		log.Infof("school ptb failed     : %v", schoolPTBFailed.Load())
		log.Infof("special detail total  : %v", specialDetailTotal.Load())
		log.Infof("special detail failed : %v", specialDetailFailed.Load())
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
	// [school,province] -> [[year,type,batch], [year,type,batch]...]
	// if key = year, school, province: 215541 groups -> too many files
	// if key = year, school: 13272 groups -> too less concurrency
	// key = school, prov: 49606 groups
	detailDistributionMap := make(map[[2]string][][3]string)
	ptbFile, err := os.Open(schoolPTBFile)
	if err != nil {
		log.Fatalw("open ptb list failed", zap.Error(err))
	}
	defer ptbFile.Close()
	scanner := bufio.NewScanner(ptbFile)
	for scanner.Scan() {
		// year, school, prov, type, batch
		fields := strings.Split(scanner.Text(), ",")
		key := [2]string{fields[1], fields[2]}
		if v, ok := detailDistributionMap[key]; ok {
			v = append(v, [3]string{fields[0], fields[3], fields[4]})
		} else {
			v = [][3]string{{fields[0], fields[3], fields[4]}}
			detailDistributionMap[key] = v
		}
	}
	// Check
	for _, v := range detailDistributionMap {
		if len(v) > 1 {
			log.Info(v) //竟然没有一个多的？？？
		}
	}
	os.Exit(1)
	// 4.1 init
	reqCh := make(chan detailGroup, chanBuffer)
	collectorCh := make(chan SchoolProv, chanBuffer)
	mkdir(specialDetailDir)
	// 4.2 start collector
	collectorWG.Add(1)
	go specialDetailCollector(collectorCh, collectorWG)
	// 4.3 start worker
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go specialDetailWorker(reqCh, collectorCh, wg)
	}
	// 4.4 producer
	index := 0
	for k, v := range detailDistributionMap {
		reqCh <- detailGroup{
			Key:   k,
			Value: v,
		}
		if index%100 == 0 {
			log.Infof("%v/%v special group have been processed", index, len(detailDistributionMap))
		}
		index++
	}
	close(reqCh)
	wg.Wait()
	close(collectorCh)
	collectorWG.Wait()
}

func specialDetailCollector(dataCh chan SchoolProv, wg *sync.WaitGroup) {
	defer wg.Done()
	for schoolProv := range dataCh {
		if len(schoolProv.YTBSpecials) > 1 {
			log.Info("ytbspecial > 1", schoolProv.SchoolID, schoolProv.ProvinceID)
		}
		content, err := json.MarshalIndent(schoolProv, "", "  ")
		if err != nil {
			log.Errorw("marshal special detail failed",
				zap.Error(err),
				zap.String("school", schoolProv.SchoolID),
				zap.String("prov", schoolProv.ProvinceID))
			continue
		}
		file, err := os.OpenFile(path.Join(specialDetailDir, fmt.Sprintf("%v_%v.json", schoolProv.SchoolID, schoolProv.ProvinceID)), os.O_CREATE|os.O_RDWR, 0666)
		if _, err := file.Write(content); err != nil {
			log.Errorw("write special detail file failed", zap.String("file", file.Name()))
			continue
		}
	}
}

// group: [[school, prov] -> [y,t,b], [y,t,b]]
func specialDetailWorker(groupCh chan detailGroup, collectorCh chan SchoolProv, wg *sync.WaitGroup) {
	defer wg.Done()
	for group := range groupCh {
		ytbSpecials := make([]YTBSpecial, 0)
		// year, type, batch
		for _, oneYTBData := range group.Value {
			specialDetailTotal.Add(1)
			specials := getSpecialDetailByPage(oneYTBData[0], group.Key[0], group.Key[1], oneYTBData[1], oneYTBData[2])
			if specials == nil || len(specials) == 0 {
				specialDetailFailed.Add(1)
				continue
			}
			ytbSpecials = append(ytbSpecials, YTBSpecial{
				Year:    oneYTBData[0],
				Typ:     oneYTBData[1],
				Batch:   oneYTBData[2],
				Special: specials,
			})
		}
		if len(ytbSpecials) != 0 {
			collectorCh <- SchoolProv{
				SchoolID:    group.Key[0],
				ProvinceID:  group.Key[1],
				YTBSpecials: ytbSpecials,
			}
		}
	}
}

func getSpecialDetailByPage(year, school, prov, typ, batch string) []Special {
	firstPageURL := fmt.Sprintf(specialDetailURLFormat, year, school, prov, typ, batch, 1)
	content, err := request(firstPageURL, false)
	if err != nil {
		return nil
	}
	var ss SchoolSpecial
	if err := json.Unmarshal(content, &ss); err != nil {
		log.Error("unmarshal school special failed.", err, year, school, prov, typ, batch)
		return nil
	}
	res := make([]Special, 0, ss.Data.NumFound)
	// add page 1 data
	res = append(res, ss.Data.Item...)
	for page := 2; page < int(math.Ceil(float64(ss.Data.NumFound)/10)); page++ {
		content, err = request(firstPageURL, false)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(content, &ss); err != nil {
			log.Error("unmarshal school special failed.", err, year, school, prov, typ, batch, page)
			continue
		}
		res = append(res, ss.Data.Item...)
	}
	return res
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
