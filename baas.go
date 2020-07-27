package baas

import (
	"errors"
	"fmt"
	"github.com/rz1226/cache2"
	"github.com/rz1226/encrypt"
	"github.com/rz1226/gobutil"
	"github.com/rz1226/mysqlx"
	"os"
	"reflect"
	"time"
)

/**
增加digest， 适应长列表数据可能过大的问题
实际应用的时候，一般来说content的内容包括digest，
*/

var ccacheCount = cache2.NewCCache(1000)
var ccacheFetchDigest = cache2.NewCCache(10000)
var ccacheFetch = cache2.NewCCache(10000)

/**

CREATE TABLE `baas_item` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `key` varchar(50) DEFAULT NULL,
  `digest`  BLOB default null comment'摘要',
  `content` MEDIUMBLOB DEFAULT NULL comment'数据内容',

  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

  `last_update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  unique (`key`)

) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4;



*/

// 本质是数据库的一条数据

type baasItem struct {
	ID         int64  `orm:"id" auto:"1"`
	Key        string `orm:"key"`
	Digest     string `orm:"digest"` //非详情信息，一般用于列表展示的时候，不需要把所有的数据都拿出来，可能会比较大
	Content    string `orm:"content"`
	CreateTime string `orm:"create_time" auto:"1"`
	UpdateTime string `orm: "last_update_time" auto:"1"`
}

/**
baas 的背后是mongo数据库吗
不是。
这个简单的baas库
基于mysql

实现几个功能
1 保持string
2 根据id查询json
3 根据id删除
4 根据id修改

5 获取列表，排序规则是添加时间
*/

type Baas struct {
	Dbkit *mysqlx.DB
	Table string
}

// 返回数据唯一表示 key
func (b *Baas) set(key, data string, digest string) error {
	if b.exist(key) {

		return b.replace(key, data, digest)
	} else {
		return b.add(key, data, digest)
	}

}

func (b *Baas) add(key, data string, digest string) error {
	if key == "" {
		return errors.New("key 不能为空")
	}
	item := new(baasItem)
	item.Key = key
	item.Digest = digest
	item.Content = data

	sql, err := mysqlx.NewBM(item).ToSQLInsert(b.Table)
	if err != nil {
		return err
	}
	_, err = sql.Exec(b.Dbkit)
	if err != nil {
		return err
	}
	return nil
}

func (b *Baas) exist(key string) bool {
	res, err := mysqlx.SQLStr("select id from " + b.Table + "  where `key` = ? limit 1").AddParams(key).Query(b.Dbkit)
	if err != nil {
		return false
	}
	if len(res.Data()) == 0 {
		return false
	}
	return true

}

// 修改数据
func (b *Baas) replace(key, data string, digest string) error {
	_, err := mysqlx.SQLStr("update "+b.Table+" set content = ? ,digest = ? where `key` = ? limit 1").AddParams(data, digest, key).Exec(b.Dbkit)
	return err
}

//不带缓存
func (b *Baas) _get(key string) (string, error) {
	item := new(baasItem)
	//var item *Item
	res, err := mysqlx.SQLStr("select `id`,`key`,`content` from " + b.Table + " where `key` = ? limit 1 ").AddParams(key).Query(b.Dbkit)
	if err != nil {
		return "", err
	}
	err = res.ToStruct(item)
	if err != nil {
		return "", err
	}
	return item.Content, nil
}

func (b *Baas) get(key string, isCache bool) (string, error) {
	if isCache == false {
		return b._get(key)
	}

	f := func() (interface{}, error) {
		return b._get(key)
	}
	res, err := ccacheFetch.Use(key, f, time.Second*5)
	if err != nil {
		return "", err
	}
	content, ok := res.(string)
	if ok {
		return content, nil
	}
	return b._get(key)
}

func (b *Baas) _getDigest(key string) (string, error) {
	item := new(baasItem)
	//var item *Item
	res, err := mysqlx.SQLStr("select `id`,`key`,`digest` from " + b.Table + " where `key` = ? limit 1 ").AddParams(key).Query(b.Dbkit)
	if err != nil {
		return "", err
	}
	err = res.ToStruct(item)
	if err != nil {
		return "", err
	}
	return item.Digest, nil
}

