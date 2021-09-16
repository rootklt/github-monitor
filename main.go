package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//-----------------Server酱返回信息------------
type ServerJiangResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data
}

type Data struct {
	Pushid  string `json:"pushid"`
	Readkey string `json:"readkey"`
	MsgErr  string `json:"error"`
}

//-----------------Server酱返回信息 END----------

//-----------------Server请求体-----------------
type ServerJiangRequest struct {
	Title   string `json:"title"`
	Desp    string `json:"desp"`
	Encoded string `json:"encoded,omitempty"`
}

//-----------------Server请求体 END-------------

var (
	//参数
	Query    string
	Interval int
	Client   *http.Client
	Key      string
	Filename string
)

//github响应
type gitResponse struct {
	TotalCount        int     `json:"total_count"`
	IncompleteResults bool    `json:"incomplete_results"`
	ItemsSlice        []Items `json:"items"`
}

type Items struct {
	Name    string `json:"name"`
	HtmlUrl string `json:"html_url"`
}

//-------------------------------------------------

func init() {
	flag.StringVar(&Query, "q", "", "查询内容")
	flag.IntVar(&Interval, "i", 0, "设置查询时间间隔，单位为秒，默认为5秒")
	flag.StringVar(&Key, "k", "", "Server酱获取sendKey")
	flag.StringVar(&Filename, "f", "output.log", "指定输出的文件名")
	flag.Parse()

	if Key == "" {
		log.Fatal("没有给定Server酱的sendKey")
	}

	Client = &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			//Proxy:           proxy,
		},
	}
}

func main() {

	isFirstTime(Query)
	//如果未配置间隔，则默认5秒
	timer := time.NewTicker(time.Second * 5)

	if Interval > 0 {
		timer.Reset(time.Duration(Interval) * time.Second)
	}

	for {
		<-timer.C
		items := GetGithubResp(Query)
		if items != nil {
			for i, v := range *items {
				b := hasSent(v.Name)
				if !b {
					title := fmt.Sprintf("[+]发现新的%s信息:%s\n", Query, v.Name)
					log.Printf(title)
					SendMessage(title, v.HtmlUrl)
					writeTofile(v.Name)
				}
				if i >= 5 {
					//最多发送5个
					break
				}
			}
		}
	}
}

func isFirstTime(query string) bool {

	var s, n string
	items := GetGithubResp(query)
	writeTofile("")

	if items == nil {
		return false
	}

	for i, v := range *items {
		if i < 5 {
			//因为第一次写入了文件，再判断是不是已经存在文件时就可能存在不能把第一次发现的推送出去
			//所以第一次时发送5个出去
			s += v.HtmlUrl + "\n"
			n += v.Name + ";"
		}
		st := hasSent(v.Name)
		if !st {
			writeTofile(v.Name)
		}

	}
	SendMessage(n, s)
	return true

}

func writeTofile(title string) error {
	f, err := os.OpenFile(Filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println("[-]打开日志文件失败")
		return err
	}
	defer f.Close()
	writer := bufio.NewWriter(f)

	if title != "" {
		writer.WriteString(title + "\n")
	} else {
		writer.WriteString("")
	}

	writer.Flush()

	return nil
}

func hasSent(title string) bool {

	f, err := os.Open(Filename)

	if err != nil {
		log.Println("[-]打开日志文件失败")
		return false
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.Contains(strings.ToLower(scanner.Text()), strings.ToLower(title)) {
			return true
		}
	}
	return false
}

func GetGithubResp(query string) *[]Items {
	respBody := &gitResponse{}

	gitUrl := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=updated&order=desc", query)

	req, _ := http.NewRequest("GET", gitUrl, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3100.0 Safari/537.36")
	resp, err := Client.Do(req)

	if err != nil {
		log.Println("[-]github请求错误", err.Error())
		return nil
	}

	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Println("[-]读取github错误，未发送成功", err.Error())
		return nil
	}

	json.Unmarshal(buf, respBody)

	if len(respBody.ItemsSlice) <= 0 {
		log.Println("[-]没有数据")
		return nil
	}

	log.Println("[+]获取github数据成功")

	return &respBody.ItemsSlice
}

func SendMessage(title, desp string) {

	if title == "" || desp == "" {
		log.Println("[-]没有发送的数据")
		return
	}
	sendUrl := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", Key)

	body := "title=" + title + "&" + "desp=" + desp

	req, _ := http.NewRequest("POST", sendUrl, strings.NewReader(body))

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3100.0 Safari/537.36")
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")

	resp, err := Client.Do(req)

	if err != nil {
		log.Println("[-]发送信息错误", err.Error())
		return
	}

	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Println("[-]读取消息错误，未发送成功", err.Error())
		return
	}

	serverRespone := &ServerJiangResponse{}
	json.Unmarshal(buf, serverRespone)

	if strings.Contains(serverRespone.MsgErr, "SUCC") {
		log.Println("[+]消息发送成功.")
	}
}
