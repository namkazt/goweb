package gocore

import (
	"crypto/ecdsa"
	"crypto/tls"
	"io/ioutil"
	"path/filepath"

	// android - FCM
	"github.com/NaySoftware/go-fcm"

	// iOS - APNs
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"sync"
)

type AppNotification struct {
	// list of client
	clients 							map[string]*NotificationClient
	// config object loaded from file
	config 								NotificationConfig
}

type NotificationClient struct{
	Platform 							int
	Mutex 								*sync.Mutex
	// android
	SenderID							string

	// iOS
	AppBundleID 						string

	androidClient 						*fcm.FcmClient
	iOSClient							*apns2.Client
}


type NotificationConfig struct {
	AndroidList							[]FCMConfig
	IOSList 							[]APNsConfig
}

type NotificationMessage struct {
	Title 								string
	Body 								string
	PayloadData 						interface{}
	Tokens 								[]string
	Topic 								string
}

const (
	PLATFORM_APNs = 1
	PLATFORM_FCM = 2
)

func (this *AppNotification) InitModule() bool{
	this.clients = make(map[string]*NotificationClient)
	// load from config file
	if !this.loadConfig() {
		return false
	}
	Log().Info().Str("module", "AppNotification").Msg("Init Notification module successfully")
	return true
}

func (this *AppNotification) loadConfig() bool{
	notificationConfigFilePath := "data/notification.cfg"
	if !FileIsExists(notificationConfigFilePath) {
		Log().Error().Str("module", "AppNotification").Msg("Fail to load config. Please make sure `data/notification.cfg` existed")
		return false
	}
	// read raw data
	rawData, err := ioutil.ReadFile(notificationConfigFilePath)
	if err != nil {
		Log().Error().Str("module", "AppNotification").Msg("Error when read notification config >>> remove file")
		return false
	}
	// parse notification
	err = json.Unmarshal(rawData, &this.config)
	if err != nil {
		Log().Error().Str("module", "AppNotification").Msg("Error when read notification config >>> remove file")
		return false
	}
	// init notification client
	for _, android := range this.config.AndroidList {
		androidClient := this.FCMInitFromConfig(&android)
		client := &NotificationClient{
			Platform: PLATFORM_FCM,
			Mutex: &sync.Mutex{},
			androidClient: androidClient,
			SenderID: android.ClientID,
		}
		this.clients["FCM_" + android.ID] = client
	}
	for _, iOS := range this.config.IOSList {
		iOSClient := this.APNsInitFromConfig(&iOS)
		client := &NotificationClient{
			Platform: PLATFORM_APNs,
			Mutex: &sync.Mutex{},
			iOSClient: iOSClient,
			AppBundleID: iOS.AppBundleID,
		}
		this.clients["APN_" + iOS.ID] = client
	}

	return true
}

func (this *AppNotification) SendMessageForAll(msg *NotificationMessage) {
	for key, client := range this.clients {
		go this.SendMessage(client.Platform, key,  msg)
	}
}

func (this *AppNotification) SendMessageForAndroid(msg *NotificationMessage) {
	for key, client := range this.clients {
		if client.Platform == PLATFORM_FCM {
			go this.SendMessage(client.Platform, key, msg)
		}
	}
}

func (this *AppNotification) SendMessageForIOS(msg *NotificationMessage) {
	for key, client := range this.clients {
		if client.Platform == PLATFORM_APNs {
			go this.SendMessage(client.Platform, key,  msg)
		}
	}
}

func (this *AppNotification) SendMessage(platform int, clientID string, msg *NotificationMessage) {
	if client, found := this.clients[clientID]; found {
		switch client.Platform {
		case PLATFORM_FCM:
			this._sendFCM(client, msg)
			break
		case PLATFORM_APNs:
			this._sendAPNs(client, msg)
			break
		default:
			Log().Error().Str("module", "AppNotification").Int("platform", platform).Msg("Unsupported platform ID")
			break
		}
	}else{
		Log().Error().Str("module", "AppNotification").Str("client", clientID).Msg("Client not found")
	}
}

func(this*AppNotification) _sendAPNs(client *NotificationClient, msg *NotificationMessage) {
	client.Mutex.Lock()
	// create payload data
	pData := payload.NewPayload()
	pData.AlertTitle(msg.Title)
	pData.AlertBody(msg.Body)
	pData.MutableContent()
	pData.Category(msg.Topic)
	msgData := msg.PayloadData.(map[string]interface{})
	for k, v := range msgData {
		pData.Custom(k, v)
	}
	// setup notification
	notification := &apns2.Notification{
		DeviceToken: "",
		Payload: pData,
		Topic: client.AppBundleID,
	}

	// send
	for _, deviceToken := range msg.Tokens {
		notification.DeviceToken = deviceToken
		status, err := client.iOSClient.Push(notification)
		if err != nil {
			Log().Error().Str("module", "AppNotification").Str("platform", "APNs").Err(err)
		}else{
			if status.Sent() {
				Log().Info().Str("module", "AppNotification").Str("platform", "APNs").Msg("Sent successfully")
			}else{
				Log().Info().Str("module", "AppNotification").Str("platform", "APNs").
					Int("status", status.StatusCode).Str("APNsID", status.ApnsID).Str("reason", status.Reason).Msg("Sent failed")
			}
		}
	}
	client.Mutex.Unlock()
}

