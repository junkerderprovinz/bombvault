/**
 * authorizeCommand builds the ready-to-paste shell command that authorizes
 * BombVault's public key on the server, both for the live session and across a
 * reboot: Unraid restores /root/.ssh from /boot/config/ssh/root.pubkeys. Every
 * card that hands the command out shares this one, so they cannot drift apart.
 */
export function authorizeCommand(publicKey: string): string {
  const key = publicKey.trim();
  if (!key) return "";
  return `mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
echo '${key}' | tee -a /root/.ssh/authorized_keys /boot/config/ssh/root.pubkeys >/dev/null
chmod 600 /root/.ssh/authorized_keys`;
}
