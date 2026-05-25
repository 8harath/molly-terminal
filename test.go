package main

import (
	"fmt"
	"github.com/ploglabs/molly-terminal/internal/guilds"
)

func main() {
	cl := guilds.NewClient("https://molly.ploglabs.com:8443", "")
	list, err := cl.FetchGuilds("")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for i, g := range list {
		fmt.Printf("[%d] ID=%s Name=%q\n", i, g.ID, g.Name)
	}
}
