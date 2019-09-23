package main

import (
	"context"
	"flag"
	"os"

	"github.com/meglory/k8s-secret-sync-operator/pkg/controller"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var log = logf.Log.WithName("cmd")

func main() {

	pflag.CommandLine.AddFlagSet(zap.FlagSet())
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	logf.SetLogger(zap.Logger())

	ctx := context.Background()

	// leader elect
	if err := leader.Become(ctx, "k8s-secret-sync-operator-lock"); err != nil {
		log.Error(err, "leader elect error")
		os.Exit(1)
	}

	// 获取kubeconfig
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("adding scheme.")

	// 添加scheme
	if err := controller.AddScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "add scheme error")
		os.Exit(1)
	}

	log.Info("registering controllers.")

	// 注册controller
	if err := controller.RegisterToManager(mgr); err != nil {
		log.Error(err, "register controller error")
		os.Exit(1)
	}

	log.Info("starting manager.")

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "manager start error")
		os.Exit(1)
	}
}
