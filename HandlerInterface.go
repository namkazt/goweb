package gocore

import (
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	
	"github.com/h2non/filetype"
	"github.com/labstack/echo/v4"
	"github.com/metal3d/go-slugify"
	
	"github.com/buckket/go-blurhash"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
)

type HandlerBase struct {
	Name 				string
	App 				*App
}

func (this *HandlerBase) InitHandler () {}
func (this *HandlerBase) RegisterRouteGroup(api *echo.Group) {}

func (this *HandlerBase) NotFound() *echo.HTTPError{
	return echo.NewHTTPError(http.StatusNotFound)
}

func (this *HandlerBase) ResultCode(c echo.Context, code int, message string, data interface{}) {
	ret := echo.Map{
		"code": code,
		"msg": message,
		"data": data,
	}
	c.JSON(200,ret )
	if c.Echo().Debug {
		dg, _ := json.MarshalToString(ret)
		Log().Debug().Str("data", dg).Msg("Request Response")
	}
}
func (this *HandlerBase) ResultFail(c echo.Context, message string, data interface{}) {
	ret := echo.Map{
		"code": -1,
		"msg": message,
		"data": data,
	}
	c.JSON(200,ret )
	if c.Echo().Debug {
		dg, _ := json.MarshalToString(ret)
		Log().Debug().Str("data", dg).Msg("Request Response")
	}
}
func (this *HandlerBase) ResultSuccess(c echo.Context, message string, data interface{}) {
	ret := echo.Map{
		"code": 1,
		"msg": message,
		"data": data,
	}
	c.JSON(200, ret )
	if c.Echo().Debug {
		dg, _ := json.MarshalToString(ret)
		Log().Debug().Str("data", dg).Msg("Request Response")
	}
}

//----------------------------------------------------------------------
// Resources helper
//----------------------------------------------------------------------
func(this*HandlerBase) Page(c echo.Context) (int, int) {
	page := this.PostInt(c, "page") - 1
	rows := this.PostInt(c, "rows")
	if rows <= 0 { rows = 5 }
	if page < 0 { page = 0 }
	return page, rows
}
func (this *HandlerBase) RemoveFile(filePath string) error {
	return os.Remove(filePath)
}
func (this *HandlerBase) RemoveFilesInFolder(folderPath string) error{
	d, err := os.Open(folderPath)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(folderPath, name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *HandlerBase)ValidateImage(head *[]byte) bool {
	return filetype.IsImage(*head)
}
func (this *HandlerBase)ValidateVideo(head *[]byte) bool {
	return filetype.IsVideo(*head)
}
func (this *HandlerBase)ValidateArchive(head *[]byte) bool {
	return filetype.IsArchive(*head)
}
func (this *HandlerBase)ValidateAudio(head *[]byte) bool {
	return filetype.IsAudio(*head)
}
func (this *HandlerBase)ValidateExtension(head *[]byte, extension string) bool {
	return filetype.IsExtension(*head, extension)
}

func (this *HandlerBase) GenerateBlurHash(path string, x, y int) string {
	imageData, success := OpenImage(path)
	if !success {
		return ""
	}
	str, err := blurhash.Encode(x, y, &imageData)
	if err != nil { return "" }
	return str
}

func (this *HandlerBase) PickMultipart(c echo.Context, name string, callback func (files []*multipart.FileHeader)) {
	if c.Request().MultipartForm != nil {
		if fileUploaded, found := c.Request().MultipartForm.File[name]; found {
			callback(fileUploaded)
		}
	}
}

// make sure saveLocation have not last /
func (this *HandlerBase) SaveFile(file *multipart.FileHeader, path string, filename string, timeNamed bool) string {
	MakeSureDirExists(path)
	timestamp := "_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	if !timeNamed {
		timestamp = ""
	}
	extStr := FileExtension(file.Filename)
	finalFileName := slugify.Marshal(filename) + timestamp + "." + extStr
	// write to location
	out, err := os.Create(path + finalFileName)
	if err != nil {
		Log().Error().Err(err).Str("path", path + finalFileName).Msg("Failed to create file at location")
		return ""
	}
	defer out.Close()
	tempFile, err := file.Open()
	if err != nil {
		Log().Error().Err(err).Str("path", path + finalFileName).Msg("Failed to open file")
		return ""
	}
	defer tempFile.Close()
	_, err = io.Copy(out, tempFile)
	if err != nil {
		Log().Error().Err(err)
		return ""
	}
	return finalFileName
}

