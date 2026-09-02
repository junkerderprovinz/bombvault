# Aan de slag

Deze pagina leidt je van een verse Unraid-machine naar je eerste back-up.

## Vereisten

| Vereiste | Opmerkingen |
|---|---|
| **Unraid 6.12+** | Eerdere versies zijn niet getest. |
| **Locatie restic-repo** | Een lokaal pad (aanbevolen: je array of cache), SMB, NFS, of elke rclone-backend. |
| **Docker-socket** | Automatisch gemount door de template (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | In zijn geheel automatisch gemount door de template (`/boot` naar `/host/boot`). Maakt flash-back-up mogelijk en laat een herstelde container weer verschijnen als een normale, bewerkbare Unraid-app. |
| **KVM-VM's** (opt-in) | VM-back-up praat met libvirt via SSH, geen libvirt-mount. Stel het in bij Instellingen (zie [Configuratie](configuration.md)). |

## Installeren op Unraid

De makkelijkste weg is **Community Applications**.

1. Open het tabblad **Apps** in Unraid.
2. Zoek naar **BombVault**.
3. Klik op **Install**, stel de vereiste variabelen (hieronder) in en pas toe.

!!! tip "Template handmatig installeren"
    Als je de template liever met de hand toevoegt:

    1. Ga naar **Docker, Add Container, Template repositories** en voeg toe:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Zoek naar **BombVault** in Templates.
    3. Stel de vereiste variabelen in en klik op **Apply**.

## Generieke Docker-host

Geen Unraid? BombVault draait ook als gewone container op elke Docker-host (dat is ook waar de containerondersteuning op TrueNAS Scale op rust, vooruitlopend op een eigen vermelding in de app-catalogus daar).

1. Haal het kant-en-klaar te bewerken bestand [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) uit de repository.
2. Stel `APP_KEY` in (zie hieronder) en richt het Host Data-volume op je echte datawortel: de opmerkingen in het bestand lopen beide door.
3. `docker compose up -d`, open daarna `https://<host-ip>:3443/`.

Wat er anders is dan bij Unraid:

- **Geen flash-/USB-domein.** Er is geen opstart-USB om vast te leggen of terug te zetten, dus het Flash-domein in de instellingen heeft hier niets te doen. In plaats daarvan biedt het Bestanden-domein de suggestie met één klik **Voorinstelling toevoegen: hostsysteemconfiguratie** (een eerste set `/etc`-bestanden die je nakijkt en aanpast voor je opslaat), als praktisch generiek equivalent.
- **Geen Unraid-eigen meldingen.** BombVaults eigen meldingskanalen (webhook, waarschuwingen bij mislukte off-site en dergelijke) werken gewoon; alleen de Unraid-specifieke melding aan diens eigen meldingssysteem blijft achterwege, omdat zo'n systeem hier niet bestaat.
- **Back-up van virtuele machines is optioneel en vraagt een aparte libvirtd-host die via SSH bereikbaar is.** Zie het uitgecommentarieerde blok in het compose-bestand. Een generieke Docker-host heeft zelf geen VM-beheer.

## De ene vereiste instelling

De enige variabele die je moet instellen is `APP_KEY`, een 32-byte hex-geheim (64 hex-tekens) dat wordt gebruikt om het wachtwoord van de restic-repository af te leiden.

Genereer er een op een willekeurige machine:

```bash
openssl rand -hex 32
```

Plak het resultaat in het `APP_KEY`-veld van de template.

!!! danger "Raak je APP_KEY niet kwijt"
    Als je `APP_KEY` kwijtraakt, zijn je versleutelde back-ups onherstelbaar. Bewaar het ergens veilig en gescheiden van de server. Zodra BombVault draait, gebruik je de **herstelkit voor de encryptiesleutel** met één klik (zie [Off-site en herstel](offsite-recovery.md)) om de volledige herstelbundel op te slaan.

De template mount ook de Docker-socket, de flash (`/boot`) en de root **Host Data** (`/mnt`) voor je. Back-up*bronnen* en *bestemmingen* leven allebei onder Host Data. Voor de volledige variabelenreferentie en de off-site setup, zie [Configuratie](configuration.md).

## Eerste keer draaien

![Het dashboard na een eerste back-up: wat beschermd is, wat er volgt en een live logboek.](assets/screenshots/dashboard.png)

*Het dashboard na een eerste back-up: wat beschermd is, wat er volgt en een live logboek.*

1. Open de web-UI op `https://<jouw-unraid-ip>:3443` (out-of-the-box een zelfondertekend certificaat).
2. Schakel bij **Instellingen** de back-updomeinen in die je wilt (Containers, VM's, Flash, Config, Bestanden) en kies een accentkleur.
3. Kies op het tabblad **Containers** een container en klik op **Back-up maken** om je eerste herstelpunt te maken. Repository-paden gaan standaard naar `/mnt/user/bombvault/{container,vms,flash,config,files}` en worden bij de eerste back-up aangemaakt.
4. Stel de planning in via **Instellingen, Planningen**. Er is een *alles opnemen in planning*-optie met één klik voor containers en VM's.

!!! tip "Optioneel: kies een back-upvolgorde"
    Als sommige containers altijd vóór andere geback-upt moeten worden (bijvoorbeeld een database vóór de app die hem gebruikt), open dan het paneel **back-upvolgorde** op de Containers-pagina en sleep ze in de gewenste volgorde. Geplande en meervoudige selecties volgen die volgorde daarna; alles wat je ongeordend laat, wordt geback-upt met het meest-achterstallige eerst, zoals voorheen.

!!! note "Controle van hostintegratie"
    Open `/spike` in de web-UI nadat de container is gestart. Het test elke mount en CLI (Docker-socket, libvirt, restic, qemu-img, rclone) en meldt eventuele ontbrekende onderdelen, zodat je kunt bevestigen dat de container correct is aangesloten voordat je erop vertrouwt.

## Simpel vs Geavanceerd

![Instellingen hebben geen Opslaan-knop: elke wijziging wordt meteen weggeschreven.](assets/screenshots/settings.png)

*Instellingen hebben geen Opslaan-knop: elke wijziging wordt meteen weggeschreven.*

Standaard toont de interface alleen de essentie (back-uppen, herstellen, plannen). Gebruik de schakelaar **Simpel / Geavanceerd** in de zijbalk om de expertbediening te onthullen: retentie, off-site kopie, pre/post-hooks, herstel op bestandsniveau, meldingen, Prometheus-metrics en de integriteits-/onderhoudstools. Het is een voorkeur per browser en standaard uit, zodat nieuwkomers een schone UI krijgen en poweruser alles.

## Volgende stappen

- Blader door de volledige **[Functies](features.md)**.
- Voeg een of meer **[Off-site en herstel](offsite-recovery.md)**-replica's toe (elk domein kan tegelijk naar meerdere bestemmingen sturen) en bewaar je herstelkit.
- Een setup klonen of naar een nieuwe machine verhuizen? Neem je hele configuratie mee met de kaart **Instellingen exporteren en importeren**. Zie [Configuratie](configuration.md#portable-settings-export-and-import).
- Loop je vast? Zie **[Probleemoplossing](troubleshooting.md)**.
