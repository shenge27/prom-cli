package prom-cli

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
)

type URL string

func (u *URL) Set(s string) error {
	if s == "" {
		return errors.New("empty string")
	}
	if _, err := url.Parse(s); err != nil {
		return err
	}
	*u = URL(s)
	return nil
}

func (u URL) String() string {
	return string(u)
}

func (u URL) Type() string {
	return "string"
}

type Client struct {
	Token string       // optional
	URL   URL          // required
	HTTP  *http.Client // http.DefaultClient if nil
}

func (c *Client) RegisterFlags(f *pflag.FlagSet) {
	f.StringVarP(&c.Token, "token", "t", "", "Authorization token")
	f.Var(&c.URL, "url", "Remote Storage endpoint")
}

func (c *Client) Read(r *prompb.ReadRequest) (*prompb.ReadResponse, error) {
	req, err := c.MakeRequest(r)
	if err != nil {
		return nil, xerrors.Errorf("error creating request: %s", err)
	}

	resp, err := c.http().Do(req)
	if err != nil {
		return nil, xerrors.Errorf("error sending request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		p, _ := httputil.DumpResponse(resp, true)
		os.Stderr.Write(p)
		return nil, xerrors.Errorf("request failed: %s", http.StatusText(resp.StatusCode))
	}

	var rr prompb.ReadResponse

	p, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, xerrors.Errorf("error reading response: %s", err)
	}

	q, err := snappy.Decode(nil, p)
	if err != nil {
		return nil, xerrors.Errorf("error decompressing with snappy: %s", err)
	}

	if err := proto.Unmarshal(q, &rr); err != nil {
		return nil, xerrors.Errorf("error decoding protobuf: %s", err)
	}

	return &rr, nil
}

func (c *Client) MakeRequest(r *prompb.ReadRequest) (*http.Request, error) {
	p, err := proto.Marshal(r)
	if err != nil {
		return nil, xerrors.Errorf("error marshalling to protobuf: %s", err)
	}

	p = snappy.Encode(nil, p)

	req, err := http.NewRequest("POST", c.URL.String(), bytes.NewReader(p))
	if err != nil {
		return nil, xerrors.Errorf("error creating request: %s", err)
	}

	req.Header.Add("Accept-Encoding", "snappy")
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Add("Content-Type", "application/x-protobuf")
	req.Header.Add("User-Agent", "Prometheus/2.10.0")
	req.Header.Add("X-Prometheus-Remote-Read-Version", "0.1.0")

	if c.Token != "" {
		req.Header.Add("Authorization", "Bearer "+c.Token)
	}

	return req, nil
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