func (b *Baas) getDigest(key string, isCache bool) (string, error) {
	if isCache == false {
		return b._getDigest(key)
	}

	f := func() (interface{}, error) {
		return b._getDigest(key)
	}
	res, err := ccacheFetchDigest.Use(key, f, time.Second*5)
	if err != nil {
		return "", err
	}
	content, ok := res.(string)
	if ok {
		return content, nil
	}
	return b._getDigest(key)
}

//批量get 最大1000个
func (b *Baas) getDataBatch(keys ...string) ([]string, error) {
	//var item *Item
	sql := mysqlx.SQLStr("select `key`,`content` from "+b.Table+" where ").AddParams(nil).In("key", keys).Limit(1000)
	fmt.Println(sql.Info())
	res, err := sql.Query(b.Dbkit)
	if err != nil {
		return nil, err
	}
	var items []*baasItem
	err = res.ToStruct(&items)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(items))
	for _, v := range items {
		result = append(result, v.Content)
	}
	return result, nil

}

func (b *Baas) del(key string) error {
	_, err := mysqlx.SQLStr("delete from " + b.Table + "  where `key` = ? limit 1").AddParams(key).Exec(b.Dbkit)
	return err
}

//默认根据时间排序 返回内容列表   注意baas的key同时存在在内容里
func (b *Baas) list(page int, pagesize int) ([]string, error) {
	if page < 1 {
		page = 1
	}
	if pagesize < 1 || pagesize > 10000 {
		pagesize = 10
	}
	offset := pagesize * (page - 1)
	limit := pagesize
	sql := mysqlx.SQLStr("select `key`,`content` from " + b.Table + " order by id desc limit " + fmt.Sprint(limit) + " offset  " + fmt.Sprint(offset))
	res, err := sql.Query(b.Dbkit)
	if err != nil {
		return nil, err
	}
	var items []*baasItem
	err = res.ToStruct(&items)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(items))
	for _, v := range items {
		result = append(result, v.Content)
	}
	return result, nil
}

//默认根据时间排序 返回digest列表 注意baas的key同时存在在digest里
func (b *Baas) listDigest(page int, pagesize int) ([]string, error) {
	if page < 1 {
		page = 1
	}
	if pagesize < 1 || pagesize > 10000 {
		pagesize = 10
	}
	offset := pagesize * (page - 1)
	limit := pagesize
	sql := mysqlx.SQLStr("select `key`,`digest` from " + b.Table + " order by id desc limit " + fmt.Sprint(limit) + " offset  " + fmt.Sprint(offset))
	res, err := sql.Query(b.Dbkit)
	if err != nil {
		return nil, err
	}
	var items []*baasItem
	err = res.ToStruct(&items)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(items))
	for _, v := range items {
		result = append(result, v.Digest)
	}
	return result, nil
}

func (b *Baas) Count() (int64, error) {
	f := func() (interface{}, error) {
		return b.count()
	}
	res, err := ccacheCount.Use("count:"+b.Table, f, time.Second*5)
	if err != nil {
		return 0, err
	}
	count, ok := res.(int64)
	if ok {
		return count, nil
	}
	return b.count()
}

func (b *Baas) count() (int64, error) {

	sql := mysqlx.SQLStr("select count(*) from " + b.Table)
	res, err := sql.Query(b.Dbkit)
	if err != nil {
		return 0, err
	}
	return res.ToInt64()
}

// set key  如果forceSet == false 则没有在Key的位置是空的时候，才会set一个随机key    返回值为strut最终的key
func setKey(dstStruct interface{}, key string, forceSet bool) string {

	defer func() {
		if co := recover(); co != nil {
			str := "SetKey error:发生panic :" + fmt.Sprint(co)
			fmt.Println(str)
			os.Exit(1)
		}
	}()
	v := reflect.ValueOf(dstStruct)

	switch v.Kind() {
	case reflect.Ptr:
		t := v.Type().Elem()

		for i := 0; i < v.Elem().NumField(); i++ {
			fieldName := t.Field(i).Name
			vType := t.Field(i).Type
			if fmt.Sprint(vType) == "string" && fieldName == "Key" {
				oriKey := v.Elem().Field(i).Interface().(string)
				if oriKey == "" {
					v.Elem().Field(i).Set(reflect.ValueOf(key))
					return v.Elem().Field(i).Interface().(string)
				}
				if forceSet {
					v.Elem().Field(i).Set(reflect.ValueOf(key))
					return v.Elem().Field(i).Interface().(string)
				}

				return v.Elem().Field(i).Interface().(string)
			}
		}

	default:
		panic("SetKey error:要传入的是结构体指针")

	}
	panic("SetKey 没有成功, 是不是没有定义Key属性")
	return ""
}