func (this *HandlerBase) UploadedFile(formName string, saveLocation string, c echo.Context, isTypePassCallback func(head *[]byte) bool) string {
	MakeSureDirExists(saveLocation)
	// get request upload file
	file, header , err := c.Request().FormFile(formName)
	if err != nil {
		Log().Error().Err(err)
		return ""
	}
	// process file
	extIndex := strings.LastIndex(header.Filename,".")
	filename := ""
	extStr := ""
	if extIndex > 0 {
		filename = strings.ToUpper(header.Filename[0:extIndex])
		extStr = strings.ToUpper(header.Filename[extIndex+1:len(header.Filename)])
	}else {
		filename = strings.ToUpper(header.Filename)
	}
	// update final file name
	timestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	finalFileName := slugify.Marshal(filename) + "." + timestamp + "." + extStr
	if isTypePassCallback != nil {
		tempFile, _ := header.Open()
		// get header buffer
		headBuffer := make([]byte, 261)
		tempFile.Read(headBuffer)
		headBuffer = headBuffer[:261]
		// close file
		tempFile.Close()
		if !isTypePassCallback(&headBuffer) {
			return ""
		}
	}
	// write to location
	out, err := os.Create(saveLocation + finalFileName)
	if err != nil {
		Log().Error().Err(err)
		return ""
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		Log().Error().Err(err)
		return ""
	}
	return finalFileName
}

func (this *HandlerBase) MakeThumbnail(width int, height int, filePath string) string {
	imgData, success := OpenImage(filePath)
	var result image.Image
	//------------------------------------------------------------
	if !success {
		Log().Error().Str("path", filePath).Msg("Can't load image")
		return ""
	}
	//------------------------------------------------------------
	imgW := imgData.Bounds().Size().X
	imgH := imgData.Bounds().Size().Y
	if imgW >= imgH {
		result = resize.Resize (0, uint(height), imgData, resize.Lanczos3)
	}else{
		result = resize.Resize (uint(width), 0, imgData, resize.Lanczos3)
	}
	//-----------------------------------
	// after resize we crop it from center by height
	finalImg, _ := cutter.Crop(result, cutter.Config{
		Width: int(width),
		Height: int(height),
		Mode: cutter.Centered,
	})
	//-----------------------------------
	os.Remove(filePath)
	out, err := os.Create(filePath)
	defer out.Close()
	if err != nil {
		Log().Error().Err(err).Msg("Error when create new file")
		return ""
	}
	//----------------------------------
	// jpeg format only
	success = SaveImage(out, &finalImg)
	if !success {
		Log().Error().Err(err).Msg("Error when encode file")
		return ""
	}
	return filePath
}

//=============================================================================================================
// Some web helper functions
//=============================================================================================================
func (this *HandlerBase) IsNumber(input string) bool {
	_, err := strconv.Atoi(input)
	return err == nil
}

func (this *HandlerBase) PostTrim(c echo.Context, name string) string {
	return strings.Trim(c.FormValue(name), " ")
}
func (this *HandlerBase) PostBool(c echo.Context, name string) bool {
	raw := this.PostTrim(c, name)
	return raw == "true"
}
func (this *HandlerBase) PostTime(c echo.Context, name string, layout string) (time.Time, bool) {
	rawStr := strings.Trim(c.FormValue(name), " ")
	timeOut, err := time.Parse(layout, rawStr)
	if err != nil {
		Log().Error().Err(err).Str("raw", rawStr).Str("layout", layout).Msg("Error when convert time string")
		return timeOut, false
	}
	return timeOut, true
}
func (this *HandlerBase) PostMobile(c echo.Context, name string) (string, bool) {
	mobile := strings.Trim(c.FormValue(name), " ")
	mobile = strings.ReplaceAll(mobile, " ", "")
	return mobile, len(mobile) > 9 && len(mobile) < 12
}
func (this *HandlerBase) PostFloat(c echo.Context, name string) float32 {
	raw := this.PostTrim(c, name)
	result, err := strconv.ParseFloat(raw, 32)
	if err != nil {
		Log().Error().Err(err).Str("raw", raw).Str("name", name).Msg("Error to convert to float32")
	}
	return float32(result)
}
func (this *HandlerBase) PostInt(c echo.Context, name string) int {
	raw := this.PostTrim(c, name)
	result, err := strconv.Atoi(raw)
	if err != nil {
		Log().Error().Err(err).Str("raw", raw).Str("name", name).Msg("Error to convert to int")
	}
	return result
}
func (this *HandlerBase) GetInt(c echo.Context, name string) int {
	raw := c.QueryParam(name)
	result, err := strconv.Atoi(raw)
	if err != nil {
		Log().Error().Err(err).Str("raw", raw).Str("name", name).Msg("Error to convert to int")
	}
	return result
}
func (this *HandlerBase) ParamInt(c echo.Context, name string) int {
	raw := c.Param(name)
	result, err := strconv.Atoi(raw)
	if err != nil {
		Log().Error().Err(err).Str("raw", raw).Str("name", name).Msg("Error to convert to int")
	}
	return result
}

type HandlerInterface interface {
	RegisterRouteGroup(engine *echo.Echo)
}