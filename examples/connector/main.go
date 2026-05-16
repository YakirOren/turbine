package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/turbine"
)

// SSHClient wraps an SSH connection. In production this would use crypto/ssh.
type SSHClient struct {
	host string
}

func DialSSH(ctx context.Context, host string) (*SSHClient, error) {
	turbine.LoggerFrom(ctx).Info("connecting", "host", host)
	return &SSHClient{host: host}, nil
}

func (c *SSHClient) Run(ctx context.Context, cmd string) (string, error) {
	turbine.LoggerFrom(ctx).Info("running command", "host", c.host, "cmd", cmd)
	return "ok", nil
}

func (c *SSHClient) Close(ctx context.Context) error {
	turbine.LoggerFrom(ctx).Info("disconnecting", "host", c.host)
	return nil
}

// Deployer groups steps that share a live SSH connection.
// The struct holds the connection, methods are steps passed to turbine.Do.
type Deployer struct {
	host string
	ssh  *SSHClient
}

func (d *Deployer) Connect(ctx context.Context) (*SSHClient, error) {
	client, err := DialSSH(ctx, d.host)
	if err != nil {
		return nil, err
	}
	d.ssh = client
	return client, nil
}

func (d *Deployer) Deploy(ctx context.Context) (string, error) {
	return d.ssh.Run(ctx, "./deploy.sh")
}

func (d *Deployer) HealthCheck(ctx context.Context) (bool, error) {
	out, err := d.ssh.Run(ctx, "curl -s localhost:8080/health")
	if err != nil {
		return false, err
	}
	return out == "ok", nil
}

// DeployWorkflow uses WithoutCheckpoint for the connect step.
// The connection always re-establishes on recovery, it can't be serialized.
// Subsequent steps are checkpointed normally and replay from the database.
func DeployWorkflow(ctx turbine.Context, host string) (string, error) {
	d := &Deployer{host: host}

	_, err := turbine.Do(ctx, d.Connect, turbine.WithoutCheckpoint(), turbine.WithStepName("connect"))
	if err != nil {
		return "", err
	}
	defer func() { _ = d.ssh.Close(ctx) }()

	_, err = turbine.Do(ctx, d.Deploy, turbine.WithStepName("deploy"))
	if err != nil {
		return "", err
	}

	_, err = turbine.Do(ctx, d.HealthCheck, turbine.WithStepName("health-check"))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("deployed to %s", host), nil
}

func main() {
	rt := turbine.NewStandalone(turbine.Config{})
	defer func() { _ = rt.Shutdown() }()

	turbine.Register(rt, DeployWorkflow)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, err := turbine.Run(rt, DeployWorkflow, "prod-1")
	if err != nil {
		log.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result)
}
