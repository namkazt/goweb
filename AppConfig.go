package gocore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"github.com/chai2010/webp"
	"github.com/json-iterator/go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasttemplate"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewJSON() jsoniter.API {
	return jsoniter.Config{
		EscapeHTML:             true,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
		MarshalFloatWith6Digits:true,
		UseNumber: 				true,
	}.Froze()
}

var xlog *zerolog.Logger
func Log() *zerolog.Logger {
	if xlog == nil {
		logger := log.Logger.With().Stack().Logger()
		xlog = &logger
	}
	return xlog
}

func InitLogger() {
	currentTime := time.Now()
	timeName :=  time.Now().Format("01_02_2006")
	folderPath := "logs/" + timeName
	MakeSureDirExists(folderPath)
	filePath := fmt.Sprintf("logs/%s/log_%d.log", timeName, currentTime.Unix())
	ljLogger := &lumberjack.Logger{
		Filename:   filePath,
		MaxBackups: 10,			// files
		MaxSize:    5,			// megabytes
		MaxAge:     28,			// days
	}
	multiWriter := io.MultiWriter(
		zerolog.ConsoleWriter{
			Out: os.Stderr,
			NoColor: false,
			TimeFormat: time.RFC3339,
		},
		ljLogger,
	)
	log.Logger = zerolog.New(multiWriter).With().Timestamp().Logger()
	log.Logger.Info().
		Str("logDirectory", folderPath).
		Str("fileName", filePath).
		Int("maxSizeMB", ljLogger.MaxSize).
		Int("maxBackups", ljLogger.MaxBackups).
		Int("maxAgeInDays", ljLogger.MaxAge).
		Msg("logging configured")
}

func LogSeparetor() {
	Log().Debug().Msg("===============================================================")
}
func CustomEngineLog() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if !c.Echo().Debug {
				return next(c)
			}
			if c.Request().Method == http.MethodPost {
				le := Log().Debug()
				haveData := false
				for key, data := range c.Request().PostForm {
					le = le.Interface(key, data)
					haveData = true
				}
				if haveData {
					le.Msg("Post data")
				}else{
					le.Discard()
				}
			}
			if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodPost{
				le := Log().Debug()
				haveData := false
				list := c.Request().URL.Query()
				for key, data := range list {
					le = le.Interface(key, data)
					haveData = true
				}
				if haveData {
					le.Msg("Query data")
				}else{
					le.Discard()
				}
			}
			return next(c)
		}
	}
}

func DirectoryIsExists(location string, create bool) bool {
	_, err := os.Stat(location)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		if create {
			os.MkdirAll(location, os.ModePerm)
		}else {
			return false
		}
	}
	return false
}

func FileIsExists(location string) bool {
	_, err := os.Stat(location)
	return err == nil
}


const TL_API_letterBytes = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	TL_API_letterIdxBits = 6                    // 6 bits to represent a letter index
	TL_API_letterIdxMask = 1<<TL_API_letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	TL_API_letterIdxMax  = 63 / TL_API_letterIdxBits   // # of letter indices fitting in 63 bits
)
func GetUniqueCode(length int) string {
	//===============================================================================
	// We using this method:
	// source: https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang
	// NOTE:
	// If using it on multi-threads or goroutine better pass each thread/goroutine 1 Random source
	// or for simple just add Mutex lock here to lock source when using it.
	//===============================================================================
	var TL_API_SRC_RANDOM = rand.NewSource(time.Now().UnixNano())
	b := make([]byte, length)
	for i, cache, remain := length-1, TL_API_SRC_RANDOM.Int63(), TL_API_letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = TL_API_SRC_RANDOM.Int63(), TL_API_letterIdxMax
		}
		if idx := int(cache & TL_API_letterIdxMask); idx < len(TL_API_letterBytes) {
			b[i] = TL_API_letterBytes[idx]
			i--
		}
		cache >>= TL_API_letterIdxBits
		remain--
	}
	return string(b)
}

const TL_API_numberBytes = "0123456789"
func GetUniqueNumberCode(length int) string {
	b := make([]byte, length)
	var TL_API_SRC_RANDOM = rand.NewSource(time.Now().UnixNano())
	for i, cache, remain := length-1, TL_API_SRC_RANDOM.Int63(), TL_API_letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = TL_API_SRC_RANDOM.Int63(), TL_API_letterIdxMax
		}
		if idx := int(cache & TL_API_letterIdxMask); idx < len(TL_API_numberBytes) {
			b[i] = TL_API_numberBytes[idx]
			i--
		}
		cache >>= TL_API_letterIdxBits
		remain--
	}
	return string(b)
}


