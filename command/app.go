package command

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type App struct {
	context.Context

	// Global flags.
	Format string // -f|--format
}

func (app *App) Register(cmd *cobra.Command) {
	f := cmd.PersistentFlags()

	f.StringVarP(&app.Format, "format", "f", "json", "Format type of output")
}

func (app *App) Render(v interface{}) error {
	switch strings.ToLower(app.Format) {
	case "json":
		return jsonNewEncoder(os.Stdout).Encode(v)
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(v)
	default:
		return errors.New(`unexpected format: "` + app.Format + `"`)
	}
}

func jsonNewEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	return enc
}
