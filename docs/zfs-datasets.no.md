# ZFS-datasett

Siden **ZFS** tar sikkerhetskopi av ZFS-datasett. Et element er ett datasett sammen med alle datasett under det. For hver sikkerhetskopi tar BombVault ett ZFS-øyeblikksbilde av hele treet, slik at hvert datasett i det fanges i samme øyeblikk. Deretter leser det filene i hvert datasett fra det øyeblikksbildet, lagrer dem med restic på samme måte som en mappe og fjerner øyeblikksbildet rett etterpå. Sikkerhetskopiene er deduplisert, du kan bla gjennom hver av dem, og enkeltfiler kan gjenopprettes.

BombVault bruker aldri `zfs send` for datasett, ruller aldri et datasett tilbake og sletter aldri et.

## Krav {#requirements}

- **SSH-forbindelsen til denne serveren.** ZFS-datasett bruker samme nøkkel, vert og bruker som VM-sikkerhetskopier. Fungerer VM-sikkerhetskopier allerede, fungerer dette også. Ellers følger du [veiledningen for VM-sikkerhetskopi over SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub. Malfeltene heter **Host SSH: Address**, **Host SSH: Port** og **Host SSH: User**.
- **Kommandoen `zfs` på den verten.** Unraid 6.12 og nyere og TrueNAS SCALE har den.
- **Host Data mappet som `/mnt` med Access Mode Read/Write - Slave.** Det er malens standard. Øyeblikksbildet av et datasett dukker først opp i datasettets mappe `.zfs/snapshot` etter at BombVault har startet, så containeren må motta monteringene verten lager senere.
- **Datasettene montert under `/mnt`.** På Unraid ligger pools under `/mnt/<pool>`, så det er allerede tilfellet.

Slå på domenet under **Innstillinger, Generelt** (ZFS-datasett). ZFS-siden viser da kortet **Forbindelse til denne serveren**. Det tester SSH-forbindelsen, nevner brukeren og verten det kobler til, og sier hva som mangler når noe mangler. Vertsintegrasjonssjekken (`/spike`) viser samme resultat.

## Elementer og underdatasett {#items-and-children}

Åpne **Legg til datasett** på ZFS-siden. Listen kommer fra serveren. Velg datasettet som ligger øverst i det du vil sikkerhetskopiere, for eksempel `cache/appdata`, og elementet dekker det og alle datasett under det.

- **Nye underdatasett blir med av seg selv.** Et datasett som opprettes senere under elementet, sikkerhetskopieres ved neste kjøring, og den kjøringen nevner det som nytt. Den første sikkerhetskopien leser det helt én gang; deretter leses bare endringer.
- **Du kan utelate enkelte underdatasett.** Slå et av i elementets innstillinger, så blir det utelatt sammen med alt under det. Et utelatt underdatasett som ikke lenger finnes på serveren, merkes slik og kan fjernes fra listen.
- **Underdatasett som ikke kan leses, hoppes over, aldri i stillhet.** Kjøringen lister dem, elementet viser hvor mange som ble hoppet over, og dekningskortet på dashbordet teller hvert av dem som ubeskyttet. Kjøringen sikkerhetskopierer likevel alt annet og feiler ikke på grunn av et underdatasett som ble hoppet over. Årsakene står i [tabellen over årsakskoder](#reason-codes): et datasett som ikke er montert, har `canmount=off`, et `legacy`- eller ikke noe monteringspunkt, en krypteringsnøkkel som ikke er lastet, avslått tilgang til øyeblikksbilder eller et monteringspunkt BombVault ikke ser.
- **Et datasett som hoppes over, tar ikke med seg underdatasettene sine.** Et datasett med `canmount=off` som bare inneholder andre datasett, hoppes over (vist som "bare struktur"), og de monterte underdatasettene sikkerhetskopieres. Et kryptert datasett der nøkkelen ikke er lastet, hoppes over sammen med underdatasettene som deler nøkkelen.
- **Underdatasett som er VM-disker eller systemdata, starter avslått** i dialogen for å legge til, med årsaken ved siden av bryteren. Legger du til en hel pool, må du bekrefte det i en dialog som lister hva den inneholder.

### Volumer {#volumes}

Et volum (zvol) inneholder en virtuell disk i stedet for filer, og ZFS-siden sikkerhetskopierer aldri et.

- Et volum som en VM bruker, sikkerhetskopieres med den VM-en på siden **VM-er**.
- Et volum som ingen VM bruker (en iSCSI-extent, en disk du har koblet fra), **sikkerhetskopieres ikke av BombVault**. Dialogen for å legge til og ZFS-siden teller disse volumene og sier det. En senere versjon vil sikkerhetskopiere dem.

Volumer i et elements tre hoppes over og nevnes ved hver kjøring.

### Dockers lagring {#docker-storage}

Med Dockers ZFS-lagringsdriver er hvert bildelag et datasett med `legacy`-monteringspunkt. Dialogen for å legge til samler dem i én linje per overordnet datasett. Et tre med mer enn 20 slike datasett kan ikke bli et element: Så lenge det finnes et øyeblikksbilde av det, kan ikke Docker fjerne bildelag. Legg i stedet til datasettene under det, for eksempel `appdata`.

### Elementer overlapper aldri {#overlap}

Et datasett kan bare høre til ett element. BombVault avviser et nytt element som ligger inni et eksisterende eller som ville inneholde et. For å slå sammen flere underelementer til ett overordnet element sletter du først underelementene og velger å beholde sikkerhetskopiene deres, og legger så til det overordnede. Hvert datasett beholder historikken under sitt eget navn, så neste sikkerhetskopi fortsetter der de gamle elementene slapp, og leser ikke alt på nytt.

## Stoppe containere og kjøre kommandoer rundt øyeblikksbildet {#consistency}

Et øyeblikksbilde av en database som kjører, er som et plutselig strømbrudd: Databasen kommer seg som regel, men den må gjøre det. Hvert element kan gjøre to ting med det, og begge gjelder bare øyeblikket for øyeblikksbildet, ikke hele sikkerhetskopien.

- **Stopp disse containerne for øyeblikksbildet.** BombVault stopper containerne på listen, tar øyeblikksbildet og starter dem igjen med en gang. Containere på samme avhengighetsnivå stopper parallelt, de avhengige først, så hele vinduet varer som regel noen få sekunder; kjøringen viser hvor lenge. Sikkerhetskopien leser så det frosne øyeblikksbildet mens appene allerede kjører igjen. Bare containere som kjørte, stoppes.
- **En kommando før og etter øyeblikksbildet.** Den kjører inne i en container du velger, for eksempel for å dumpe en database inn i datasettet rett før øyeblikksbildet uten å stoppe noe. Feiler kommandoen før øyeblikksbildet, feiler sikkerhetskopien, og det tas ikke noe øyeblikksbilde. En kommando etter øyeblikksbildet som feiler, vises på kjøringen, men får ikke sikkerhetskopien til å feile.

Hva som skjer når noe går galt:

- Kan en container ikke stoppes, starter BombVault dem det allerede har stoppet, og sikkerhetskopien feiler med navnet på containeren. Det faller aldri tilbake på et øyeblikksbilde av apper som kjører.
- Stoppet venter til en pågående containersikkerhetskopi er ferdig (opptil 30 minutter ved en manuell kjøring, opptil tidsgrensen for sikkerhetskopien ved en planlagt), så de to aldri stopper og starter samme container samtidig.
- Før den første containeren stopper, skriver BombVault ned hvilke det stopper. Blir BombVault drept inne i vinduet, starter det de containerne igjen neste gang det starter, sender et varsel, og elementet viser en rød merknad for hver container det ikke kunne starte.

De automatiske databasedumpene (se [Funksjoner](features.md)) kjører med en containers egen sikkerhetskopi på siden **Kontainere**, ikke med et ZFS-element. En database der containeren bare sikkerhetskopieres gjennom datasettet sitt, får ingen dump, så gi den en kommando her.

En container kan stå på denne listen og på siden **Kontainere** samtidig. Dataene lagres da to ganger, i to repositories, og **Full sikkerhetskopi** stopper den to ganger. Elementet sier fra om det.

## Gjenoppretting {#restore}

Åpne **Sikkerhetskopier** på elementet, velg sikkerhetskopien og deretter datasettet. Som standard er det elementets øverste datasett.

- **Gjenopprett inn i datasettet.** Filer fra sikkerhetskopien skrives inn i datasettets monteringspunkt. Filer med samme navn overskrives, andre blir liggende. Datasettet rulles aldri tilbake eller erstattes. BombVault sjekker at datasettet er montert, synlig og skrivbart, én gang før det starter og igjen rett før det skriver. Der et underdatasett er montert inni det, skrives ingenting: Underdatasettet beholder filene, eieren og tillatelsene sine og gjenopprettes fra sin egen sikkerhetskopi.
- **Gjenopprett til en mappe.** Velg en mappe under `/mnt`. BombVault sjekker at mappen ligger på en montert pool eller deling, og at det er nok ledig plass. Dette virker uten SSH-forbindelsen og for datasett som ikke lenger finnes.
- **Velg filer** (avansert): skriv bare filene og mappene du velger tilbake i datasettet.
- **Alle datasett i denne sikkerhetskopien** (avansert): hvert datasett i treet i sin egen undermappe av mappen du velger. Datasett som ble hoppet over i den sikkerhetskopien, nevnes.
- **Fra en annen server:** Siden **Gjenoppretting** gjenoppretter fra en annen BombVaults repository, alltid til en mappe: alle datasett i én sikkerhetskopi, hvert i sin egen undermappe, eller ett datasett i treet, helt eller valgte filer.

Elementets liste over containere som skal stoppes, tilbys også ved gjenoppretting inn i datasettet. De containerne forblir stoppet under hele gjenopprettingen, og containersikkerhetskopier venter så lenge.

### Sikkerhetsøyeblikksbildet {#safety-snapshot}

Før det skriver inn i et datasett, tar BombVault et ZFS-øyeblikksbilde av bare det datasettet, med navnet `bombvault-prerestore-<tid>`. Det er slått på som standard; å slå det av krever en ekstra bekreftelse. Kan øyeblikksbildet ikke tas, gjenopprettes ingenting.

BombVault sletter aldri et sikkerhetsøyeblikksbilde av seg selv. Elementet lister dem med alder og størrelse, hvert med handlingen **Slett**, og advarer når det eldste er mer enn 30 dager gammelt, fordi det holder slettede og endrede data fast i poolen.

For å gå tilbake etter en gjenoppretting kopierer du enkeltfiler fra `.zfs/snapshot/bombvault-prerestore-<tid>` inne i datasettet. `zfs rollback <dataset>@bombvault-prerestore-<tid>` virker bare så lenge det er datasettets nyeste øyeblikksbilde. `zfs rollback -r` sletter alle nyere øyeblikksbilder, også de automatiske.

### Gjenopprette som nytt datasett {#new-dataset}

BombVault oppretter ikke datasett. Opprett det på serveren med egenskapene du vil ha, og gjenopprett så til en mappe som er monteringspunktet:

```
zfs create -o compression=lz4 cache/appdata-restored
```

og gjenopprett i BombVault til mappen `cache/appdata-restored` under `/mnt`.

## Hva sikkerhetskopien inneholder {#contents}

I sikkerhetskopien: filene og mappene i hvert sikkerhetskopierte datasett, med eier, tillatelser, tidsstempler og utvidede attributter slik restic lagrer dem.

Ikke i sikkerhetskopien:

- datasettenes ZFS-egenskaper (compression, recordsize, quota, mountpoint og resten);
- eier og tillatelser for selve den øverste mappen i hvert datasett (alt under den er med). En gjenoppretting inn i datasettet lar den eksisterende øverste mappen være som den er, en gjenoppretting til en mappe oppretter den med tillatelsene `0755`;
- eksisterende ZFS-øyeblikksbilder;
- underdatasett som ble hoppet over eller utelatt;
- volumer.

For å gjenopprette til en ny pool oppretter du først datasettene med egenskapene du vil ha. Om NFSv4-ACL-er, slik TrueNAS bruker dem på SMB-datasett, kommer tilbake slik du forventer, er ikke kontrollert ennå, så test en gjenoppretting med dine egne data før du stoler på dem.

## Krypterte datasett {#encryption}

Et kryptert datasett sikkerhetskopieres bare mens nøkkelen er lastet. Ellers hoppes det over med en advarsel; last nøkkelen med `zfs load-key` og monter datasettet. BombVault leser dataene dekryptert og lagrer dem i restics repository, som er kryptert. Har du slått av kryptering i BombVault, er ikke det repositoryet kryptert.

## Gjenværende øyeblikksbilder {#leftover-snapshots}

Øyeblikksbildet av en sikkerhetskopi heter `<dataset>@bombvault-<14 sifre>`, for eksempel `cache/appdata@bombvault-20260924021500` (UTC). BombVault fjerner det rett etter sikkerhetskopien. Mislykkes det, for eksempel fordi datasettet er opptatt eller BombVault ble stoppet, fjerner BombVault det:

- før neste sikkerhetskopi av det elementet,
- når BombVault starter, for hvert element, også med domenet slått av,
- når du sletter elementet,
- når du trykker **Fjern nå** på elementet, som også viser hvor mange som er igjen.

Bare navn som er nøyaktig `bombvault-` pluss 14 sifre, fjernes. Sikkerhetsøyeblikksbilder, dine egne øyeblikksbilder og automatiske øyeblikksbilder røres aldri. For å fjerne et for hånd:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Avvik {#anomalies}

Et underdatasett som er tømt, endrer nesten ikke totalen for et stort tre, så avviksoppdagelsen følger med på hvert datasett i et element for seg: størrelse, antall filer, nye data og restic-tid har hver sin egen historikk. Den historikken hører til datasettets navn, så den blir værende når treet senere sikkerhetskopieres av et annet element.

Et datasett som forrige kjøring sikkerhetskopierte og som denne kjøringen ikke kunne lese, teller som tømt, så lenge elementets utvalg ikke er endret. Det dekker en nøkkel som ikke er lastet, et datasett som ikke er montert, og et som har forsvunnet fra treet. Et underdatasett du utelukker selv, endrer utvalget, så historikken begynner på nytt i stedet. Så lenge et funn om tapte data er åpent, beholder oppbevaringen de gamle sikkerhetskopiene av akkurat det datasettet og beskjærer resten av treet som vanlig.

På fanen **Elementer** på siden **Avvik** har hvert datasett sin egen linje under elementet sitt, og elementets tre på denne siden viser de åpne funnene ved hvert datasett. Lenken i et funn åpner elementets gjenopprettingspanel ved datasettets siste gode sikkerhetskopi. Om en kjøring blir ferdig, vurderes for hele elementet, fordi en kjøring lykkes eller feiler som en helhet.

Selve kontrollene er beskrevet under [Funksjoner](features.md). En assistent som er koblet til via [MCP-serveren](mcp.md), kan liste gjenopprettingspunktene til et ZFS-element, starte sikkerhetskopien og lese funnene, men et funn kvitteres på siden **Avvik**.

## Årsakskoder {#reason-codes}

Siden, kjøringshistorikken og varslene nevner et problem med en av disse kodene. De fleste har også løsningen ved siden av på siden.

| Kode | Betydning | Hva du gjør |
|---|---|---|
| `ssh-missing` | SSH-forbindelsen er ikke satt opp i denne containeren. | Sett opp SSH-forbindelsen som for VM-sikkerhetskopier. |
| `host-placeholder` | Host SSH: Address er fortsatt eksempelverdien, og `host.docker.internal` svarte heller ikke. | Sett Host SSH: Address til denne serverens LAN-IP. |
| `host-fallback` | Host SSH: Address er fortsatt eksempelverdien, og `host.docker.internal` fungerer. | Ingenting, eller sett LAN-IP-en. |
| `ssh-unreachable` | Serveren kan ikke nås over SSH. | Sjekk adresse og port, og at SSH er slått på. |
| `ssh-auth` | Serveren avviste BombVaults nøkkel. | Kjør kommandoen fra forbindelseskortet én gang på serveren. |
| `zfs-not-found` | SSH-verten har ingen `zfs`-kommando. | Pek Host SSH: Address mot maskinen som eier poolene. |
| `zfs-permission` | SSH-brukeren har ikke lov til å kjøre denne zfs-kommandoen. | Bruk root, eller se [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nevner en annen vert eller bruker enn SSH-feltene. | Få dem til å stemme, eller tøm SSH-feltene så begge kommer fra URI-en. |
| `zfs-error` | zfs rapporterte en annen feil. | Detaljene viser meldingen. |
| `propagation-missing` | Nye monteringer på verten når ikke containeren. | Sett Access Mode for Host Data til Read/Write - Slave og start BombVault på nytt. |
| `invalid-name` | Et datasettnavn BombVault ikke godtar. | Gi datasettet nytt navn. |
| `name-too-long` | Et datasett i treet er for langt for et navn på et øyeblikksbilde. | Gi det nytt navn, eller legg til et datasett under det som element. |
| `invalid-exclude` | Et ekskluderingsmønster eller et utelatt underdatasett passer ikke til elementet. | Rett oppføringen meldingen nevner. For å utelate et helt underdatasett slår du det av i stedet for å skrive et mønster. |
| `not-found` | Datasettet finnes ikke på serveren. | Fjern elementet, eller opprett datasettet på nytt. Sikkerhetskopiene kan fortsatt gjenopprettes. |
| `not-filesystem` | Dette er et volum, ikke et filsystem. | Se [Volumer](#volumes). |
| `overlaps-item` | Datasettet overlapper et eksisterende element. | Se [Elementer overlapper aldri](#overlap). |
| `docker-storage` | Treet inneholder Dockers bildelagring. | Se [Dockers lagring](#docker-storage). |
| `nothing-readable` | Ingen datasett i elementet kan leses akkurat nå. | Se på kodene for datasettene som ble hoppet over. |
| `snapshot-failed` | Øyeblikksbildet kunne ikke opprettes. | Detaljene viser meldingen fra zfs. |
| `containers-busy` | En containersikkerhetskopi kjørte fortsatt da containerne måtte stoppe. | Start igjen senere. Planlagte kjøringer venter av seg selv. |
| `consistency-stop-failed` | En container kunne ikke stoppes, så det ble ikke tatt noe øyeblikksbilde. | Sjekk containeren, eller ta den av listen. |
| `pre-snapshot-failed` | Kommandoen før øyeblikksbildet feilet. | Kjøringsdetaljene viser utdataene. |
| `container-unknown` | En container på listen finnes ikke. | Ta den av listen. |
| `container-is-self` | BombVault kan ikke stoppe sin egen container. | Ta den av listen. |
| `leftover-snapshots` | Øyeblikksbilder BombVault ikke kunne fjerne, ligger fortsatt på serveren. | Trykk **Fjern nå**, se [Gjenværende øyeblikksbilder](#leftover-snapshots). |
| `zvol` | Et volum i treet, hoppet over. | Se [Volumer](#volumes). |
| `canmount-off` | Aldri montert (`canmount=off`), hoppet over. | Inneholder det data, monter det eller flytt dataene til et underdatasett. |
| `legacy-mount` | Legacy-monteringspunkt, hoppet over. | Gi det et monteringspunkt under `/mnt`. |
| `no-mountpoint` | Ikke noe monteringspunkt, hoppet over. | Gi det et monteringspunkt under `/mnt`. |
| `not-mounted` | Ikke montert på serveren, hoppet over. | Monter det med `zfs mount`, eller sett `canmount=on`. |
| `key-not-loaded` | Kryptert og nøkkelen er ikke lastet, hoppet over. | `zfs load-key`, monter det deretter. |
| `snapdir-disabled` | Tilgang til øyeblikksbilder er slått av, hoppet over. | `zfs set snapdir=hidden <dataset>`. Mappen `.zfs` forblir skjult. |
| `not-visible` | BombVault ser ikke datasettets monteringspunkt. | Flytt monteringspunktet under Host Data-stien, eller map det inn i containeren på samme sti med Read/Write - Slave. |
| `shfs-only` | Datasettet er bare synlig gjennom `/mnt/user`, som skjuler øyeblikksbilder. | Map `/mnt`, ikke `/mnt/user`, som Host Data. |
| `snapshot-not-visible` | Øyeblikksbildet ble opprettet, men dukket ikke opp inne i BombVault. | Kjør **Test tilgangen til øyeblikksbilder**; se nedenfor. |
| `snapshot-loop` | Øyeblikksbildet nådde ikke BombVault fordi Host Data ikke sender nye monteringer videre. | Sett Access Mode for Host Data til Read/Write - Slave og start BombVault på nytt. |
| `backup-failed` | restic feilet for dette datasettet. | Kjøringsdetaljene viser hvorfor. |
| `not-reached` | Kjøringen sluttet før dette datasettet. | Kjør sikkerhetskopien på nytt. |
| `gone` | Datasettet er ikke lenger på serveren. | Ingenting. Sikkerhetskopiene kan fortsatt gjenopprettes. |
| `read-only-mount` | BombVault kan bare lese datasettet og kan derfor ikke gjenopprette inn i det. | Sett mappingen til Read/Write - Slave, eller gjenopprett til en mappe. |
| `destination-not-mounted` | Mappen ligger ikke på en montert pool eller deling. | Velg en mappe på en pool eller deling. |
| `not-enough-space` | Ikke nok ledig plass på målet. | Frigjør plass, eller velg en annen mappe. |
| `safety-snapshot-failed` | Sikkerhetsøyeblikksbildet kunne ikke tas, så ingenting ble gjenopprettet. | Detaljene viser meldingen fra zfs. |
| `safety-name-too-long` | Datasettnavnet er for langt for et sikkerhetsøyeblikksbilde. | Slå av sikkerhetsøyeblikksbildet, eller gjenopprett til en mappe. |

### Sjekke hva containeren ser {#mountinfo}

**Test tilgangen til øyeblikksbilder** på et element tar et ekte øyeblikksbilde av treet, ser etter det inne i BombVault for hvert datasett og fjerner det igjen. Det er den raskeste måten å bevise hele veien på før den første planlagte kjøringen.

For å se selv kjører du dette på serveren:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Hver linje er en montering inne i containeren. Linjen for et datasett viser stien inne i containeren (under `/host/user`) og datasettnavnet. Et felt `master:N` på den linjen betyr at monteringen mottar monteringene verten lager senere, og det er det tilgang til øyeblikksbilder trenger. Mangler det, setter du Access Mode for Host Data til Read/Write - Slave og starter BombVault på nytt.

## TrueNAS SCALE {#truenas}

- Når `LIBVIRT_URI` er satt (som for VM-sikkerhetskopier på TrueNAS), henter BombVault SSH-vert, bruker og port for zfs-kommandoene sine fra URI-en, hver av dem som ikke er satt for seg. Uten VM-sikkerhetskopier setter du i stedet `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` og `LIBVIRT_SSH_PORT`. Legg til variablene under **Additional Environment Variables**.
- En annen bruker enn root trenger tillatelser på elementets øverste datasett, som da dekker alle datasett under det:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  En SSH-økt uten root på TrueNAS har ikke `/usr/sbin` i stien; BombVault kaller da `/usr/sbin/zfs` direkte.
- Appens **Host Data** må være en vertssti over datasettene, for eksempel `/mnt/tank`, ikke et ixVolume. Med en vertssti sender appen vertens nye monteringer videre til BombVault (`rslave`), og det trenger tilgang til øyeblikksbilder.
