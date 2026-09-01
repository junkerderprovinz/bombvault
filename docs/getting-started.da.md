# Kom godt i gang

Denne side fører dig fra en frisk Unraid-boks til din første sikkerhedskopi.

## Krav

| Krav | Bemærkninger |
|---|---|
| **Unraid 6.12+** | Tidligere versioner er ikke testet. |
| **Restic-repo-placering** | En lokal sti (anbefalet: dit array eller cache), SMB, NFS eller en hvilken som helst rclone-backend. |
| **Docker-socket** | Monteres automatisk af skabelonen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteres i sin helhed automatisk af skabelonen (`/boot` til `/host/boot`). Driver flash-sikkerhedskopiering og lader en gendannet container dukke op igen som en normal, redigerbar Unraid-app. |
| **KVM-VM'er** (tilvalg) | VM-sikkerhedskopiering taler med libvirt over SSH, ingen libvirt-montering. Sæt det op i Indstillinger (se [Konfiguration](configuration.md)). |

## Installer på Unraid

Den nemmeste vej er **Community Applications**.

1. Åbn fanen **Apps** i Unraid.
2. Søg efter **BombVault**.
3. Klik på **Install**, angiv de nødvendige variabler (nedenfor), og anvend.

!!! tip "Manuel skabeloninstallation"
    Hvis du foretrækker at tilføje skabelonen manuelt:

    1. Gå til **Docker, Add Container, Template repositories**, og tilføj:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Søg efter **BombVault** under Templates.
    3. Angiv de nødvendige variabler, og klik på **Apply**.

## Generisk Docker-vært

Ikke Unraid? BombVault kører også som almindelig container på enhver Docker-vært (det er også det, der bærer containerunderstøttelsen på TrueNAS Scale, forud for en egen post i dens app-katalog).

1. Hent den redigeringsklare [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) fra arkivet.
2. Sæt `APP_KEY` (se nedenfor) og peg Host Data-diskenheden på din rigtige datarod: kommentarerne i filen gennemgår begge dele.
3. `docker compose up -d`, åbn derefter `https://<vært-ip>:3443/`.

Hvad der er anderledes end på Unraid:

- **Intet flash-/USB-domæne.** Der er ingen boot-USB at indfange eller genskabe, så Flash-domænet i indstillingerne har intet at lave her. I stedet tilbyder Fil-domænet etklik-forslaget **Tilføj forudindstilling: værtssystemets konfiguration** (et sæt `/etc`-filer til at begynde med, som du gennemgår og retter, før du gemmer) som praktisk generisk modstykke.
- **Ingen Unraid-egne meddelelser.** BombVaults egne meddelelseskanaler (webhook, advarsler om mislykket off-site og lignende) virker som sædvanlig; kun den Unraid-specifikke besked til dens eget meddelelsessystem udelades, da et sådant system ikke findes her.
- **Sikkerhedskopiering af virtuelle maskiner er valgfri og kræver en separat libvirtd-vært, der kan nås over SSH.** Se den udkommenterede blok i compose-filen. En generisk Docker-vært har ingen indbygget VM-styring.

## Den ene påkrævede indstilling

Den eneste variabel, du skal angive, er `APP_KEY`, en 32-byte hex-hemmelighed (64 hex-tegn), der bruges til at udlede restic-repositoriets adgangskode.

Generer en på en hvilken som helst maskine:

```bash
openssl rand -hex 32
```

Indsæt resultatet i skabelonens `APP_KEY`-felt.

!!! danger "Mist ikke din APP_KEY"
    Mister du `APP_KEY`, kan dine krypterede sikkerhedskopier ikke gendannes. Opbevar den et sikkert sted adskilt fra serveren. Når BombVault kører, kan du bruge dens ét-klik **gendannelseskit til krypteringsnøglen** (se [Off-site og gendannelse](offsite-recovery.md)) til at gemme den fulde gendannelsespakke.

Skabelonen monterer også Docker-socket'en, flashen (`/boot`) og **Host Data**-roden (`/mnt`) for dig. Både *kilder* og *destinationer* for sikkerhedskopier lever under Host Data. For den fulde variabelreference og off-site-opsætningen, se [Konfiguration](configuration.md).

## Første kørsel

1. Åbn web-UI'en på `https://<your-unraid-ip>:3443` (selvsigneret certifikat fra start).
2. Aktivér i **Indstillinger** de sikkerhedskopidomæner, du vil have (Containers, VMs, Flash, Config, Files), og vælg en accentfarve.
3. Vælg en container på fanen **Containers**, og klik på **Sikkerhedskopier** for at oprette dit første gendannelsespunkt. Repository-stier defaulter til `/mnt/user/bombvault/{container,vms,flash,config,files}` og oprettes ved den første sikkerhedskopi.
4. Opsæt planlægning fra **Indstillinger, Tidsplaner**. Der er en ét-klik *inkludér alle i tidsplan* for containere og VM'er.

!!! tip "Valgfrit: vælg en sikkerhedskopi-rækkefølge"
    Hvis nogle containere altid skal sikkerhedskopieres før andre (for eksempel en database før den app, der bruger den), så åbn panelet **sikkerhedskopi-rækkefølge** på Containers-siden, og træk dem ind i den ønskede rækkefølge. Planlagte og fler-valgs-kørsler følger den derefter; alt, du lader stå uordnet, sikkerhedskopieres mest-overskredet-først, som før.

!!! note "Vært-integrationstjek"
    Åbn `/spike` i web-UI'en, når containeren er startet. Den prober hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer eventuelle manglende dele, så du kan bekræfte, at containeren er korrekt forbundet, før du forlader dig på den.

## Simpel vs. Avanceret

Som standard viser grænsefladen kun det væsentlige (sikkerhedskopier, gendan, planlæg). Brug **Simpel / Avanceret**-kontakten i sidebjælken for at afsløre ekspertkontrollerne: opbevaring, off-site-kopi, pre/post-hooks, gendannelse på filniveau, notifikationer, Prometheus-metrics og integritets-/vedligeholdelsesværktøjerne. Det er en præference pr. browser og slået fra som standard, så nybegyndere får en ren UI, og power-brugere får det hele.

## Næste skridt

- Gennemse de fulde **[Funktioner](features.md)**.
- Tilføj en eller flere **[Off-site og gendannelse](offsite-recovery.md)**-replikaer (hvert domæne kan sende til flere destinationer på én gang), og gem dit gendannelseskit.
- Kloner du en opsætning eller flytter til en ny boks? Bær hele din konfiguration over med kortet **Eksportér og importér indstillinger**. Se [Konfiguration](configuration.md#portable-settings-export-and-import).
- Løb ind i et problem? Se **[Fejlfinding](troubleshooting.md)**.
