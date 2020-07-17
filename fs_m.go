package main

import (
	//"github.com/fsnotify/fsnotify"
	"encoding/json"
	"github.com/dietsche/rfsnotify"
	"gopkg.in/fsnotify.v1"
	redisv4 "gopkg.in/redis.v4"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Cfg struct {
	Redis   redis    `yaml:"redis"`
	Monitor []string `yaml:"monitor"`
	Exclude []string `yaml:"exclude"`
	Suffix  []string `yaml:"suffix"`
	Prefix  []string `yaml:"prefix"`
	Paths   []string `yaml:"paths"`
	Customs []string `yaml:"customs"`
}

type redis struct {
	Ip   string `yaml:"ip"`
	Port string `yaml:"port"`
}

type Push struct {
	Host    string `json:"host"`
	Type    string `json:"type"`
	Event   string `json:"Event"`
	Date    string `json:"date"`
	Path    string `json:"PATH"`
	Message string `json:"message"`
}

func NewPush(event, path string) string {
	var result = new(Push)
	host, _ := os.Hostname()
	date := time.Now().Format("2006-01-02 15:04:05")
	result.Host = host
	result.Type = "fs_monitor"
	result.Event = event
	result.Date = date
	result.Path = path
	result.Message = path
	json_body, _ := json.Marshal(result)
	return string(json_body)
}

func Match(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// 创建 redis 客户端
func createClient(ip string) *redisv4.Client {
	client := redisv4.NewClient(&redisv4.Options{
		Addr:     ip,
		Password: "",
		DB:       0,
		PoolSize: 5,
	})

	pong, err := client.Ping().Result()
	log.Println(pong, err)

	return client
}

func Lpush(json_body string, ip string) {
	client := createClient(ip)
	defer client.Close()
	client.LPush("logstash", json_body)
}

func main() {
	buf, err := ioutil.ReadFile("./cfg.yaml")
	if err != nil {
		panic(err)
	}

	var config Cfg
	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		panic(err)
	}

	redisip := config.Redis.Ip + ":" + config.Redis.Port
	log.Println(redisip)

	//watcher, err := fsnotify.NewWatcher()
	watcher, err := rfsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				dir, file := filepath.Split(event.Name)
				suffix := path.Ext(file)
				prefix := strings.TrimSuffix(file, suffix)
				//log.Println(dir,file,suffix,prefix)
				if Match(config.Exclude, file) || Match(config.Prefix, prefix) || Match(config.Suffix, suffix) || Match(config.Paths, dir) || event.Name == "" {
					log.Println("exclude:", event.Name)
					continue
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					Lpush(NewPush(event.Op.String(), event.Name), redisip)
					log.Println(NewPush(event.Op.String(), event.Name))
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					//新创建目录不能监控问题
					if IsDir(event.Name) {
						log.Println("add:", event.Name)
						watcher.AddRecursive(event.Name)
					}
					Lpush(NewPush(event.Op.String(), event.Name), redisip)
					log.Println(NewPush(event.Op.String(), event.Name))
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					Lpush(NewPush(event.Op.String(), event.Name), redisip)
					log.Println(NewPush(event.Op.String(), event.Name))
				}
				if event.Op&fsnotify.Rename == fsnotify.Rename {
					Lpush(NewPush(event.Op.String(), event.Name), redisip)
					log.Println(NewPush(event.Op.String(), event.Name))
				}
				if event.Op&fsnotify.Chmod == fsnotify.Chmod {
					//Lpush(NewPush(event.Op.String(), event.Name), redisip)
					//log.Println(NewPush(event.Op.String(), event.Name))
					continue
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	for _, dir := range config.Monitor {
		//err = watcher.Add(dir)
		watcher.AddRecursive(dir)
		//watcher.RemoveRecursive("/tmp/")
		if err != nil {
			log.Fatal(err)
		}
	}
	<-done
}
