package lib

import (
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"
)

const (
	EncryptedPrefix = "encrypted://"
)

var (
	K = [16]byte{23, 45, 56, 198, 32, 124, 87, 90, 77, 212, 3, 108, 234, 157, 132, 182}
)

func PreparePassword(password string) [16]byte {
	return md5.Sum([]byte(password))
}

func Encrypt(key [16]byte, data string) (string, error) {
	result := bytes.NewBuffer(nil)

	hasher := hmac.New(sha256.New, key[:])
	hasher.Write([]byte(data))
	sign := hasher.Sum(nil)
	if _, err := result.Write(sign); err != nil {
		return "", err
	}

	if encrypted, err := aesEncrypt(key[:], []byte(data)); err != nil {
		return "", err
	} else {
		if _, err := result.Write(encrypted); err != nil {
			return "", err
		}
	}

	return EncryptedPrefix + base64.StdEncoding.EncodeToString(result.Bytes()), nil
}

func Decrypt(key [16]byte, data string) (string, error) {
	if !strings.HasPrefix(data, EncryptedPrefix) {
		return data, nil
	}

	data = data[len(EncryptedPrefix):]
	binaryData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	reader := bytes.NewReader(binaryData)

	sign := make([]byte, 32)
	_, err = reader.Read(sign)
	if err != nil {
		return "", fmt.Errorf("decrypt failed, err = %v", err)
	}

	encrypted, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("decrypt failed, err = %v", err)
	}

	if decrypted, err := aesDecrypt(key[:], encrypted); err != nil {
		return "", fmt.Errorf("decrypt failed, err = %v", err)
	} else {
		hasher := hmac.New(sha256.New, key[:])
		hasher.Write(decrypted)
		if !hmac.Equal(hasher.Sum(nil), sign) {
			return "", fmt.Errorf("decrypt failed, err = verify hmac-sha256 signature failed")
		} else {
			return string(decrypted), nil
		}
	}
}

func aesEncrypt(key, data []byte) ([]byte, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}

	paddingLen := aesBlock.BlockSize() - len(data)%aesBlock.BlockSize()
	padding := bytes.Repeat([]byte{byte(paddingLen)}, paddingLen)
	data = append(data, padding...)

	for i := 0; i < len(data); i += aesBlock.BlockSize() {
		aesBlock.Encrypt(data[i:i+aesBlock.BlockSize()], data[i:i+aesBlock.BlockSize()])
	}
	return data, nil
}

func aesDecrypt(key, data []byte) (result []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("invalid password or cipher data")
		}
	}()

	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}

	cipherTextLen := len(data)
	if cipherTextLen%aesBlock.BlockSize() != 0 {
		return []byte{}, fmt.Errorf("crypto/cipher: input not full blocks")
	}

	dataLen := len(data)

	for i := 0; i < dataLen; i += aesBlock.BlockSize() {
		aesBlock.Decrypt(data[i:i+aesBlock.BlockSize()], data[i:i+aesBlock.BlockSize()])
		if dataLen-i == aesBlock.BlockSize() {
			paddingLen := int(data[dataLen-1])
			data = data[:(dataLen - paddingLen)]
		}
	}
	return data, nil
}
