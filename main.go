package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Ti-mo-k/daraja-go/auth"
)

func main() {
	ctx := context.Background()

	response, err := auth.GetAccessToken(ctx)
	if err != nil{
		log.Fatal(err)
	}

	fmt.Println(response.AccessToken)
	fmt.Println(response.ExpiresIn)
}

