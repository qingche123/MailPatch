package main

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
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

var logger *log.Logger

func logFatal(arg ...string) {
	log.Println(arg)
	logger.Fatalln(arg)
}

func logPrint(arg ...string) {
	log.Println(arg)
	logger.Println(arg)
}

// sendEmailUseTLS ---
func sendEmailUseTLS(sender, password, smtpServer, to, subject, emailBody,
	mailType string) error {
	senderName := strings.Split(sender, "@")

	//create smtp client
	conn, err := tls.Dial("tcp", smtpServer, nil)
	if err != nil {
		logPrint("Dialing Error:", err.Error())
		return err
	}

	smtpSrv := strings.Split(smtpServer, ":")
	client, err := smtp.NewClient(conn, smtpSrv[0])
	if err != nil {
		logPrint("Create smpt client error:", err.Error())
		return err
	}
	defer client.Close()

	auth := smtp.PlainAuth("", sender, password, smtpSrv[0])
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err = client.Auth(auth); err != nil {
				logPrint("Error during AUTH", err.Error())
				return err
			}
		}
	}

	if err = client.Mail(sender); err != nil {
		logPrint("Error during Mail", err.Error())
		return err
	}

	recepter := strings.Split(to, ";")
	for _, addr := range recepter {
		if err = client.Rcpt(addr); err != nil {
			logPrint("Error during Rcpt", err.Error())
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		logPrint("Error during Data", err.Error())
		return err
	}

	var contentType string
	if mailType == "html" {
		contentType = "Content-Type: text/" + mailType + "; charset=UTF-8"
	} else {
		contentType = "Content-Type: text/plain" + "; charset=UTF-8"
	}
	email := []byte("To: " + to + "\r\nFrom: " + senderName[0] + "<" + sender +
		">\r\nSubject: " + subject + "\r\n" + contentType + "\r\n\r\n" + emailBody)

	_, err = writer.Write(email)
	if err != nil {
		logPrint("Error during Write", err.Error())
		return err
	}

	err = writer.Close()
	if err != nil {
		logPrint("Error during Close", err.Error())
		return err
	}

	return client.Quit()
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

	email := []byte("To: " + to + "\r\nFrom: " + senderName[0] + "<" + sender +
		">\r\nSubject: " + subject + "\r\n" + contentType + "\r\n\r\n" + emailBody)
	receiver := strings.Split(to, ";")

	err := smtp.SendMail(smtpServer, auth, sender, receiver, email)
	return err
}

// getPatch parse the notification and get the patch
func getPatch(receiveBytes []byte) (string, error) {
	//get the patchURL from the notification
	js, err := simplejson.NewJson(receiveBytes)
	if err != nil {
		logPrint("Simplejson NewJson error: ", err.Error())
		return "", err
	}

	patchURL := js.Get("pull_request").Get("patch_url").MustString()
	if 0 == len(patchURL) {
		logPrint("Get patchURL error: ")
		return "", err
	}
	logPrint("PatchURL: ", patchURL)

	//get the patch byte the patchURL
	respPatch, err := http.Get(patchURL)
	if err != nil {
		logPrint("Get patch's content error: ", err.Error())
		return "", err
	}
	defer respPatch.Body.Close()

	patch, err := ioutil.ReadAll(respPatch.Body)
	if err != nil {
		logPrint("Parse response error: ", err.Error())
		return "", err
	}
	logPrint("Patch :\n", string(patch))

	return string(patch), nil
}

