// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/cri"
	cli "github.com/virtual-kubelet/node-cli"
	logruscli "github.com/virtual-kubelet/node-cli/logrus"
	opencensuscli "github.com/virtual-kubelet/node-cli/opencensus"
	"github.com/virtual-kubelet/node-cli/opts"
	"github.com/virtual-kubelet/node-cli/provider"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"github.com/virtual-kubelet/virtual-kubelet/trace/opencensus"
)

var (
	buildVersion = "N/A"
	buildTime    = "N/A"
	k8sVersion   = "v1.15.2" // This should follow the version of k8s.io/kubernetes we are importing
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = cli.ContextWithCancelOnSignal(ctx)

	logger := logrus.StandardLogger()
	log.L = logruslogger.FromLogrus(logrus.NewEntry(logger))
	logConfig := &logruscli.Config{LogLevel: "info"}

	trace.T = opencensus.Adapter{}
	traceConfig := opencensuscli.Config{
		AvailableExporters: map[string]opencensuscli.ExporterInitFunc{
			"ocagent": initOCAgent,
		},
	}

	o := opts.New()
	o.Provider = "cri"
	o.Version = strings.Join([]string{k8sVersion, "vk-cri", buildVersion}, "-")

	// XXX: set the node name to the local hostname
	cmd := exec.Command("scutil", "--get", "LocalHostName")
	localHostName, err := cmd.Output()
	if err != nil {
		log.G(ctx).Fatal(err)
	}
	o.NodeName = strings.TrimSpace(string(localHostName))
	o.DisableTaint = true

	node, err := cli.New(ctx,
		cli.WithBaseOpts(o),
		cli.WithCLIVersion(buildVersion, buildTime),
		cli.WithProvider("cri", func(cfg provider.InitConfig) (provider.Provider, error) {
			return cri.NewProvider(cfg.NodeName, cfg.OperatingSystem, cfg.InternalIP, cfg.ResourceManager, cfg.DaemonPort)
		}),
		cli.WithPersistentFlags(logConfig.FlagSet()),
		cli.WithPersistentPreRunCallback(func() error {
			return logruscli.Configure(logConfig, logger)
		}),
		cli.WithPersistentFlags(traceConfig.FlagSet()),
		cli.WithPersistentPreRunCallback(func() error {
			return opencensuscli.Configure(ctx, &traceConfig, o)
		}),
	)
	if err != nil {
		log.G(ctx).Fatal(err)
	}

	if err := node.Run(ctx); err != nil {
		log.G(ctx).Fatal(err)
	}
}
