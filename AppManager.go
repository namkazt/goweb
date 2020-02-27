package gocore

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"strings"
	"time"
)

type HostInstance struct {
	hostName 			[]string
	handler 			http.Handler
}
type HostSwitch map[string]HostInstance
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//----------------------------------------
	// Split host port in case
	domain := r.Host
	if dP := strings.Index(r.Host, ":"); dP >= 0 {
		domain = domain[:dP]
	}
	//----------------------------------------
	handled := false
	for key := range hs {
		if strContains(hs[key].hostName, domain) {
			hs[key].handler.ServeHTTP(w, r)
			handled = true
		}
	}
	if !handled {
		http.Error(w, "Forbidden", 403)
	}
	// ISSUE: Go HTTP: Too Many Open Files
	// URL: http://craigwickesser.com/2015/01/golang-http-to-many-open-files/
	// close connection after request ?
	r.Close = true
}
func strContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

type AppManager struct {
	//--------------------------------------------------
	// APP SETUP
	//--------------------------------------------------
	hosts							HostSwitch
	port 							string
	apps 							map[string]*App

	gfExist 						*AppGracefulExist
	//--------------------------------------------------
	// Profile
	//--------------------------------------------------
	heapProfileTimer 				*time.Ticker
	heapProfileCounter 				int
}

var AppManagerInstance *AppManager

func NewAppManager() *AppManager{
	appManager := &AppManager{}
	appManager.Init()
	AppManagerInstance = appManager
	return appManager
}

func (this* AppManager) Init() {
	this.hosts = make(HostSwitch)
	this.apps = make(map[string]*App)
	this.gfExist = NewGracefulExist()
	this.gfExist.Start()
}


func (this* AppManager) AddGracefulCallback(key string, callback func()) {
	this.gfExist.AddGracefulCallback(key, callback)
}

func (this* AppManager) RemoveGracefulCallback(key string) {
	this.gfExist.RemoveGracefulCallback(key)
}


func (this* AppManager) Profiling(heapCaptureDuration time.Duration) {
	// cleanup profile folder
	os.RemoveAll("profile")

	// start profile
	StartCPUProfile()
	this.AddGracefulCallback("profiling", func() {
		StopCPUProfile()
		// stop heap capture timer
		this.heapProfileTimer.Stop()
		// capture last end heap
		CaptureMemoryHeap("end")
	})
	// capture memory every X minutes
	if heapCaptureDuration.Minutes() < 1 {
		heapCaptureDuration = time.Minute
	}
	this.heapProfileTimer = time.NewTicker(heapCaptureDuration)
	this.heapProfileCounter = 0
	go func() {
		for {
			select {
			case <- this.heapProfileTimer.C:
				this.heapProfileCounter++
				CaptureMemoryHeap(fmt.Sprintf("c%d", this.heapProfileCounter))
				break
			}
		}
	}()
}


func (this *AppManager) GetAppByDomainKey(domain string) *App {
	for key := range this.hosts {
		if strContains(this.hosts[key].hostName, domain) {
			return this.apps[this.hosts[key].hostName[0]]
		}
	}
	return nil
}

func (this*AppManager) SetMode(mode string){
	Log().Info().Msg("[Info] App run with mode: " + mode)
	if mode == "DEBUG"{
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}else{
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

func (this* AppManager) RegisterApp(app *App, domains string) {
	if len(domains) == 0 {
		Log().Fatal().Msg("App have no ref domain. This app will not activate. Please input follow format: 127.0.0.1,localhost,... ")
		Log().Panic()
		return
	}
	domainList := strings.Split(domains, ",")
	//--------------------------------------------------
	host := HostInstance{}
	host.handler = app.engine
	host.hostName = domainList
	this.hosts[domainList[0]] = host
	//--------------------------------------------------
	this.apps[domainList[0]] = app
	app.host = domainList[0]
	Log().Info().Msg("Registered app for domains")
	log.Print(domainList)
}

func (this* AppManager) FormatHostPort(host string) string {
	if this.port == "80" {
		return "http://" + host
	}else{
		return "http://" + host + ":" + this.port
	}
}

func (this* AppManager) Run(port string) {
	this.port = port
	Log().Info().Str("port", port).Msg("Run App Manager")
	http.ListenAndServe(":" + port, this.hosts)
}

func (this* AppManager) RunTLS(port string, certFile string, keyFile string) {
	this.port = port
	Log().Info().Str("port", port).Msg("Run App Manager on TLS mode")
	http.ListenAndServeTLS(":" + port, certFile, keyFile, this.hosts)
}