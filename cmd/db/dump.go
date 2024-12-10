// SPDX-FileCopyrightText: (C) 2019 Grendel Authors
//
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"github.com/ubccr/grendel/cmd"
	"github.com/ubccr/grendel/pkg/model"
)

var (
	dumpCmd = &cobra.Command{
		Use:   "dump",
		Short: "Dump database",
		Long:  `Dump database`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(command *cobra.Command, args []string) error {
			gc, err := cmd.NewClient()
			if err != nil {
				return err
			}

			hostList, _, err := gc.HostApi.HostList(context.Background())
			if err != nil {
				return cmd.NewApiError("Failed to list hosts", err)
			}

			imageList, _, err := gc.ImageApi.ImageList(context.Background())
			if err != nil {
				return cmd.NewApiError("Failed to list images", err)
			}

			userList, _, err := gc.UserApi.UserList(context.Background())
			if err != nil {
				return cmd.NewApiError("Failed to list users", err)
			}

			data := model.DataDump{
				Hosts:  hostList,
				Images: imageList,
				Users:  userList,
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "    ")
			if err := enc.Encode(data); err != nil {
				return err
			}

			return nil

		},
	}
)

func init() {
	dbCmd.AddCommand(dumpCmd)
}
