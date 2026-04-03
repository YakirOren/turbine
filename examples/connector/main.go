package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
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
// The struct holds the connection; methods are steps passed to turbine.Do.
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
// The connection always re-establishes on recovery — it can't be serialized.
// Subsequent steps are checkpointed normally and replay from the database.
func DeployWorkflow(ctx turbine.Context, host string) (string, error) {
	d := &Deployer{host: host}

	// Connect — not checkpointed, re-runs on recovery
	_, err := turbine.Do(ctx, d.Connect, turbine.WithoutCheckpoint(), turbine.WithStepName("connect"))
	if err != nil {
		return "", err
	}
	defer d.ssh.Close(ctx)

	// Durable steps — checkpointed as normal
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
	app := pocketbase.New()

	rt := turbine.Setup(app, turbine.Config{})

	turbine.Register(rt, DeployWorkflow)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/deploy/{host}", func(re *core.RequestEvent) error {
			host := re.Request.PathValue("host")

			handle, err := turbine.Run(rt, DeployWorkflow, host)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			result, err := handle.GetResult()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": result})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
