package main

import (
	"encoding/json"
	"log"
	"os"
	"sgroupbot"
)

// 机器人配置
var ticket = sgroupbot.Ticket{
	AppID: 000,
	Token: "xxx",
}

var idiomsPath = "./idioms.json"

func main() {

	// 加载成语集合
	idioms, err := LoadIdioms(idiomsPath)
	if err != nil || len(idioms) == 0 {
		log.Println("read idioms fail", len(idioms), err)
		return
	}
	log.Println("load idioms, count", len(idioms))

	// 构建频道机器人api
	var api = sgroupbot.API{
		Target:   sgroupbot.SandboxSgroupTarget,
		Ticket:   ticket,
		Handlers: make(map[string]sgroupbot.EventHandler),
	}

	// 构建成语接龙服务
	var is = NewIdiomsSolitaire(idioms, 60*5)

	// 整合api_server
	var s = NewApiServer(&api, is)

	if err := s.Start(); err != nil {
		log.Println("startWs", err)
	}

}

func LoadIdioms(path string) ([]Idiom, error) {
	f, err := os.Open(idiomsPath)
	if err != nil {
		return nil, err
	}

	var idioms []Idiom
	if err := json.NewDecoder(f).Decode(&idioms); err != nil {
		return nil, err
	}

	return idioms, nil
}
