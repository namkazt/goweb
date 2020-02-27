package gocore

import (
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"reflect"
	"strings"
)

type xDB struct {}
var DB = &xDB{}

func (db *xDB)ArrayStringtoID(list []string) []primitive.ObjectID{
	idList := make([]primitive.ObjectID, 0)
	for _, idStr := range list {
		idList = append(idList, DB.ObjectID(idStr))
	}
	return idList
}

// core.DB.func ?

func (db *xDB)WithSelect(a interface{}, b interface{}) interface{}{
	if a != nil {
		return a
	}else {
		return b
	}
}

func (db *xDB) ObjectID(hex string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		Log().Error().Err (err).Msg("Func: ObjectID")
	}
	return id
}

var _type_objectId = reflect.TypeOf(primitive.ObjectID{})
func (db *xDB) VerifyObjectId(Id interface{}) (bool, primitive.ObjectID, string){
	if reflect.TypeOf(Id).ConvertibleTo(_type_objectId) {
		objectId := Id.(primitive.ObjectID)
		return !objectId.IsZero(), objectId, objectId.Hex()
	}else if reflect.TypeOf(Id).Name() == "string" {
		stringId := Id.(string)
		return !db.ObjectID(stringId).IsZero(), db.ObjectID(stringId), stringId
	}
	return false, primitive.ObjectID{} , ""
}

func (db *xDB) Skip(number int) bson.D {
	return bson.D {{"$skip", number}}
}
func (db *xDB) Limit(number int) bson.D {
	return bson.D {{"$limit", number}}
}
func (db *xDB) Exists(val bool) bson.D {
	return bson.D {{"$exists", val}}
}
// order must be like : Sort("state DESC", "created_date ASC")
func (db *xDB) Sort(orders []string) bson.D {
	orderObj := make([]bson.E, 0)
	for _, order := range orders {
		r := strings.Split(order, " ")
		if len(r) < 2 {
			continue
		}
		orderInt := 1
		if r[1] == "DESC" { orderInt = -1 }
		orderObj = append(orderObj, bson.E{
			Key: r[0],
			Value: orderInt,
		})
	}
	return bson.D {{"$sort", bson.D(orderObj)}}
}

func (db *xDB) Lookup(collection string, localField string, foreignField string, as string) bson.D {
	return bson.D{
		{"$lookup", bson.D{
			{"from", collection},
			{"localField", localField},
			{"foreignField", foreignField},
			{"as", as},
		}},
	}
}

func (db *xDB) LookupPipeLine(collection string, let bson.D, pipeline bson.A, as string) bson.D {
	return bson.D{
		{"$lookup", bson.D{
			{"from", collection},
			{"let", let},
			{"pipeline", pipeline},
			{"as", as},
		}},
	}
}

func (db *xDB) Unwind2(path string, includeArrayIndex string, preserveNullAndEmptyArrays bool) bson.D {
	return bson.D {
		{"$unwind", bson.D{
			{"path", "$" + path},
			{"includeArrayIndex", includeArrayIndex},
			{"preserveNullAndEmptyArrays", preserveNullAndEmptyArrays},
		}},
	}
}

func (db *xDB) Unwind1(path string, preserveNullAndEmptyArrays bool) bson.D {
	return bson.D {
		{"$unwind", bson.D{
			{"path", "$" + path},
			{"preserveNullAndEmptyArrays", preserveNullAndEmptyArrays},
		}},
	}
}

func (db *xDB) Unwind(path string) bson.D {
	return bson.D {
		{"$unwind", "$" + path},
	}
}

func (db *xDB) Project(values ...bson.E) bson.D {
	input := bson.D{}
	input = append(input, values...)
	return bson.D {
		{"$project", input},
	}
}

func (db *xDB) Group(expr interface{}, fields ...bson.E) bson.D {
	input := bson.D{
		{"_id", expr},
	}
	input = append(input, fields...)
	return bson.D {
		{"$group", input},
	}
}

func (db *xDB) Expr(query bson.D) bson.D {
	return bson.D{
		{"$expr", query},
	}
}

