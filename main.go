package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
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
var emailSender string
var emailSenderName string
var senderPasswd string
var smtpServerAddr string
var emailReceivers string
var emailTLSEnable bool

// Github private repository user
var username string
var secret string

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
func sendEmailUseTLS(sender, senderName, password, smtpServer, to, subject,
	emailBody, mailType string) error {

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
	email := []byte("To: " + to + "\r\nFrom: " + senderName + "<" + sender +
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
func sendEmail(sender, senderName, password, smtpServer, to, subject, emailBody,
	mailType string) error {
	smtpSrv := strings.Split(smtpServer, ":")
	auth := smtp.PlainAuth("", sender, password, smtpSrv[0])

	var contentType string
	if mailType == "html" {
		contentType = "Content-Type: text/" + mailType + "; charset=UTF-8"
	} else {
		contentType = "Content-Type: text/plain" + "; charset=UTF-8"
	}

	email := []byte("To: " + to + "\r\nFrom: " + senderName + "<" + sender +
		">\r\nSubject: " + subject + "\r\n" + contentType + "\r\n\r\n" + emailBody)
	receiver := strings.Split(to, ";")

	err := smtp.SendMail(smtpServer, auth, sender, receiver, email)
	return err
}

// getPatchFromURL get the patch from url
func getPatchFromURL(patchURL string) (string, error) {
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
	//logPrint("Patch :\n", string(patch))
	logPrint("Got the patch from public repository")
	return string(patch), nil
}

// getPubRepoPatch parse the notification and get the patch
func getPubRepoPatch(js *simplejson.Json) (string, error) {
	//get the patchURL from the notification
	patchURL := js.Get("pull_request").Get("patch_url").MustString()
	if 0 == len(patchURL) {
		logPrint("Get patchURL error: ")
		return "", errors.New("Can't find pull_request->patch_url")
	}
	logPrint("PatchURL: ", patchURL)

	return getPatchFromURL(patchURL)
}

// getPatchByCmd get the patch
func getPatchByCmd(commandName string, params []string) (string, bool) {
	patchContent := ""
	cmd := exec.Command(commandName, params...)

	logPrint("Execute: ", strings.Join(cmd.Args[2:], " "))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logPrint("StdoutPipe error: ", err.Error())
		return "", false
	}

	//Start execute the command, but it wonn't wait for the return
	cmd.Start()

	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil || io.EOF == err {
			break
		}
		patchContent += line
	}

	//Wait return the command's return code and release the resources
	err = cmd.Wait()
	if err != nil {
		logPrint("Cmd Wait error: ", err.Error())
		return "", false
	}
	return patchContent, true
}

// getPrvRepoPatch parse the notification and get the patch
func getPrvRepoPatch(js *simplejson.Json) (string, error) {
	//get the patchURL from the notification
	statusesURL := js.Get("pull_request").Get("statuses_url").MustString()
	if 0 == len(statusesURL) {
		logPrint("Get statusesURL error: ")
		return "", errors.New("Can't find pull_request->statuses_url")
	}
	patchURL := strings.Replace(statusesURL, "statuses", "commits", 1)
	//logPrint("PatchURL: ", patchURL)

	//downloadCmd := "curl -u \"qingche123:ggyyff1989\" -H \"Accept: application/vnd.github.patch\"
	// https://api.github.com/repos/qingche123/GoOnchainNTRU/commits/c7e709b5b0bf44f948c88fe8b629e7529bd78ac0"
	downloadCmd := "curl -u \"" + username + ":" + secret +
		"\" -H \"Accept: application/vnd.github.patch\" " + patchURL
	command := "/bin/bash"
	params := []string{"-c", downloadCmd}

	patch, ret := getPatchByCmd(command, params)
	if false == ret {
		return "", errors.New("getPatchByCmd error")
	}
	//logPrint("Patch :\n", patch)
	logPrint("Got the patch from private repository")
	return patch, nil
}

// getPatch get the patch from notification
func getPatch(receiveBytes []byte) (string, error) {
	js, err := simplejson.NewJson(receiveBytes)
	if err != nil {
		logPrint("Simplejson NewJson error: ", err.Error())
		return "", err
	}

	priv := js.Get("repository").Get("private").MustBool()
	if priv {
		logPrint("The repository is private")

		if 0 == len(secret) || 0 == len(username) {
			logPrint("Username or Secret error")
			return "", errors.New("Username or Secret error")
		}
		return getPrvRepoPatch(js)
	}

	logPrint("The repository is public")
	return getPubRepoPatch(js)
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

	//get patch's commit header as emailSubject
	commitTitleHeadIndex := strings.Index(patchContent, "[PATCH]")
	commitHead := patchContent[commitTitleHeadIndex:]
	commitTitleEndIndex := strings.Index(commitHead, "\n")
	emailSubject := commitHead[0:commitTitleEndIndex]

	//add the commiter email addr to the receivers
	commiterHead := strings.Split(patchContent, ">")
	commiter := strings.Split(commiterHead[0], "<")
	receivers := emailReceivers + ";" + commiter[1]
	logPrint("Receivers: ", receivers)

	//send email
	if emailTLSEnable {
		err = sendEmailUseTLS(emailSender, emailSenderName, senderPasswd, smtpServerAddr,
			receivers, emailSubject, patchContent, "txt")
	} else {
		err = sendEmail(emailSender, emailSenderName, senderPasswd, smtpServerAddr,
			receivers, emailSubject, patchContent, "txt")
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
	emailSender = js.Get("emailSender").MustString()
	if 0 == len(emailSender) {
		logFatal("load config emailSender error: ")
		return errors.New("load config emailSender error: ")
	}
	logPrint("emailSender:\t", emailSender)
	emailSenderName = js.Get("emailSenderName").MustString()
	if 0 == len(emailSenderName) {
		logFatal("load config emailSenderName error: ")
		return errors.New("load config emailSenderName error: ")
	}
	logPrint("emailSenderName:\t", emailSenderName)
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
	emailTLSEnable = js.Get("enableTLS").MustBool()
	if emailTLSEnable {
		logPrint("emailTLSEnable:\t", "True")
	} else {
		logPrint("emailTLSEnable:\t", "False")
	}

	username = js.Get("username").MustString()
	logPrint("github username:\t", username)

	secret = js.Get("secret").MustString()
	logPrint("github secret:\t", secret)

	return nil
}

func main() {
	log.Println("--------------------------------------------------------------------")
	file, err := os.Create("MailPatch.log")
	if err != nil {
		log.Fatalln("Fail to create MailPatch.log file!", err.Error())
	}
	logger = log.New(file, "", log.LstdFlags)

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
