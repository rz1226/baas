package baas

import (
	"fmt"
)

var DefaultBaas *Baas = new(Baas)

//代表一篇文章

type Article struct {
	Key        string
	Title      string
	Content    string
	Author     string
	CreateTime string
	Public     bool
}

//保存文章
func (a *Article) Save() (string, error) {
	return DefaultBaas.SaveObj(a)
}

func DelArticle(key string) error {
	return DefaultBaas.DelObj(key)
}

//获取详情
func GetArticle(key string, article *Article) error {
	return DefaultBaas.FetchObj(key, article)
}

func ListArticle() error {
	tmp := make([]*Article, 0)
	var objs *[]*Article = &tmp
	err := DefaultBaas.ListObj(1, 30, objs)
	for _, v := range *objs {
		fmt.Println(*v)
	}
	return err
}
