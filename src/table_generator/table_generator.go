package main

import (
	"encoding/csv"
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
	index      int32
}

type this_table struct {
	sheet            *xlsx.Sheet
	cols             []*column
	header_index     int32
	value_type_index int32
	user_type_index  int32
	data_start_index int32
}

func _upper_first_char(str string) string {
	if str == "" {
		return str
	}
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
	str += "	\"encoding/csv\"\n"
	str += "	\"io/ioutil\"\n"
	str += "	\"log\"\n"
	str += "	\"strconv\"\n"
	str += "	\"strings\"\n"
	str += ")\n\n"

	// table struct
	_, src_file := filepath.Split(f.Name())
	tname := _upper_first_char(strings.TrimSuffix(src_file, ".go"))
	str += "type " + tname + " struct {\n"
	for _, c := range tt.cols {
		str += ("	" + c.header + " " + c.value_type + "\n")
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
	str += "	if file_path_name == \"\" {\n"
	str += ("		file_path_name = \"" + RuntimeRootDir + GenerateTabDir + "/" + strings.TrimSuffix(src_file, ".go") + ".csv\"\n")
	str += "	}\n"
	str += "	cs, err := ioutil.ReadFile(file_path_name)\n"
	str += "	if err != nil {\n"
	str += "		log.Printf(\"" + tmname + ".Read err: %v\", err.Error())\n"
	str += "		return false\n"
	str += "	}\n\n"
	str += " 	r := csv.NewReader(strings.NewReader(string(cs)))\n"
	str += "	ss, _ := r.ReadAll()\n"
	str += "   	sz := len(ss)\n"
	str += ("	this.id2items = make(map[int32]*" + tname + ")\n")
	str += "    for i := int32(1); i < int32(sz); i++ {\n"
	str += ("		//if i < " + strconv.Itoa(int(tt.data_start_index)) + " {\n")
	str += "		//	continue\n"
	str += "		//}\n"
	str += ("		var v " + tname + "\n")
	var has_int32, has_float32 bool
	for n := 0; n < len(tt.cols); n++ {
		c := tt.cols[n]
		if c == nil {
			continue
		}
		if !has_int32 && c.value_type == "int32" {
			str += ("		var intv, id int\n")
			has_int32 = true
		} else if !has_float32 && c.value_type == "float32" {
			str += ("		var floatv float32\n")
			has_float32 = true
		}
	}
	for n := 0; n < len(tt.cols); n++ {
		var s string = "ss[i][" + strconv.Itoa(n) + "]"
		c := tt.cols[n]
		if c == nil {
			continue
		}
		// column type
		str += "		// " + c.header + "\n"
		if c.value_type == "int32" {
			var s2 string = "strconv.Atoi(" + s + ")"
			str += ("		intv, err = " + s2 + "\n")
			str += ("		if err != nil {\n")
			str += ("			log.Printf(\"table " + tname + " convert column " + c.header + " value %v with row %v err %v\", " + s + ", " + strconv.Itoa(n) + ", err.Error())\n")
			str += ("			return false\n")
			str += ("		}\n")
			str += ("		v." + c.header + " = int32(intv)\n")
			if n == 0 {
				str += ("		id = intv\n")
			}
		} else if c.value_type == "float32" {
			var s2 string = "strconv.ParseFloat(" + s + ", 32)"
			str += ("		floatv, err = " + s2 + "\n")
			str += ("		if err != nil {\n")
			str += ("			log.Printf(\"table " + tname + " convert column " + c.header + " value %v with row %v err %v\", " + s + ", " + strconv.Itoa(n) + ", err.Error())\n")
			str += ("			return false\n")
			str += ("		}\n")
			str += ("		v." + c.header + " = floatv\n")
		} else {
			str += ("		v." + c.header + " = " + s + "\n")
		}
	}
	str += "		if id <= 0 {\n"
	str += "			continue\n"
	str += "		}\n"
	str += "		this.id2items[int32(id)] = &v\n"
	str += "		this.items_array = append(this.items_array, &v)\n"
	str += "   	}\n"
	str += "	return true\n"
	str += "}\n\n"

	// get function
	str += "func (this *" + tmname + ") Get(id int32) *" + tname + " {\n"
	str += "	return this.id2items[id]\n"
	str += "}\n\n"
	str += "func (this *" + tmname + ") GetByIndex(idx int32) *" + tname + " {\n"
	str += "	if int(idx) >= len(this.items_array) {\n"
	str += "		return nil\n"
	str += "	}\n"
	str += "	return this.items_array[idx]\n"
	str += "}\n\n"
	str += "func (this *" + tmname + ") GetNum() int32 {\n"
	str += "	return int32(len(this.items_array))\n"
	str += "}\n\n"

	_, err = f.WriteString(str)
	if err != nil {
		return
	}

	return
}

func _write_csv(f *os.File, dest_dir string, tt *this_table) (err error) {
	str := "\xEF\xBB\xBF"
	_, err = f.WriteString(str)
	if err != nil {
		return
	}
	w := csv.NewWriter(f)
	// write header
	var headers []string
	for i := 0; i < len(tt.cols); i++ {
		headers = append(headers, tt.cols[i].header)
	}
	err = w.Write(headers)
	if err != nil {
		return
	}
	for i := int(tt.data_start_index); i < tt.sheet.MaxRow; i++ {
		var datas []string
		for j := 0; j < len(tt.cols); j++ {
			datas = append(datas, tt.sheet.Cell(i, int(tt.cols[j].index)).Value)
		}
		err = w.Write(datas)
		if err != nil {
			return
		}
	}
	w.Flush()
	return
}

func _gen_x_file(table_path, table_file, dest_path, file_suffix string, tt *this_table) bool {
	err := create_dirs(dest_path)
	if err != nil {
		return false
	}

	src_file := dest_path + "/" + strings.TrimSuffix(table_file, ".xlsx") + file_suffix
	var f *os.File
	f, err = os.OpenFile(src_file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Printf("打开文件%v失败 %v\n", src_file, err.Error())
		return false
	}

	if file_suffix == ".go" {
		err = _write_source(f, dest_path, tt)
	} else if file_suffix == ".csv" {
		err = _write_csv(f, dest_path, tt)
	} else {
		log.Printf("不支持的后缀 %v\n", file_suffix)
		return false
	}

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
			col.header = _upper_first_char(c.Value)
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

			// 第一列类型必须是int
			if idx == 0 && (col.value_type != "int32" && col.value_type != "int64") {
				log.Printf("first column type must be int32 or int64")
				return true
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
		col.index = int32(idx)
		tt.cols = append(tt.cols, &col)
	}

	if tt.cols == nil || len(tt.cols) == 0 {
		log.Printf("table %v no columns\n", excel_file)
		return false
	}

	tt.header_index = header_index
	tt.value_type_index = value_type_index
	tt.user_type_index = user_type_index
	tt.data_start_index = data_start_index
	tt.sheet = sheet

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
