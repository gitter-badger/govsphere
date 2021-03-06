// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package soap

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

type Envelope struct {
	XMLName       xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	EncodingStyle string   `xml:"http://schemas.xmlsoap.org/soap/encoding/ encodingStyle,attr"`
	Header        Header   `xml:"http://schemas.xmlsoap.org/soap/envelope/ Header"`
	Body          Body     `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
}

type Header struct {
	Header interface{}
}

type Body struct {
	Fault *Fault `xml:"fault,omitempty"` //It has to be a pointer or omitempty doesn't work
	Body  string `xml:",innerxml"`
}

type Fault struct {
	faultcode   string `xml:"http://schemas.xmlsoap.org/soap/envelope/ faultcode"`
	faultstring string `xml:"faultstring"`
	faultactor  string `xml:"faultactor"`
	detail      string `xml:"detail"`
}

type Client struct {
	url, soapAction string
	tls             bool
}

func NewClient(url, soapAction string, tls bool) *Client {
	return &Client{
		url:        url,
		soapAction: soapAction,
		tls:        tls,
	}
}

var timeout = time.Duration(30 * time.Second)

func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, timeout)
}

func (s *Client) Call(request interface{}, response interface{}, cookies []*http.Cookie) ([]*http.Cookie, error) {
	envelope := Envelope{
		Header:        Header{},
		EncodingStyle: "http://schemas.xmlsoap.org/soap/encoding/",
	}

	reqXml, err := xml.MarshalIndent(request, "  ", "    ")
	if err != nil {
		return nil, err
	}

	envelope.Body = Body{
		Body: string(reqXml),
	}

	buffer := &bytes.Buffer{}

	encoder := xml.NewEncoder(buffer)
	encoder.Indent("  ", "    ")

	err = encoder.Encode(envelope)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", s.url, buffer)
	req.Header.Add("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Add("SOAPAction", s.soapAction)
	req.Header.Set("User-Agent", "govsphere/1.0")

	for _, cookie := range cookies {
		if cookie != nil {
			req.AddCookie(cookie)
		}
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: s.tls,
		},
		Dial: dialTimeout,
	}

	client := &http.Client{Transport: tr}

	rawReq, _ := httputil.DumpRequestOut(req, true)
	log.Println("===========REQUEST===========")
	log.Println(string(rawReq) + "\n")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	rawRes, _ := httputil.DumpResponse(res, true)
	log.Println("===========RESPONSE===========")
	log.Println(string(rawRes) + "\n")

	body, err := ioutil.ReadAll(res.Body)

	respEnvelope := &Envelope{}

	err = xml.Unmarshal(body, respEnvelope)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 500 {
		log.Printf("%#v\n", respEnvelope.Body)
	}

	if response == nil {
		return res.Cookies(), nil
	}

	err = xml.Unmarshal([]byte(respEnvelope.Body.Body), response)
	if err != nil {
		return nil, err
	}

	return res.Cookies(), nil
}
