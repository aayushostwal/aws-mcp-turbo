package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aayushostwal/aws-mcp-turbo/internal/turbo"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("aws-mcp-turbo", "error", err)
		os.Exit(1)
	}
}

func run() error {
	profile := flag.String("profile", "", "AWS shared profile (otherwise SDK default chain)")
	region := flag.String("region", "", "AWS region (otherwise SDK configuration)")
	enable := flag.Bool("enable-mutations", false, "enable audited mutation previews and approvals")
	auditPath := flag.String("audit-file", "", "private append-only JSONL mutation audit file")
	hookPath := flag.String("approval-hook", "", "absolute executable; JSON on stdin, approved:true on stdout")
	timeout := flag.Duration("timeout", 30*time.Second, "maximum AWS query/macro/mutation duration")
	maxBytes := flag.Int("max-output-bytes", 32768, "maximum bytes in each tool result")
	showVersion := flag.Bool("version", false, "print version and exit")
	schemas := flag.Bool("print-tool-schema", false, "print compact tool schemas and exit")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if *schemas {
		return json.NewEncoder(os.Stdout).Encode(turbo.ToolDefinitions())
	}
	if *timeout <= 0 || *maxBytes < 1024 || *maxBytes > 1024*1024 {
		return fmt.Errorf("timeout must be positive; max-output-bytes must be 1024..1048576")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var opts []func(*config.LoadOptions) error
	if *profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(*profile))
	}
	if *region != "" {
		opts = append(opts, config.WithRegion(*region))
	}
	opts = append(opts, config.WithRetryMaxAttempts(3))
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("load AWS configuration: %w", err)
	}
	e := turbo.NewAWSEngine(cfg)
	e.MaxBytes = *maxBytes
	e.Timeout = *timeout
	if *enable {
		if *auditPath == "" {
			return fmt.Errorf("--enable-mutations requires --audit-file")
		}
		f, err := turbo.OpenAudit(*auditPath)
		if err != nil {
			return err
		}
		defer f.Close()
		e.Mutations = &turbo.MutationGuard{Enabled: true, Audit: f}
		if *hookPath != "" {
			hook, err := turbo.Hook(*hookPath)
			if err != nil {
				return err
			}
			e.Mutations.Approve = hook
		}
	} else if *auditPath != "" || *hookPath != "" {
		return fmt.Errorf("mutation flags require --enable-mutations")
	}
	return turbo.NewServer(e, version).Run(ctx, &mcp.StdioTransport{})
}
