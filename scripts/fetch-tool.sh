#!/bin/sh
# Usage: fetch-tool.sh restic|rclone VERSION ARCH DEST
#
# Installs the upstream linux/ARCH build of restic or rclone as DEST/<tool>.
# The archive has to match upstream's SHA256SUMS, and those have to carry a
# signature by one of the keys in keys/<tool>.asc, so a new version needs no
# new hashes in this repository.
set -eu

tool=$1 version=$2 arch=$3 dest=$4
keys=$(cd "$(dirname "$0")/keys" && pwd)
mkdir -p "$dest"
dest=$(cd "$dest" && pwd)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"
gpg --dearmor <"$keys/$tool.asc" >keyring.gpg

case $tool in
restic)
	url=https://github.com/restic/restic/releases/download/v$version
	file=restic_${version}_linux_$arch.bz2
	curl -fsSL -o SHA256SUMS "$url/SHA256SUMS" -o SHA256SUMS.asc "$url/SHA256SUMS.asc" -o "$file" "$url/$file"
	gpgv --keyring ./keyring.gpg SHA256SUMS.asc SHA256SUMS
	mv SHA256SUMS sums
	;;
rclone)
	url=https://downloads.rclone.org/v$version
	file=rclone-v$version-linux-$arch.zip
	curl -fsSL -o SHA256SUMS "$url/SHA256SUMS" -o "$file" "$url/$file"
	# The sums are clearsigned. Lines outside the signed block still pass
	# gpgv, so only what it writes out is trusted.
	gpgv --keyring ./keyring.gpg --output sums SHA256SUMS
	;;
*)
	echo "unknown tool: $tool" >&2
	exit 2
	;;
esac

awk -v f="$file" '$2 == f' sums >want
if [ "$(wc -l <want)" -ne 1 ]; then
	echo "$file: not listed once in the signed SHA256SUMS" >&2
	exit 1
fi
sha256sum -c want

case $tool in
restic) bunzip2 -c "$file" >bin ;;
rclone) unzip -p "$file" "rclone-v$version-linux-$arch/rclone" >bin ;;
esac
install -m 0755 bin "$dest/$tool"
