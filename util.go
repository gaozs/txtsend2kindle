package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func GbkToUtf8(gbkData []byte) ([]byte, error) {
	return simplifiedchinese.GBK.NewDecoder().Bytes(gbkData)
}

func isGBK(data []byte) bool {
	length := len(data) - 1
	if length > 1024*64 {
		length = 1024 * 64
	}
	var i int = 0
	for i < length {
		if data[i] <= 0x7f {
			//编码0~127,只有一个字节的编码，兼容ASCII码
			i++
			continue
		}
		//大于127的使用双字节编码，落在gbk编码范围内的字符
		if data[i] >= 0x81 &&
			data[i] <= 0xfe &&
			data[i+1] >= 0x40 &&
			data[i+1] <= 0xfe {
			i += 2
			continue
		}
		return false

	}
	return true
}

func base64Wrap(w io.Writer, r io.Reader) error {
	maxRaw := 57
	maxLineLength := base64.StdEncoding.EncodedLen(maxRaw)

	buffer := make([]byte, maxLineLength+len("\r\n"))
	copy(buffer[maxLineLength:], "\r\n")
	var b = make([]byte, maxRaw)
	for {
		n, err := r.Read(b)
		if n != 0 {
			if n == maxRaw {
				base64.StdEncoding.Encode(buffer, b[:n])
				w.Write(buffer)
			} else {
				out := make([]byte, base64.StdEncoding.EncodedLen(n)+len("\r\n"))
				base64.StdEncoding.Encode(out, b[:n])
				copy(out[base64.StdEncoding.EncodedLen(n):], "\r\n")
				w.Write(out)
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func itos(i int, d time.Duration) string {
	if i < 0 {
		return "-" + itos(0-i, d)
	}

	bps := float64(i) / float64(d) * float64(time.Second)
	var bpsStr string
	switch {
	case bps >= 1<<20:
		bpsStr = fmt.Sprintf("(%.3fMB/Sec)", bps/(1<<20))
	case bps >= 1<<10:
		bpsStr = fmt.Sprintf("(%.3fKB/Sec)", bps/(1<<10))
	default:
		bpsStr = fmt.Sprintf("(%.3fByte/Sec)", bps)
	}

	if i < 1<<10 {
		return fmt.Sprintf("%d Byte", i) + bpsStr
	}
	if i < 1<<20 {
		return fmt.Sprintf("%.3f KB", float64(i)/(1<<10)) + bpsStr
	}
	return fmt.Sprintf("%.3f MB", float64(i)/(1<<20)) + bpsStr

}
