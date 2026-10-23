package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"io/ioutil"
	"path/filepath"
	"gopkg.in/yaml.v2"
	"encoding/json"
	"../JsonHttpClient"
)

type gateway struct {
	Token string `yaml:"token"`
	AuthServer string `yaml:"authSSH"`
}

type configType struct {
    Gateway gateway `yaml:"gateway"`
}

type RequestData struct {
	UserID string `json:"userID"`
	Repo string `json:"repo"`
	Action string `json:"action"`
}

type Payload struct {
	Uid string `json:"uid"`
	Path string `json:"path"`
	Action string `json:"action"`
}

type ResponseData struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data Payload `json:"data"`
}

func readConfig () configType {
  var conf configType
  // get current dir
  dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
  if err != nil {
    fmt.Println(err.Error())
  }

  // get yaml file content
  configData, err := ioutil.ReadFile(dir + "/../../env.yaml")
  if err != nil {
    fmt.Println(err.Error())
  }

  err = yaml.Unmarshal(configData, &conf)

  if err != nil {
    fmt.Println(err.Error())
  }

  return conf
}

var config configType

// init 把标准日志重定向到工作目录下的 message.txt。
//
// 注意：这里是 **forced command** 的入口，进程以 ssh 用户（git）身份运行，
// 工作目录是 /home/git。因此：
//   - 打不开日志文件**不能** panic —— 否则整个 SSH push / clone 全部失败，
//     而且报错信息只有客户端看得到（"Permission denied"），极难定位；
//   - 退而求其次写 stderr（sshd 会把它送到客户端），至少不丢日志。
func init() {
	file := "./" + "message" + ".txt"
	logFile, err := os.OpenFile(file, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)

	if err != nil {
		// 日志文件不可写（例如工作目录不属于 ssh 用户，或残留文件属主是 root）。
		// 这里退回 stderr，绝不 panic。
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
		fmt.Fprintf(os.Stderr, "[CodeFever Next]: 无法打开日志文件 %s: %v\n", file, err)
		return
	}

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
	return
}

func main () {
	// read yml config to global
	config = readConfig()

	if (len(os.Args) == 4) {
		pattern, _ := regexp.Compile(`^(git-(receive|upload)-pack)$`)
		sub := pattern.FindSubmatch([]byte(os.Args[1]))

		if (len(sub) > 1) {
			requestData, err := json.Marshal(RequestData{
				UserID: os.Args[3],
				Repo: os.Args[2],
				Action: string(sub[1]),
			})

			if (err != nil) {
				fmt.Println(err.Error())
				return
			}

			status, response := jsonHttpClient.Request(config.Gateway.Token, "GET", config.Gateway.AuthServer, requestData)

			responseData := &ResponseData{}
			err = json.Unmarshal(response, responseData)

			if (status != 200 || responseData.Code != 0) {
				return
			}

			os.Setenv("PGYER_UID", responseData.Data.Uid)

			cmd := exec.Command(os.Args[1], responseData.Data.Path)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err = cmd.Start()

			if (err != nil) {
				fmt.Println(err.Error())
				return
			}

			err = cmd.Wait()

			if (err != nil) {
				fmt.Println(err.Error())
				return
			}
		}
	}

	fmt.Println("[CodeFever Next]: Interaction shell is not allowed.")
	return
}