func MD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func IsDirectoryExisted(location string) bool {
	_, err := os.Stat(location)
	if err == nil {return true}
	if os.IsNotExist(err) {return false}
	return true
}


func MakeSureDirExists(location string) {
	loc := location
	if loc[len(loc) - 1:] == "/" {
		loc = loc [:len(loc) - 1]
	}
	if !IsDirectoryExisted(loc) {
		err := os.MkdirAll(loc, os.ModePerm)
		if err != nil {
			Log().Fatal().Err(err)
		}
	}
}

func DownloadFileUrl(url string, savePath string) bool {
	if FileIsExists(savePath) {
		Log().Error().Msg("Failed to download file: " +savePath)
		return false
	}
	//-------------------------------------------------------------------------
	output, err := os.Create(savePath)
	if err != nil {
		Log().Error().Err(err).Msg("Failed to create file at location: " + savePath)
		return false
	}
	defer output.Close()
	//-------------------------------------------------------------------------
	response, err := http.Get(url)
	if err != nil {
		Log().Error().Err(err).Msg("Failed while download file at url: " + url)
		return false
	}
	defer response.Body.Close()
	//-------------------------------------------------------------------------
	fileSize, err := io.Copy(output, response.Body)
	if err != nil {
		Log().Error().Err(err).Msg("Failed while save file at url: " + savePath)
		return false
	}
	//-------------------------------------------------------------------------
	Log().Info().Str("url", url).Int64("size", fileSize).Msg("Downloaded file successfully")
	return true
}

func SliceIndex(limit int, predicate func(i int) bool) int {
	for i := 0; i < limit; i++ {
		if predicate(i) {
			return i
		}
	}
	return -1
}

func FileExtension(path string) string{
	return strings.ToLower(path[strings.LastIndex(path, ".") + 1:len(path)])
}

func OpenImage(path string) (image.Image, bool) {
	file, _ := os.Open(path)
	defer file.Close()
	var imgData image.Image
	extStr := FileExtension(path)
	if extStr == "jpg" || extStr == "jpeg" {
		imgData, _ = jpeg.Decode(file)
	}else if extStr == "png" {
		imgData, _ = png.Decode(file)
	}else if extStr == "webp" {
		imgData, _ = webp.Decode(file)
	}else if extStr == "bmp" {
		imgData, _ = bmp.Decode(file)
	}else if extStr == "tiff" {
		imgData, _ = tiff.Decode(file)
	}else{
		return imgData, false
	}
	return imgData, true
}


func SaveImage(file *os.File, image *image.Image) bool {
	extStr := FileExtension(file.Name())
	if extStr == "jpg" || extStr == "jpeg" {
		err := jpeg.Encode(file, *image, nil)
		if err != nil { return false }
	}else if extStr == "png" {
		err := png.Encode(file, *image)
		if err != nil { return false }
	}else if extStr == "webp" {
		err := webp.Encode(file, *image, nil)
		if err != nil { return false }
	}else if extStr == "bmp" {
		err := bmp.Encode(file, *image)
		if err != nil { return false }
	}else if extStr == "tiff" {
		err := tiff.Encode(file, *image, nil)
		if err != nil { return false }
	}else{
		return false
	}
	return true
}

//------------------------------------------------------------
// Encrypt/decrypt
//------------------------------------------------------------
// key : 32 bytes
// nonce : 12 bytes
func EncryptSHA256String(key []byte, text []byte) ([]byte, []byte){
	block, err := aes.NewCipher(key)
	if err != nil {
		Log().Error().Err(err).Msg("Encrypt - Error when encrypt string")
		return nil, nil
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(crand.Reader, nonce); err != nil {
		Log().Error().Err(err).Msg("Encrypt - Error when create nonce")
		return nil, nil
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		Log().Error().Err(err).Msg("Encrypt - Error when create new GCM block")
		return nil, nil
	}
	ciphertext := aesgcm.Seal(nil, nonce, text, nil)
	return ciphertext, nonce
}

func DecryptSHA256String(key []byte, nonce []byte, ciphertext []byte) string {
	block, err := aes.NewCipher(key)
	if err != nil {
		Log().Error().Err(err).Msg("Decrypt - Error when decrypt string")
		return ""
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		Log().Error().Err(err).Msg("Decrypt - Error when create new GCM block")
		return ""
	}
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		Log().Error().Err(err).Msg("Decrypt - Error when decrypt string")
		return ""
	}
	return string(plaintext)
}

func ToHexString(input []byte) string {
	dst := make([]byte, hex.EncodedLen(len(input)))
	hex.Encode(dst, input)
	return string(dst)
}


