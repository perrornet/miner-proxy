package encryption

import (
	"github.com/jmcvetta/randutil"
	"miner-proxy/pkg"
)

var (
	proxyStart = []byte{87, 62, 64, 57, 136, 6, 18, 50, 118, 135, 214, 247}
	proxyEnd   = []byte{93, 124, 242, 154, 241, 48, 161, 242, 209, 90, 73, 163}
	// proxyJustConfusionStart 只是混淆数据才回使用的开头
	//proxyJustConfusionStart = []byte{113,158,190,157,204,56,4,142,189,85,168,56}
	proxyConfusionStart = []byte{178, 254, 235, 166, 15, 61, 52, 198, 83, 207, 6, 83, 183, 115, 50, 58, 110, 6, 13, 60, 143, 242, 254, 143}
	proxyConfusionEnd   = []byte{114, 44, 203, 23, 55, 50, 148, 231, 241, 154, 112, 180, 115, 126, 148, 149, 180, 55, 115, 242, 98, 119, 170, 249}
)

// SeparateConfusionData 分离混淆的数据
func SeparateConfusionData(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	var result = make([]byte, 0, len(data)/2)
	for index, v := range data {
		if index%2 == 0 {
			continue
		}
		result = append(result, v)
	}
	return result
}

// BuildConfusionData 构建混淆数据
// 从 10 - 135中随机一个数字作为本次随机数据的长度 N
// 循环 N 次, 每次从 1 - 255 中随机一个数字作为本次随机数据
// 最后在头部加入 proxyConfusionStart 尾部加入 proxyConfusionStart
func BuildConfusionData() []byte {
	number, _ := randutil.IntRange(10, 135)
	var data = make([]byte, number)
	for i := 0; i < number; i++ {
		index, _ := randutil.IntRange(1, 255)
		data[i] = uint8(index)
	}
	data = append(data, proxyConfusionEnd...)
	return append(proxyConfusionStart, data...)
}

// EncryptionData 构建需要发送的加密数据
// 先使用 SecretKey aes 加密 data 如果 UseSendConfusionData 等于 true
// 那么将会每25个字符插入 buildConfusionData 生成的随机字符
func EncryptionData(data []byte, UseSendConfusionData bool, secretKey string) ([]byte, error) {
	if UseSendConfusionData { // 插入随机混淆数据
		confusionData := BuildConfusionData()
		confusionData = confusionData[len(proxyConfusionStart) : len(confusionData)-len(proxyConfusionEnd)]
		var result []byte
		for _, v := range data {
			result = append(result, confusionData[0])
			confusionData = append(confusionData[1:], confusionData[0])
			result = append(result, v)
		}
		data = result
	}
	data, err := pkg.AesEncrypt(data, []byte(secretKey))
	if err != nil {
		return nil, err
	}
	data = append(proxyStart, data...)
	return append(data, proxyEnd...), nil
}

// DecryptData 解密数据
func DecryptData(data []byte, UseSendConfusionData bool, secretKey string) ([]byte, error) {
	data = data[len(proxyStart) : len(data)-len(proxyEnd)]

	data, err := pkg.AesDecrypt(data, []byte(secretKey))
	if err != nil {
		return nil, err
	}
	if UseSendConfusionData { // 去除随机混淆数据
		data = SeparateConfusionData(data)
	}
	return data, nil
}
