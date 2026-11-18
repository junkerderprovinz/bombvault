package hostsnap

import "strings"

// preamble finds the tools and tells a sleeping disk from a spinning one
// without waking it. Every script this package sends starts with it.
//
// A disk counts as asleep only on positive evidence: Unraid's own record,
// hdparm's or smartctl's answer to a power check, none of which spins a disk
// up. A disk neither can answer for stays "unknown" and is read, since holding
// the whole list back for it would hide the snapshots of every SSD.
const preamble = `tab=$(printf '\t')
have() { command -v "$1" >/dev/null 2>&1; }
pick() {
	if have "$1"; then echo "$1"; return; fi
	for d in /usr/sbin /sbin /usr/local/sbin; do
		if [ -x "$d/$1" ]; then echo "$d/$1"; return; fi
	done
}
ZFS=$(pick zfs); ZPOOL=$(pick zpool); BTRFS=$(pick btrfs)
HDPARM=$(pick hdparm); SMARTCTL=$(pick smartctl)
INI=/var/local/emhttp/disks.ini
errf=$(mktemp 2>/dev/null || echo /tmp/bombvault-snap.$$)
trap 'rm -f "$errf"' EXIT
base() {
	n=$(readlink -f "$1" 2>/dev/null); n=${n##*/}
	[ -n "$n" ] || n=${1##*/}
	if [ -e "/sys/class/block/$n/partition" ]; then
		p=$(readlink -f "/sys/class/block/$n/.." 2>/dev/null); n=${p##*/}
	fi
	echo "$n"
}
power() {
	d="$1"
	if [ -r "$INI" ]; then
		key=""
		case "$d" in md[0-9]*) key="disk${d#md}"; key=${key%%p*} ;; esac
		s=$(awk -F= -v key="$key" -v dev="$d" '
			function flush() { if (sec != "" && ((key != "" && sec == key) || devv == dev) && sp != "") print sp }
			/^\[/ { flush(); sec = $0; gsub(/[\[\]"]/, "", sec); devv = ""; sp = ""; next }
			{ v = $0; sub(/^[^=]*=/, "", v); gsub(/"/, "", v) }
			$1 == "device" { devv = v }
			$1 == "spundown" { sp = v }
			END { flush() }' "$INI" | head -n 1)
		case "$s" in 1) echo asleep; return ;; 0) echo awake; return ;; esac
	fi
	case "$d" in nvme*|zram*|loop*|nbd*|sr*) echo awake; return ;; dm-*|md*) echo unknown; return ;; esac
	if [ "$(cat "/sys/block/$d/queue/rotational" 2>/dev/null)" = 0 ]; then echo awake; return; fi
	if [ -n "$HDPARM" ]; then
		case "$("$HDPARM" -C "/dev/$d" 2>/dev/null)" in
			*standby*|*sleeping*) echo asleep; return ;;
			*active*|*idle*) echo awake; return ;;
		esac
	fi
	if [ -n "$SMARTCTL" ]; then
		out=$("$SMARTCTL" -n standby -i "/dev/$d" 2>&1); rc=$?
		case "$out" in *"STANDBY mode"*|*"SLEEP mode"*) echo asleep; return ;; esac
		if [ "$rc" = 0 ]; then echo awake; return; fi
	fi
	echo unknown
}
emit() {
	P="$1"; shift
	if out=$("$@" 2>"$errf"); then
		[ -n "$out" ] && printf '%s\n' "$out" | P="$P" awk '{ print ENVIRON["P"] "\t" $0 }'
		return 0
	fi
	printf 'E\t%s\t%s\n' "$P" "$(tr '\t\n' '  ' < "$errf")"
	return 1
}
`

