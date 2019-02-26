package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

const (
	RuntimeRootDir = "../"
	GenerateSrcDir = "src/csv_readers" // 生成代码目录
	GenerateTabDir = "game_csv"        // 生成csv表目录
	ExcelDir       = "game_excel"      // 拷贝excel文件目录
)

type TableGeneratorConfig struct {
	ExcelPath      string // excel文件源目录
	HeaderIndex    int32  // 表头行索引
	ValueTypeIndex int32  // 值类型行索引
	UserTypeIndex  int32  // 用户类型行索引
	DataStartIndex int32  // 数据开始行索引
}

func create_dirs(dest_path string) (err error) {
	if err = os.MkdirAll(dest_path, os.ModePerm); err != nil {
		log.Printf("创建目录结构%v错误 %v\n", dest_path, err.Error())
		return
	}
	if err = os.Chmod(dest_path, os.ModePerm); err != nil {
		log.Printf("修改目录%v权限错误 %v\n", dest_path, err.Error())
		return
	}
	return
}

func file_copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}

	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func main() {
	var config_path string
	if len(os.Args) > 1 {
		tmp_path := flag.String("f", "", "config path")
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
	excel_path := config.ExcelPath
	files, err = ioutil.ReadDir(excel_path)
	if err != nil {
		log.Printf("读取excel源目录%v失败 %v\n准备读取本地路径下的excel目录\n", excel_path, err.Error())
		excel_path = RuntimeRootDir + ExcelDir
		files, err = ioutil.ReadDir(excel_path)
		if err != nil {
			log.Printf("本地excel目录%v读取失败 %v\n", excel_path, err.Error())
			return
		}
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".xlsx") {
			continue
		}

		log.Printf("读取excel文件%v>>>\n", f.Name())

		generate_src_path := RuntimeRootDir + GenerateSrcDir
		generate_tab_path := RuntimeRootDir + GenerateTabDir
		if !GenSourceAndCsv(excel_path, f.Name(), generate_src_path, generate_tab_path, config.HeaderIndex, config.ValueTypeIndex, config.UserTypeIndex, config.DataStartIndex) {
			continue
		}

		log.Printf("<<<excel文件%v读取结束\n", f.Name())
	}
}
