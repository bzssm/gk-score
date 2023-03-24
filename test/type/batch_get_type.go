package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type provinceType struct {
	Data struct {
		Data []struct {
			Year     int `json:"year"`
			Province []struct {
				Pid        int           `json:"pid"`
				Type       []int         `json:"type"`
				Batch      []int         `json:"batch"`
				BatchGroup interface{}   `json:"batch_group"`
				First      []interface{} `json:"first"`
			} `json:"province"`
		} `json:"data"`
	} `json:"data"`
}

func main() {
	dir, err := os.ReadDir("school_province_score")
	if err != nil {
		panic(err)
	}

	resContent := strings.Builder{}
	res := make([][5]int, 0)
	for _, file := range dir {
		content, err := os.ReadFile(fmt.Sprintf("school_province_score/%v", file.Name()))
		if err != nil {
			panic(err)
		}
		var pt provinceType
		if err := json.Unmarshal(content, &pt); err != nil {
			panic(fmt.Sprintf("file: %v, error: %v", file.Name(), err))
		}
		schoolID, _ := strconv.Atoi(strings.Split(file.Name(), "_")[0])

		// do combination
		for _, yearData := range pt.Data.Data {
			for _, prov := range yearData.Province {
				for _, tb := range combination(prov.Type, prov.Batch) {
					res = append(res, [5]int{yearData.Year, schoolID, prov.Pid, tb[0], tb[1]})
					resContent.WriteString(fmt.Sprintf("%v,%v,%v,%v,%v\n", yearData.Year, schoolID, prov.Pid, tb[0], tb[1]))
				}
			}
		}
	}

	fmt.Println(len(res)) // 433381
	os.WriteFile("school_batch_type_comb.txt", []byte(resContent.String()), 0777)
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
