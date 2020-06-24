package baas

import (
	"errors"
	"fmt"
	"github.com/rz1226/mysqlx"
)

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

//表示本baas服务的底层数据，本质是一个字符串

// 返回数据唯一表示 key
func set(key, data string) error {
	if exist(key) {
		return replace(key, data)
	} else {
		return add(key, data)
	}

}

func add(key, data string) error {
	if key == "" {
		return errors.New("key 不能为空")
	}

	item := new(baasItem)
	item.Key = key
	item.Content = data

	sql, err := mysqlx.NewBM(item).ToSQLInsert("baas_item")
	if err != nil {
		return err
	}
	_, err = sql.Exec(Dbkit)
	if err != nil {
		return err
	}
	return nil
}

func exist(key string) bool {
	res, err := mysqlx.SQLStr("select id from baas_item  where `key` = ? limit 1").AddParams(key).Query(Dbkit)
	if err != nil {
		return false
	}
	if len(res.Data()) == 0 {
		return false
	}
	return true

}

// 修改数据
func replace(key, data string) error {
	_, err := mysqlx.SQLStr("update baas_item set content = ? where `key` = ? limit 1").AddParams(data, key).Exec(Dbkit)
	return err
}

func get(key string) (string, error) {
	item := new(baasItem)
	//var item *Item
	res, err := mysqlx.SQLStr("select * from baas_item where `key` = ? limit 1 ").AddParams(key).Query(Dbkit)
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
func getDataBatch(keys ...string) ([]string, error) {
	//var item *Item
	sql := mysqlx.SQLStr("select `key`,`content` from baas_item where ").AddParams(nil).In("key", keys).Limit(1000)
	fmt.Println(sql.Info())
	res, err := sql.Query(Dbkit)
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

func del(key string) error {
	_, err := mysqlx.SQLStr("delete from baas_item  where `key` = ? limit 1").AddParams(key).Exec(Dbkit)
	return err
}

//默认根据时间排序 返回key列表
func list(page int, pagesize int) ([]string, error) {
	if page < 1 {
		page = 1
	}
	if pagesize < 1 || pagesize > 10000 {
		pagesize = 10
	}
	offset := pagesize * (page - 1)
	limit := pagesize
	sql := mysqlx.SQLStr("select `key`,`content` from baas_item order by id desc limit " + fmt.Sprint(limit) + " offset  " + fmt.Sprint(offset))
	res, err := sql.Query(Dbkit)
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
