/*
 * Copyright (c) SAS Institute Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package servecmd

import (
	"errors"

	"gerrit-pdt.unx.sas.com/tools/relic.git/cmdline/shared"
	"gerrit-pdt.unx.sas.com/tools/relic.git/server"
	"github.com/spf13/cobra"
)

var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Offer signing services over a HTTPS API",
	RunE:  serveCmd,
}

type InitHookFunc func(*server.Server) error

var serveCmdInitHooks []InitHookFunc

func AddHook(f InitHookFunc) {
	serveCmdInitHooks = append(serveCmdInitHooks, f)
}

func init() {
	shared.RootCmd.AddCommand(ServeCmd)
}

func serveCmd(cmd *cobra.Command, args []string) error {
	if err := shared.InitConfig(); err != nil {
		return err
	}
	if shared.CurrentConfig.Server == nil {
		return errors.New("Missing server section in configuration file")
	}
	if shared.CurrentConfig.Server.KeyFile == "" {
		return errors.New("Missing keyfile option in server configuration file")
	}
	if shared.CurrentConfig.Server.CertFile == "" {
		return errors.New("Missing certfile option in server configuration file")
	}
	if shared.CurrentConfig.Clients == nil {
		return errors.New("Missing clients section in configuration file")
	}
	if shared.CurrentConfig.Server.Listen == "" {
		shared.CurrentConfig.Server.Listen = ":8888"
	}
	srv := server.New(shared.CurrentConfig)
	for _, hook := range serveCmdInitHooks {
		if err := hook(srv); err != nil {
			return err
		}
	}
	return srv.Serve()
}