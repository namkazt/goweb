package gocore

import (
	"github.com/go-redis/redis/v7"
	"time"
)

const (
	KEY_AUTHENTICAITON = "k_auth|"

	RESULT_SUCCESS = 0
	RESULT_FAILED = -1
)


var _sharedRedis *TokkorRedisCommon
type TokkorRedisCommon struct {
	app 						*AppRedis
}

func TokkorRedis() *TokkorRedisCommon{
	if _sharedRedis == nil {
		_sharedRedis = &TokkorRedisCommon{}
	}
	return _sharedRedis
}

func (this* TokkorRedisCommon) Setup(address string, password string){
	this.app = NewRedisApp(address, password)
}


func (this* TokkorRedisCommon) App() *AppRedis {
	return this.app
}

func (this* TokkorRedisCommon) Do() *redis.Client {
	return this.app.Client
}

func (this*TokkorRedisCommon) Publish(channel string, data interface{}) error {
	message, err := json.MarshalToString(&data)
	if err != nil {
		Log().Error().Err(err).Interface("data", data).Msg("Error when encode message")
		return err
	}
	err = this.Do().Publish(channel, message).Err()
	if err != nil {
		Log().Error().Err(err).Str("channel", channel).Msg("Error when publish message")
	}
	return err
}

//======================================================================================
// Authentication functions
//======================================================================================
func (this* TokkorRedisCommon) SetAuthentication(data *UserAuthData) bool{
	if data == nil {
		return false
	}
	ret, err := json.MarshalToString(data)
	if err != nil {
		Log().Error().Err(err).Msg("Error when Marschal to string")
		return false
	}
	err = this.Do().Set(KEY_AUTHENTICAITON + data.UserID, ret, 3 * time.Hour).Err()
	if err != nil {
		Log().Error().Err(err).Msg("Error when update to redis")
		return false
	}
	return true
}

func (this* TokkorRedisCommon) ValidateAuthentication(userID string, accessToken string, ip string) *UserAuthData {
	data, err := this.Do().Get(KEY_AUTHENTICAITON + userID).Bytes()
	if err != nil {
		return nil
	}
	var authData UserAuthData
	err = json.Unmarshal(data, &authData)
	if err != nil {
		Log().Error().Err(err).Str("data", string(data)).Msg("Error when unmarshal UserAuthData")
		return nil
	}
	if authData.UserID == userID && authData.AccessToken == accessToken{
		if ip != "" {
			if authData.IP == ip {
				return &authData
			}
		}else {
			return &authData
		}
	}
	return nil
}

func (this* TokkorRedisCommon) DeleteAuthentication(userID string) bool{
	err := this.Do().Del(KEY_AUTHENTICAITON + userID).Err()
	if err != nil {
		return false
	}
	return true
}

//======================================================================================
// Common structure for redis that we can share in other server
//======================================================================================

type UserAuthData struct {
	UserID 								string			`json:"id"`
	UserName 							string 			`json:"user_name"`
	Avatar 								string 			`json:"avatar"`
	AccessToken							string 			`json:"access_token"`
	IP 									string 			`json:"ip"`
}