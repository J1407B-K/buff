package main

import (
	"log"
	"time"

	"github.com/J1407B-K/buff/buff"
)

func main() {
	e := buff.NewEngine()
	e.Use(buff.Logger(), buff.Timeout(5*time.Second))

	e.GET("/ping", PongHandler)
	e.GET("/hello/:name", func(c *buff.Context) {
		c.JSON(200, map[string]string{"hi": c.Param("name")})
	})

	if err := e.R.Verify(); err != nil {
		log.Fatal(err)
	}

	log.Fatal(e.Run(":8080"))
}
