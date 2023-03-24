package main

import "fmt"

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

type detailGroup struct {
	Key   [2]string   // [school, prov]
	Value [][3]string // [[year, type, batch]....]
}

func (g *detailGroup) String() string {
	return fmt.Sprintf("year: %v, school: %v, province: %v, type: %v, batch: %v", g.Value[0], g.Key[0], g.Key[1], g.Value[1], g.Value[2])
}

type SchoolSpecial struct {
	Data struct {
		NumFound int       `json:"numFound"`
		Item     []Special `json:"item"`
	} `json:"data"`
}

type YTBSpecial struct {
	Year    string
	Typ     string
	Batch   string
	Special []Special
}

type Special struct {
	SchoolID       string `json:"school_id"`
	SpecialID      string `json:"special_id"`
	Type           string `json:"type"`
	Batch          string `json:"batch"`
	Zslx           string `json:"zslx"`
	Max            string `json:"max"`
	Min            string `json:"min"`
	Average        string `json:"average"`
	MinSection     string `json:"min_section"`
	Province       string `json:"province"`
	SpeID          string `json:"spe_id"`
	Info           string `json:"info"`
	SpecialGroup   string `json:"special_group"`
	FirstKm        string `json:"first_km"`
	SpType         string `json:"sp_type"`
	SpFxk          string `json:"sp_fxk"`
	SpSxk          string `json:"sp_sxk"`
	SpInfo         string `json:"sp_info"`
	Level1Name     string `json:"level1_name"`
	Level2Name     string `json:"level2_name"`
	Level3Name     string `json:"level3_name"`
	Level1         string `json:"level1"`
	Level2         string `json:"level2"`
	Level3         string `json:"level3"`
	Spname         string `json:"spname"`
	ZslxName       string `json:"zslx_name"`
	LocalBatchName string `json:"local_batch_name"`
	SgFxk          string `json:"sg_fxk"`
	SgSxk          string `json:"sg_sxk"`
	SgType         string `json:"sg_type"`
	SgName         string `json:"sg_name"`
	SgInfo         string `json:"sg_info"`
}

type SchoolProv struct {
	SchoolID    string
	ProvinceID  string
	YTBSpecials []YTBSpecial
}
