package gocore

import (
	"crypto/tls"
	"gopkg.in/gomail.v2"
	"io/ioutil"
	"time"
)

type AppMailer struct {
	isDaemon				bool
	mailSignal 				chan *gomail.Message
	config 					AppMailerConfig
	dialer 					*gomail.Dialer
}
type AppMailerConfig struct {
	// Host represents the host of the SMTP server.
	Host 					string

	// Port represents the port of the SMTP server.
	Port 					int

	// Username is the username to use to authenticate to the SMTP server.
	UserName 				string

	// Password is the password to use to authenticate to the SMTP server.
	Password 				string

	// TSLConfig represents the TLS configuration used for the TLS (when the
	// STARTTLS extension is used) or SSL connection.
	// Set: nil if do not use
	TLS 					*tls.Config
}

func (this *AppMailer) SetConfig(_config AppMailerConfig) {
	this.config = _config
	this.dialer = &gomail.Dialer{
		Host: this.config.Host,
		Port: this.config.Port,
		SSL:  false,
	}
	if this.config.TLS != nil {
		this.dialer.TLSConfig = this.config.TLS
	} else {
		this.dialer.Username = this.config.UserName
		this.dialer.Password = this.config.Password
	}
}

func (this *AppMailer) SendEmail(msg *gomail.Message){
	if this.isDaemon {
		this.mailSignal <- msg
	}else{
		if err := this.dialer.DialAndSend(msg); err != nil {
			Log().Error().Err(err).Str("module", "App Mailer")
		}
	}
}

func (this *AppMailer) GetMailContentFromTemplate(path string) string{
	content, err := ioutil.ReadFile("resources/templates/emails/" + path)
	if err != nil {
		Log().Error().Err(err).Str("module", "App Mailer")
		return ""
	}
	return string(content)
}

func (this* AppMailer) StartDaemon() {
	this.isDaemon = true
	go this.daemonRunner()
}

func (this* AppMailer) daemonRunner() {
	d := gomail.NewDialer(this.config.Host, this.config.Port, this.config.UserName, this.config.Password)

	var s gomail.SendCloser
	var err error
	open := false
	for {
		select {
		case m, ok := <- this.mailSignal:
			if !ok {
				this.isDaemon = false
				return
			}
			if !open {
				if s, err = d.Dial(); err != nil {
					Log().Error().Err(err).Str("module", "App Mailer")
					this.isDaemon = false
					return
				}
				open = true
			}
			if err := gomail.Send(s, m); err != nil {
				Log().Error().Err(err).Str("module", "App Mailer")
			}
		// Close connection to SMTP server if no email was sent in last 1 minutes
		case <- time.After(1 * time.Minute):
			if open {
				if err := s.Close(); err != nil {
					Log().Error().Err(err).Str("module", "App Mailer")
					this.isDaemon = false
					return
				}
				open = false
			}
		}
	}
}