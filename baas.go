package baas

import (
	"errors"
	"fmt"
	"github.com/rz1226/cache"
	"github.com/rz1226/encrypt"
	"github.com/rz1226/gobutil"
	"github.com/rz1226/mysqlx"
	"os"
	"reflect"
	"time"
)

var ccache = cache.NewCCache(1000)

/**

CREATE TABLE `baas_item` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `key` varchar(50) DEFAULT NULL,
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
func (b *Baas) set(key, data string) error {
	if b.exist(key) {
		return b.replace(key, data)
	} else {
		return b.add(key, data)
	}

}

func (b *Baas) add(key, data string) error {
	if key == "" {
		return errors.New("key 不能为空")
	}

	item := new(baasItem)
	item.Key = key
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
func (b *Baas) replace(key, data string) error {
	_, err := mysqlx.SQLStr("update "+b.Table+" set content = ? where `key` = ? limit 1").AddParams(data, key).Exec(b.Dbkit)
	return err
}

func (b *Baas) get(key string) (string, error) {
	item := new(baasItem)
	//var item *Item
	res, err := mysqlx.SQLStr("select * from " + b.Table + " where `key` = ? limit 1 ").AddParams(key).Query(b.Dbkit)
	if err != nil {
		return "", err
	}
	err = res.ToStruct(item)
	if err != nil {
		return "", err
	}
	return item.Content, nil
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

//默认根据时间排序 返回key列表
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

func (b *Baas) Count() (int64, error) {
	//加一个时间很短的缓存
	key := cache.NewKey("count:" + b.Table)
	resCache, err := key.FetchFromCCache(ccache)
	if err == nil {
		if count, ok := resCache.(int64); ok {
			return count, nil
		}
	}

	sql := mysqlx.SQLStr("select count(*) from " + b.Table)
	res, err := sql.Query(b.Dbkit)
	if err != nil {
		return 0, err
	}

	data := cache.NewData(res).SetKey("count:" + b.Table)
	data.ToCCache(ccache, time.Second*1)

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
	isSet := false
	v := reflect.ValueOf(dstStruct)

	switch v.Kind() {
	case reflect.Ptr:
		t := v.Type().Elem()

		for i := 0; i < v.Elem().NumField(); i++ {
			fieldName := t.Field(i).Name
			vType := t.Field(i).Type
			if fmt.Sprint(vType) == "string" && fieldName == "Key" {
				if v.Elem().Field(i).Interface().(string) == "" {
					v.Elem().Field(i).Set(reflect.ValueOf(key))
				}
				if forceSet {
					v.Elem().Field(i).Set(reflect.ValueOf(key))
				}
				isSet = true
				return v.Elem().Field(i).Interface().(string)
			} else {

			}
		}

	default:
		panic("SetKey error:要传入的是结构体指针")

	}
	if !isSet {
		panic("SetKey 没有成功, 是不是没有设置Key属性")

	}
	return ""
}

//删除
func (b *Baas) DelObj(key string) error {
	return b.del(key)
}

//保存  参数是指针
func (b *Baas) SaveObj(a interface{}) (string, error) {
	key := encrypt.MakeUUID()
	newKey := setKey(a, key, false)
	//利用反射加入一个key的值  如果没有Key属性，就报错。
	str, err := gobutil.ToBytes(a)
	if err != nil {
		return "", err
	}
	err = b.set(newKey, string(str))
	if err != nil {
		return "", err
	}
	return key, nil
}

//第二个参数是指针
func (b *Baas) FetchObj(key string, obj interface{}) error {
	data, err := b.get(key)
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
		return errors.New("only support struct pointer")
	}

}
