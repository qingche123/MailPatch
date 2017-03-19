package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"strings"

	simplejson "github.com/bitly/go-simplejson"
)

const (
	// MAILPATCHJSON is the config file path
	MAILPATCHJSON string = "./mailpatch.json"
)

// Your server addr
var localServer string

// SMTP config
var emailSubject string
var emailSender string
var senderPasswd string
var smtpServerAddr string
var emailReceivers string

// sendEmail ---
func sendEmail(sender, password, smtpServer, to, subject, emailBody, mailType string) error {
	smtpSrv := strings.Split(smtpServer, ":")
	senderName := strings.Split(sender, "@")
	auth := smtp.PlainAuth("", sender, password, smtpSrv[0])

	var contentType string
	if mailType == "html" {
		contentType = "Content-Type: text/" + mailType + "; charset=UTF-8"
	} else {
		contentType = "Content-Type: text/plain" + "; charset=UTF-8"
	}

	email := []byte("To: " + to + "\r\nFrom: " + senderName[0] + "<" + sender + ">\r\nSubject: " +
		subject + "\r\n" + contentType + "\r\n\r\n" + emailBody)
	receiver := strings.Split(to, ";")

	err := smtp.SendMail(smtpServer, auth, sender, receiver, email)
	return err
}

// getPatch parse the notification and get the patch
func getPatch(receiveBytes []byte) (string, error) {
	//get the patchURL from the notification
	js, err := simplejson.NewJson(receiveBytes)
	if err != nil {
		fmt.Println("Simplejson NewJson error: ", err.Error())
	}

	patchURL := js.Get("pull_request").Get("patch_url").MustString()
	if 0 == len(patchURL) {
		return "", errors.New("Get patchURL error: ")
	}
	fmt.Println("PatchURL: ", patchURL)

	//get the patch byte the patchURL
	respPatch, err := http.Get(patchURL)
	if err != nil {
		fmt.Println("Get patch's content error: ", err.Error())
	}
	defer respPatch.Body.Close()

	patch, err := ioutil.ReadAll(respPatch.Body)
	if err != nil {
		fmt.Println("Parse response error: ", err.Error())
	}
	fmt.Println("Patch :\n", string(patch))

	return string(patch), nil
}

// mailPatch ---
func mailPatch(w http.ResponseWriter, req *http.Request) {
	fmt.Println("--------------------------------------------------------------------")
	fmt.Println("Received a notification...")
	fmt.Println("Req.ContentLength: ", req.ContentLength)

	//receive the notification from github
	receiveBytes := make([]byte, req.ContentLength)

	readSumLen := 0
	for readLen := 0; int64(readSumLen) < req.ContentLength; readSumLen += readLen {
		readLen, _ = req.Body.Read(receiveBytes[readSumLen:])
	}

	fmt.Println("Received DataLen: ", readSumLen)
	w.Write([]byte("Received!!!"))

	//get the patch's content
	patchContent, err := getPatch(receiveBytes)
	if err != nil {
		fmt.Println("GetPatch error!")
		fmt.Println(err)
	}

	//sendEmail of patch to
	err = sendEmail(emailSender, senderPasswd, smtpServerAddr, emailReceivers,
		emailSubject, patchContent, "txt")
	if err != nil {
		fmt.Println("SendEmail error!")
		fmt.Println(err)
	} else {
		fmt.Println("SendEmail success!")
	}
}

func loadConf(path string) error {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return errors.New("Open config file error")
	}

	conf, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.New("ioutil ReadAll error")
	}

	js, err := simplejson.NewJson(conf)
	if err != nil {
		fmt.Println("Simplejson NewJson error: ", err.Error())
		return err
	}

	localServer = js.Get("localServer").MustString()
	if 0 == len(localServer) {
		return errors.New("load config localServer error: ")
	}
	fmt.Println(localServer)
	emailSubject = js.Get("emailSubject").MustString()
	if 0 == len(emailSubject) {
		return errors.New("load config emailSubject error: ")
	}
	fmt.Println(emailSubject)
	emailSender = js.Get("emailSender").MustString()
	if 0 == len(emailSender) {
		return errors.New("load config emailSender error: ")
	}
	fmt.Println(emailSender)
	senderPasswd = js.Get("senderPasswd").MustString()
	if 0 == len(senderPasswd) {
		return errors.New("load config senderPasswd error: ")
	}
	fmt.Println(senderPasswd)
	smtpServerAddr = js.Get("smtpServerAddr").MustString()
	if 0 == len(smtpServerAddr) {
		return errors.New("load config smtpServerAddr error: ")
	}
	fmt.Println(smtpServerAddr)
	emailReceivers = js.Get("emailReceivers").MustString()
	if 0 == len(emailReceivers) {
		return errors.New("load config emailReceivers error: ")
	}
	fmt.Println(emailReceivers)
	return nil
}

func main() {
	err := loadConf(MAILPATCHJSON)
	if err != nil {
		fmt.Println("LoadConf error: ", err.Error())
	}

	fmt.Println("MailPatch Server Start!")

	http.HandleFunc("/mailPatch/", mailPatch)

	err = http.ListenAndServe(localServer, nil)
	if err != nil {
		fmt.Println("ListenAndServe error: ", err.Error())
	}
}