//删除
func (b *Baas) DelObj(key string) error {
	return b.del(key)
}

//从一个baas obj ,生成digest，根据tag, 其实digest和obj是同一个结构，只是正文字段为空而已。
func genDigest(a interface{}) (interface{}, error) {
	t := reflect.TypeOf(a)
	v := reflect.ValueOf(a)
	res := reflect.New(t.Elem())
	switch v.Kind() {
	case reflect.Ptr:
		length := t.Elem().NumField()
		for i := 0; i < length; i++ {
			tag := t.Elem().Field(i).Tag.Get("digest")
			if tag != "" {
				res.Elem().Field(i).Set(v.Elem().Field(i))
			}
		}

	default:
		return nil, errors.New("only support struct pointer")
	}
	return res.Interface(), nil
}

//保存  参数是指针  .digest会从a参数copy一模一样的值
func (b *Baas) SaveObj(a interface{}) (string, error) {

	key := encrypt.MakeUUID()
	//利用反射加入一个key的值  如果没有Key属性，就报错。
	newKey := setKey(a, key, false)
	digest, err := genDigest(a)
	if err != nil {
		return "", err
	}
	str, err := gobutil.ToBytes(a)
	if err != nil {
		return "", err
	}
	digestStr, err := gobutil.ToBytes(digest)
	if err != nil {
		return "", err
	}
	err = b.set(newKey, string(str), string(digestStr))
	if err != nil {
		return "", err
	}
	return key, nil
}

//第二个参数是指针
func (b *Baas) FetchObj(key string, obj interface{}, isCache bool) error {
	data, err := b.get(key, isCache)
	if err != nil {
		return err
	}
	err = gobutil.ToStruct([]byte(data), obj)

	if err != nil {
		return err
	}
	return nil
}
func (b *Baas) FetchObjDegist(key string, obj interface{}, isCache bool) error {
	data, err := b.getDigest(key, isCache)
	if err != nil {
		return err
	}
	err = gobutil.ToStruct([]byte(data), obj)

	if err != nil {
		return err
	}
	return nil
}

//list
func (b *Baas) ListObj(page int, pagesize int, dstStruct interface{}) (resErr error) {
	strs, err := b.list(page, pagesize)
	if err != nil {
		return err
	}
	v := reflect.ValueOf(dstStruct)
	switch v.Kind() {
	case reflect.Ptr:
		t := v.Type().Elem()
		tEle := t.Elem()
		if tEle.Kind() != reflect.Ptr {
			panic("数组元素应该是*Struct,而不是Struct")
		}
		v2 := v.Elem()

		for _, data := range strs {
			newObj := reflect.New(tEle.Elem())
			err = gobutil.ToStruct([]byte(data), newObj.Interface())
			if err != nil {
				return err
			}
			v2 = reflect.Append(v2, newObj)
		}

		v.Elem().Set(v2)
		return nil
	default:
		return errors.New("ListObj : only support struct pointer")
	}
}

func (b *Baas) ListObjDigest(page int, pagesize int, dstStruct interface{}) (resErr error) {
	strs, err := b.listDigest(page, pagesize)
	if err != nil {
		return err
	}
	v := reflect.ValueOf(dstStruct)
	switch v.Kind() {
	case reflect.Ptr:
		t := v.Type().Elem()
		tEle := t.Elem()
		if tEle.Kind() != reflect.Ptr {
			panic("数组元素应该是*Struct,而不是Struct")
		}
		v2 := v.Elem()
		for _, data := range strs {
			newObj := reflect.New(tEle.Elem())
			err = gobutil.ToStruct([]byte(data), newObj.Interface())
			if err != nil {
				return err
			}
			v2 = reflect.Append(v2, newObj)
		}
		v.Elem().Set(v2)
		return nil
	default:
		return errors.New("ListObjDigest : only support struct pointer")
	}
}
