/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/GoogleCloudPlatform/open-match/test/cmd/client/player"
	"github.com/GoogleCloudPlatform/open-match/test/cmd/client/redis/playerq"

	"github.com/gomodule/redigo/redis"
	"github.com/spf13/viper"
)

func main() {

	conf, err := readConfig("", map[string]interface{}{
		"REDIS_SENTINEL_SERVICE_HOST": "127.0.0.1",
		"REDIS_SENTINEL_SERVICE_PORT": "6379",
	})
	check(err, "QUIT")

	// As per https://www.iana.org/assignments/uri-schemes/prov/redis
	// redis://user:secret@localhost:6379/0?foo=bar&qux=baz
	redisURL := "redis://" + conf.GetString("REDIS_SENTINEL_SERVICE_HOST") + ":" + conf.GetString("REDIS_SENTINEL_SERVICE_PORT")

	pool := redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.DialURL(redisURL) },
	}

	redisConn := pool.Get()
	defer redisConn.Close()

	// Make a new player generator
	player.New()

	const numPlayers = 20
	fmt.Println("Starting client api stub...")

	for {
		start := time.Now()

		for i := 1; i <= numPlayers; i++ {
			_, err = queuePlayer(redisConn)
			check(err, "")
		}

		elapsed := time.Since(start)
		check(err, "")
		fmt.Printf("Redis queries and Xid generation took %s\n", elapsed)
		fmt.Println("Sleeping")
		time.Sleep(5 * time.Second)
	}
}

// Queue a player; dump all their matchmaking constraints into a JSON string.
func queuePlayer(redisConn redis.Conn) (playerID string, err error) {
	playerID, playerData, debug := player.Generate()
	_ = debug // TODO.  For now you could copy this into playerdata before creating player if you want it available in redis
	pdJSON, _ := json.Marshal(playerData)
	err = playerq.Create(redisConn, playerID, string(pdJSON))
	check(err, "")
	// This assumes you have at least ping data for this one region, comment out if you don't need this output
	fmt.Printf("Generated player %v in %v\n\tPing to %v: %3dms\n", playerID, debug["city"], "gcp.europe-west2", playerData["region.europe-west2"])
	return
}

func check(err error, action string) {
	if err != nil {
		if action == "QUIT" {
			log.Fatal(err)
		} else {
			log.Print(err)
		}
	}
}

func readConfig(filename string, defaults map[string]interface{}) (*viper.Viper, error) {
	/*
		REDIS_SENTINEL_PORT_6379_TCP=tcp://10.55.253.195:6379
		REDIS_SENTINEL_PORT=tcp://10.55.253.195:6379
		REDIS_SENTINEL_PORT_6379_TCP_ADDR=10.55.253.195
		REDIS_SENTINEL_SERVICE_PORT=6379
		REDIS_SENTINEL_PORT_6379_TCP_PORT=6379
		REDIS_SENTINEL_PORT_6379_TCP_PROTO=tcp
		REDIS_SENTINEL_SERVICE_HOST=10.55.253.195
	*/
	v := viper.New()
	for key, value := range defaults {
		v.SetDefault(key, value)
	}
	v.SetConfigName(filename)
	v.AddConfigPath(".")
	v.AutomaticEnv()

	// Optional read from config if it exists
	err := v.ReadInConfig()
	if err != nil {
		//fmt.Printf("error when reading config: %v\n", err)
		//fmt.Println("continuing...")
		err = nil
	}
	return v, err
}
