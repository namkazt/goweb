package gocore

import (
	"fmt"
	"github.com/siddontang/go/ioutil2"
	"golang.org/x/net/context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io/ioutil"
	"os"
	"time"
)

var json = NewJSON()

type AppAPIBase struct {
	CurrentApp 					*App
	MongoDB 					*mongo.Database
	// ----------------------------------
	dbConfigs					*DBConfig
}

type iAppAPIBase interface {
	Initialize(withApp *App)
	//-------------------------------------------------------------------------------------
	// Init all your api need here
	//-------------------------------------------------------------------------------------
	ExtendInitialize()
}
func (this *AppAPIBase) ExtendInitialize() {}

type DBConfig struct {
	//-----------------------------------------
	// support 2 type for now ( set DBType )
	// 1: mssql 		: "sqlserver"
	// 2: postgres 		: "postgres"
	// 3: mongodb 		: "mongodb"
	//-----------------------------------------
	DBType 						string
	DBUserName					string
	DBPassword 					string
	DBServerIP 					string
	DBServerPort 				string
	DBName 						string
}

func (this*AppAPIBase) Initialize(withApp *App) {
	//-----------------------------------------------
	MakeSureDirExists("data/")
	if !this.InitConfigs() {
		Log().Panic().Msg("[Critical] Database config file can't load. Please check again before application can run")
		return
	}
	this.CurrentApp = withApp
	this.InitDatabase()
}

func (this*AppAPIBase) InitConfigs() bool{
	dbCfgPath := "data/database.cfg"
	this.dbConfigs = &DBConfig{}
	if !ioutil2.FileExists(dbCfgPath) {
		rawData, err := json.Marshal(this.dbConfigs)
		if err != nil {
			Log().Error().Msg("Error when create new server config.")
			return false
		}
		//------------------------------------------------------
		err = ioutil.WriteFile(dbCfgPath, rawData, os.ModePerm)
		if err != nil {
			Log().Error().Msg("Error when write new server config.")
			return false
		}
	}else{
		rawData, err := ioutil.ReadFile(dbCfgPath)
		if err != nil {
			Log().Error().Msg("Error when read server config.")
			// remove config file and init config file again
			os.Remove(dbCfgPath)
			return this.InitConfigs()
		}
		err = json.Unmarshal(rawData, this.dbConfigs)
		if err != nil {
			Log().Error().Msg("Error when parse server config.")
			// remove config file and init config file again
			os.Remove(dbCfgPath)
			return this.InitConfigs()
		}
	}
	return true
}

func (this *AppAPIBase) NewMongoDB(username, password, serverIP, dbName string) *mongo.Database{
	// generate connection string
	connectionString := fmt.Sprintf("mongodb://%s:%s@%s:27017/%s",
		username,
		password,
		serverIP,
		dbName,
	)
	// connect
	ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)
	client, err := mongo.NewClient(options.Client().ApplyURI(connectionString))
	if err != nil {
		return nil
	}
	err = client.Connect(ctx)
	if err != nil {
		return nil
	}
	db := client.Database(dbName)
	return db
}

func (this *AppAPIBase) InitDatabase() {
	//-----------------------------------------------------------------
	// generate connection string
	var connectionString string
	if this.dbConfigs.DBType == "mongodb" {
		connectionString = fmt.Sprintf("mongodb://%s:%s@%s:%s/%s",
			this.dbConfigs.DBUserName,
			this.dbConfigs.DBPassword,
			this.dbConfigs.DBServerIP,
			this.dbConfigs.DBServerPort,
			this.dbConfigs.DBName,
		)
	}else{
		Log().Error().Str("DBType", this.dbConfigs.DBType).Msg("We not support this type of database")
		return
	}
	//-----------------------------------------------------------------
	// connect
	ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)
	client, err := mongo.NewClient(options.Client().ApplyURI(connectionString))
	if err != nil {
		Log().Error().Err(err).Str("DBType", this.dbConfigs.DBType).Msg("Can't create client")
		Log().Info().Str("Connection String", connectionString)
		return
	}
	err = client.Connect(ctx)
	if err != nil {
		Log().Error().Str("DBType", this.dbConfigs.DBType).Msg("Can not connect to database.")
		Log().Info().Str("Connection String", connectionString)
		return
	}
	this.MongoDB = client.Database(this.dbConfigs.DBName)
	Log().Info().Str("type", this.dbConfigs.DBType).Msg("Connected successfully.")
	Log().Info().Str("IP", this.dbConfigs.DBServerIP).Str("port",this.dbConfigs.DBServerPort).
		Str("user", this.dbConfigs.DBUserName).Str("collection", this.dbConfigs.DBName)
}