func (db *xDB) Match(query bson.D) bson.D {
	return bson.D{
		{"$match", query},
	}
}
func (db *xDB) Pipe(stages ...bson.D) mongo.Pipeline {
	return mongo.Pipeline(stages)
}

func (db *xDB) NewD() bson.D {
	return bson.D{}
}
func (db *xDB) NewDArray() []bson.D {
	return []bson.D{}
}


func (db *xDB) NewID() primitive.ObjectID {
	return primitive.NewObjectID()
}

func (db *xDB) AddFields(fields bson.D) bson.D {
	return bson.D{
		{"$addFields", fields},
	}
}


func (db *xDB) Nor(value ...interface{}) bson.D {
	return bson.D{
		{"$nor", bson.A(value)},
	}
}
func (db *xDB) Not(value ...interface{}) bson.D {
	return bson.D{
		{"$not", bson.A(value)},
	}
}
func (db *xDB) Or(value ...interface{}) bson.D {
	return bson.D{
		{"$or", bson.A(value)},
	}
}
func (db *xDB) OrArr(value []interface{}) bson.D {
	return bson.D{
		{"$or", bson.A(value)},
	}
}

func (db *xDB) And(value ...interface{}) bson.D {
	return bson.D{
		{"$and", bson.A(value)},
	}
}

func (db *xDB) AndArr(value []interface{}) bson.D {
	return bson.D{
		{"$and", bson.A(value)},
	}
}

func (db *xDB) Equal(field string, value interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$eq", value}}},
	}
}
func (db *xDB) Greater(field string, value interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$gt", value}}},
	}
}
func (db *xDB) GreaterOrEqual(field string, value interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$gte", value}}},
	}
}
func (db *xDB) Lesser(field string, value interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$lt", value}}},
	}
}
func (db *xDB) LesserOrEqual(field string, value interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$lte", value}}},
	}
}
func (db *xDB) NotEqual(field string, value interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$ne", value}}},
	}
}
func (db *xDB) NotIn(field string, value []interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$nin", value}}},
	}
}
func (db *xDB) InArrayRaw(field string, value bson.A) bson.D {
	return bson.D{
		{field, bson.D{{"$in",value }}},
	}
}
func (db *xDB) InArray(field string, value []interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$in", value}}},
	}
}

func (db *xDB) In(field string, value ...interface{}) bson.D {
	return bson.D{
		{field, bson.D{{"$in", value}}},
	}
}

func (db *xDB) ExprInArray(field string, value []interface{}) bson.D {
	return bson.D{
		{"$in", bson.A{ "$" + field, value}},
	}
}
func (db *xDB) ExprIn(field string, value ...interface{}) bson.D {
	return bson.D{
		{"$in", bson.A{ "$" + field, value}},
	}
}
func (db *xDB) ExprInDefine(field string, value string) bson.D {
	return bson.D{
		{"$in", bson.A{ "$" + field, value}},
	}
}
func (db *xDB) Increment(field ...primitive.E) bson.D {
	return bson.D{
		{"$inc", bson.D(field)},
	}
}

func(db*xDB) NotEmpty(field string) bson.D{
	return bson.D{{
		field, bson.D{
			{"$exists", true},
			{"$ne", ""},
		},
	}}
}

func(db*xDB) FieldExist(field string, exist bool) bson.D{
	return bson.D{{
		field, bson.D{
			{"$exists", exist},
		},
	}}
}
func(db*xDB) Size(field string) bson.D{
	return bson.D{
		{"$size", field},
	}
}


func (db *xDB) UpdateTemplate(collection *mongo.Collection, filter bson.D, update bson.D) *mongo.UpdateResult {
	result, err := collection.UpdateMany(nil, filter, bson.D{
		{"$set", update},
		{"$currentDate", bson.D{
			{"_modified", true},
		}},
	})
	if err != nil {
		Log().Error().Err(err).Msg("Error when decode object: UpdateTemplate")
		return nil
	}
	return result
}

const (
	REGEX_START_WITH = "^"
	REGEX_OPTION_NO_CASE_SENSITIVE = "i"
)
func (db *xDB)  Regex(field string, value string, option string) bson.D {
	return bson.D {
		{field, bson.D {
			{"$regex", value},
			{"$options", option},
		}},
	}
}