// input:
// @key: cipher key
// @data: input text
// return: hex string encrypted
func EncryptAES(key string, data string) (string, error){
	cipherKey := []byte(key)
	plainText := []byte(data)

	block, err := aes.NewCipher(cipherKey)
	if err != nil {
		return "", err
	}
	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	cipherText := make([]byte, aes.BlockSize + len(plainText))
	iv := cipherText[:aes.BlockSize]
	if _, err = io.ReadFull(crand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plainText)

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func DecryptAES(key string, data string) (string, error) {
	cipherKey := []byte(key)
	cipherText, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(cipherKey)
	if err != nil {
		return "", err
	}

	if len(cipherText) < aes.BlockSize {
		err = errors.New(fmt.Sprintf("Cipher text block size small than BlockSize: %d", aes.BlockSize))
		return "", err
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(cipherText, cipherText)

	return string(cipherText), nil
}

func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		Log().Error().Err(err).Msg("Error when try to dial")
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}




type (
	// LoggerConfig defines the config for Logger middleware.
	LoggerConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper  middleware.Skipper
		
		// Tags to construct the logger format.
		//
		// - time_unix
		// - time_unix_nano
		// - time_rfc3339
		// - time_rfc3339_nano
		// - time_custom
		// - id (Request ID)
		// - remote_ip
		// - uri
		// - host
		// - method
		// - path
		// - protocol
		// - referer
		// - user_agent
		// - status
		// - error
		// - latency (In nanoseconds)
		// - latency_human (Human readable)
		// - bytes_in (Bytes received)
		// - bytes_out (Bytes sent)
		// - header:<NAME>
		// - query:<NAME>
		// - form:<NAME>
		//
		// Example "${remote_ip} ${status}"
		//
		// Optional. Default value DefaultLoggerConfig.Format.
		Format string `yaml:"format"`
		
		// Optional. Default value DefaultLoggerConfig.CustomTimeFormat.
		CustomTimeFormat string `yaml:"custom_time_format"`
		
		// Output is a writer where logs in JSON format are written.
		// Optional. Default value os.Stdout.
		Output io.Writer
		
		template *fasttemplate.Template
		colorer  *color.Color
		pool     *sync.Pool
	}
)

var (
	// DefaultLoggerConfig is the default Logger middleware config.
	DefaultLoggerConfig = LoggerConfig{
		Skipper: middleware.DefaultSkipper,
		Format: `{"time":"${time_rfc3339_nano}","id":"${id}","remote_ip":"${remote_ip}",` +
			`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
			`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02 15:04:05.00000",
		colorer:          color.New(),
	}
)

// Logger returns a middleware that logs HTTP requests.
func Logger() echo.MiddlewareFunc {
	return LoggerWithConfig(DefaultLoggerConfig)
}

