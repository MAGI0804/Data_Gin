package qimai

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
)

// KSort 对参数字典的键进行字典序排序
func KSort(params map[string]string) string {
	// 对键进行排序
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建键值对字符串
	var paramStrs []string
	for _, key := range keys {
		paramStrs = append(paramStrs, key+"="+params[key])
	}

	// 用&连接所有键值对
	joinedStr := strings.Join(paramStrs, "&")

	// 对整个字符串进行URL编码
	encodedStr := url.QueryEscape(joinedStr)

	// 替换特定字符（Java代码中的特殊处理）
	encodedStr = strings.ReplaceAll(encodedStr, "%3D", "=")
	encodedStr = strings.ReplaceAll(encodedStr, "%26", "&")

	return encodedStr
}

// ComputeSignature 使用HmacSHA1算法签名并进行Base64编码
func ComputeSignature(baseString, keyString string) string {
	// 创建HmacSHA1签名
	h := hmac.New(sha1.New, []byte(keyString))
	h.Write([]byte(baseString))

	// 获取签名的二进制数据并进行Base64编码
	signatureBytes := h.Sum(nil)
	base64Signature := base64.StdEncoding.EncodeToString(signatureBytes)

	return base64Signature
}

// GenerateToken 生成最终的token
func GenerateToken(params map[string]string, secretKey string) string {
	// 1. 对参数进行KSort处理
	signatureText := KSort(params)

	// 2. 使用HmacSHA1算法签名并进行Base64编码
	base64Signature := ComputeSignature(signatureText, secretKey)

	// 3. 对Base64编码后的字符串进行URL编码
	token := url.QueryEscape(base64Signature)

	return token
}
