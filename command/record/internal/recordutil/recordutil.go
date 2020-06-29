package recordutil

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"golang.org/x/xerrors"
)

type Request struct {
	Method  string      `json:"method,omitempty"`
	URL     string      `json:"url,omitempty"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body,omitempty"`
}

func (r *Request) Request() *http.Request {
	req, err := http.NewRequest(r.Method, r.URL, bytes.NewReader(r.Body))
	if err != nil {
		panic("unexpected request error: " + err.Error())
	}
	req.Header = r.Headers
	return req
}

type Response struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body,omitempty"`
}

func (r *Response) Response() *http.Response {
	return &http.Response{
		Status:     http.StatusText(r.Status),
		StatusCode: r.Status,
		Header:     r.Headers,
		Body:       ioutil.NopCloser(bytes.NewReader(r.Body)),
	}
}

type Record struct {
	Request  Request   `json:"request"`
	Response Response  `json:"response"`
	Modtime  time.Time `json:"-"`
}

func OpenArchive(path string) (*zip.Reader, error) {
	var r *bytes.Reader

	u, err := url.Parse(path)
	if err == nil {
		switch u.Scheme {
		case "http", "https":
			resp, err := http.Get(u.String())
			if err != nil {
				return nil, xerrors.Errorf("error retrieving %s: %s", path, err)
			}

			if resp.StatusCode != 200 {
				err := http.StatusText(resp.StatusCode)
				return nil, nonil(xerrors.Errorf("error retrieving %s: %s", path, err), resp.Body.Close())
			}

			p, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, nonil(xerrors.Errorf("error reading body of %s: %s", path, err), resp.Body.Close())
			}

			r = bytes.NewReader(p)
		case "s3":
			s, err := newSession()
			if err != nil {
				return nil, xerrors.Errorf("error creating S3 session: %s", err)
			}

			var buf aws.WriteAtBuffer

			input := &s3.GetObjectInput{
				Bucket: &u.Host,
				Key:    &u.Path,
			}

			_, err = s3manager.NewDownloader(s).Download(&buf, input)
			if err != nil {
				return nil, xerrors.Errorf("error downloading from S3: %s", err)
			}

			ioutil.WriteFile("debug.zip", buf.Bytes(), 0644)

			r = bytes.NewReader(buf.Bytes())
		default:
			return nil, xerrors.Errorf("unsupported scheme: %q", u.Scheme)
		}
	} else {
		p, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, xerrors.Errorf("error opening %s: %s", path, err)
		}

		r = bytes.NewReader(p)
	}

	return zip.NewReader(r, r.Size())
}

func ReadArchive(path string) ([]Record, error) {
	zr, err := OpenArchive(path)
	if err != nil {
		return nil, xerrors.Errorf("error opening archive %s: %s", path, err)
	}

	var rec []Record

	for _, f := range zr.File {
		r := Record{Modtime: f.Modified}

		rc, err := f.Open()
		if err != nil {
			return nil, xerrors.Errorf("error opening archive file %s: %s", f.Name, err)
		}

		p, err := ioutil.ReadAll(rc)
		rc.Close()
		if p = bytes.TrimSpace(p); len(p) == 0 {
			continue // ignore empty files
		}
		if err != nil {
			return nil, xerrors.Errorf("error reading archive file %s: %s", f.Name, err)
		}

		if err := json.Unmarshal(p, &r); err != nil {
			return nil, xerrors.Errorf("error unmarshaling archive file %s: %s", f.Name, err)
		}

		rec = append(rec, r)
	}

	sort.Slice(rec, func(i, j int) bool {
		return rec[i].Modtime.Before(rec[j].Modtime)
	})

	return rec, nil
}

func ReadFile(file string) ([]byte, error) {
	switch stat, _ := os.Stdin.Stat(); {
	case file == "-":
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			return ioutil.ReadAll(os.Stdin)
		}
		return nil, errors.New("no data being piped")
	default:
		return ioutil.ReadFile(file)
	}
}

var metadataSession = session.Must(session.NewSession(&aws.Config{}))

func newSession() (*session.Session, error) {
	return session.NewSession(&aws.Config{
		Region: aws.String(nonempty(os.Getenv("AWS_REGION"), "us-east-1")),
		Credentials: credentials.NewChainCredentials([]credentials.Provider{
			&credentials.EnvProvider{},
			&ec2rolecreds.EC2RoleProvider{
				Client: ec2metadata.New(metadataSession),
			},
		}),
	})
}

func nonil(err ...error) error {
	for _, e := range err {
		if e != nil {
			return e
		}
	}
	return nil
}

func nonempty(s ...string) string {
	for _, s := range s {
		if s != "" {
			return s
		}
	}
	return ""
}
