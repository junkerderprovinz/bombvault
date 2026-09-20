# BombVault

[BombVault](https://github.com/junkerderprovinz/bombvault) backs up Docker containers and KVM/libvirt VMs with restic and restores them by recreating the container or VM.

Recognised PostgreSQL, MySQL and MariaDB containers are dumped before each backup, straight into the repository. On TrueNAS that matters more than elsewhere: an app's `ix-apps` mounts usually sit outside the paths BombVault backs up, so the container's card says the data folder is not in any backup and the dump is the only copy of that database. Leave it on.
