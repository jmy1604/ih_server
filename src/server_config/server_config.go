package server_config

import (
	"encoding/json"
	"flag"
	"fmt"
	"ih_server/libs/log"
	"io/ioutil"
	"os"
)

const (
	SERVER_TYPE_CENTER      = 1
	SERVER_TYPE_LOGIN       = 2
	SERVER_TYPE_GAME        = 3
	SERVER_TYPE_RPC         = 4
	SERVER_TYPE_TEST_CLIENT = 100
)

const (
	RuntimeRootDir = "../"
	ConfigDir      = "conf/"
	LogConfigDir   = "conf/log/"
	GameDataDir    = "game_data/"
	LogDir         = "log/"
)

type ServerConfig interface {
	GetType() int32
	GetLogConfigFile() string
}

// 中心服务器配置
type CenterServerConfig struct {
	LogConfigFile             string // 日志配置文件地址
	ListenLoginIP             string // 监听LoginServer
	MaxLoginConnections       int32  // 最大Login连接数
	ListenGameIP              string // 监听game_server的IP
	MaxGameConnections        int32  // 最大game_server连接数
	GmIP                      string // GM命令的地址
	HallServerGroupConfigFile string // 大厅配置文件地址
	MYSQL_NAME                string
	MYSQL_IP                  string
	MYSQL_ACCOUNT             string
	MYSQL_PWD                 string
	DBCST_MIN                 int
	DBCST_MAX                 int
	MYSQL_COPY_PATH           string
}

func (this *CenterServerConfig) GetType() int32 {
	return int32(SERVER_TYPE_CENTER)
}

func (this *CenterServerConfig) GetLogConfigFile() string {
	return this.LogConfigFile
}

// 登陆服务器配置
type LoginServerConfig struct {
	ServerId           int32
	InnerVersion       string
	ServerName         string
	ListenClientIP     string
	ListenGameIP       string // 监听game_server连接
	MaxGameConnections int32  // game_server最大连接数
	LogConfigFile      string // 日志配置文件
	CenterServerIP     string // 连接AssistServer
	RedisServerIP      string // 连接redis
	VerifyAccount      bool   // 验证账号
	FacebookAppID      string
	FacebookAppSecret  string

	MYSQL_IP      string
	MYSQL_ACCOUNT string
	MYSQL_PWD     string
	MYSQL_NAME    string
	DBCST_MIN     int
	DBCST_MAX     int
}

func (this *LoginServerConfig) GetType() int32 {
	return int32(SERVER_TYPE_LOGIN)
}

func (this *LoginServerConfig) GetLogConfigFile() string {
	return this.LogConfigFile
}

// 游戏服务器配置
type GameServerConfig struct {
	ServerId             int32
	InnerVersion         string
	ServerName           string
	ListenRoomServerIP   string
	ListenClientInIP     string
	ListenClientOutIP    string
	MaxClientConnections int32
	RpcServerIP          string
	ListenRpcServerIP    string
	LogConfigFile        string // 日志配置文件
	CenterServerIP       string // 中心服务器IP
	MatchServerIP        string // 匹配服务器IP
	RecvMaxMSec          int64  // 接收超时毫秒数
	SendMaxMSec          int64  // 发送超时毫秒数
	RedisServerIP        string
	MYSQL_NAME           string
	MYSQL_IP             string
	MYSQL_ACCOUNT        string
	MYSQL_PWD            string
	DBCST_MIN            int
	DBCST_MAX            int
	MYSQL_COPY_PATH      string
	ApplePayIsSandBox    bool
}

func (this *GameServerConfig) GetType() int32 {
	return int32(SERVER_TYPE_GAME)
}

func (this *GameServerConfig) GetLogConfigFile() string {
	return this.LogConfigFile
}

// RPC服务器配置
type RpcServerConfig struct {
	LogConfigFile             string
	ListenIP                  string
	MaxConnections            int
	GameServerGroupConfigFile string
	RedisServerIP             string
}

func (this *RpcServerConfig) GetType() int32 {
	return int32(SERVER_TYPE_RPC)
}

func (this *RpcServerConfig) GetLogConfigFile() string {
	return this.LogConfigFile
}

// 测试客户端配置
type TestClientConfig struct {
	LoginServerIP     string
	LogConfigFile     string
	RegisterUrl       string
	BindNewAccountUrl string
	LoginUrl          string
	SelectServerUrl   string
	AccountPrefix     string
	AccountStartIndex int32
	AccountNum        int32
	UseHttps          bool
}

func (this *TestClientConfig) GetType() int32 {
	return int32(SERVER_TYPE_TEST_CLIENT)
}

func (this *TestClientConfig) GetLogConfigFile() string {
	return this.LogConfigFile
}

func _get_config_path(config_file string) (config_path string) {
	if len(os.Args) > 1 {
		arg_config_file := flag.String("f", "", "config file path")
		fmt.Printf("os.Args %v", os.Args)
		if nil != arg_config_file {
			flag.Parse()
			fmt.Printf("配置参数 %v", *arg_config_file)
			config_path = *arg_config_file
		}
	} else {
		config_path = RuntimeRootDir + ConfigDir + config_file
	}
	return
}

func ServerConfigLoad(config_file string, config ServerConfig) bool {
	config_path := _get_config_path(config_file)
	data, err := ioutil.ReadFile(config_path)
	if err != nil {
		fmt.Printf("读取配置文件[%v]失败 %v", config_path, err)
		return false
	}
	err = json.Unmarshal(data, config)
	if err != nil {
		fmt.Printf("解析配置文件[%v]失败 %v", config_path, err)
		return false
	}

	// 加载日志配置
	log_config_path := RuntimeRootDir + LogConfigDir + config.GetLogConfigFile()

	log.Init("", log_config_path, true)
	//defer log.Close()

	return true
}

func ServerConfigClose() {
	log.Close()
}

func GetGameDataPathFile(data_file string) string {
	return RuntimeRootDir + GameDataDir + data_file
}

func GetConfPathFile(config_file string) string {
	return RuntimeRootDir + ConfigDir + config_file
}
