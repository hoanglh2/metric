package main

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	"time"
)

func main() {
	client := resty.New().SetBaseURL("http://localhost:9000")

	go func() {
		for {
			time.Sleep(3 * time.Second)
			resp, err := client.R().
				Get("/ping")
			if err != nil {
				panic(err)
			}
			fmt.Println(resp.String())
		}
	}()
	go func() {
		for {
			resp, err := client.R().
				Get("/hello")
			if err != nil {
				panic(err)
			}
			fmt.Println(resp.String())
		}
	}()
	for {
		resp, err := client.R().
			Get("/world")
		if err != nil {
			panic(err)
		}
		fmt.Println(resp.String())
	}

}
