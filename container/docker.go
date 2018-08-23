package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func StartGogs(dockerApiVersion string) string {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithVersion(dockerApiVersion))

	failFast(err)

	pullImage("jerdct/dahu-gogs", cli, ctx)

	internalSshPort, _ := nat.NewPort("tcp", "22")
	internalHttpPort, _ := nat.NewPort("tcp", "3000")

	exposedPorts := nat.PortSet{
		internalSshPort:  {},
		internalHttpPort: {},
	}

	containerConf := &container.Config{Image: "jerdct/dahu-gogs", ExposedPorts: exposedPorts}

	externalSshPort := nat.PortBinding{HostIP: "0.0.0.0", HostPort: "10022"}
	externalHttpPort := nat.PortBinding{HostIP: "0.0.0.0", HostPort: "10080"}

	portBindings := nat.PortMap{
		internalSshPort:  []nat.PortBinding{externalSshPort},
		internalHttpPort: []nat.PortBinding{externalHttpPort},
	}

	hostConfig := &container.HostConfig{PortBindings: portBindings}

	networkConfig := &network.NetworkingConfig{}

	var createdContainer container.ContainerCreateCreatedBody
	createdContainer, err = cli.ContainerCreate(ctx, containerConf, hostConfig, networkConfig, "gogs_for_test")

	failFast(err)

	err = cli.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})

	failFast(err)

	waitForService("10080")

	return createdContainer.ID
}

func StartDockerRegistry(dockerApiVersion string) string {

	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithVersion(dockerApiVersion))

	failFast(err)

	pullImage("jerdct/dahu-docker-registry", cli, ctx)

	internalPort, _ := nat.NewPort("tcp", "5000")

	exposedPorts := nat.PortSet{
		internalPort: {},
	}

	containerConf := &container.Config{Image: "jerdct/dahu-docker-registry", ExposedPorts: exposedPorts}

	externalPort := nat.PortBinding{HostIP: "0.0.0.0", HostPort: "5000"}

	portBindings := nat.PortMap{
		internalPort: []nat.PortBinding{externalPort},
	}

	hostConfig := &container.HostConfig{PortBindings: portBindings}

	networkConfig := &network.NetworkingConfig{}

	var createdContainer container.ContainerCreateCreatedBody
	createdContainer, err = cli.ContainerCreate(ctx, containerConf, hostConfig, networkConfig, "docker_registry_for_test")

	failFast(err)

	err = cli.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})

	failFast(err)

	waitForService("5000")

	return createdContainer.ID
}

func StopContainer(id string, dockerApiVersion string) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithVersion(dockerApiVersion))

	failFast(err)

	removeOpt := types.ContainerRemoveOptions{Force: true, RemoveVolumes: true}

	err = cli.ContainerRemove(ctx, id, removeOpt)

	failFast(err)
}

type ContainerDetail struct {
	Ip string
}

func FindContainerDetails(id string, dockerApiVersion string) ContainerDetail {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithVersion(dockerApiVersion))
	failFast(err)

	var inspectResult types.ContainerJSON
	inspectResult, err = cli.ContainerInspect(ctx, id)
	failFast(err)

	return ContainerDetail{Ip: inspectResult.NetworkSettings.IPAddress}
}

func waitForService(tcpPort string) {
	try := 0
	for {
		if try > 3 {
			panic(errors.New("gogs http port unreachable"))
		}
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", tcpPort))
		try++
		if err != nil {
			<-time.After(1 * time.Second)
		} else {
			conn.Close()
			break
		}
	}
}

func VolumeExist(volumeName string, dockerApiVersion string) bool {
	return findVolumeByName(volumeName, dockerApiVersion) != nil
}

func CleanVolume(volumeName string, dockerApiVersion string) {
	ctx := context.Background()

	cli, err1 := client.NewClientWithOpts(client.WithVersion(dockerApiVersion))

	failFast(err1)

	err2 := cli.VolumeRemove(ctx, volumeName, true)

	failFast(err2)
}

func findVolumeByName(volumeName string, dockerApiVersion string) *types.Volume {
	ctx := context.Background()

	cli, err1 := client.NewClientWithOpts(client.WithVersion(dockerApiVersion))

	failFast(err1)

	volList, err2 := cli.VolumeList(ctx, filters.Args{})

	failFast(err2)

	for _, volume := range volList.Volumes {
		if volume.Name == volumeName {
			return volume
		}
	}

	return nil
}

func pullImage(imageName string, cli *client.Client, ctx context.Context) {
	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})

	failFast(err)

	_, err = io.Copy(os.Stdout, out)

	failFast(err)

	err = out.Close()

	failFast(err)
}

func failFast(err error) {
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
