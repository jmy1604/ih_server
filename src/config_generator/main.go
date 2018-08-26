package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	data, err := ioutil.ReadFile("../run/ih_server/conf/all_config.json.template")
	if err != nil {
		fmt.Printf("读取配置文件失败 %v", err)
		return
	}
	var all_config = make(map[string]string)

	d := strings.Replace(string(data), "\r", "", -1)
	d = strings.Replace(d, "\n", "", -1)
	d = strings.Replace(d, "\t", "", -1)
	d = strings.Replace(d, "\"", "", -1)
	d = strings.Replace(d, "\\", "", -1)
	r := bytes.NewReader([]byte(d))
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	err = decoder.Decode(&all_config)
	if err != nil {
		fmt.Printf("解码json出错(%v)", err.Error())
		return
	}

	// 遍历json配置中的所有成员
	for k, v := range all_config {
		s, e := json.Marshal(v)
		if e != nil {
			fmt.Printf("value[%v] marshal to json error[%v]", v, e.Error())
			return
		}
		ss := strings.Replace(string(s), "{", "{\n\t", -1)
		ss = strings.Replace(ss, "}", "\n}", -1)
		ss = strings.Replace(ss, ",", ",\n\t", -1)
		ioutil.WriteFile("../conf/"+k, []byte(ss), os.ModePerm)
		fmt.Printf("pezhi: %v[%v]\n", k, v)
	}
}
