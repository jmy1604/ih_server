package table_generator

import (
	"fmt"
	"os"
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
	str := "package " + dest_dir + "\n\nimport (\n"
	str += ("	\"os\"\n")
	str += ")\n\n"
	_, err = f.WriteString(str)
	if err != nil {
		return
	}

	// table struct
	tname := _upper_first_char(f.Name())
	str = "type " + tname + " struct {\n"
	for _, c := range tt.cols {
		str += ("	" + _upper_first_char(c.header) + " " + c.value_type)
	}
	str += "}\n\n"
	_, err = f.WriteString(str)
	if err != nil {
		return
	}

	// table manager struct
	tmname := tname + "Mgr"
	str = "type " + tmname + " struct {\n"
	str += "}\n\n"

	// read function
	str = "func (this *" + sname + ") Read() {"
	str += "}\n"

	return
}

func GenSource(table_path, table_file, dest_path string, header_index, value_type_index, user_type_index, data_start_index int32) {
	xf, err := xlsx.OpenFile(table_file)
	if err != nil {
		fmt.Printf("读取文件%v失败 %v", table_file, err.Error())
		return
	}

	var tt this_table
	sheets := xf.Sheets
	for idx := int32(0); idx < int32(len(sheets)); idx++ {
		s := sheets[idx]
		var col column
		if idx == header_index {
			col.header = s.Name
		} else if idx == value_type_index {
			col.value_type = s.Name
		} else if idx == user_type_index {
			col.user_type = s.Name
		}
		tt.cols = append(tt.cols, &col)
	}
	tt.data_start_index = data_start_index

	if err = os.MkdirAll(dest_path, os.ModePerm); err != nil {
		fmt.Printf("创建目录结构%v错误 %v", dest_path, err.Error())
		return
	}
	if err = os.Chmod(dest_path, os.ModePerm); err != nil {
		fmt.Printf("修改目录%v权限错误 %v", dest_path, err.Error())
		return
	}

	src_file := strings.TrimSuffix(table_file, ".xlsx") + ".go"
	var f *os.File
	f, err = os.OpenFile(src_file, os.O_RDWR|os.O_APPEND, 0755)
	if err != nil {
		fmt.Printf("打开文件%v失败 %v", src_file, err.Error())
		return
	}

	err = _write_source(f, dest_path, &tt)
	if err != nil {
		fmt.Printf("写文件%v错误 %v", f.Name, err.Error())
		return
	}

	if err = f.Sync(); err != nil {
		fmt.Printf("同步文件%v失败 %v", src_file, err.Error())
		return
	}
	if err = f.Close(); err != nil {
		fmt.Printf("关闭文件%v失败 %v", src_file, err.Error())
		return
	}

}

func GenCsvTable() {

}
