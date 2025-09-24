package main

import "github.com/J1407B-K/buff/buff"

type Foo struct {
	Bar string `json:"bar"`
}

func PongHandler(c *buff.Context) {
	var foo Foo
	c.Bind(&foo)
	c.JSON(200, foo)
}
