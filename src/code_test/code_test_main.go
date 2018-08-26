package main

import (
	"fmt"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
)

func check_remove_card(cfgid int32) {
	cards := []int32{1, 2, 2, 5, 42, 4}

	len_t := int32(len(cards))
	set_idx := int32(0)
	idx := int32(0)
	for ; idx < len_t; idx++ {
		if cards[idx] != cfgid {
			cards[set_idx] = cards[idx]
			set_idx++
		}
	}

	for ; set_idx < idx; set_idx++ {
		cards[set_idx] = 0
	}

	fmt.Println("check_remove_card %v", cards)

}

func proto_marshal() {
	test_msg := &msg_client_message.C2SLoginRequest{}
	test_msg.Acc = "410ad4e2b4f0ab0b511740e10430aa7b0e2dc857"
	test_msg.Password = "1488189071"
	data, err := proto.Marshal(test_msg)
	if nil != err {
		fmt.Println("marshal error ", err.Error())
	} else {
		fmt.Println("%d data", len(data), data)
	}
}

func math_test() {
	var fval = float32(2.0111)
	fmt.Println("math %f  %f", math.Sqrt(2), float32(int32(fval)))
}

func SendMail() {
	fmt.Printf("请输入mail_id:")
	var mail_id int32
	fmt.Scanf("%d", &mail_id)
	url_str := "http://123.207.182.67:10002/send_mail?pids=40|1&mail_id=2&last_sec=30000"
	_, err := http.Get(url_str)
	if nil != err {
		fmt.Println("http get error ", err.Error())
	}
}

func GmMailTest() {
	fmt.Printf("请输入mail_id:")
	var mail_id int32
	fmt.Scanf("%d", &mail_id)
	url_str := "http://123.207.182.67:10002/gm_cmd?cmd_str=give_chest 58 6001"
	_, err := http.Get(url_str)
	if nil != err {
		fmt.Println("http get error ", err.Error())
	}
}

func GmCampTest() {
	fmt.Printf("请输入camp_id:")
	var camp_id int32
	fmt.Scanf("%d\n", &camp_id)
	fmt.Printf("请输入player_id:")
	var player_id int32
	fmt.Scanf("%d", &player_id)
	url_str := fmt.Sprintf("set_camp %d %d", player_id, camp_id)
	url_str = "http://192.168.10.156:10002/gm_cmd?cmd_str=" + url.QueryEscape(url_str)
	_, err := http.Get(url_str)
	if nil != err {
		fmt.Println("http get error ", err.Error())
	}
}

func GmLvlTest() {
	fmt.Printf("请输入Lvl:")
	var lvl int32
	fmt.Scanf("%d\n", &lvl)
	fmt.Printf("请输入player_id:")
	var player_id int32
	fmt.Scanf("%d", &player_id)
	url_str := fmt.Sprintf("set_lvl %d %d", player_id, lvl)
	//url_str = "http://192.168.10.156:10002/gm_cmd?cmd_str=" + url.QueryEscape(url_str)
	url_str = "http://123.207.182.67:10002/gm_cmd?cmd_str=" + url.QueryEscape(url_str)
	_, err := http.Get(url_str)
	if nil != err {
		fmt.Println("http get error ", err.Error())
	}
}

func GmAddCardTest() {
	fmt.Printf("请输入card_id:")
	var card_id int32
	fmt.Scanf("%d\n", &card_id)
	var player_id int32
	fmt.Printf("请输入player_id:")
	fmt.Scanf("%d\n", &player_id)
	url_str := fmt.Sprintf("give_card %d %d", player_id, card_id)
	url_str = "http://192.168.10.156:10002/gm_cmd?cmd_str=" + url.QueryEscape(url_str)
	//url_str = "http://123.207.182.67:10002/gm_cmd?cmd_str=" + url.QueryEscape(url_str)
	_, err := http.Get(url_str)
	if nil != err {
		fmt.Println("http get error ", err.Error())
	}
}

func GmRebootTest() {
	url_str := fmt.Sprintf("rebot")
	//url_str = "http://192.168.10.156:10002/gm_cmd?cmd_str=" + url.QueryEscape(url_str)
	url_str = "http://192.168.10.113:10002/gm_cmd?cmd_str=" + url.QueryEscape(url_str)
	_, err := http.Get(url_str)
	if nil != err {
		fmt.Println("http get error ", err.Error())
	}
}

func TimeTest() {
	t, _ := time.Parse("2006 Jan 02 15:04:05", "2017 Dec 03 15:04:05")
	fmt.Printf("年 %d", t.Year())
}

func string_test() {
	group_arr := strings.Split("dad|dad", "|")
	fmt.Println(len(group_arr))
}

func SliceTest() {
	a_slice := make([]int32, 5)
	a_slice[4] = 10
	b_slice := a_slice
	a_slice = make([]int32, 3)
	c_slice := b_slice
	fmt.Println("b_slice[4] = ", b_slice[4], " a_slice", a_slice[0], " c_slice", c_slice[4])
}

