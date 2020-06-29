package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"

	"github.com/prometheus/prometheus/prompb"
	"github.com/shenge27/prom-cli"
	"github.com/shenge27/prom-cli/command"
	"github.com/shenge27/prom-cli/command/record"
	"golang.org/x/xerrors"

	"github.com/spf13/cobra"
)

func NewCommand(app *command.App) *cobra.Command {
	m := &promreadCmd{App: app}

	cmd := &cobra.Command{
		Use:          "prom-cli",
		Short:        "CLI for Promtheus Remote storage",
		RunE:         m.run,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		record.NewCommand(app),
	)

	app.Register(cmd)
	m.register(cmd)

	return cmd
}

type promreadCmd struct {
	*command.App
	promread.Client
	input string
	curl  bool
}

func (m *promreadCmd) register(cmd *cobra.Command) {
	f := cmd.Flags()

	m.Client.RegisterFlags(f)
	f.StringVarP(&m.input, "input", "i", "", "JSON-encoded Remote Storage request")
	f.BoolVar(&m.curl, "curl", false, "Whether to build curl command line only")

	cmd.MarkFlagRequired("url")
	cmd.MarkFlagRequired("input")
}

func (m *promreadCmd) run(*cobra.Command, []string) error {
	p, err := m.readFile(m.input)
	if err != nil {
		return xerrors.Errorf("error reading file: %s", err)
	}

	req := new(prompb.ReadRequest)

	if err := json.Unmarshal(p, req); err != nil {
		return xerrors.Errorf("error unmarshalling: %s", err)
	}

	if m.curl {
		return m.buildCurl(req)
	}

	resp, err := m.Client.Read(req)
	if err != nil {
		return xerrors.Errorf("error calling api: %s", err)
	}

	return jsonNewEncoder(os.Stdout).Encode(resp)
}

func (m *promreadCmd) buildCurl(req *prompb.ReadRequest) error {
	r, err := m.Client.MakeRequest(req)
	if err != nil {
		return xerrors.Errorf("error creating request: %s", err)
	}

	p, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return xerrors.Errorf("error reading body: %s", err)
	}

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "echo '%s' | base64 -d | ", base64.StdEncoding.EncodeToString(p))
	fmt.Fprintf(&buf, "curl --show-error --silent --location --connect-timeout 10 --max-time 60 --fail --data-binary @-")
	fmt.Fprintf(&buf, " -X %s", r.Method)

	var headers []string

	for k := range r.Header {
		headers = append(headers, fmt.Sprintf("%s: %s", k, r.Header.Get(k)))
	}

	sort.Strings(headers)

	for _, h := range headers {
		fmt.Fprintf(&buf, " -H '%s'", h)
	}

	fmt.Fprintf(&buf, " %s > request.pb.snappy", r.URL.String())

	fmt.Println(buf.String())

	return nil
}

func (m *promreadCmd) readFile(file string) ([]byte, error) {
	switch stat, _ := os.Stdin.Stat(); {
	case file == "-":
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			return ioutil.ReadAll(os.Stdin)
		}
		return nil, xerrors.New("no data being piped")
	default:
		return ioutil.ReadFile(file)
	}
}

func jsonNewEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	enc.SetEscapeHTML(false)
	return enc
}