// surveyScript lists everything in one round trip. $1 is 1 to read pools
// whose disks sleep too, $2 is "states" to report the power states alone.
// Records are tab separated and start with their type; mountpoints keep the
// octal escapes of /proc/self/mounts, so a path can never break a record.
const surveyScript = preamble + `wake="$1"; mode="$2"
printf 'Z\t%s\n' "$(date +%z)"
if [ -n "$ZFS" ] && [ -n "$ZPOOL" ]; then
	echo "T${tab}zfs${tab}ok"
	devs=$("$ZPOOL" status -P 2>/dev/null | awk '
		$1 == "pool:" { pool = $2; skip = 0; next }
		$1 == "spares" { skip = 1; next }
		$1 == "logs" || $1 == "cache" || $1 == "special" || $1 == "dedup" { skip = 0; next }
		skip == 0 && $1 ~ /^\/dev\// { print pool "\t" $1 }')
	read=""
	for pool in $("$ZPOOL" list -H -o name 2>/dev/null); do
		asleep=0
		for dev in $(printf '%s\n' "$devs" | awk -F'\t' -v p="$pool" '$1 == p { print $2 }'); do
			d=$(base "$dev"); st=$(power "$d")
			printf 'D\tzfs\t%s\t%s\t%s\n' "$pool" "$d" "$st"
			[ "$st" = asleep ] && asleep=1
		done
		printf 'P\tzfs\t%s\t%s\n' "$pool" "$asleep"
		if [ "$asleep" = 0 ] || [ "$wake" = 1 ]; then read="$read $pool"; fi
	done
	if [ "$mode" != states ] && [ -n "$read" ]; then
		emit F "$ZFS" list -H -p -t filesystem,volume -o name,type,mountpoint,used,written -r $read
		emit S "$ZFS" list -H -p -t snapshot -s name -o name,creation,used,referenced,userrefs,clones -r $read
		if [ "$wake" = 1 ]; then
			for pool in $read; do
				for dev in $(printf '%s\n' "$devs" | awk -F'\t' -v p="$pool" '$1 == p { print $2 }'); do
					d=$(base "$dev"); printf 'A\tzfs\t%s\t%s\t%s\n' "$pool" "$d" "$(power "$d")"
				done
			done
		fi
	fi
else
	echo "T${tab}zfs${tab}missing"
fi
if [ -n "$BTRFS" ]; then
	echo "T${tab}btrfs${tab}ok"
	awk '$3 == "btrfs" && $1 !~ /^\/dev\/loop/ && $2 !~ /^\/var\/lib\/docker/ && $2 !~ /^\/etc\/libvirt/ { print $1 "\t" $2 "\t" $4 }' /proc/self/mounts |
	awk -F'\t' '{
		top = ($3 ~ /(^|,)subvolid=5(,|$)/ || $3 ~ /(^|,)subvol=\/(,|$)/)
		if (!($1 in best) || (top && !istop[$1])) { best[$1] = $2; istop[$1] = top; order[++n] = $1 }
	} END { for (i = 1; i <= n; i++) { d = order[i]; if (!(d in done)) { done[d] = 1; print d "\t" best[d] "\t" istop[d] } } }' |
	while IFS="$tab" read -r dev esc istop; do
		[ -n "$dev" ] || continue
		m=$(printf '%b' "$esc")
		printf 'BF\t%s\t%s\t%s\n' "$esc" "$dev" "$istop"
		k=$(readlink -f "$dev" 2>/dev/null); k=${k##*/}
		members=""
		for x in /sys/fs/btrfs/*/devices/"$k"; do
			[ -e "$x" ] && members=$(ls "${x%/*}" 2>/dev/null)
		done
		[ -n "$members" ] || members="$k"
		asleep=0
		for md in $members; do
			d=$(base "/dev/$md"); st=$(power "$d")
			printf 'BD\t%s\t%s\t%s\n' "$esc" "$d" "$st"
			[ "$st" = asleep ] && asleep=1
		done
		printf 'BP\t%s\t%s\n' "$esc" "$asleep"
		if [ "$mode" != states ] && { [ "$asleep" = 0 ] || [ "$wake" = 1 ]; }; then
			emit "BT${tab}$esc" "$BTRFS" subvolume show "$m"
			emit "BL${tab}$esc" "$BTRFS" subvolume list -p -u -q "$m"
			emit "BS${tab}$esc" "$BTRFS" subvolume list -s -u -q "$m"
			emit "BR${tab}$esc" "$BTRFS" subvolume list -r -u "$m"
			"$BTRFS" qgroup show --raw "$m" 2>/dev/null | P="BQ${tab}$esc" awk '{ print ENVIRON["P"] "\t" $0 }'
			if [ "$wake" = 1 ]; then
				for md in $members; do
					d=$(base "/dev/$md"); printf 'BA\t%s\t%s\t%s\n' "$esc" "$d" "$(power "$d")"
				done
			fi
		fi
	done
else
	echo "T${tab}btrfs${tab}missing"
fi
if [ "$mode" != states ]; then
	for f in /etc/libvirt/qemu/snapshotdb/*/snapshots.db; do
		[ -r "$f" ] || continue
		vm=${f%/snapshots.db}; vm=${vm##*/}
		printf 'U\t%s\t%s\n' "$vm" "$(base64 < "$f" | tr -d '\n')"
	done
fi
`

