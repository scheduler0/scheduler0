package cmd

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	nat "github.com/docker/go-connections/nat"
	"github.com/spf13/cobra"
	"os"
	server "scheduler0/server/src"
	"scheduler0/server/src/utils"
	"time"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a local version of the server",
	Long:  `The would run the server on your`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Setenv("POSTGRES_ADDRESS", "localhost:5432")
		os.Setenv("POSTGRES_DATABASE", "scheduler0_test")
		os.Setenv("POSTGRES_USER", "core")
		os.Setenv("POSTGRES_PASSWORD", "localdev")

		utils.Info("Starting Database.")

		client, err := client.NewClientWithOpts()
		if err != nil {
			utils.Error(err.Error())
			return
		}

		filterArgs := filters.NewArgs(filters.Arg("name", "scheduler_0_postgres"))

		containers, err := client.ContainerList(context.Background(), types.ContainerListOptions{
			All:     true,
			Filters: filterArgs,
		})

		if err != nil {
			utils.Error(err.Error())
			return
		}

		if len(containers) < 1 {
			_, err = client.ContainerCreate(
				context.Background(),
				&container.Config{
					Hostname: "0.0.0.0",
					Image:    "postgres:13-alpine",
					ExposedPorts: nat.PortSet{
						"5432": struct{}{},
					},
					Env: []string{
						fmt.Sprintf("POSTGRES_ADDRESS=%s", os.Getenv("POSTGRES_ADDRESS")),
						fmt.Sprintf("POSTGRES_DB=%s", os.Getenv("POSTGRES_DATABASE")),
						fmt.Sprintf("POSTGRES_USER=%s", os.Getenv("core")),
						fmt.Sprintf("POSTGRES_PASSWORD=%s", os.Getenv("POSTGRES_PASSWORD")),
					},
				},
				&container.HostConfig{
					PortBindings: nat.PortMap{
						"5432": []nat.PortBinding{{
							HostIP:   "0.0.0.0",
							HostPort: "5432",
						}},
					},
				},
				&network.NetworkingConfig{},
				nil,
				"scheduler_0_postgres",
			)
			if err != nil {
				utils.Error(err.Error())
				return
			}
		}

		err = client.ContainerStart(context.Background(), "scheduler_0_postgres", types.ContainerStartOptions{})
		if err != nil {
			utils.Error(err.Error())
			return
		}

		time.Sleep(5 * time.Second)
		utils.Info("Starting Server.")
		server.Start()
	},
}
