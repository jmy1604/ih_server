package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tealeg/xlsx"
)

type column struct {
	header     string
	value_type string
	user_type  string
}

type this_table struct {
	cols             []*column
	header_index     int32
	value_type_index int32
	user_type_index  int32
	data_start_index int32
}

func _upper_first_char(str string) string {
	c := []byte(str)
	var uppered bool
	if int32(c[0]) >= int32('a') && int32(c[0]) <= int32('z') {
		c[0] = byte(int32(c[0]) + int32('A') - int32('a'))
		uppered = true
	}
	if !uppered {
		return str
	}
	return string(c)
}

func _write_source(f *os.File, dest_dir string, tt *this_table) (err error) {
	_, pkg := filepath.Split(dest_dir)
	str := "package " + pkg + "\n\nimport (\n"
	str += ("	\"encoding/csv\"\n")
	str += ("	\"io/ioutil\"\n")
	str += "	\"log\"\n"
	str += "	\"strings\"\n"
	str += ")\n\n"

	// table struct
	_, src_file := filepath.Split(f.Name())
	tname := _upper_first_char(strings.TrimSuffix(src_file, ".go"))
	str += "type " + tname + " struct {\n"
	for _, c := range tt.cols {
		str += ("	" + _upper_first_char(c.header) + " " + c.value_type + "\n")
	}
	str += "}\n\n"

	// table manager struct
	tmname := tname + "Mgr"
	str += "type " + tmname + " struct {\n"
	str += ("	id2items map[int32]*" + tname + "\n")
	str += ("	items_array []*" + tname + "\n")
	str += "}\n\n"

	// read function
	str += "func (this *" + tmname + ") Read(file_path_name string) bool {\n"
	str += "	cs, err := ioutil.ReadFile(file_path_name)\n"
	str += "	if err != nil {\n"
	str += "		log.Printf(\"" + tmname + ".Read err: %v\", err.Error())\n"
	str += "		return false\n"
	str += "	}\n\n"
	str += " 	r := csv.NewReader(strings.NewReader(string(cs)))\n"
	str += "	ss, _ := r.ReadAll()\n"
	str += "   	sz := len(ss)\n"
	str += ("	this.id2items = make(map[int32]*" + tname + ")\n")
	str += "    for i := int32(0); i < int32(sz); i++ {\n"
	str += "    	//log.Printf(ss[i])\n"
	str += "        //log.Printf(ss[i][0]) //  key的数据  可以作为map的数据的值\n"
	str += ("		if i < " + strconv.Itoa(int(tt.data_start_index)) + " {\n")
	str += "			continue\n"
	str += "		}\n"
	str += ("		var v " + tname + "\n")
	for n := 0; n < len(tt.cols); n++ {
		str += ("		v." + tt.cols[n].header + " = ss[i][" + strconv.Itoa(n) + "]\n")
	}
	str += "		this.id2items[ss[i][0]] = &v\n"
	str += "		this.items_array = append(this.items_array, &v)\n"
	str += "   	}\n"
	str += "}\n\n"

	// get function
	str += "func (this *" + tmname + ") Get(id int32) {\n"
	str += "	return this.id2items[id]\n"
	str += "}\n\n"
	str += "func (this *" + tmname + ") GetByIndex(idx int32) {\n"
	str += "	if idx >= len(this.items_array) {\n"
	str += "		return nil\n"
	str += "	}\n"
	str += "	return this.items_array[idx]\n"
	str += "}\n\n"

	_, err = f.WriteString(str)
	if err != nil {
		return
	}

	return
}

func _write_csv(f *os.File, dest_dir string, tt *this_table) (err error) {
	return
}

func _gen_x_file(table_path, table_file, dest_path, file_suffix string, tt *this_table) bool {
	var err error
	if err = os.MkdirAll(dest_path, os.ModePerm); err != nil {
		log.Printf("创建目录结构%v错误 %v\n", dest_path, err.Error())
		return false
	}
	if err = os.Chmod(dest_path, os.ModePerm); err != nil {
		log.Printf("修改目录%v权限错误 %v\n", dest_path, err.Error())
		return false
	}

	src_file := dest_path + "/" + strings.TrimSuffix(table_file, ".xlsx") + file_suffix
	var f *os.File
	f, err = os.OpenFile(src_file, os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		log.Printf("打开文件%v失败 %v\n", src_file, err.Error())
		return false
	}

	err = _write_source(f, dest_path, tt)
	if err != nil {
		log.Printf("写文件%v错误 %v\n", f.Name, err.Error())
		return false
	}

	if err = f.Sync(); err != nil {
		log.Printf("同步文件%v失败 %v\n", src_file, err.Error())
		return false
	}
	if err = f.Close(); err != nil {
		log.Printf("关闭文件%v失败 %v\n", src_file, err.Error())
		return false
	}

	return true
}

func GenSourceAndCsv(excel_path, excel_file, src_dest_path, csv_dest_path string, header_index, value_type_index, user_type_index, data_start_index int32) bool {
	excel_file_path := excel_path + "/" + excel_file
	xf, err := xlsx.OpenFile(excel_file_path)
	if err != nil {
		log.Printf("读取文件%v失败 %v\n", excel_file_path, err.Error())
		return false
	}

	var tt this_table
	sheet := xf.Sheets[0]
	for idx := 0; idx < sheet.MaxCol; idx++ {
		var col column
		if header_index < int32(sheet.MaxRow) {
			c := sheet.Cell(int(header_index), idx)
			if c == nil {
				log.Printf("cell row[%v] col[%v] is null\n", header_index, idx)
				continue
			}
			col.header = c.Value
		}
		if value_type_index < int32(sheet.MaxRow) {
			c := sheet.Cell(int(value_type_index), idx)
			if c == nil {
				log.Printf("cell row[%v] col[%v] is null\n", value_type_index, idx)
				continue
			}
			if c.Value == "float" || c.Value == "float32" {
				col.value_type = "float32"
			} else if c.Value == "float64" || c.Value == "double" {
				col.value_type = "float64"
			} else if c.Value == "int" || c.Value == "int32" {
				col.value_type = "int32"
			} else if c.Value == "int64" {
				col.value_type = "int64"
			} else if c.Value == "string" {
				col.value_type = "string"
			} else {
				log.Printf("value type %v invalid in column %v\n", c.Value, idx)
				continue
			}
		}
		if user_type_index < int32(sheet.MaxRow) {
			c := sheet.Cell(int(user_type_index), idx)
			if c == nil {
				log.Printf("cell row[%v] col[%v] is null\n", user_type_index, idx)
				continue
			}
			if !(c.Value == "c|s" || c.Value == "s") {
				if c.Value != "c" {
					log.Printf("unsupported user type %v\n", c.Value)
				}
				continue
			}
			col.user_type = c.Value
		}
		tt.cols = append(tt.cols, &col)
	}
	tt.header_index = header_index
	tt.value_type_index = value_type_index
	tt.user_type_index = user_type_index
	tt.data_start_index = data_start_index

	log.Printf("this table struct: %v", tt.cols)

	// gen source
	if !_gen_x_file(excel_path, excel_file, src_dest_path, ".go", &tt) {
		return false
	}

	// gen csv
	if !_gen_x_file(excel_path, excel_file, csv_dest_path, ".csv", &tt) {
		return false
	}

	return true
}
