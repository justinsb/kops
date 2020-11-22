package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/go-github/v32/github"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

const tagKeyGithubWorkflowURL = "github.com/workflow"

func main() {
	r := &Runner{
		MaxUptime: time.Hour,

		Region:          "us-east-2",
		SubnetID:        "subnet-0954ef9a66ccd7e92",
		InstanceType:    "m6gd.medium",
		SecurityGroupID: "sg-015c0ca3ba2f726f0",
		SSHKeyName:      "justinsb",
		ImageID:         "ami-07bc0b7b8fe124499",
		PrivateKeyPath:  "~/.ssh/id_rsa",

		GithubOwner: "justinsb",
		GithubRepo:  "kops",
	}
	ctx := context.Background()
	if err := r.run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

type Runner struct {
	MaxUptime time.Duration

	Region          string
	SubnetID        string
	ImageID         string
	InstanceType    string
	SecurityGroupID string
	SSHKeyName      string
	PrivateKeyPath  string

	GithubOwner string
	GithubRepo  string

	githubClient *github.Client
	ec2Client    *ec2.EC2
}

func expandHomeDir(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("error getting home directory: %w", err)
		}
		p = strings.Replace(p, "~", home, 1)
	}
	return p, nil
}

func (r *Runner) buildGithubClient(ctx context.Context) error {
	type GithubCLIConfig struct {
		OAuthToken  string `json:"oauth_token"`
		GitProtocol string `json:"git_protocol"`
		User        string `json:"user"`
	}

	githubConfigPath, err := expandHomeDir("~/.config/gh/hosts.yml")
	if err != nil {
		return err
	}
	githubConfigBytes, err := ioutil.ReadFile(githubConfigPath)
	if err != nil {
		return fmt.Errorf("error reading github config file %q: %w", githubConfigPath, err)
	}
	var githubConfigMap map[string]*GithubCLIConfig
	if err := yaml.Unmarshal(githubConfigBytes, &githubConfigMap); err != nil {
		return fmt.Errorf("error parsing github config file %q: %w", githubConfigPath, err)
	}
	githubConfig := githubConfigMap["github.com"]
	if githubConfig == nil {
		return fmt.Errorf("github config file %q did not have entry for %q", githubConfigPath, "github.com")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubConfig.OAuthToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	r.githubClient = github.NewClient(tc)

	return nil
}
func (r *Runner) run(ctx context.Context) error {
	p, err := expandHomeDir(r.PrivateKeyPath)
	if err != nil {
		return err
	} else {
		r.PrivateKeyPath = p
	}

	if err := r.buildGithubClient(ctx); err != nil {
		return err
	}

	pendingJobs, err := r.ListPendingJobs(ctx)
	if err != nil {
		return err
	}

	cfg := aws.NewConfig().WithRegion(r.Region)
	r.ec2Client = ec2.New(session.New(cfg))

	var allInstances []*ec2.Instance
	{
		request := &ec2.DescribeInstancesInput{}
		// TODO: We want all states, I guess ... as we do want terminated to clean up self-hosted runners
		request.Filters = append(request.Filters, &ec2.Filter{
			Name:   aws.String("instance-state-name"),
			Values: aws.StringSlice([]string{"pending", "running", "shutting-down", "stopping", "stopped"}),
		})
		if err := r.ec2Client.DescribeInstancesPagesWithContext(ctx, request, func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			for _, reservation := range page.Reservations {
				allInstances = append(allInstances, reservation.Instances...)
			}
			return true
		}); err != nil {
			return fmt.Errorf("error listing instances: %w", err)
		}
	}

	for _, job := range pendingJobs {
		klog.Infof("pending job %+v", job.GetURL())

		// TODO: Only if no idle runners?

		// Check to see if we have an instance that is launching, that has not yet accepted the job
		// TODO: Move to kubernetes instead for richer tracking?
		// TODO: Use ClientToken instead?
		var foundInstance *ec2.Instance
		for _, instance := range allInstances {
			for _, tag := range instance.Tags {
				if aws.StringValue(tag.Key) == tagKeyGithubWorkflowURL {
					if aws.StringValue(tag.Value) == job.GetURL() {
						foundInstance = instance
					}
				}
			}
		}
		if foundInstance != nil {
			klog.Infof("found pending ec2 instance for job: %v", aws.StringValue(foundInstance.InstanceId))
			continue
		}

		tags := map[string]string{
			tagKeyGithubWorkflowURL: job.GetURL(),
		}
		instance, err := r.runInstance(ctx, tags)
		if err != nil {
			return err
		}

		klog.Infof("created instance %v", aws.StringValue(instance.InstanceId))
		allInstances = append(allInstances, instance)

		if err := r.startRunner(ctx, instance); err != nil {
			return fmt.Errorf("failed to run agent on machine: %w", err)
		}
	}

	instancesMap := make(map[string]*ec2.Instance)
	for _, instance := range allInstances {
		// klog.Infof("instance ID=%s PublicIP=%s State=%s", aws.StringValue(instance.InstanceId), aws.StringValue(instance.PublicIpAddress), aws.StringValue(instance.State.Name))
		instancesMap[aws.StringValue(instance.InstanceId)] = instance
	}

	{
		runners, err := r.ListRunners(ctx)
		if err != nil {
			return err
		}

		var keepRunners []*github.Runner
		var removeRunners []*github.Runner
		for _, runner := range runners {
			instance := instancesMap[runner.GetName()]
			if instance == nil {
				// I think these eventually time out (30 days?)
				klog.Infof("todo: how to handle completely lost runners: %q", runner.GetName())
				removeRunners = append(removeRunners, runner)
				continue
			}

			state := aws.StringValue(instance.State.Name)
			switch state {
			case "running", "pending", "shutting-down":
				// Ignore - an active runner
				keepRunners = append(keepRunners, runner)

			case "stopped", "terminated":
				removeRunners = append(removeRunners, runner)

				if err := r.terminateInstance(ctx, instance); err != nil {
					klog.Warningf("failed to terminate instance %q: %v", aws.StringValue(instance.InstanceId), err)
				}

			default:
				keepRunners = append(keepRunners, runner)
				klog.Warningf("ignoring ec2 instance in unknown state %q", state)
			}
		}

		if len(keepRunners)+len(removeRunners) != len(runners) {
			klog.Fatalf("runners should either be categorized as remove or keep")
		}

		for i, runner := range removeRunners {
			if i == 0 && len(keepRunners) == 0 {
				// We need to keep a self-hosted runner for each label, even if that runner is offline
				// The reason is that otherwise github actions will say "no runners available" even though a runner will shortly be available
				continue
			}
			klog.Infof("removing runner %q", runner.GetID())
			if _, err := r.githubClient.Actions.RemoveRunner(ctx, r.GithubOwner, r.GithubRepo, runner.GetID()); err != nil {
				klog.Warningf("failed to remove runner %q: %w", runner.GetName(), err)
			}
		}

	}

	for _, instance := range instancesMap {
		instanceID := aws.StringValue(instance.InstanceId)
		state := aws.StringValue(instance.State.Name)
		uptime := time.Since(aws.TimeValue(instance.LaunchTime))
		klog.Infof("instance ID=%s PublicIP=%s State=%s Uptime=%s", instanceID, aws.StringValue(instance.PublicIpAddress), state, uptime)
		if uptime > r.MaxUptime && state != "shutting-down" {
			klog.Infof("terminating instance %q because uptime %v > maxUptime %v", instanceID, uptime, r.MaxUptime)
			if err := r.terminateInstance(ctx, instance); err != nil {
				klog.Warningf("failed to terminate instance %v: %w", instanceID, err)
			}
		}
		instancesMap[aws.StringValue(instance.InstanceId)] = instance
	}

	return nil
}

