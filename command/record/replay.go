package record

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/shenge27/prom-cli/command"
	"github.com/shenge27/prom-cli/command/record/internal/recordutil"
	"golang.org/x/xerrors"

	"github.com/spf13/cobra"
)

func NewReplyCommand(app *command.App) *cobra.Command {
	m := &replayCmd{App: app}

	cmd := &cobra.Command{
		Use:          "replay",
		Short:        "Debug recorded Prometheus requests",
		Args:         cobra.MinimumNArgs(1),
		RunE:         m.run,
		SilenceUsage: true,
	}

	m.register(cmd)

	return cmd
}

type replayCmd struct {
	*command.App
	duration time.Duration
	timeout  time.Duration
	parallel int
}

func (m *replayCmd) register(cmd *cobra.Command) {
	f := cmd.Flags()

	f.DurationVar(&m.timeout, "timeout", 60*time.Second, "Max round trip time")
	f.DurationVar(&m.duration, "duration", 0, "Keep sending request over a period of time")
	f.IntVarP(&m.parallel, "parallel", "p", 1, "Controls parallelism level for requests")
}

func (m *replayCmd) run(cmd *cobra.Command, args []string) error {
	var records []recordutil.Record
	var wg sync.WaitGroup

	log.Printf("# reading records from %d archives ...", len(args))

	for _, path := range args {
		rec, err := recordutil.ReadArchive(path)
		if err != nil {
			return err
		}

		records = append(records, rec...)
	}

	log.Printf("# read %d records ...", len(records))
	log.Printf("# warming up with %d workers ...", m.parallel)

	client := &http.Client{Timeout: m.timeout}
	work := make(chan recordutil.Record, len(records))

	for _, rec := range records {
		work <- rec
	}

	ctx := m.App.Context

	if m.duration != 0 {
		var cancel func()

		ctx, cancel = context.WithTimeout(ctx, m.duration)
		defer cancel()

		go func() {
			defer close(work)

			for i := 0; ; i = (i + 1) % len(records) {
				select {
				case <-ctx.Done():
					return
				default:
					work <- records[i]
				}
			}
		}()
	} else {
		close(work)
	}

	for i := 0; i < m.parallel; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case rec, ok := <-work:
					if !ok {
						return
					}

					if err := replay(ctx, client, &rec); err != nil {
						log.Println(err)
					}
				}
			}
		}()
	}

	log.Printf("# replaying requests ...")

	wg.Wait()

	return nil
}

func replay(ctx context.Context, client *http.Client, r *recordutil.Record) error {
	req := r.Request.Request().WithContext(ctx)

	log.Printf("# sending request to %s (%dB) ...", req.URL, len(r.Request.Body))

	resp, err := client.Do(req)
	if err != nil {
		return xerrors.Errorf("error sending request: %s", err)
	}

	n, err := io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	log.Printf("# received response %d (%dB)", resp.StatusCode, n)

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return xerrors.Errorf("response error: %s", resp.Status)
	}

	if err != nil {
		return xerrors.Errorf("error reading response: %s", err)
	}

	return nil
}
