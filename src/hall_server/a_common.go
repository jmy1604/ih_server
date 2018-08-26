package main

import (
	"ih_server/libs/log"
	"strconv"
	"strings"
)

type ItemInfo struct {
	Id  int32
	Num int32
}

// 解析 [...] 格式的字符串数组
func parse_xml_str_arr(instr string, spe_str string) (ret_ivals []int32) {
	if "" == instr {
		return nil
	}

	if "[]" == instr {
		return make([]int32, 0)
	}

	if len(instr) <= 2 {
		log.Error("parse_xml_str_arr instr[%s] err", instr)
		return nil
	}

	tmp_byte := []byte(instr)
	tmp_byte = tmp_byte[1 : len(tmp_byte)-1]
	strs := strings.Split(string(tmp_byte), spe_str)

	//log.Info("parse_xml_str_arr %s arr[%v]", string(tmp_byte), strs)
	tmp_len := int32(len(strs))
	var ival int
	var err error
	if tmp_len > 0 {
		ret_ivals = make([]int32, 0, tmp_len)
		for tmp_i := int32(0); tmp_i < tmp_len; tmp_i++ {
			ival, err = strconv.Atoi(strs[tmp_i])
			if nil != err {
				log.Error("parse_xml_str_arr failed to convrt[%s] err [%s] strslen[%d] tmp_i[%d]!", strs[tmp_i], err.Error(), tmp_len, tmp_i)
				return nil
			}

			ret_ivals = append(ret_ivals, int32(ival))
		}
	}

	return
}

// 解析 数字,数字... 格式的字符串数组
func parse_xml_str_arr2(instr string, spe_str string) (ret_ivals []int32) {
	if "" == instr {
		return nil
	}

	strs := strings.Split(instr, spe_str)

	//log.Info("parse_xml_str_arr %s arr[%v]", string(tmp_byte), strs)
	tmp_len := int32(len(strs))
	var ival int
	var err error
	if tmp_len > 0 {
		ret_ivals = make([]int32, 0, tmp_len)
		for tmp_i := int32(0); tmp_i < tmp_len; tmp_i++ {
			ival, err = strconv.Atoi(strs[tmp_i])
			if nil != err {
				log.Error("parse_xml_str_arr failed to convrt[%s] err [%s] strslen[%d] tmp_i[%d]!", strs[tmp_i], err.Error(), tmp_len, tmp_i)
				return nil
			}

			ret_ivals = append(ret_ivals, int32(ival))
		}
	}

	return
}

// 解析 id,num|id,num|... 格式的字串
func parse_id_nums_string(in_str string) []*ItemInfo {
	idnum_str_arr := strings.Split(in_str, "|")
	tmp_len := int32(len(idnum_str_arr))
	if tmp_len < 1 {
		log.Error("parse_id_nums_string tmp_len < 1")
		return nil
	}

	ret_infos := make([]*ItemInfo, 0, tmp_len)

	var id, num int
	var err error
	var tmp_iteminfo *ItemInfo
	for idx := int32(0); idx < tmp_len; idx++ {
		idnum_str := idnum_str_arr[idx]
		id_num_strs := strings.Split(idnum_str, ",")
		if 2 != len(id_num_strs) {
			log.Error("parse_id_nums_string idnum_str[%s] error !", idnum_str)
			continue
		}

		id, err = strconv.Atoi(id_num_strs[0])
		if nil != err {
			log.Error("parse_id_nums_string id_num_strs id[%s] error !", id_num_strs[0])
			continue
		}

		num, err = strconv.Atoi(id_num_strs[1])
		if nil != err {
			log.Error("parse_id_nums_string id_num_strs num[%s] error !", id_num_strs[1])
			continue
		}

		tmp_iteminfo = &ItemInfo{}
		tmp_iteminfo.Id = int32(id)
		tmp_iteminfo.Num = int32(num)

		ret_infos = append(ret_infos, tmp_iteminfo)
	}

	return ret_infos
}
