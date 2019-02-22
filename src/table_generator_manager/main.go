package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/tealeg/xlsx"
)

type TableGeneratorConfig struct {
	ExcelPath       string // excel文件目录
	GenerateSrcPath string // 生成代码目录
	GenerateTabPath string // 生成csv表目录
	HeaderIndex     int32  // 表头行索引
	ValueTypeIndex  int32  // 值类型行索引
	UserTypeIndex   int32  // 用户类型行索引
	DataStartIndex  int32  // 数据开始行索引
}

func main() {
	config_path := "../conf/table_generator.json"
	data, err := ioutil.ReadFile(config_path)
	if err != nil {
		fmt.Printf("读取配置文件[%v]失败 %v", config_path, err.Error())
		return
	}

	var config TableGeneratorConfig
	err = json.Unmarshal(data, config)
	if err != nil {
		fmt.Printf("解析配置文件[%v]失败 %v", config_path, err.Error())
		return
	}

	var files []os.FileInfo
	files, err = ioutil.ReadDir(config.ExcelPath)
	if err != nil {
		fmt.Printf("读取excel目录失败 %v", config.ExcelPath, err.Error())
		return
	}

	var xf *xlsx.File
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".xlsx") {
			continue
		}

	}
}
