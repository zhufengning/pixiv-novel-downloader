package main

import (
	"bytes"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/PuerkitoBio/goquery"
	"github.com/valyala/fastjson"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const ApiSeriesContent = "https://www.pixiv.net/ajax/novel/series_content/"
const ApiChapterContext = "https://www.pixiv.net/novel/show.php?id="

var seriesID = ""
var proxyUrl = ""
var phpSessid = ""
var outDir = ""

type chapter struct {
	id    string
	title string
}
var w fyne.Window
func main() {

	myApp := app.New()

	w = myApp.NewWindow("Config")
	sidInput := widget.NewEntry()
	sidInput.SetPlaceHolder("Series ID")
	sidInput.SetText(seriesID)
	proxyInput := widget.NewEntry()
	proxyInput.SetPlaceHolder("Proxy,such as socks://127.0.0.1:1080")
	proxyInput.SetText(proxyUrl)
	loginInput := widget.NewEntry()
	loginInput.SetPlaceHolder("Value of cookie:PHPSESSID")
	loginInput.SetText(phpSessid)
	outInput := widget.NewEntry()
	outInput.SetPlaceHolder("Out Directory,such as out/")
	outInput.SetText(outDir)
	okButton := widget.NewButton("OK", nil)
	okButton.OnTapped = func() {
		okButton.Disable()
		okButton.Text = "Running"
		seriesID = sidInput.Text
		proxyUrl = proxyInput.Text
		phpSessid = loginInput.Text
		outDir = outInput.Text
		work()
		okButton.Enable()
		okButton.Text = "OK"
		dialog.ShowInformation("Notice", "Finished", w)
	}
	l := layout.NewVBoxLayout()

	v := container.New(l, sidInput, proxyInput, loginInput, outInput, okButton)
	w.Resize(fyne.NewSize(480, 200))
	w.SetContent(v)

	w.ShowAndRun()
}

func getDom(myUrl string) (*goquery.Document, string, error) {
	proxy, err := url.Parse(proxyUrl)
	if err != nil {
		onErr(err)
	}
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
		},
	}
	//cd, err := iconv.Open("utf-8", "gbk") // convert utf-8 to gbk
	//if err != nil {
	//	onErr("iconv.Open failed!")
	//}
	//defer cd.Close()
	req, err := http.NewRequest("GET", myUrl, nil)
	req.AddCookie(&http.Cookie{
		Name:    "PHPSESSID",
		Value:   phpSessid,
		Expires: time.Now().Add(time.Hour),
	})
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0")
	if err != nil {
		return nil, "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, "", errors.New(fmt.Sprintf("http error %d", res.StatusCode))
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)
	raw := buf.String()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return nil, "", err
	}
	return doc, raw, nil
}

func work() {
	proxy, err := url.Parse(proxyUrl)
	if err != nil {
		onErr(err)
	}
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
		},
	}
	fmt.Println("Getting content")
	req, err := http.NewRequest("GET", ApiSeriesContent+seriesID+"?order_by=asc", nil)
	req.AddCookie(&http.Cookie{
		Name:    "PHPSESSID",
		Value:   phpSessid,
		Expires: time.Now().Add(time.Hour),
	})
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0")

	if err != nil {
		onErr(err)
	}
	res, err := client.Do(req)
	if err != nil {
		onErr(err)
	}

	contentJ, err := ioutil.ReadAll(res.Body)
	if err != nil {
		onErr(err)
	}
	contentS := string(contentJ)
	var p fastjson.Parser
	content, err := p.Parse(contentS)
	if err != nil {
		onErr(err)
	}
	err = res.Body.Close()
	if err != nil {
		onErr(err)
	}

	if content.GetBool("error") == true {
		onErr(content.Get("message"))
	}

	chapters := make([]chapter, 0)
	ar := content.GetArray("body", "seriesContents")
	for _, v := range ar {
		chapters = append(chapters, chapter{string(v.GetStringBytes("id")), string(v.GetStringBytes("title"))})
	}
	cnt := len(chapters) + 1
	ln := 0
	for cnt > 0 {
		ln += 1
		cnt /= 10
	}
	fmt.Println("Running")
	for k, v := range chapters {
		fileName := fmt.Sprintf("%0"+fmt.Sprint(ln)+"d %s", k, v.title)
		fileName = strings.ReplaceAll(fileName, "/", "")
		fileName = strings.ReplaceAll(fileName, "\\", "")
		fileName = strings.ReplaceAll(fileName, ":", "")
		fileName = strings.ReplaceAll(fileName, "*", "")
		fileName = strings.ReplaceAll(fileName, "?", "")
		fileName = strings.ReplaceAll(fileName, "\"", "")
		fileName = strings.ReplaceAll(fileName, "<", "")
		fileName = strings.ReplaceAll(fileName, ">", "")
		fileName = strings.ReplaceAll(fileName, "|", "")
		fmt.Println(fileName)
		u := ApiChapterContext + v.id
		doc, _, err := getDom(u)
		if err != nil {
			onErr(err)
		}
		var con string
		mgd := doc.Find("#meta-preload-data")
		mgd.Each(func(i int, selection *goquery.Selection) {
			con, _ = selection.Attr("content")
		},
		)
		var p fastjson.Parser
		conJ, err := p.Parse(con)
		if err != nil {
			onErr(err)
		}
		err = os.MkdirAll(outDir, os.ModePerm)
		if err != nil {
			onErr(err)
		}
		f, err := os.Create(outDir + "/" + fileName + ".txt")
		if err != nil {
			onErr(err)
		}
		_, err = io.WriteString(f, string(conJ.GetStringBytes("novel", v.id, "description"))+string(conJ.GetStringBytes("novel", v.id, "content")))

		if err != nil {
			onErr(err)
		}
		err = f.Close()
		if err != nil {
			onErr(err)
		}
	}
}

func onErr(err interface{}) {
	dialog.ShowInformation("Error", fmt.Sprint("err"), w)
}
