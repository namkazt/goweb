package gocore

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gopkg.in/gomail.v2"
	"math/rand"
	"time"
)

type App struct {
	appManager								*AppManager
	//------------------------------------------------
	host									string
	name									string
	//------------------------------------------------
	// engine
	//------------------------------------------------
	engine									*echo.Echo
	//------------------------------------------------
	// modules
	//------------------------------------------------
	notification 							*AppNotification
	mailer									*AppMailer

	//------------------------------------------------
	// handlers and api
	//------------------------------------------------
	handlers								map[string]HandlerInterface
	validateToken							map[string]string
	api										iAppAPIBase
}

func (this *App) Init(am *AppManager, name string){
	rand.Seed(time.Now().UnixNano())

	this.name = name
	this.appManager = am
	this.validateToken = make(map[string]string)
	//================================================
	InitLogger()
	//================================================
	// init origin server\
	this.engine = echo.New()
	this.engine.Use(middleware.Recover())
	// Add a logger middleware, which:
	//   - Logs all requests, like a combined access and error log.
	//   - Logs to stdout.
	logConfig := DefaultLoggerConfig
	logConfig.Output = Log()
	logConfig.Format = `${remote_ip} ${data_in_out} | ${method}:${uri} | ${status} | ${latency_human} | ${error}`
	this.engine.Use(LoggerWithConfig(logConfig))
	//this.engine.Use(CustomEngineLog())
	//================================================
	// init storage
	this.handlers = make(map[string]HandlerInterface)
}

//================================================
// command middle use
//================================================
func (this *App) UseMiddleware(md echo.MiddlewareFunc) {
	this.engine.Use(md)
}

//================================================
// Notification server support
// Android : FCM
// iOS : APNs
//================================================
func (this *App) UseNotificationModule() {
	this.notification = &AppNotification{}
	this.notification.InitModule()
}
func (this *App) Notification() *AppNotification{
	return this.notification
}


//================================================
// Use app mailer
//================================================
func (this *App) UseAppMailer(config AppMailerConfig, daemon bool) {
	this.mailer = &AppMailer{}
	this.mailer.SetConfig(config)
	if daemon {
		this.mailer.StartDaemon()
	}
}
func (this *App) SendEmail(msg *gomail.Message) {
	if this.mailer != nil {
		this.mailer.SendEmail(msg)
	}
}


//================================================
// Sever static resources
//================================================
func (this *App) UseStaticResources(resources map[string]string) {
	for router, path := range resources {
		this.engine.Static(router, path)
	}
}

func (this *App) StaticFile(url string, path string) {
	this.engine.Static(url, path)
}

//================================================
// register router
//================================================
func (this *App) addSplashToURl(url string) string {
	if url == "" {
		url = "/"
	}else{
		if url[len(url) - 1] != '/'{
			url += "/"
		}
	}
	return url
}

func (this *App) ServeVueApp(url string, path string){
	// make sure url have splash at end
	url = this.addSplashToURl(url)
	this.engine.Use(middleware.Static(path))
}

func (this *App) RegisterRouter(){
	for i := range this.handlers {
		this.handlers[i].RegisterRouteGroup(this.engine)
	}
}

//================================================
// api instance of app
//================================================
func (this *App) SetAPI( v iAppAPIBase) {
	this.api = v
	this.api.Initialize(this)
	this.api.ExtendInitialize()
}
func (this *App) GetAPI() iAppAPIBase{
	return this.api
}
func (this *App) HostName() string {
	return this.host
}

func (this *App) AppName() string {
	return this.name
}

func (this *App) Echo() *echo.Echo {
	return this.engine
}
//================================================
// Handler and components
//================================================
func (this *App) AddHandler(h HandlerInterface, name string) {
	this.handlers[name] = h
}

func (this *App) GetHandler(name string) (HandlerInterface, bool) {
	h, found := this.handlers[name]
	return h, found
}
//================================================
// run app standalone ( not by app manager )
//================================================
func (this *App) Run(addr string, port string) {
	this.engine.Logger.Fatal(this.engine.Start(addr + ":" + port))
}

func (this *App) RunTLS(addr string, port string, certFile, keyFile interface{}) {
	this.engine.Logger.Fatal(this.engine.StartTLS(addr + ":" + port, certFile, keyFile))
}

func (this *App) RunAutoTLS(addr string, port string) {
	this.engine.Logger.Fatal(this.engine.StartAutoTLS(addr + ":" + port))
}