// mountScan is the loop the busy check and the release share. It reads the
// mount table of every mount namespace once, through the first process seen
// in it, and calls found with the pid, the namespace and the escaped
// mountpoint of each mount whose source is the snapshot $esc.
const mountScan = `hostns=$(readlink /proc/1/ns/mnt 2>/dev/null)
seen=""
scan() {
	for f in /proc/[0-9]*/mountinfo; do
		pid=${f#/proc/}; pid=${pid%/mountinfo}
		ns=$(readlink "/proc/$pid/ns/mnt" 2>/dev/null) || continue
		case " $seen " in *" $ns "*) continue ;; esac
		seen="$seen $ns"
		hits=$(S="$esc" awk '{ for (i = 7; i <= NF; i++) if ($i == "-") { if ($(i + 2) == ENVIRON["S"]) print $5; break } }' "$f" 2>/dev/null)
		[ -n "$hits" ] || continue
		cg=$(tr '\n\t' '  ' < "/proc/$pid/cgroup" 2>/dev/null)
		for mp in $hits; do found "$pid" "$ns" "$mp" "$cg"; done
	done
}
`

// whereScript names every mount of one snapshot. $1 is the snapshot's full
// name, $2 the same with the escapes mountinfo uses.
const whereScript = preamble + mountScan + `esc="$2"
found() { printf 'M\t%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$([ "$2" = "$hostns" ] && echo 1 || echo 0)" "$4"; }
scan
`

// freeScript unmounts the snapshot wherever it is mounted and then destroys
// it. The host's own mount goes first, since a container that receives the
// host's mounts loses its copy along with it. A process keeps running; only
// the mount goes, so the container it belongs to does not stop.
const freeScript = preamble + mountScan + `snap="$1"; esc="$2"
found() {
	mp=$(printf '%b' "$3")
	if [ "$2" = "$hostns" ]; then out=$(umount -- "$mp" 2>&1); else out=$(nsenter -t "$1" -m -- umount -- "$mp" 2>&1); fi
	rc=$?
	printf 'U\t%s\t%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$([ "$2" = "$hostns" ] && echo 1 || echo 0)" "$rc" "$(printf '%s' "$out" | tr '\t\n' '  ')"
}
for f in /proc/1/mountinfo; do
	hits=$(S="$esc" awk '{ for (i = 7; i <= NF; i++) if ($i == "-") { if ($(i + 2) == ENVIRON["S"]) print $5; break } }' "$f" 2>/dev/null)
	for mp in $hits; do found 1 "$hostns" "$mp" ""; done
done
seen="$hostns"
scan
if out=$("$ZFS" destroy -- "$snap" 2>&1); then echo "R${tab}ok"; else printf 'R\tfail\t%s\n' "$(printf '%s' "$out" | tr '\t\n' '  ')"; fi
`

// shQuote quotes s for a POSIX shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// mountEscape is a snapshot name as mountinfo writes it.
func mountEscape(s string) string {
	r := strings.NewReplacer(`\`, `\134`, " ", `\040`, "\t", `\011`, "\n", `\012`)
	return r.Replace(s)
}

// mountUnescape reverses the octal escapes of a mount table.
func mountUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && isOct(s[i+1]) && isOct(s[i+2]) && isOct(s[i+3]) {
			b.WriteByte((s[i+1]-'0')<<6 | (s[i+2]-'0')<<3 | (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isOct(c byte) bool { return c >= '0' && c <= '7' }
