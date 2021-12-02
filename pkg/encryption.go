package pkg

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
)

// code from  https://www.jianshu.com/p/b5959b2defdb

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

// AES加密,CBC
func AesEncrypt(origData, key []byte) ([]byte, error) {
	//defer func() {
	//	if err := recover(); err != nil{
	//		log.Println("[ERROR]: AesEncrypt error: ", err)
	//	}
	//}()
	//log.Printf("AesEncrypt data size: %d", len(origData))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

// AES解密
func AesDecrypt(crypted, key []byte) (result []byte, err error) {
	//defer func() {
	//	if err := recover(); err != nil{
	//		log.Printf("[ERROR]: AesDecrypt error: %s ; data size: %d", err, len(crypted))
	//	}
	//}()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS7UnPadding(origData)
	//log.Printf("AesDecrypt data size: %d", len(origData))
	return origData, nil
}