func (r *Runner) startRunner(ctx context.Context, instance *ec2.Instance) error {
	instanceID := aws.StringValue(instance.InstanceId)

	keyBytes, err := ioutil.ReadFile(r.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("error reading SSH private key from %q: %w", r.PrivateKeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("error parsing SSH private key from %q: %v", r.PrivateKeyPath, err)
	}

	config := &ssh.ClientConfig{
		User: "ec2-user",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Difficult to get the host key for an EC2 instance
	}

	instance, err = r.waitForPublicIP(ctx, instance)
	if err != nil {
		return err
	}

	ip := aws.StringValue(instance.PublicIpAddress)
	if ip == "" {
		return fmt.Errorf("did not get PublicIPAddress")
	}

	sshClient, err := r.sshConnect(ctx, ip, config)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	token, _, err := r.githubClient.Actions.CreateRegistrationToken(ctx, r.GithubOwner, r.GithubRepo)
	if err != nil {
		return fmt.Errorf("error creating runner token: %w", err)
	}

	{
		script := CONFIG_SCRIPT

		repoURL := "https://github.com/" + r.GithubOwner + "/" + r.GithubRepo
		configArgs := "--url " + repoURL + " --token " + token.GetToken() + " --labels linux-arm64 --name " + instanceID
		script = strings.ReplaceAll(script, "{{CONFIG_ARGS}}", configArgs)

		if err := r.runScript(ctx, sshClient, script); err != nil {
			return err
		}
	}

	{
		script := RUN_SCRIPT

		if err := r.runScript(ctx, sshClient, script); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) runScript(ctx context.Context, sshClient *ssh.Client, script string) error {
	name := fmt.Sprintf("~/script-%d", time.Now().UnixNano())

	if err := r.runSSHCommand(ctx, sshClient, "tee "+name, []byte(script), true); err != nil {
		return fmt.Errorf("error writing script: %w", err)
	}
	if err := r.runSSHCommand(ctx, sshClient, "chmod +x "+name, nil, false); err != nil {
		return fmt.Errorf("error chmodding script: %w", err)
	}
	if err := r.runSSHCommand(ctx, sshClient, name, nil, false); err != nil {
		return fmt.Errorf("error running script: %w", err)
	}
	if err := r.runSSHCommand(ctx, sshClient, "rm "+name, nil, false); err != nil {
		return fmt.Errorf("error removing  script: %w", err)
	}
	return nil
}

func (r *Runner) runSSHCommand(ctx context.Context, sshClient *ssh.Client, command string, stdin []byte, secret bool) error {
	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	session.Stdout = &stdout
	var stderr bytes.Buffer
	session.Stderr = &stderr
	klog.Infof("Running %s", command)
	if err := session.Run(command); err != nil {
		if !secret {
			klog.Infof("\tstdout: %s", stdout.String())
			if stderr.Len() != 0 {
				klog.Infof("\tstderr: %s", stderr.String())
			}
		}
		return fmt.Errorf("failed to run command %q: %w", command, err)
	}
	if !secret {
		klog.Infof("\tstdout: %s", stdout.String())
		if stderr.Len() != 0 {
			klog.Infof("\tstderr: %s", stderr.String())
		}
	}
	return nil
}

// TODO: Use /dev/nvme1n1 - does it speed things up a lot??

var CONFIG_SCRIPT = `
#!/bin/bash

set -e
#set -x

uptime
uname -a

# libicu60 is for the runner
sudo yum -y install libicu60
# gcc make git are assumed to be present in the build image
sudo yum -y install gcc make git

mkdir -p ~/actions-runner
curl --output /tmp/actions-runner.tar.gz -L https://github.com/actions/runner/releases/download/v2.274.2/actions-runner-linux-arm64-2.274.2.tar.gz
tar -xz --directory=${HOME}/actions-runner --file=/tmp/actions-runner.tar.gz

~/actions-runner/config.sh {{CONFIG_ARGS}}

cat > ~/actions-runner/run-once-and-shutdown.sh << EOF
#!/bin/bash
set -e
set -x

~/actions-runner/run.sh --once
sudo shutdown -h now
EOF
chmod +x ~/actions-runner/run-once-and-shutdown.sh
`

var RUN_SCRIPT = `
#!/bin/bash

set -e
#set -x

nohup ~/actions-runner/run-once-and-shutdown.sh >> ~/action-runner.log 2>&1 &
`

func (r *Runner) ListRunners(ctx context.Context) ([]*github.Runner, error) {
	runners, _, err := r.githubClient.Actions.ListRunners(ctx, r.GithubOwner, r.GithubRepo, &github.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing github runners for %s/%s: %w", r.GithubOwner, r.GithubRepo, err)
	}
	// for _, runner := range runners.Runners {
	// 	klog.Infof("runner: %+v", runner)
	// }
	return runners.Runners, nil
}

func (r *Runner) ListPendingJobs(ctx context.Context) ([]*github.WorkflowRun, error) {
	options := &github.ListWorkflowRunsOptions{
		Status: "queued",
	}
	runs, _, err := r.githubClient.Actions.ListRepositoryWorkflowRuns(ctx, r.GithubOwner, r.GithubRepo, options)
	if err != nil {
		return nil, fmt.Errorf("error listing github workflow runs for %s/%s: %w", r.GithubOwner, r.GithubRepo, err)
	}
	return runs.WorkflowRuns, nil
}

func (r *Runner) waitForPublicIP(ctx context.Context, instance *ec2.Instance) (*ec2.Instance, error) {
	instanceID := aws.StringValue(instance.InstanceId)

	attempt := 0
	maxAttempts := 60
	for {
		if attempt != 0 {
			time.Sleep(5 * time.Second)
		}
		attempt++

		ip := aws.StringValue(instance.PublicIpAddress)
		if ip != "" {
			return instance, nil
		}

		if attempt >= maxAttempts {
			return instance, fmt.Errorf("timeout waiting for public IP for instance %q", instanceID)
		}

		if i, err := r.describeInstance(ctx, instanceID); err != nil {
			return instance, err
		} else {
			instance = i
		}
	}
}

func (r *Runner) sshConnect(ctx context.Context, ip string, config *ssh.ClientConfig) (*ssh.Client, error) {
	attempt := 0
	maxAttempts := 60
	for {
		if attempt != 0 {
			time.Sleep(5 * time.Second)
		}
		attempt++

		sshClient, err := ssh.Dial("tcp", ip+":22", config)
		if err != nil {
			if attempt >= maxAttempts {
				return nil, fmt.Errorf("timed out trying to make SSH connection: %w", err)
			}
			klog.Warningf("failed to connect over SSH: %v", err)

		} else {
			return sshClient, nil
		}
	}
}

func (r *Runner) terminateInstance(ctx context.Context, instance *ec2.Instance) error {
	instanceID := aws.StringValue(instance.InstanceId)
	request := &ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	}
	if _, err := r.ec2Client.TerminateInstances(request); err != nil {
		return fmt.Errorf("failed to terminate instance %q: %w", instanceID, err)
	}
	return nil
}

func (r *Runner) runInstance(ctx context.Context, tags map[string]string) (*ec2.Instance, error) {
	useSpot := false // Not a lot of capacity for arm instances (?)
	if useSpot {
		return r.runSpotInstance(ctx, tags)
	} else {
		return r.runOnDemandInstance(ctx, tags)
	}
}

func (r *Runner) runSpotInstance(ctx context.Context, tags map[string]string) (*ec2.Instance, error) {
	request := &ec2.RequestSpotInstancesInput{
		// TODO: BlockDurationMinutes
		InstanceCount: aws.Int64(1),
		LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
			ImageId:      aws.String(r.ImageID),
			InstanceType: aws.String(r.InstanceType),
			KeyName:      aws.String(r.SSHKeyName),
		},
	}

	// TODO: Tag the instance also
	for k, v := range tags {
		request.TagSpecifications = append(request.TagSpecifications, &ec2.TagSpecification{
			ResourceType: aws.String("spot-instances-request"),
			Tags: []*ec2.Tag{
				{
					Key:   aws.String(k),
					Value: aws.String(v),
				},
			},
		})
	}

	request.LaunchSpecification.NetworkInterfaces = append(request.LaunchSpecification.NetworkInterfaces, &ec2.InstanceNetworkInterfaceSpecification{
		SubnetId:                 aws.String(r.SubnetID),
		AssociatePublicIpAddress: aws.Bool(true),
		Groups:                   aws.StringSlice([]string{r.SecurityGroupID}),
		DeviceIndex:              aws.Int64(0),
	})

	response, err := r.ec2Client.RequestSpotInstancesWithContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("error creating spot instance: %w", err)
	}
	spotRequestID := aws.StringValue(response.SpotInstanceRequests[0].SpotInstanceRequestId)

	// TODO: Cancel spot request on failure?
	attempt := 0
	maxAttempts := 60
	for {
		attempt++

		if attempt == maxAttempts {
			return nil, fmt.Errorf("spot request was not fulfilled: %w", err)
		}

		spotRequest, err := r.ec2Client.DescribeSpotInstanceRequestsWithContext(ctx, &ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: aws.StringSlice([]string{spotRequestID}),
		})
		if err != nil {
			// Eventual consistency error  InvalidSpotInstanceRequestID.NotFound
			if isAWSError(err, "InvalidSpotInstanceRequestID.NotFound") {
				klog.Infof("ignoring spot instance request not found %q (eventual consistency)", spotRequestID)
				continue
			}
			return nil, fmt.Errorf("error describe spot instance request %q: %w", spotRequestID, err)
		}

		var instanceIDs []string
		for _, req := range spotRequest.SpotInstanceRequests {
			if req.InstanceId != nil {
				instanceIDs = append(instanceIDs, aws.StringValue(req.InstanceId))
			}
		}

		if len(instanceIDs) > 1 {
			return nil, fmt.Errorf("found multiple ec2 instances for spot request %q: %d", spotRequestID, len(instanceIDs))
		}

		if len(instanceIDs) == 0 {
			klog.Infof("waiting for spot instance request %q to be fulfilled", spotRequestID)
			time.Sleep(5 * time.Second)
			continue
		}

		instance, err := r.describeInstance(ctx, instanceIDs[0])
		if err != nil {
			return nil, err
		}
		return instance, nil
	}
}

