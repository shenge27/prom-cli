package record

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/shenge27/prom-cli/command"
	"github.com/shenge27/prom-cli/command/record/internal/recordutil"
	"golang.org/x/xerrors"

	"github.com/spf13/cobra"
)

func NewCommand(app *command.App) *cobra.Command {
	m := &debugCmd{App: app}

	cmd := &cobra.Command{
		Use:          "record",
		Short:        "recorded Prometheus requests",
		Args:         cobra.ExactArgs(1),
		RunE:         m.run,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		NewReplyCommand(app),
	)

	return cmd
}

type debugCmd struct {
	*command.App
}

func (m *debugCmd) run(cmd *cobra.Command, args []string) error {
	p, err := readFile(args[0])
	if err != nil {
		return err
	}

	req, resp, err := parse(p)
	if err != nil {
		return xerrors.Errorf("parsing error: %s", err)

	}

	if req != nil {
		fmarshal(os.Stderr, req)
	}

	if resp != nil {
		fmarshal(os.Stdout, resp)
	}

	return nil
}

func parse(p []byte) (*prompb.ReadRequest, *prompb.ReadResponse, error) {
	var r recordutil.Record
	var req prompb.ReadRequest
	var resp prompb.ReadResponse

	if json.Unmarshal(p, &r) == nil {
		if len(r.Request.Body) != 0 {
			if err := decode(r.Request.Body, &req); err != nil {
				return nil, nil, xerrors.Errorf("error decoding request: %s", err)
			}
		}

		if len(r.Response.Body) != 0 {
			if err := decode(r.Response.Body, &resp); err != nil {
				return nil, nil, xerrors.Errorf("error decoding response: %s", err)
			}
		}

		return &req, &resp, nil
	}

	if decode(p, &req) == nil {
		return &req, nil, nil
	}

	if decode(p, &resp) == nil {
		return nil, &resp, nil
	}

	return nil, nil, xerrors.New("unable to decode data")
}

func readFile(file string) ([]byte, error) {
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

func decode(p []byte, msg proto.Message) error {
	q, err := snappy.Decode(nil, p)
	if err != nil {
		return err
	}
	return proto.Unmarshal(q, msg)
}

func fmarshal(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func nonil(err ...error) error {
	for _, e := range err {
		if e != nil {
			return e
		}
	}
	return nil
}
