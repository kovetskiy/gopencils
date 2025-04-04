// Copyright 2014 Vadim Kravcenko
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Gopencils is a Golang REST Client with which you can easily consume REST API's. Supported Response formats: JSON
package gopencils

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"
)

var ErrCantUseAsQuery = errors.New("can't use options[0] as Query")

// Resource is basically an url relative to given API Baseurl.
type Resource struct {
	Api         *ApiStruct
	Url         string
	id          string
	QueryValues url.Values
	Payload     io.Reader
	Headers     http.Header
	Response    interface{}
	Raw         *http.Response
	Logger      Logger
}

// Creates a new Resource.
func (r *Resource) Res(options ...interface{}) *Resource {
	if len(options) > 0 {
		var url string
		if len(r.Url) > 0 {
			url = r.Url + "/" + options[0].(string)
		} else {
			url = options[0].(string)
		}

		header := r.Headers
		if header == nil {
			header = http.Header{}
		}
		newR := &Resource{
			Url:     url,
			Api:     r.Api,
			Headers: header,
			Logger:  r.Logger,
		}

		if len(options) > 1 {
			newR.Response = options[1]
		}

		return newR
	}
	return r
}

// Same as Res() Method, but returns a Resource with url resource/:id
func (r *Resource) Id(options ...interface{}) *Resource {
	if len(options) > 0 {
		id := ""
		switch v := options[0].(type) {
		default:
			id = v.(string)
		case int:
			id = strconv.Itoa(v)
		case int64:
			id = strconv.FormatInt(v, 10)
		}
		url := r.Url + "/" + id
		header := r.Headers
		if header == nil {
			header = http.Header{}
		}
		newR := &Resource{
			id:       id,
			Url:      url,
			Api:      r.Api,
			Headers:  header,
			Response: &r.Response,
			Logger:   r.Logger,
		}

		if len(options) > 1 {
			newR.Response = options[1]
		}
		return newR
	}
	return r
}

// Sets QueryValues for current Resource
func (r *Resource) SetQuery(querystring map[string]string) *Resource {
	r.QueryValues = make(url.Values)
	for k, v := range querystring {
		r.QueryValues.Set(k, v)
	}
	return r
}

// Performs a GET request on given Resource
// Accepts map[string]string as parameter, will be used as querystring.
func (r *Resource) Get(options ...interface{}) (*Resource, error) {
	if len(options) > 0 {
		if qry, ok := options[0].(map[string]string); ok {
			r.SetQuery(qry)
		} else {
			return nil, ErrCantUseAsQuery
		}
	}
	return r.do("GET")
}

// Performs a HEAD request on given Resource
// Accepts map[string]string as parameter, will be used as querystring.
func (r *Resource) Head(options ...interface{}) (*Resource, error) {
	if len(options) > 0 {
		if qry, ok := options[0].(map[string]string); ok {
			r.SetQuery(qry)
		} else {
			return nil, ErrCantUseAsQuery
		}
	}
	return r.do("HEAD")
}

// Performs a PUT request on given Resource.
// Accepts interface{} as parameter, will be used as payload.
func (r *Resource) Put(options ...interface{}) (*Resource, error) {
	if len(options) > 0 {
		r.Payload = r.SetPayload(options[0])
	}
	return r.do("PUT")
}

// Performs a POST request on given Resource.
// Accepts interface{} as parameter, will be used as payload.
func (r *Resource) Post(options ...interface{}) (*Resource, error) {
	if len(options) > 0 {
		r.Payload = r.SetPayload(options[0])
	}
	return r.do("POST")
}

// Performs a Delete request on given Resource.
// Accepts map[string]string as parameter, will be used as querystring.
func (r *Resource) Delete(options ...interface{}) (*Resource, error) {
	if len(options) > 0 {
		if qry, ok := options[0].(map[string]string); ok {
			r.SetQuery(qry)
		} else {
			return nil, ErrCantUseAsQuery
		}
	}
	return r.do("DELETE")
}