func CmdExecTest() {
	exec.Command("/bin/sh", "/root/server/bin/up_data_restart_all_server.sh")
	/*
		cmd := exec.Command("/bin/sh", "/root/server/bin/up_data_restart_all_server.sh")
		out_b, err := cmd.Output()

		fmt.Println("", string(out_b), err.Error())
	*/
}

func CopyTest() {
	src_bytes := make([]byte, 30)
	print_bytes := src_bytes
	copy(src_bytes, []byte{1, 2, 3})
	copy(src_bytes[3:], []byte{5, 6, 7})

	fmt.Println("out_bytes %v", print_bytes)
}

type TestObj struct {
	ObjVal int32
}

func slice_assign_test() {
	tst_obj1 := &TestObj{ObjVal: 1}
	tst_obj2 := &TestObj{ObjVal: 2}

	tst_slice1 := make([]*TestObj, 2)
	tst_slice1[0] = tst_obj1
	tst_slice1[1] = tst_obj2

	tst_slice2 := tst_slice1

	tmp_val := tst_slice2[1]
	tst_slice2[1] = tst_slice2[0]
	tst_slice2[0] = tmp_val

	fmt.Println("  ", *tst_slice1[0], *tst_slice1[1], *tst_slice2[0], *tst_slice2[1])
}

func small_rank_test() {
	tmp_rank := NewSmallRankService(10000, 1, SMALL_RANK_SORT_TYPE_B)
	start_unix := time.Now().Unix()
	for idx := int32(0); idx < 100000; idx++ {
		tmp_rank.AddUpdateRank(idx, rand.Int31n(100000), 0, "test", "test", "test")
	}

	fmt.Println(" cost sec %d", time.Now().Unix()-start_unix)
}

func big_rank_array_test() {
	tmp_rank := NewBigRankArrayItem(10, SMALL_RANK_SORT_TYPE_B, nil)
	start_unix := time.Now().Unix()

	for idx := int32(0); idx < 100; idx++ {
		tmp_rank.SetVal(rand.Int31n(100), rand.Int31n(100))

		log.Info("=====================")
		tmp_rank.PrintInfo()
	}

	tmp_rank.SetVal(0, 81)
	tmp_rank.SetVal(1, 99)
	tmp_rank.SetVal(2, 101)

	log.Info(" cost sec %d count %d", time.Now().Unix()-start_unix, tmp_rank.CurCount)

	tmp_rank.PrintInfo()
}

func big_rank_debug() {
	tmp_rank := NewBigRankService(3, 5, 0)
	//tmp_rank.AddVal(1, 99)
	//tmp_rank.AddVal(2, 99)
	//tmp_rank.AddVal(3, 100)

	//tmp_rank.AddVal(4, 80)
	//tmp_rank.AddVal(4, 101)

	//tmp_rank.AddVal(5, 102)

	var id, val int32
	for idx := int32(0); idx < 200; idx++ {
		id = rand.Int31n(100)
		val = idx //rand.Int31n(100)
		tmp_rank.AddVal(id, val)
		log.Info("add id[%d] val[%d]", id, val)
		tmp_rank.PrintAllRecords()
		log.Info("=================================================")
	}

	tmp_rank.PrintAllRecords()
}

var g_log_out bool

func big_rank_test() {
	g_log_out = true
	tmp_rank := NewBigRankService(5, 3, 0)
	rand.Seed(time.Now().Unix())
	start_unix := time.Now().Unix()

	var id, val int32
	for idx := int32(0); idx < 100; idx++ {
		//tmp_rank.AddVal(id, rand.Int31n(6000))
		id = rand.Int31n(5)
		val = rand.Int31n(20)
		if g_log_out {
			log.Info("After add id[%d] val[%d] ", id, val)
		}
		tmp_rank.AddVal(id, val)
		if g_log_out {
			tmp_rank.PrintAllRecords()
			log.Info("=================================================")
		}
	}

	log.Info(" cost sec %d count", time.Now().Unix()-start_unix)

	tmp_rank.PrintAllRecords()
}

func convert_time(unix int32) {
	t := time.Unix(int64(unix), 0)
	fmt.Println(t.Format("2006-01-02 15:04:05.999999999 -0700 MST"))
}

func main() {
	defer func() {
		log.Event("关闭", nil)
		if err := recover(); err != nil {
			log.Stack(err)
		}
		time.Sleep(time.Second * 5)
	}()
	//check_remove_card(2)
	//check_remove_card(1)
	//check_remove_card(4)
	//proto_marshal()

	log.Init("", "D:/server/MM/conf/log/code_test.json", true)

	//test_1 := int32(-1)
	//test_2 := int16(-1)

	//tmpx := int32(5<<16) | (-5 & 0x0000FFFF)
	//tmpx16 := int16(tmpx)
	//fmt.Println("", int32(tmpx16))

	//small_rank_test()

	//big_rank_array_test()
	//big_rank_test()
	//big_rank_debug()

	//gm_mgr.StartHttp()
	//CopyTest()

	//slice_assign_test()

	convert_time(1514266160)
}
