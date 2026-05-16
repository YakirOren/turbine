package main

import (
	"log"

	"github.com/YakirOren/turbine"
)

func Greet(ctx turbine.Context, name string) (string, error) {
	return "hello, " + name, nil
}

func main() {
	rt := turbine.NewStandalone(turbine.Config{})
	defer rt.Shutdown()

	turbine.Register(rt, Greet)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, err := turbine.Run(rt, Greet, "world")
	if err != nil {
		log.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result)
}
