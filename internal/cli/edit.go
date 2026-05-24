package cli

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/editor"
)

// newEditCmd starts the web-based game editor ("lono studio"). Unlike the other
// commands it is long-running and prints human-readable status rather than a
// JSON envelope.
func (a *app) newEditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "edit",
		Short: "Launch the web game editor (lono studio)",
		Long: "Start a local web app for authoring lono game definitions. It reads and\n" +
			"writes *.lono.json files in --dir, validates them with the engine, and lets\n" +
			"you playtest a game without leaving the editor.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			dir, _ := cmd.Flags().GetString("dir")
			noOpen, _ := cmd.Flags().GetBool("no-open")

			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if info, err := os.Stat(abs); err != nil || !info.IsDir() {
				return fmt.Errorf("--dir %q is not a directory", dir)
			}

			srv := editor.NewServer(abs)
			addr := fmt.Sprintf("127.0.0.1:%d", port)
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("cannot listen on %s: %w (try --port)", addr, err)
			}
			url := fmt.Sprintf("http://localhost:%d", port)

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "lono studio is running.\n\n")
			fmt.Fprintf(out, "  editor:  %s\n", url)
			fmt.Fprintf(out, "  editing: %s/*.lono.json\n\n", abs)
			fmt.Fprintf(out, "Press Ctrl+C to stop.\n")

			if !noOpen {
				go func() { time.Sleep(300 * time.Millisecond); openBrowser(url) }()
			}

			httpSrv := &http.Server{Handler: srv.Handler()}
			return httpSrv.Serve(ln)
		},
	}
	c.Flags().IntP("port", "p", 4321, "port to listen on")
	c.Flags().String("dir", ".", "directory holding *.lono.json files")
	c.Flags().Bool("no-open", false, "do not open a browser automatically")
	return c
}

// openBrowser best-effort opens url in the default browser for the platform.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}
