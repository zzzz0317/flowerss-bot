package tgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
	"html"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func DownloadAndUploadToTelegraph(imgSrc string) (string, error) {
	//var socks5 = "10.99.0.251:1024"
	var client = &http.Client{}
	if socks5Proxy != "" {
		var proxy, _ = url.Parse("socks5://" + socks5Proxy)
		tr := &http.Transport{
			Proxy: http.ProxyURL(proxy),
		}
		client = &http.Client{
			Transport: tr,
			Timeout:   time.Second * 5, //超时时间
		}
	}
	resp, err := http.Get(imgSrc)
	if err != nil {
		return imgSrc, err
	}
	defer resp.Body.Close()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("file", "blob")
	if err != nil {
		zap.S().Warnf("Image upload failed(writer.CreateFormFile): %s", err)
		return imgSrc, nil
	}
	_, err = io.Copy(fw, resp.Body)
	if err != nil {
		zap.S().Warnf("Image upload failed(io.Copy): %s", err)
		return imgSrc, nil
	}
	err = writer.Close() // close writer before POST request
	if err != nil {
		zap.S().Warnf("Image upload failed(writer.Close): %s", err)
		return imgSrc, nil
	}
	upresp, err := client.Post("https://telegra.ph/upload", writer.FormDataContentType(), body)
	if err != nil {
		zap.S().Warnf("Image upload failed(client.Post): %s", err)
		return imgSrc, nil
	}
	upbody, err := ioutil.ReadAll(upresp.Body)
	if err != nil {
		zap.S().Warnf("Image upload failed(ioutil.ReadAll): %s", err)
		return imgSrc, nil
	}
	type telegraphUpResult struct {
		Src string `json:src`
	}
	datas := make([]telegraphUpResult, 0)
	err = json.Unmarshal(upbody, &datas)
	if err != nil {
		zap.S().Warnf("Image upload failed(json.Unmarshal): %s\n%s", err, upbody)
		return imgSrc, nil
	}
	//var uploadedSrc = "https://telegra.ph" + datas[0].Src
	var uploadedSrc = datas[0].Src
	zap.S().Infof("Image upload finished: %s", uploadedSrc)
	zap.S().Infof("from: %s", imgSrc)
	return uploadedSrc, nil
}

func FormatHtmlContent(htmlContent string) string {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		//var s2, _ = s.Html()
		var imgsrc, _ = DownloadAndUploadToTelegraph(s.AttrOr("src", ""))
		s.SetAttr("src", imgsrc)
		//fmt.Printf(imgsrc)
		//fmt.Printf("%d: %s", i, s2)
		//fmt.Printf("\n")
	})
	var dochtml, _ = doc.Html()
	return dochtml
}

func PublishHtml(sourceTitle string, title string, rawLink string, htmlContent string) (string, error) {
	//html = fmt.Sprintf(
	//	"<p>本文章由 <a href=\"https://github.com/indes/flowerss-bot\">flowerss</a> 抓取自RSS，版权归<a href=\"\">源站点</a>所有。</p><hr>",
	//) + html + fmt.Sprintf(
	//	"<hr><p>本文章由 <a href=\"https://github.com/indes/flowerss-bot\">flowerss</a> 抓取自RSS，版权归<a href=\"\">源站点</a>所有。</p><p>查看原文：<a href=\"%s\">%s - %s</p>",
	//	rawLink,
	//	title,
	//	sourceTitle,
	//)

	//zap.S().Infof(htmlContent)
	htmlContent = FormatHtmlContent(htmlContent)
	htmlContent = fmt.Sprintf(
		"<hr><p>本文章由 <a href=\"https://github.com/indes/flowerss-bot\">flowerss</a> 抓取自RSS，版权归<a href=\"\">源站点</a>所有。</p><p>查看原文：<a href=\"%s\">%s</p>",
		rawLink,
		rawLink) + html.UnescapeString(htmlContent)
	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator
	client := clientPool[rand.Intn(len(clientPool))]

	if page, err := client.CreatePageWithHTML(title, sourceTitle, rawLink, htmlContent, true); err == nil {
		zap.S().Infof("Created telegraph page url: %s", page.URL)
		return page.URL, err
	} else {
		zap.S().Warnf("Create telegraph page failed, error: %s", err)
		return "", nil
	}
}