func (r *Runner) runOnDemandInstance(ctx context.Context, tags map[string]string) (*ec2.Instance, error) {
	request := &ec2.RunInstancesInput{
		MinCount: aws.Int64(1),
		MaxCount: aws.Int64(1),

		ImageId:      aws.String(r.ImageID),
		InstanceType: aws.String(r.InstanceType),
		KeyName:      aws.String(r.SSHKeyName),
	}

	for k, v := range tags {
		request.TagSpecifications = append(request.TagSpecifications, &ec2.TagSpecification{
			ResourceType: aws.String("instance"),
			Tags: []*ec2.Tag{
				{
					Key:   aws.String(k),
					Value: aws.String(v),
				},
			},
		})
	}
	request.NetworkInterfaces = append(request.NetworkInterfaces, &ec2.InstanceNetworkInterfaceSpecification{
		SubnetId:                 aws.String(r.SubnetID),
		AssociatePublicIpAddress: aws.Bool(true),
		Groups:                   aws.StringSlice([]string{r.SecurityGroupID}),
		DeviceIndex:              aws.Int64(0),
	})

	created, err := r.ec2Client.RunInstancesWithContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("error creating instance: %w", err)
	}

	if len(created.Instances) != 1 {
		return nil, fmt.Errorf("unexpected numebr of instances created %d", len(created.Instances))
	}

	return created.Instances[0], nil
}

func (r *Runner) describeInstance(ctx context.Context, instanceID string) (*ec2.Instance, error) {
	request := &ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	}

	var allInstances []*ec2.Instance
	if err := r.ec2Client.DescribeInstancesPagesWithContext(ctx, request, func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
		for _, reservation := range page.Reservations {
			allInstances = append(allInstances, reservation.Instances...)
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("error listing instance %q: %w", instanceID, err)
	}
	if len(allInstances) != 1 {
		return nil, fmt.Errorf("unexpected result listing instance %q, got %d results", instanceID, len(allInstances))
	}
	return allInstances[0], nil
}

func isAWSError(err error, code string) bool {
	if awsError, ok := err.(awserr.Error); ok {
		if awsError.Code() == code {
			return true
		}
	}
	return false
}
