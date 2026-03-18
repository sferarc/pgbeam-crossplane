package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/pgbeam/provider-pgbeam/apis/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pgbeamcontroller "github.com/pgbeam/provider-pgbeam/internal/controller"

	// Import k8s auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	var (
		syncPeriod     = time.Hour
		leaderElection = false
	)

	zl := zap.New(zap.UseDevMode(true))
	ctrl.SetLogger(zl)
	log := logging.NewLogrLogger(zl.WithName("provider-pgbeam"))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Debug("Cannot get Kubernetes config", "error", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   leaderElection,
		LeaderElectionID: "crossplane-provider-pgbeam." + v1alpha1.Group,
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
	})
	if err != nil {
		log.Debug("Cannot create controller manager", "error", err)
		os.Exit(1)
	}

	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Debug("Cannot add PgBeam API types to scheme", "error", err)
		os.Exit(1)
	}

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: 1,
		PollInterval:            time.Minute,
		Features:                &feature.Flags{},
		GlobalRateLimiter:       ratelimiter.NewGlobal(10),
	}

	if err := pgbeamcontroller.Setup(mgr, o); err != nil {
		log.Debug("Cannot setup controllers", "error", err)
		os.Exit(1)
	}

	log.Debug("Starting provider", "name", filepath.Base(os.Args[0]))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Debug("Cannot start controller manager", "error", err)
		os.Exit(1)
	}
}
