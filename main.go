package main

import (
	"fmt"

	"github.com/Happy2018new/evercare-journey-backend/service"
)

func main() {
	router := service.InitAndMakeRouter()
	router.Run(fmt.Sprintf(":%d", 80))
}
