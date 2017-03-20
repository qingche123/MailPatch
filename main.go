package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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
var enableTLS bool

// sendEmailUseTLS ---
func sendEmailUseTLS(sender, password, smtpServer, to, subject, emailBody, mailType string) error {
	senderName := strings.Split(sender, "@")

	//create smtp client
	conn, err := tls.Dial("tcp", smtpServer, nil)
	if err != nil {
		log.Println("Dialing Error:", err)
		return err
	}

	smtpSrv := strings.Split(smtpServer, ":")
	c, err := smtp.NewClient(conn, smtpSrv[0])
	if err != nil {
		log.Println("Create smpt client error:", err)
		return err
	}
	defer c.Close()

	auth := smtp.PlainAuth("", sender, password, smtpSrv[0])
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(auth); err != nil {
				log.Println("Error during AUTH", err)
				return err
			}
		}
	}

	if err = c.Mail(sender); err != nil {
		return err
	}

	recepter := strings.Split(to, ";")
	for _, addr := range recepter {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	var contentType string
	if mailType == "html" {
		contentType = "Content-Type: text/" + mailType + "; charset=UTF-8"
	} else {
		contentType = "Content-Type: text/plain" + "; charset=UTF-8"
	}
	email := []byte("To: " + to + "\r\nFrom: " + senderName[0] + "<" + sender + ">\r\nSubject: " +
		subject + "\r\n" + contentType + "\r\n\r\n" + emailBody)

	_, err = w.Write(email)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return c.Quit()
}

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

	//add the commiter email addr to the receivers
	commiterHead := strings.Split(patchContent, ">")
	commiter := strings.Split(commiterHead[0], "<")
	emailReceivers += ";"
	emailReceivers += commiter[1]

	//send email
	if enableTLS {
		err = sendEmailUseTLS(emailSender, senderPasswd, smtpServerAddr, emailReceivers,
			emailSubject, patchContent, "txt")
	} else {
		err = sendEmail(emailSender, senderPasswd, smtpServerAddr, emailReceivers,
			emailSubject, patchContent, "txt")
	}
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
	fmt.Println("localServer:\t", localServer)
	emailSubject = js.Get("emailSubject").MustString()
	if 0 == len(emailSubject) {
		return errors.New("load config emailSubject error: ")
	}
	fmt.Println("emailSubject:\t", emailSubject)
	emailSender = js.Get("emailSender").MustString()
	if 0 == len(emailSender) {
		return errors.New("load config emailSender error: ")
	}
	fmt.Println("emailSender:\t", emailSender)
	senderPasswd = js.Get("senderPasswd").MustString()
	if 0 == len(senderPasswd) {
		return errors.New("load config senderPasswd error: ")
	}
	fmt.Println("senderPasswd:\t", senderPasswd)
	smtpServerAddr = js.Get("smtpServerAddr").MustString()
	if 0 == len(smtpServerAddr) {
		return errors.New("load config smtpServerAddr error: ")
	}
	fmt.Println("smtpServerAddr:\t", smtpServerAddr)
	emailReceivers = js.Get("emailReceivers").MustString()
	if 0 == len(emailReceivers) {
		return errors.New("load config emailReceivers error: ")
	}
	fmt.Println("emailReceivers:\t", emailReceivers)
	enableTLS = js.Get("enableTLS").MustBool()
	if enableTLS {
		fmt.Println("TLS: ", "TLS enable")
	} else {
		fmt.Println("TLS: ", "TLS disable")
	}

	return nil
}

func main() {
	err := loadConf(MAILPATCHJSON)
	if err != nil {
		fmt.Println("LoadConf error: ", err.Error())
		return
	}

	fmt.Println("MailPatch Server Start!")

	http.HandleFunc("/mailPatch/", mailPatch)

	err = http.ListenAndServe(localServer, nil)
	if err != nil {
		fmt.Println("ListenAndServe error: ", err.Error())
	}
}
