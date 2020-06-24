package baas

import (
	"errors"
	"fmt"
	"github.com/rz1226/encrypt"
	"github.com/rz1226/gobutil"
	"os"
	"reflect"
)

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
func DelObj(key string) error {
	return del(key)
}

//保存  参数是指针
func SaveObj(a interface{}) (string, error) {
	key := encrypt.MakeUUID()
	newKey := setKey(a, key, false)
	//利用反射加入一个key的值  如果没有Key属性，就报错。
	str, err := gobutil.ToBytes(a)
	if err != nil {
		return "", err
	}
	err = set(newKey, string(str))
	if err != nil {
		return "", err
	}
	return key, nil
}

//第二个参数是指针
func FetchObj(key string, obj interface{}) error {
	data, err := get(key)
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
func ListObj(page int, pagesize int, dstStruct interface{}) (resErr error) {
	strs, err := list(page, pagesize)
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
