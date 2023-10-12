package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gaozs/cfg"
	"github.com/gaozs/smtpauth"
)

var host string
var serverAddr string
var user string     //发送邮件的邮箱
var password string //发送邮件邮箱的密码
var sendTo string   //kindle推送邮箱
var boundary string

func init() {
	cfg.ParseIniFile(filepath.Dir(os.Args[0]) + string(os.PathSeparator) + "cfg.ini")
	if cfg.GetAttr("main", "version") == "" {
		panic("Error parse INI file")
	}
	host = cfg.GetAttr("main", "host")
	serverAddr = cfg.GetAttr("main", "serverAddr")
	user = cfg.GetAttr("main", "user")
	password = cfg.GetAttr("main", "password")
	sendTo = cfg.GetAttr("main", "sendTo")
	boundary = cfg.GetAttr("main", "boundary")
}

var client *smtp.Client
var outBuf *bytes.Buffer

type fileMsg struct {
	fName  string        // file name
	outBuf *bytes.Buffer // record log info
	msg    *bytes.Buffer // file mail msg
	err    error         // file generate error
}

var ch = make(chan error)                      // for get smtp init result
var msgCh = make(chan fileMsg, len(os.Args)-1) // msgs to be sent

func main() {
	defer func() {
		fmt.Println("All Done, press enter to exit.")
		fmt.Scanln()
	}()

	if len(os.Args) <= 1 {
		fmt.Println("No file to send!")
		return
	}

	go initSMTPClient()

	fmt.Println("Converting files...")
	go func() {
		for _, f := range os.Args[1:] {
			addFileMsg(f)
		}
		fmt.Println("All files are converted!")
	}()

	err := <-ch // check if smtp init complete
	if err != nil {
		fmt.Println(err)
		return
	}
	defer client.Close()

	sendMSgQueue()
}

// init smtp server client
func initSMTPClient() {
	var err error
	fmt.Println("connecting email server:", serverAddr)
	defer fmt.Println("connected email server:", serverAddr)

	for i := 0; i < 3; i++ {
		client, err = smtp.Dial(serverAddr)
		if err != nil {
			fmt.Println("err:", err)
			continue
		}

		if err = client.Hello(host); err != nil {
			fmt.Println("err:", err)
			continue
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			config := &tls.Config{ServerName: host}
			if err = client.StartTLS(config); err != nil {
				fmt.Println("err:", err)
				continue
			}
		}

		auth := smtpauth.LoginAuth(user, password)
		if err = client.Auth(auth); err != nil {
			fmt.Println("err:", err)
			continue
		}
		break
	}

	ch <- err
}

func addFileMsg(filename string) {
	var f fileMsg
	outBuf = new(bytes.Buffer)
	f.fName = filename // store file name
	f.outBuf = outBuf
	filename, f.err = processIfTxt(filename) // convert txt file to html file
	if f.err != nil {
		msgCh <- f
		return
	}
	f.msg, f.err = genFileMsg(filename)
	msgCh <- f

	if filename != f.fName {
		// remove tmp html file
		os.Remove(filename)
	}
}

func processIfTxt(filename string) (newFilename string, err error) {
	if strings.ToLower(filepath.Ext(filename)) != ".txt" {
		// not txt file, no need to process
		fmt.Fprintln(outBuf, "\nnot txt file, just send it!")
		newFilename = filename
		return
	}

	fmt.Fprintf(outBuf, "\n--------Covert txt file(%s) starting...--------\n", filename)
	defer fmt.Fprintf(outBuf, "\n--------Covert txt file(%s) ended!--------\n", filename)

	data, err := getTxtData(filename)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}

	baseName := filepath.Base(filename)
	baseName = baseName[:len(baseName)-4]

	htmlFilename := filepath.Join(filepath.Dir(filename), baseName+".html")
	err = genHTMLFile(htmlFilename, data)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}

	newFilename = htmlFilename

	return
}

func getTxtData(filename string) (data []byte, err error) {
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}
	defer f.Close()

	data, err = ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}
	// check if GBK txt file and convert
	isGBKFlag := isGBK(data)
	fmt.Fprintln(outBuf, "File is GBK?:", isGBKFlag)
	if isGBKFlag {
		fmt.Fprintln(outBuf, "convert GBK to utf8")
		data, err = GbkToUtf8(data)
		if err != nil {
			fmt.Fprintln(outBuf, err)
			return
		}
	}
	return
}

func genHTMLFile(htmlFilename string, data []byte) (err error) {
	fmt.Fprintln(outBuf, "Generating html File: ", htmlFilename)
	defer fmt.Fprintln(outBuf, "Generated html File: ", htmlFilename)

	w, err := os.Create(htmlFilename)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}
	defer w.Close()

	// write html header
	_, err = w.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta http-equiv=Content-Type content="text/html; charset=utf-8">
