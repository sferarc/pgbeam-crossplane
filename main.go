package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/sferarc/pgbeam-crossplane/apis/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pgbeamcontroller "github.com/sferarc/pgbeam-crossplane/internal/controller"

	// Import k8s auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type options struct {
	Debug            bool
	SyncPeriod       time.Duration
	PollInterval     time.Duration
	MaxReconcileRate int
	LeaderElection   bool
}

func parseOptions(name string, args []string, output io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	fs.BoolVar(&o.Debug, "debug", false, "Run with debug logging.")
	fs.DurationVar(&o.SyncPeriod, "sync", time.Hour, "How often all resources are re-listed from the API server.")
	fs.DurationVar(&o.PollInterval, "poll", time.Minute, "How often each managed resource is checked for drift.")
	fs.IntVar(&o.MaxReconcileRate, "max-reconcile-rate", 10, "Global maximum rate of reconciles per second.")
	fs.BoolVar(&o.LeaderElection, "leader-election", false, "Use leader election so only one replica reconciles at a time.")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if o.SyncPeriod <= 0 || o.PollInterval <= 0 || o.MaxReconcileRate <= 0 {
		return options{}, errors.New("--sync, --poll and --max-reconcile-rate must be positive")
	}
	return o, nil
}

func main() {
	opts, err := parseOptions(filepath.Base(os.Args[0]), os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	zl := zap.New(zap.UseDevMode(opts.Debug))
	ctrl.SetLogger(zl)
	log := logging.NewLogrLogger(zl.WithName("provider-pgbeam"))

	// crossplane-runtime's logger has no error level, so fatal errors go out at Info.
	fatal := func(msg string, err error) {
		log.Info(msg, "error", err)
		os.Exit(1)
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		fatal("Cannot get Kubernetes config", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   opts.LeaderElection,
		LeaderElectionID: "crossplane-provider-pgbeam." + v1alpha1.Group,
		Cache: cache.Options{
			SyncPeriod: &opts.SyncPeriod,
		},
	})
	if err != nil {
		fatal("Cannot create controller manager", err)
	}

	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		fatal("Cannot add PgBeam API types to scheme", err)
	}

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: 1,
		PollInterval:            opts.PollInterval,
		Features:                &feature.Flags{},
		GlobalRateLimiter:       ratelimiter.NewGlobal(opts.MaxReconcileRate),
	}

	if err := pgbeamcontroller.Setup(mgr, o); err != nil {
		fatal("Cannot setup controllers", err)
	}

	log.Info("Starting provider", "name", filepath.Base(os.Args[0]), "sync", opts.SyncPeriod, "poll", opts.PollInterval, "maxReconcileRate", opts.MaxReconcileRate, "leaderElection", opts.LeaderElection)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fatal("Cannot start controller manager", err)
	}
}
