## kops rolling-update cluster

Rolling update a cluster

### Synopsis


Rolling update a cluster instance groups.
		
This command updates the running instances to match the cloud specifications.

To perform rolling update, you need to update the cloud resources first with "kops update cluster"

```
kops rolling-update cluster
```

### Options

```
      --bastion-interval duration    Time to wait between restarting bastions (default 5m0s)
      --cloudonly                    Perform rolling update without confirming progress with k8s
      --force                        Force rolling update, even if no changes
      --instance-group stringSlice   List of instance groups to update (defaults to all if not specified)
      --master-interval duration     Time to wait between restarting masters (default 5m0s)
      --node-interval duration       Time to wait between restarting nodes (default 2m0s)
      --yes                          perform rolling update without confirmation
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
      --config string                    config file (default is $HOME/.kops.yaml)
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files (default false)
      --name string                      Name of cluster
      --state string                     Location of state storage
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO
* [kops rolling-update](kops_rolling-update.md)	 - rolling update clusters

