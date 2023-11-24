package misc

import (
	b64 "encoding/base64"
)

func B64DecStr(str string) (origin string) {
	b, _ := b64.StdEncoding.DecodeString(str)
	origin = string(b)
	return
}

func B64EncStr(src []byte) (dst string) {
	dst = b64.StdEncoding.EncodeToString(src)
	return
}

// B64Enc base64 encode
func B64Enc(src []byte) (dst []byte) {
	dst = make([]byte, b64.StdEncoding.EncodedLen(len(src)))
	b64.StdEncoding.Encode(dst, src)
	return
}

// B64Dec base64 decode
func B64Dec(src []byte) (dst []byte, n int, err error) {
	dst = make([]byte, b64.StdEncoding.DecodedLen(len(src)))
	n, err = b64.StdEncoding.Decode(dst, src)
	dst = dst[:n]
	return
}