// LoggerWithConfig returns a Logger middleware with config.
// See: `Logger()`.
func LoggerWithConfig(config LoggerConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultLoggerConfig.Skipper
	}
	if config.Format == "" {
		config.Format = DefaultLoggerConfig.Format
	}
	
	config.template = fasttemplate.New(config.Format, "${", "}")
	config.colorer = color.New()
	config.colorer.SetOutput(config.Output)
	config.pool = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}
	
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if config.Skipper(c) {
				return next(c)
			}
			
			req := c.Request()
			res := c.Response()
			start := time.Now()
			if err = next(c); err != nil {
				c.Error(err)
			}
			stop := time.Now()
			buf := config.pool.Get().(*bytes.Buffer)
			buf.Reset()
			defer config.pool.Put(buf)
			
			dumplogger := Log().With()
			msg := "Request"
			
			if _, err = config.template.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
				switch tag {
				case "time_unix":
					dumplogger = dumplogger.Str("time", strconv.FormatInt(time.Now().Unix(), 10))
					return buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
				case "time_unix_nano":
					dumplogger = dumplogger.Str("time", strconv.FormatInt(time.Now().UnixNano(), 10))
					return buf.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
				case "time_rfc3339":
					dumplogger = dumplogger.Str("time", time.Now().Format(time.RFC3339))
					return buf.WriteString(time.Now().Format(time.RFC3339))
				case "time_rfc3339_nano":
					dumplogger = dumplogger.Str("time", time.Now().Format(time.RFC3339Nano))
					return buf.WriteString(time.Now().Format(time.RFC3339Nano))
				case "time_custom":
					dumplogger = dumplogger.Str("time", time.Now().Format(config.CustomTimeFormat))
					return buf.WriteString(time.Now().Format(config.CustomTimeFormat))
				case "id":
					id := req.Header.Get(echo.HeaderXRequestID)
					if id == "" {
						id = res.Header().Get(echo.HeaderXRequestID)
					}
					dumplogger = dumplogger.Str("id", id)
					return buf.WriteString(id)
				case "remote_ip":
					dumplogger = dumplogger.Str("ip", c.RealIP() + " / " + c.Request().RemoteAddr)
					return buf.WriteString(c.RealIP())
				case "host":
					dumplogger = dumplogger.Str("host", req.Host)
					return buf.WriteString(req.Host)
				case "uri":
					dumplogger = dumplogger.Str("uri", req.RequestURI)
					return buf.WriteString(req.RequestURI)
				case "method":
					dumplogger = dumplogger.Str("method", req.Method)
					return buf.WriteString(req.Method)
				case "path":
					p := req.URL.Path
					if p == "" {
						p = "/"
					}
					dumplogger = dumplogger.Str("path", p)
					return buf.WriteString(p)
				case "protocol":
					dumplogger = dumplogger.Str("proto", req.Proto)
					return buf.WriteString(req.Proto)
				case "referer":
					dumplogger = dumplogger.Str("ref", req.Referer())
					return buf.WriteString(req.Referer())
				case "user_agent":
					dumplogger = dumplogger.Str("agent", req.UserAgent())
					return buf.WriteString(req.UserAgent())
				case "status":
					n := res.Status
					s := config.colorer.Green(n)
					switch {
					case n >= 500:
						s = config.colorer.Red(n)
					case n >= 400:
						s = config.colorer.Yellow(n)
					case n >= 300:
						s = config.colorer.Cyan(n)
					}
					dumplogger = dumplogger.Str("status", s)
					return buf.WriteString(s)
				case "error":
					if err != nil {
						// Error may contain invalid JSON e.g. `"`
						b, _ := json.Marshal(err.Error())
						b = b[1 : len(b)-1]
						msg = "** Error **"
						// print details information when error happen
						return buf.Write(b)
					}
				case "latency":
					l := stop.Sub(start)
					dumplogger = dumplogger.Str("latency", strconv.FormatInt(int64(l), 10))
					return buf.WriteString(strconv.FormatInt(int64(l), 10))
				case "latency_human":
					dumplogger = dumplogger.Str("latency", stop.Sub(start).String())
					return buf.WriteString(stop.Sub(start).String())
				case "data_in_out":
					byteIn := req.Header.Get(echo.HeaderContentLength)
					if byteIn == "" { byteIn = "0" }
					byteOut := strconv.FormatInt(res.Size, 10)
					dumplogger = dumplogger.Str("data", fmt.Sprintf("(In: %sbytes | Out: %sbytes)", byteIn, byteOut))
					return buf.WriteString(stop.Sub(start).String())
				case "bytes_in":
					cl := req.Header.Get(echo.HeaderContentLength)
					if cl == "" {
						cl = "0"
					}
					dumplogger = dumplogger.Str("in", cl)
					return buf.WriteString(cl)
				case "bytes_out":
					dumplogger = dumplogger.Str("out", strconv.FormatInt(res.Size, 10))
					return buf.WriteString(strconv.FormatInt(res.Size, 10))
				default:
					switch {
					case strings.HasPrefix(tag, "header:"):
						dumplogger = dumplogger.Str("header:" + tag[7:], c.Request().Header.Get(tag[7:]))
						return buf.Write([]byte(c.Request().Header.Get(tag[7:])))
					case strings.HasPrefix(tag, "query:"):
						dumplogger = dumplogger.Str("query:" + tag[6:], c.QueryParam(tag[6:]))
						return buf.Write([]byte(c.QueryParam(tag[6:])))
					case strings.HasPrefix(tag, "form:"):
						dumplogger = dumplogger.Str("form:" + tag[5:], c.FormValue(tag[5:]))
						return buf.Write([]byte(c.FormValue(tag[5:])))
					case strings.HasPrefix(tag, "cookie:"):
						cookie, err := c.Cookie(tag[7:])
						if err == nil {
							dumplogger = dumplogger.Str("cookie:" + tag[7:], cookie.Value)
							return buf.Write([]byte(cookie.Value))
						}
					}
				}
				return 0, nil
			}); err != nil {
				return
			}
			
			localLogger := dumplogger.Logger()
			
			switch {
			case res.Status >= http.StatusBadRequest && res.Status < http.StatusInternalServerError:
				{
					localLogger.Warn().Msg(msg)
				}
			case res.Status >= http.StatusInternalServerError:
				{
					localLogger.Error().Msg(msg)
				}
			default:
				localLogger.Info().Msg(msg)
			}
			
			
			return
		}
	}
}
