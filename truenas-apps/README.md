# BombVault

[BombVault](https://github.com/junkerderprovinz/bombvault) backs up Docker containers and KVM/libvirt VMs with restic and restores them by recreating the container or VM.

Recognised PostgreSQL, MySQL and MariaDB containers are dumped before each backup, straight into the repository. On TrueNAS that matters more than elsewhere: an app's `ix-apps` mounts usually sit outside the paths BombVault backs up, so the container's card says the data folder is not in any backup and the dump is the only copy of that database. Leave it on.

ZFS datasets can be backed up as well, each together with the datasets below it, read from one snapshot. For that, set Host Data to a host path above the datasets (for example `/mnt/tank`) rather than an ixVolume. BombVault runs its `zfs` commands on TrueNAS over SSH: under Additional Environment Variables, either set `LIBVIRT_URI` as for VM backups, which gives it the host, user and port, or set `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` and `LIBVIRT_SSH_PORT`. A user other than root needs `zfs allow <user> snapshot,destroy,mount <dataset>`.
