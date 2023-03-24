package main

type school struct {
	Data []schoolData `json:"data"`
}

type schoolData struct {
	SchoolID string `json:"school_id"`
	Name     string `json:"name"`
}

type info struct {
	Data schoolInfo `json:"data"`
}

type schoolInfo struct {
	Address       string     `json:"address"`
	Belong        string     `json:"belong"` // 教育部？
	CityID        string     `json:"city_id"`
	CityName      string     `json:"city_name"`
	CountyID      string     `json:"county_id"` // 110108,可以具体到区域
	CreateDate    string     `json:"create_date"`
	DualClass     string     `json:"dual_class"`
	DualClassName string     `json:"dual_class_name"`
	Dualclass     []struct { // 双一流学科
		Class    string `json:"class"`
		ID       string `json:"id"`
		SchoolID string `json:"school_id"`
	} `json:"dualclass"`
	F211             string `json:"f211"` // 是否211， 1是，2不是
	F985             string `json:"f985"` // 是否985
	Level            string `json:"level"`
	LevelName        string `json:"level_name"` //普通本科
	Name             string `json:"name"`
	NatureName       string `json:"nature_name"` //公办？民办？
	NumAcademician   string `json:"num_academician"`
	NumDoctor        string `json:"num_doctor"`
	NumDoctor2       string `json:"num_doctor2"`
	NumLab           string `json:"num_lab"`
	NumLibrary       string `json:"num_library"`
	NumMaster        string `json:"num_master"`
	NumMaster2       string `json:"num_master2"`
	NumSubject       string `json:"num_subject"`
	OldName          string `json:"old_name"`
	Phone            string `json:"phone"`
	Postcode         string `json:"postcode"`
	QsRank           string `json:"qs_rank"`
	QsWorld          string `json:"qs_world"`
	RuankeRank       string `json:"ruanke_rank"`
	SchoolBatch      string `json:"school_batch"` // 批次，都好分割数组 "7,14,36,46,6,51,37,86,1564,1939,1570,12"
	SchoolID         string `json:"school_id"`
	SchoolNature     string `json:"school_nature"`
	SchoolNatureName string `json:"school_nature_name"` // 公办？
	SchoolType       string `json:"school_type"`
	SchoolTypeName   string `json:"school_type_name"` // 普通本科？
	Type             string `json:"type"`
	TypeName         string `json:"type_name"` // 综合类？
}

type ptb struct {
	Data struct {
		Data []struct {
			Year     int `json:"year"`
			Province []struct {
				Pid   int   `json:"pid"`
				Type  []int `json:"type"`
				Batch []int `json:"batch"`
			} `json:"province"`
		} `json:"data"`
	} `json:"data"`
}
