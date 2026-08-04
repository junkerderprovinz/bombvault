# Felsökning

En kort FAQ. För den fullständiga felsökningstabellen för VM-över-SSH på värdsidan (permission-denied, host-key-verifiering, saknade mallvariabler och mer), se [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Något är inte korrekt inkopplat

Öppna `/spike` i webbgränssnittet. Värdintegrationskontrollen sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar. Börja här innan du antar att det är ett fel: en saknad montering eller en onåbar värd dyker upp omedelbart.

## Jag kan inte nå webbgränssnittet

BombVault serverar HTTPS direkt ur lådan på port `3443` (självsignerat certifikat), så öppna `https://<din-unraid-ip>:3443`. Godkänn varningen om det självsignerade certifikatet, eller placera BombVault bakom en reverse proxy med ditt eget certifikat. Om du kör med `HTTP_ONLY=true` serverar den vanlig HTTP på port `3000` istället (avsedd för användning bakom en TLS-terminerande proxy).

## Jag förlorade min APP_KEY

`APP_KEY` härleder restic-repositoriets lösenord. Utan den (och utan återställningskitet för krypteringsnyckeln) kan krypterade säkerhetskopior inte återställas. Det är därför Översikten tjatar på dig att ladda ner återställningskitet. Se [Off-site och återställning](offsite-recovery.md). Generera en nyckel med `openssl rand -hex 32` och förvara den bort från servern innan du förlitar dig på någon säkerhetskopia.

## VM-säkerhetskopiering ansluter inte

VM-säkerhetskopiering pratar med libvirt över SSH, aldrig en montering.

- Bekräfta att SSH är aktiverat på värden och att BombVaults publika nyckel är auktoriserad i `/root/.ssh/authorized_keys` (Inställningar, System, VM Backup over SSH visar nyckeln och en **Testa anslutning**-knapp).
- På ett anpassat `br0.x`-nätverk, sätt `LIBVIRT_HOST` till din Unraid-LAN-IP (containern kan inte nå värden via `host.docker.internal` där). Aktivera **Inställningar, Docker, Host access to custom networks**.
- Om du ändrade Unraids SSH-port, sätt `LIBVIRT_SSH_PORT` att matcha.
- Fullständig steg-för-steg-diagnos (nåbarhetstest, VLAN-routning, `Permission denied (publickey)`, `Host key verification failed`) finns i [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## En live-VM-ögonblicksbild kördes inte

Live-ögonblicksbilder behöver qemu-gästagenten installerad i VM:en och disken på `/mnt/cache` (eller `/mnt/diskX`), inte `/mnt/user`. På en avstängd VM faller live automatiskt tillbaka till mjuk. En mjuk säkerhetskopiering stänger av VM:en, säkerhetskopierar diskarna och startar sedan om den, så den är alltid konsekvent.

## En säkerhetskopiering misslyckades med "repository is already locked"

Detta är oftast ett övergivet restic-lås som lämnats kvar när containern uppdaterades eller startades om mitt i en operation. BombVault upptäcker ett bevisligen övergivet lås, tvingar bort det och gör om en gång, automatiskt. Om det kvarstår, använd **Inställningar, Integritet och underhåll, Lås upp** för den drabbade domänen för att rensa ett fastnat lås för hand. Ett äkta problem dyker fortfarande upp istället för att döljas.

## Min off-site-kopia hände inte efter en säkerhetskopiering

Off-site-replikering är best-effort by design, så att en off-site-hicka aldrig misslyckar den lokala säkerhetskopian. Kontrollera off-site-schemat för den domänen (Inställningar, Scheman): ett tomt schema replikerar efter varje lokal säkerhetskopiering, medan en kadens skickar mer sällan. Använd **Replikera nu** på Off-site-fliken för en körning på begäran, och håll koll på replikeringsindikatorn på Översikten.

## En återställning avbröts innan den startade

Innan något stoppas eller tas bort kör återställningen en konfliktkontroll före körning: den verifierar att containerns statiska IP och publicerade värdportar är lediga. Om en annan container redan håller en av dem avbryter den med ett tydligt, åtgärdbart meddelande istället för att lämna en halvfärdig återställning. Frigör den konfliktande porten eller IP:n och försök igen.

## En vanlig export misslyckades istället för att skriva en fil

Om age-kryptering är på (Inställningar) men ingen giltig mottagare är satt misslyckas en export med ett tydligt fel istället för att skriva klartext. Lägg till en giltig mottagare (en age-publik nyckel eller en SSH-publik nyckel), eller stäng av kryptering om du avser att exporten ska vara klartext. Se [Funktioner](features.md).

## Containern startar om hela tiden eller ser osund ut

BombVault rapporterar frisk/osund från sin egen `/api/health`. Ett auto-heal-verktyg (som Autoheal) kan starta om den automatiskt om motorn någonsin skulle kärva. Kontrollera containerloggen och `/spike`-rapporten för den underliggande orsaken.

## Fortfarande fast?

- Läs de fullständiga sidorna [Konfiguration](configuration.md) och [Off-site och återställning](offsite-recovery.md).
- Fråga på [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Öppna ett [GitHub-ärende](https://github.com/junkerderprovinz/bombvault/issues).
