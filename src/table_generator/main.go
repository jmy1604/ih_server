package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

const (
	RuntimeRootDir  = "../"
	GenerateSrcPath = "src/csv_readers" // 生成代码目录
	GenerateTabPath = "game_csv"        // 生成csv表目录
)

type TableGeneratorConfig struct {
	ExcelPath      string // excel文件目录
	HeaderIndex    int32  // 表头行索引
	ValueTypeIndex int32  // 值类型行索引
	UserTypeIndex  int32  // 用户类型行索引
	DataStartIndex int32  // 数据开始行索引
}

func main() {
	var config_path string
	if len(os.Args) > 1 {
		tmp_path := flag.String("f", "", "config run path")
		log.Printf("os.Args %v\n", os.Args)
		if nil != tmp_path {
			flag.Parse()
			log.Printf("配置参数 %v\n", *tmp_path)
			config_path = *tmp_path
		}
	} else {
		log.Printf("参数不够\n")
		return
	}
	data, err := ioutil.ReadFile(config_path)
	if err != nil {
		log.Printf("读取配置文件[%v]失败 %v\n", config_path, err.Error())
		return
	}

	var config TableGeneratorConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("解析配置文件[%v]失败 %v\n", config_path, err.Error())
		return
	}

	var files []os.FileInfo
	files, err = ioutil.ReadDir(config.ExcelPath)
	if err != nil {
		log.Printf("读取excel目录失败 %v\n", config.ExcelPath, err.Error())
		return
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".xlsx") {
			continue
		}

		log.Printf("读取excel文件%v>>>\n", f.Name())

		generate_src_path := RuntimeRootDir + GenerateSrcPath
		generate_tab_path := RuntimeRootDir + GenerateTabPath
		if !GenSourceAndCsv(config.ExcelPath, f.Name(), generate_src_path, generate_tab_path, config.HeaderIndex, config.ValueTypeIndex, config.UserTypeIndex, config.DataStartIndex) {
			continue
		}

		log.Printf("<<<excel文件%v读取结束\n", f.Name())
	}
}