// Performs a Delete request on given Resource.
// Accepts map[string]string as parameter, will be used as querystring.
func (r *Resource) Options(options ...interface{}) (*Resource, error) {
	if len(options) > 0 {
		if qry, ok := options[0].(map[string]string); ok {
			r.SetQuery(qry)
		} else {
			return nil, ErrCantUseAsQuery
		}
	}
	return r.do("OPTIONS")
}

// Performs a PATCH request on given Resource.
// Accepts interface{} as parameter, will be used as payload.
func (r *Resource) Patch(options ...interface{}) (*Resource, error) {
	if len(options) > 0 {
		r.Payload = r.SetPayload(options[0])
	}
	return r.do("PATCH")
}

// Main method, opens the connection, sets basic auth, applies headers,
// parses response json.
func (r *Resource) do(method string) (*Resource, error) {
	url := *r.Api.BaseUrl
	if len(url.Path) > 0 {
		url.Path += "/" + r.Url
	} else {
		url.Path = r.Url
	}
	if r.Api.PathSuffix != "" {
		url.Path += r.Api.PathSuffix
	}

	url.RawQuery = r.QueryValues.Encode()
	req, err := http.NewRequest(method, url.String(), r.Payload)
	if err != nil {
		return r, err
	}

	if r.Api.BasicAuth != nil {
		req.SetBasicAuth(r.Api.BasicAuth.Username, r.Api.BasicAuth.Password)
	}

	if r.Headers != nil {
		for k, _ := range r.Headers {
			req.Header.Set(k, r.Headers.Get(k))
		}
	}

	if r.Logger != nil {
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			r.Logger.Printf("dump request failed: %s", err)
		} else {
			r.Logger.Printf("%s", string(dump))
		}
	}

	var requestBody []byte
	if req.Body != nil {
		// *http.Client.Do(req) will close the req.Body and not allow subsequent reads
		// we must save the body and reuse it manually for retrial logic
		requestBody, _ = io.ReadAll(req.Body)
		req.Body.Close()

		req.Body = io.NopCloser(bytes.NewReader(requestBody)) // restore req.Body after reading + saving it above
		req.ContentLength = int64(len(requestBody))
	}
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	r.Headers.Set("X-Total-Retries", strconv.Itoa(0))

	resp, err := r.Api.Client.Do(req)
	totalRetries := 0

	if err != nil {
		for i := 0; i < r.Api.RetryCount; i++ {
			if i > 0 {
				time.Sleep(time.Duration(math.Pow(2, float64(i))) * time.Second) // exponential backoff - in case requested resource is busy
			}
			if len(requestBody) > 0 {
				req.Body = io.NopCloser(bytes.NewReader(requestBody))
				req.ContentLength = int64(len(requestBody))
			}
			resp, err = r.Api.Client.Do(req)
			totalRetries++
			
			if err == nil && (resp == nil || resp.StatusCode < 500) {
				break
			}
		}
		r.Headers.Set("X-Total-Retries", strconv.Itoa(totalRetries))
		if err != nil {
			return r, err
		}
	}

	r.Raw = resp

	defer resp.Body.Close()

	if r.Logger != nil {
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			r.Logger.Printf("dump response failed: %s", err)
		} else {
			r.Logger.Printf("%s", string(dump))
		}
	}
	
	if resp.StatusCode >= 400 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return r, err
		}
		r.Raw.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // replace body with ReadCloser that can be read again
		return r, nil
	}

	for k, _ := range r.Raw.Header {
		r.SetHeader(k, r.Raw.Header.Get(k))
	}

	if resp.StatusCode == http.StatusNoContent {
		return r, nil
	}

	err = json.NewDecoder(resp.Body).Decode(r.Response)
	if err != nil {
		return r, err
	}

	return r, nil
}

// Sets Payload for current Resource
func (r *Resource) SetPayload(args interface{}) io.Reader {
	var b []byte
	b, _ = json.Marshal(args)
	r.SetHeader("Content-Type", "application/json")
	return bytes.NewBuffer(b)
}

// Sets Headers
func (r *Resource) SetHeader(key string, value string) {
    r.Headers.Add(key, value)
}

// Overwrites the client that will be used for requests.
// For example if you want to use your own client with OAuth2
func (r *Resource) SetClient(c *http.Client) {
	r.Api.Client = c
}