func(this*AppNotification) _sendFCM(client *NotificationClient, msg *NotificationMessage) {
	client.Mutex.Lock()
	// prepare and send message
	client.androidClient.NewFcmRegIdsMsg(msg.Tokens, msg.PayloadData)
	client.androidClient.SetNotificationPayload(&fcm.NotificationPayload{
		Title: msg.Title,
		Body: msg.Body,
		//TODO: if support for fcm APNs then need set other field here
	})
	status, err := client.androidClient.Send()
	if err == nil {
		Log().Info().Str("module", "AppNotification").Str("platform", "FCM").
			Int("Status Code", status.StatusCode).Int("Success", status.Success).
			Int("Fail", status.Fail).Int("Canonical_ids", status.Canonical_ids).Str("topic error", status.Err)
	}else{
		Log().Error().Str("platform", "FCM").Err(err)
	}
	client.Mutex.Unlock()
}

//=================================================================
// Android service
// - Support server key
//=================================================================
type FCMConfig struct {
	ID 					string
	ServerKey 			string
	ClientID			string
}

func (this *AppNotification) FCMInitFromConfig(config *FCMConfig) *fcm.FcmClient{
	if config.ServerKey == "" {
		Log().Error().Str("platform", "FCM").Msg("Error when init FCM client from config! Server key was empty please check again.")
		return nil
	}
	client := fcm.NewFcmClient(config.ServerKey)
	return client
}



//=================================================================
// iOS service
// - Support p12, pem, p8 key
//=================================================================
type APNsConfig struct {
	ID 					string

	// path of key file
	KeyFilePath 		string

	// not support for now
	KeyBase64 			string
	KeyBase64Type 		string

	// password for .p12 and .pem
	Password 			string

	// ID for .p8
	KeyID 				string
	TeamID 				string

	// type
	IsProduction 		bool

	AppBundleID			string
}

func (this *AppNotification) APNsInitFromConfig(config *APNsConfig) *apns2.Client{
	if !FileIsExists(config.KeyFilePath) {
		Log().Error().Str("platform", "APNs").Msg("Init APNs failed. Key path was incorrect or key not found.")
		return nil
	}

	ext := filepath.Ext(config.KeyFilePath)
	switch ext {
	case ".p12":
		certificateKey, err := certificate.FromP12File(config.KeyFilePath, config.Password)
		if err != nil {
			Log().Error().Str("platform", "APNs").Err(err).Msg("Error when create certificate, Please check key file is correct or not")
			return nil
		}
		return this.apns_NewClient_P12_Pem(certificateKey, config)
		break
	case ".pem":
		certificateKey, err := certificate.FromPemFile(config.KeyFilePath, config.Password)
		if err != nil {
			Log().Error().Str("platform", "APNs").Err(err).Msg("Error when create certificate, Please check key file is correct or not")
			return nil
		}
		return this.apns_NewClient_P12_Pem(certificateKey, config)
		break
	case ".p8":
		authKey, err := token.AuthKeyFromFile(config.KeyFilePath)
		if err != nil {
			Log().Error().Str("platform", "APNs").Err(err).Msg("Error when create authentication key, Please check key file is correct or not")
			return nil
		}
		return this.apns_NewClient_P8(authKey, config)
	default:
		Log().Error().Str("platform", "APNs").Msg("Init APNs failed. Key was invalid. APNs only support for .p12, .pem, .p8")
		break
	}
	return nil
}

func (this *AppNotification) apns_NewClient_P12_Pem(cer tls.Certificate, config *APNsConfig) *apns2.Client{
	if config.IsProduction {
		return apns2.NewClient(cer).Production()
	}else{
		return apns2.NewClient(cer).Development()
	}
}

func (this *AppNotification) apns_NewClient_P8(authKey *ecdsa.PrivateKey, config *APNsConfig) *apns2.Client{
	// init token
	token := &token.Token{
		AuthKey: authKey,
		// KeyID from developer account (Certificates, Identifiers & Profiles -> Keys)
		KeyID: config.KeyID,
		// TeamID from developer account (View Account -> Membership)
		TeamID: config.TeamID,
	}
	// create client
	if config.IsProduction {
		return apns2.NewTokenClient(token).Production()
	}else{
		return apns2.NewTokenClient(token).Development()
	}
}