// mailPatch ---
func mailPatch(w http.ResponseWriter, req *http.Request) {
	logPrint("--------------------------------------------------------------------")
	logPrint("Received a notification...")
	logPrint("Req.ContentLength: ", strconv.Itoa(int(req.ContentLength)))
	if 0 >= req.ContentLength {
		logPrint("Req.ContentLength error!")
		return
	}

	//receive the notification from github
	receiveBytes := make([]byte, req.ContentLength)

	readSumLen := 0
	for readLen := 0; int64(readSumLen) < req.ContentLength; readSumLen += readLen {
		readLen, _ = req.Body.Read(receiveBytes[readSumLen:])
	}

	logPrint("Received DataLen: ", strconv.Itoa(int(readSumLen)))
	w.Write([]byte("Received!!!"))

	//get the patch's content
	patchContent, err := getPatch(receiveBytes)
	if err != nil {
		logPrint("GetPatch error!", err.Error())
		return
	}

	//add the commiter email addr to the receivers
	commiterHead := strings.Split(patchContent, ">")
	commiter := strings.Split(commiterHead[0], "<")
	receivers := emailReceivers + ";" + commiter[1]

	//send email
	if enableTLS {
		err = sendEmailUseTLS(emailSender, senderPasswd, smtpServerAddr, receivers,
			emailSubject, patchContent, "txt")
	} else {
		err = sendEmail(emailSender, senderPasswd, smtpServerAddr, receivers,
			emailSubject, patchContent, "txt")
	}
	if err != nil {
		logPrint("SendEmail error!", err.Error())
	} else {
		logPrint("SendEmail success!")
	}
}

func loadConf(path string) error {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		logFatal("Open config file error", err.Error())
	}

	conf, err := ioutil.ReadAll(f)
	if err != nil {
		logFatal("ioutil ReadAll error", err.Error())
	}

	js, err := simplejson.NewJson(conf)
	if err != nil {
		logFatal("Simplejson NewJson error: ", err.Error())
	}

	localServer = js.Get("localServer").MustString()
	if 0 == len(localServer) {
		logFatal("load config localServer error: ")
		return errors.New("load config localServer error: ")
	}
	logPrint("localServer:\t", localServer)
	emailSubject = js.Get("emailSubject").MustString()
	if 0 == len(emailSubject) {
		logFatal("load config emailSubject error: ")
		return errors.New("load config emailSubject error: ")
	}
	logPrint("emailSubject:\t", emailSubject)
	emailSender = js.Get("emailSender").MustString()
	if 0 == len(emailSender) {
		logFatal("load config emailSender error: ")
		return errors.New("load config emailSender error: ")
	}
	logPrint("emailSender:\t", emailSender)
	senderPasswd = js.Get("senderPasswd").MustString()
	if 0 == len(senderPasswd) {
		logFatal("load config senderPasswd error: ")
		return errors.New("load config senderPasswd error: ")
	}
	logPrint("senderPasswd:\t", senderPasswd)
	smtpServerAddr = js.Get("smtpServerAddr").MustString()
	if 0 == len(smtpServerAddr) {
		logFatal("load config smtpServerAddr error: ")
		return errors.New("load config smtpServerAddr error: ")
	}
	logPrint("smtpServerAddr:\t", smtpServerAddr)
	emailReceivers = js.Get("emailReceivers").MustString()
	if 0 == len(emailReceivers) {
		logFatal("load config emailReceivers error: ")
		return errors.New("load config emailReceivers error: ")
	}
	logPrint("emailReceivers:\t", emailReceivers)
	enableTLS = js.Get("enableTLS").MustBool()
	if enableTLS {
		logPrint("TLS: ", "TLS enable")
	} else {
		logPrint("TLS: ", "TLS disable")
	}

	return nil
}

func main() {
	log.Println("--------------------------------------------------------------------")
	file, err := os.Create("MailPatch.log")
	if err != nil {
		log.Println("Fail to create MailPatch.log file!", err.Error())
		log.Fatalln("Fail to create MailPatch.log file!", err.Error())
	}
	logger = log.New(file, "", log.LstdFlags|log.Llongfile)

	err = loadConf(MAILPATCHJSON)
	if err != nil {
		logFatal("LoadConf error: ", err.Error())
	}

	logPrint("MailPatch Server Start!")

	http.HandleFunc("/mailPatch/", mailPatch)

	err = http.ListenAndServe(localServer, nil)
	if err != nil {
		logFatal("ListenAndServe error: ", err.Error())
	}
}
