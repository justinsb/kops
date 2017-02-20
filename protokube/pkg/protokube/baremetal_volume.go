package protokube

// Ideas:
// * Scan for well known metadata file under /mnt e.g. /mnt/*/.k8s.io.volume
// * Use a file lock to enforce single-mounter, for things like NFS.  We could do this everywhere.
