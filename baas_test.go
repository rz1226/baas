package baas

import (
	"fmt"
	"reflect"
	"testing"
)

type A struct {
	Key        string `digest:"1"`
	Title      string `digest:"1"`
	TitlePic   []byte
	Digest     string
	Link       string
	Content    string
	Source     string
	CreateTime string
	Public     bool `digest:"1"`
}

func Test_gendigest(t *testing.T) {

	a := new(A)
	a.Key = "key"
	a.Title = "title"
	a.Digest = "Digest"
	a.Link = "link"
	a.Public = true

	b, err := genDigest(a)

	fmt.Println(b, err)

	fmt.Println(reflect.TypeOf(a), reflect.TypeOf(b))

}
