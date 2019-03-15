package share_data

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mm_server/src/server_config"
)

type PayChannel struct {
	KeyFile     string
	PaymentType string
	*rsa.PublicKey
}

type PayChannelConfig struct {
	PayChannels []*PayChannel
	ConfigPath  string
}

func (this *PayChannelConfig) _read_key(key_file string) *rsa.PublicKey {
	path := server_config.GetGameDataPathFile(key_file)
	content, err := ioutil.ReadFile(path)
	if nil != err {
		fmt.Printf("read key failed (%s)!\n", err.Error())
		return nil
	}

	block, err := base64.StdEncoding.DecodeString(string(content)) //pem.Decode([]byte(content))
	if err != nil {
		fmt.Printf("failed to parse base64 data (%v) the public key, err: %v\n", content, err.Error())
		return nil
	}

	pub, err := x509.ParsePKIXPublicKey(block)
	if nil != err {
		fmt.Printf("read key failed to ParsePkXIPublicKey\n", err.Error())
		return nil
	}

	return pub.(*rsa.PublicKey)
}

func (this *PayChannelConfig) _read_config(data []byte) bool {
	err := json.Unmarshal(data, this)
	if err != nil {
		fmt.Printf("解析配置文件失败 %v\n", err.Error())
		return false
	}

	for i := 0; i < len(this.PayChannels); i++ {
		pc := this.PayChannels[i]
		if pc == nil {
			continue
		}
		pub_key := this._read_key(server_config.GetGameDataPathFile(pc.KeyFile))
		if pub_key == nil {
			return false
		}
		pc.PublicKey = pub_key
	}

	return true
}

func (this *PayChannelConfig) LoadConfig(filepath string) bool {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Printf("读取配置文件失败 %v\n", err)
		return false
	}

	if !this._read_config(data) {
		return false
	}

	this.ConfigPath = filepath

	return true
}

func (this *PayChannelConfig) Verify(hashedReceipt, decodedSignature []byte) *PayChannel {
	var pay_channel *PayChannel
	for i := 0; i < len(this.PayChannels); i++ {
		pay_channel = this.PayChannels[i]
		err := rsa.VerifyPKCS1v15(pay_channel.PublicKey, crypto.SHA1, hashedReceipt, decodedSignature)
		if err == nil {
			return pay_channel
		}
	}
	return nil
}
