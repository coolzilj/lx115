package main

import (
	"errors"
	"fmt"
	"github.com/deadblue/elevengo"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/go-homedir"
	gim "github.com/ozankasikci/go-image-merge"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"syscall"
)

var client *elevengo.Client

func init() {
	uid, cid, seid := getCredentials()
	client = elevengo.Default()
	client.ImportCredentials(&elevengo.Credentials{
		UID:  uid,
		CID:  cid,
		SEID: seid,
	})
}

func main() {
	http.HandleFunc("/add-magnet-to-115", addUrl)
	if err := http.ListenAndServe(":3333", nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func addUrl(writer http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	url := request.FormValue("param1")
	hash, err := client.OfflineAddUrl(url)
	if err != nil {
		if err.Error() == "请验证账号" {
			var captchaSession *elevengo.CaptchaSession
			captchaSession, err = client.CaptchaStart()

			expectImg, _ := homedir.Expand("~/115_captcha_expect.png")
			choicesImg, _ := homedir.Expand("~/115_captcha_choices.jpg")
			mergeImg, _ := homedir.Expand("~/115_captcha.jpg")
			defer os.Remove(expectImg)
			defer os.Remove(choicesImg)
			defer os.Remove(mergeImg)

			ioutil.WriteFile(expectImg, captchaSession.CodeValue, 0644)
			ioutil.WriteFile(choicesImg, captchaSession.CodeKeys, 0644)

			grids := []*gim.Grid{
				{ImageFilePath: expectImg},
				{ImageFilePath: choicesImg},
			}
			rgba, _ := gim.New(grids, 1, 2, gim.OptGridSize(255, 100)).Merge()
			file, _ := os.Create(mergeImg)
			err = jpeg.Encode(file, rgba, &jpeg.Options{Quality: 80})
			file.Close()

			openUrl(mergeImg)

			for {
				captcha, err := askCaptcha()
				if err != nil {
					fmt.Println(err)
				}

				if err := client.CaptchaSubmit(captcha, captchaSession); err != nil {
					fmt.Println(err)
					if err.Error() != "captcha code incorrect" {
						break
					}
				}

				break
			}
		}

		if err != nil {
			fmt.Println(err.Error())
		}

		http.Error(writer, "离线任务添加失败，请打开终端查看详细错误信息", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(writer, "离线任务添加成功：%s", hash)
	fmt.Printf("离线任务添加成功：%s\n", hash)
}

func openUrl(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	c := exec.Command(cmd, args...)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return c.Start()
}

func askCaptcha() (string, error) {
	validate := func(input string) error {
		_, err := strconv.ParseInt(input, 10, 16)
		if err != nil {
			return errors.New("输入 0-9 范围内的 4 个数字")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "验证码(4位数，第一行0-4, 第二行5-9)",
		Validate: validate,
	}

	return prompt.Run()
}

func getCredentials() (uid, cid, seid string) {
	configFile, err := homedir.Expand("~/.115.cookies")
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		panic("请在 home 目录下创建 .115.cookies 文件")
	}

	if data, err := ioutil.ReadFile(configFile); err != nil {
		panic(err)
	} else {
		cookies := string(data)
		uidReg := regexp.MustCompile(`UID=(\w+);`)
		cidReg := regexp.MustCompile(`CID=(\w+);`)
		seidReg := regexp.MustCompile(`SEID=(\w+);`)
		uid = uidReg.FindAllStringSubmatch(cookies, -1)[0][1]
		cid = cidReg.FindAllStringSubmatch(cookies, -1)[0][1]
		seid = seidReg.FindAllStringSubmatch(cookies, -1)[0][1]
		return
	}

	return "", "", ""
}
