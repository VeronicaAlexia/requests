package requests

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

func (c *Client) SetProxy(proxy *Proxy) *Client {
	if proxy == nil {
		c.errorArray = append(c.errorArray, fmt.Errorf("error: %s", "proxy is nil"))
		return c
	}
	value := reflect.ValueOf(proxy).Elem()
	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).String() == "" {
			c.errorArray = append(c.errorArray, fmt.Errorf("error: %s", "proxy field is empty"))
			return c
		}
	}
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s", proxy.Ip, proxy.Port))
	if err != nil {
		c.errorArray = append(c.errorArray, err)
	} else {
		proxyURL.User = url.UserPassword(proxy.UserName, proxy.Password)
		c.httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}
	return c
}

func (c *Client) Request() *Client {
	var err error
	if c.method == "" {
		c.method = http.MethodGet
	}
	if c.method == http.MethodGet {
		c.httpRequest, err = http.NewRequest(http.MethodGet, c.GetUrl(), nil)
		c.httpRequest.URL.RawQuery = c.dataForm.Encode()
	} else {
		c.httpRequest, err = http.NewRequest(c.method, c.GetUrl(), c.jsonData)
	}
	c.httpRequest.Header = c.httpHeaders
	c.httpRequest.AddCookie(c.Cookie)
	if err != nil {
		c.errorArray = append(c.errorArray, err)
	}
	return c
}

func (c *Client) Send() HttpResultInterface {
	resp, err := c.httpClient.Do(c.httpRequest)
	if err != nil {
		c.errorArray = append(c.errorArray, err)
	} else {
		return &Response{Body: resp.Body, Resp: resp}
	}
	return nil
}

func (c *Client) Stream() chan []byte {
	streamChan := make(chan []byte)
	resp, ok := c.httpClient.Do(c.httpRequest)
	if ok != nil {
		c.errorArray = append(c.errorArray, ok)
	} else {
		go func() {
			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						break
					}
					c.errorArray = append(c.errorArray, err)
				}
				streamChan <- line
			}
		}()
	}
	return streamChan
}
func (c *Client) NewRequest() HttpResultInterface {
	defer func() {
		if recover() != nil {
			for _, errorInfo := range c.errorArray {
				fmt.Printf("error: %s\n", errorInfo.Error())
			}
		}
	}()
	res := c.Request().Send()
	if res != nil {
		return res
	}
	c.errorArray = append(c.errorArray, fmt.Errorf("error: %s", "send request failed"))
	for _, errorInfo := range c.errorArray {
		fmt.Printf("error: %s\n", errorInfo.Error())
	}
	return HttpResultInterface(nil)
}

func (c *Client) SetCookie(cookie map[string]string) *Client {
	for k, v := range cookie {
		c.Cookie = &http.Cookie{Name: k, Value: v}
	}
	return c
}

func (c *Client) NewUpdateFile(readFile []byte) HttpResultInterface {
	res, err := http.Post(c.GetUrl(), "multipart/form-data", bytes.NewReader(readFile))
	if err != nil {
		c.errorArray = append(c.errorArray, err)
	} else {
		return &Response{Body: res.Body, Resp: res}
	}
	return HttpResultInterface(nil)
}
func (c *Client) Query(data interface{}) *Client {
	switch dataAny := data.(type) {
	case url.Values:
		c.Header("Content-Type", ContentTypeForm)
		for k, v := range dataAny {
			c.dataForm.Set(k, v[0])
		}
		c.jsonData = strings.NewReader(c.dataForm.Encode())
	case map[string]interface{}:
		c.Header("Content-Type", ContentTypeForm)
		for k, v := range dataAny {
			c.dataForm.Set(k, fmt.Sprintf("%v", v))
		}
		c.jsonData = strings.NewReader(c.dataForm.Encode())
	case string:
		if dataAny[:1] == "{" && dataAny[len(dataAny)-1:] == "}" {
			c.jsonData = strings.NewReader(dataAny)
			c.Header("Content-Type", ContentTypeJson)
		}
	default:
		if reflect.ValueOf(data).Kind() == reflect.Struct {
			if jsonData, err := json.Marshal(data); err != nil {
				c.errorArray = append(c.errorArray, err)
			} else {
				c.jsonData = bytes.NewReader(jsonData)
				c.Header("Content-Type", ContentTypeJson)
			}
		}
	}
	return c
}

func (c *Client) QueryFunc(f func(c *Client) interface{}) *Client {
	if data := f(c); data != nil {
		c.Query(data)
	} else {
		c.errorArray = append(c.errorArray, fmt.Errorf("error: %s", "QueryFunc return nil"))
	}
	return c
}

func (c *Client) Header(k string, value interface{}) *Client {
	c.httpHeaders.Set(k, fmt.Sprintf("%v", value))
	return c
}

func (c *Client) Headers(m map[string]interface{}) *Client {
	for k, v := range m {
		c.Header(k, v)
	}
	return c
}
func (c *Client) HeadersFunc(f func(c *Client)) *Client {
	f(c)
	return c
}
func (c *Client) UrlSite(urlSite string) *Client {
	if !strings.Contains(urlSite, "http") {
		panic("urlSite error: " + urlSite + " is not support")
	}
	c.urlSite = urlSite
	return c
}

func (c *Client) UrlPoint(urlPoint string) *Client {
	c.urlPoint = urlPoint
	return c
}
func (c *Client) GetUrl() string {
	if strings.TrimSpace(c.urlPoint) != "" {
		if c.urlSite[len(c.urlSite)-1:] != "/" && c.urlPoint[:1] != "/" {
			c.errorArray = append(c.errorArray, fmt.Errorf("urlSite error: %s%s is not support", c.urlSite, c.urlPoint))
			c.urlPoint = "/" + c.urlPoint
		}
	}
	return c.urlSite + c.urlPoint
}
func (c *Client) PostMethod() *Client {
	return c.Method(http.MethodPost)
}

func (c *Client) GetMethod() *Client {
	return c.Method(http.MethodGet)
}

func (c *Client) PutMethod() *Client {
	return c.Method(http.MethodPut)
}

func (c *Client) Method(method string) *Client {
	if strings.Contains(method, strings.Join([]string{http.MethodPost, http.MethodGet, http.MethodPut}, "")) {
		panic("method error: " + method + " is not support")
	}
	c.method = method
	return c
}
