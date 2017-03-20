# MailPatch
When you create a pull request to your repo on github, MailPatch can mail the patch to your team's mailbox.

# Config
The configuration file of "mailpatch.json" is like follow:

{
	"localServer": "xxx.xxx.xxx.xxx:4567",
	"emailSubject": "Patch From GitHub",
	"emailSender": "xxx@sina.com",
	"senderPasswd": "xxxxxx",
	"smtpServerAddr": "smtp.sina.com:25",
	"emailReceivers": "xxx1@xxxx.com;xxx2@xxxx.com",
	"enableTLS": true
}

Notice that "localServer" must be an IP which have a public address

You must also config the notification address in your repo on github. Your repo->Settings-> Webhooks->
Add webhook, then fill the Payload URL like this "http://111.111.111.111:4567/mailPatch/". Remember to 
replace the ip with your "localServer".


MailPatch only support public repo on github now.