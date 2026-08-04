# Probleemoplossing

Een korte FAQ. Voor de volledige VM-via-SSH-tabel voor probleemoplossing aan de hostkant (permission-denied, host-key-verificatie, ontbrekende template-variabelen en meer), zie de [VM-back-up-via-SSH-gids](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) op GitHub.

## Er is iets niet correct aangesloten

Open `/spike` in de web-UI. De controle van hostintegratie test elke mount en CLI (Docker-socket, libvirt, restic, qemu-img, rclone) en meldt eventuele ontbrekende onderdelen. Begin hier voordat je een bug veronderstelt: een ontbrekende mount of een onbereikbare host duikt meteen op.

## Ik kan de web-UI niet bereiken

BombVault serveert HTTPS out of the box op poort `3443` (zelfondertekend certificaat), dus open `https://<jouw-unraid-ip>:3443`. Accepteer de waarschuwing over het zelfondertekende certificaat, of zet BombVault achter een reverse proxy met je eigen certificaat. Als je draait met `HTTP_ONLY=true`, serveert het in plaats daarvan platte HTTP op poort `3000` (bedoeld voor gebruik achter een TLS-terminerende proxy).

## Ik ben mijn APP_KEY kwijt

`APP_KEY` leidt het wachtwoord van de restic-repository af. Zonder deze (en zonder de herstelkit voor de encryptiesleutel) kunnen versleutelde back-ups niet worden hersteld. Daarom zeurt het Dashboard je om de herstelkit te downloaden. Zie [Off-site en herstel](offsite-recovery.md). Genereer een sleutel met `openssl rand -hex 32` en bewaar hem buiten de server voordat je op enige back-up vertrouwt.

## VM-back-up wil geen verbinding maken

VM-back-up praat met libvirt via SSH, nooit een mount.

- Bevestig dat SSH is ingeschakeld op de host en dat BombVaults publieke sleutel geautoriseerd is in `/root/.ssh/authorized_keys` (Instellingen, Systeem, VM-back-up via SSH toont de sleutel en een knop **Verbinding testen**).
- Stel op een custom `br0.x`-netwerk `LIBVIRT_HOST` in op je Unraid LAN-IP (de container kan de host daar niet via `host.docker.internal` bereiken). Schakel **Instellingen, Docker, Host access to custom networks** in.
- Als je de SSH-poort van Unraid hebt gewijzigd, stel `LIBVIRT_SSH_PORT` overeenkomstig in.
- Volledige stap-voor-stap-diagnose (bereikbaarheidstest, VLAN-routing, `Permission denied (publickey)`, `Host key verification failed`) staat in de [VM-back-up-via-SSH-gids](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Een live VM-snapshot is niet gedraaid

Live snapshots hebben de qemu guest agent geïnstalleerd in de VM nodig en de schijf op `/mnt/cache` (of `/mnt/diskX`), niet `/mnt/user`. Op een uitgeschakelde VM valt live automatisch terug op net afsluiten. Een nette back-up sluit de VM af, maakt een back-up van de schijven en herstart hem daarna, dus hij is altijd consistent.

## Een back-up mislukte met "repository is already locked"

Dit is meestal een verweesde restic-lock die achterblijft wanneer de container werd bijgewerkt of herstart midden in een operatie. BombVault detecteert een aantoonbaar verweesde lock, wist hem geforceerd en probeert automatisch één keer opnieuw. Als het aanhoudt, gebruik **Instellingen, Integriteit en onderhoud, Ontgrendelen** voor het betrokken domein om een verouderde lock met de hand te wissen. Een echt probleem komt nog steeds boven in plaats van verborgen te worden.

## Mijn off-site kopie is niet gemaakt na een back-up

Off-site replicatie is opzettelijk best-effort, dus een off-site hapering laat de lokale back-up nooit mislukken. Controleer de off-site planning voor dat domein (Instellingen, Planningen): een lege planning repliceert na elke lokale back-up, terwijl een cadans minder vaak stuurt. Gebruik **Nu repliceren** op het tabblad Off-site voor een run op aanvraag, en let op de replicatie-indicator op het Dashboard.

## Een herstel brak af voordat het begon

Voordat er iets wordt gestopt of verwijderd, draait herstel een pre-flight conflictcontrole: het verifieert dat het statische IP en de gepubliceerde hostpoorten van de container vrij zijn. Als een andere container er al een vasthoudt, breekt het af met een duidelijke, bruikbare melding in plaats van een half afgemaakt herstel achter te laten. Maak de conflicterende poort of IP vrij en probeer opnieuw.

## Een platte export mislukte in plaats van een bestand te schrijven

Als age-versleuteling aan is (Instellingen) maar er geen geldige ontvanger is ingesteld, mislukt een export met een duidelijke fout in plaats van platte tekst te schrijven. Voeg een geldige ontvanger toe (een age-publieke sleutel of een SSH-publieke sleutel), of zet versleuteling uit als je bedoelt dat de export platte tekst is. Zie [Functies](features.md).

## De container blijft herstarten of ziet er unhealthy uit

BombVault meldt healthy/unhealthy vanuit zijn eigen `/api/health`. Een auto-heal-tool (zoals Autoheal) kan hem automatisch herstarten als de engine ooit vastloopt. Controleer het containerlog en het `/spike`-rapport voor de onderliggende oorzaak.

## Nog steeds vast?

- Lees de volledige pagina's [Configuratie](configuration.md) en [Off-site en herstel](offsite-recovery.md).
- Vraag het in de [Unraid-supportthread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Open een [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