<style>
p { margin:0.3em -0.2em; padding:0em; text-indent:2em; line-height:1; }
</style>
</head>
<body>
`)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}

	// write txt file to html body
	buf := bytes.NewBuffer(data)
	var line string
	var eof error
	for {
		line, eof = buf.ReadString(0x0a) // read one line
		line = strings.TrimSpace(line)
		if line != "" {
			_, err = w.WriteString("<p>" + line + "</p>\n")
			if err != nil {
				fmt.Fprintln(outBuf, err)
				return
			}
		}
		if eof == io.EOF {
			break
		}
		if eof != nil {
			fmt.Fprintln(outBuf, eof)
			return eof
		}
	}

	// write html ending
	_, err = w.WriteString(`
</body>
</html>
`)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}

	return
}

func genFileMsg(filename string) (msg *bytes.Buffer, err error) {
	fmt.Fprintf(outBuf, "\n--------Generating file(%s) email msg...--------\n", filename)
	defer fmt.Fprintf(outBuf, "\n--------Generated file(%s) email msg!--------\n", filename)

	msg = &bytes.Buffer{}
	header := make(textproto.MIMEHeader)
	header.Set("To", "cloud<"+sendTo+">")
	header.Set("From", "gzs<"+user+">")
	//header.Set("Subject", "convert")
	header.Set("Subject", "pls send to my kindle")
	header.Set("Content-Type", "multipart/mixed;boundary="+boundary)
	headerToBytes(msg, header)

	msgWriter := multipart.NewWriter(msg)
	msgWriter.SetBoundary(boundary)

	bodyHeader := make(textproto.MIMEHeader)
	bodyHeader.Set("Content-Type", "text/plain;charset=UTF-8")
	bodyHeader.Set("Content-Transfer-Encoding", "base64")
	body, err := msgWriter.CreatePart(bodyHeader)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}

	//_, err = body.Write([]byte(mime.QEncoding.Encode("UTF-8", "\r\npls send this body to my kindle, thx!\r\n")))
	err = base64Wrap(body, bytes.NewBufferString("pls send this file to my kindle, thx!"))
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}

	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Type", "application/octet-stream;charset=UTF-8")
	//fileHeader.Set("Content-Disposition", "attachment; filename=\""+url.PathEscape(filepath.Base(filename))+"\"")
	fileHeader.Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filepath.Base(filename)))
	fileHeader.Set("Content-Transfer-Encoding", "base64")
	file, err := msgWriter.CreatePart(fileHeader)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}

	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}
	defer f.Close()

	err = base64Wrap(file, f)
	if err != nil {
		fmt.Fprintln(outBuf, err)
		return
	}
	msgWriter.Close()
	//fmt.Fprintf(outBuf, "%s", msg.Bytes())
	return
}

func sendMSgQueue() {
	for i := 0; i < len(os.Args)-1; i++ {
		f := <-msgCh
		f.outBuf.WriteTo(os.Stdout)
		if f.err != nil {
			fmt.Printf("\nProcess file(%s) with err(%s)\n", f.fName, f.err)
		} else {
			fmt.Printf("\nSending file(%s)....\n", f.fName)
			err := sendMsg(f.msg)
			if err != nil {
				fmt.Printf("Sending file(%s) with err(%s)\n", f.fName, err)
			}
		}
	}

}

func sendMsg(msg *bytes.Buffer) (err error) {
	// send email
	if err = client.Mail(user); err != nil {
		return
	}
	if err = client.Rcpt(sendTo); err != nil {
		return
	}
	w, err := client.Data()
	if err != nil {
		return
	}
	defer w.Close()

	buf := make([]byte, 64*1024)
	var n, wn, count int
	startTime := time.Now()
	for {
		n, err = msg.Read(buf)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		wn, err = w.Write(buf[:n])
		if err != nil {
			return
		}
		if wn != n {
			err = fmt.Errorf("sending num error(sent:%d | read:%d)", wn, n)
			return
		}
		count += n
		fmt.Printf("Sent %s...                 \r", itos(count, time.Since(startTime)))
	}
	fmt.Printf("\nSent done! Take %s.\n", time.Since(startTime).String())

	return
}

func headerToBytes(w io.Writer, header textproto.MIMEHeader) {
	for field, vals := range header {
		for _, subval := range vals {
			io.WriteString(w, field)
			io.WriteString(w, ":")
			switch {
			case field == "Content-Type" || field == "Content-Disposition":
				w.Write([]byte(subval))
			default:
				w.Write([]byte(mime.QEncoding.Encode("UTF-8", subval)))
			}
			io.WriteString(w, "\r\n")
		}
	}
	io.WriteString(w, "\r\n")
}
