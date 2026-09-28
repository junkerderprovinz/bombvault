# Lagringssteder

Et lagringssted er et sted der BombVault oppbevarer sikkerhetskopier: en mappe på denne Unraid-serveren, en delt ressurs på en NAS, en bucket hos en skyleverandør, en rest-server, en SFTP-konto eller en Nextcloud. Du kobler til hvert lagringssted én gang, på **Innstillinger, Lagring**, og legitimasjonen, oppbevaringen, beskyttelsen og lokasjonen hører til lagringsstedet. Domenene (containere, VM-er, flashen, BombVaults egen konfigurasjon, filsett og ZFS-datasett) velger deretter blant lagringsstedene: hvor hvert domene lagres, og hvor det kopieres. ZFS-datasettene har sin rad på Domener-kortet så lenge ZFS-domenet er slått på.

## Legge til et lagringssted {#add-a-place}

**Legg til lagringssted** åpner et vindu med én flis per leverandør, i tre grupper: skylagring, selvhostede tjenester, og NAS-enheter og denne serveren.

1. Velg en flis og fyll ut skjemaet. Øyeknappen viser en hemmelighet du har skrevet inn.
2. **Test tilkobling** sjekker lagringsstedet og oppretter ingenting. For hver domenemappe melder den hva den fant: at mappen er tom eller ikke finnes ennå, at den allerede inneholder et restic-depot, eller feilen som stoppet den.
3. Gi lagringsstedet et navn; leverandørens navn er fylt inn. For en enhet du drifter selv, svarer du på **Hvor står enheten?**. Skyleverandører står alltid på en annen lokasjon, og en mappe på denne Unraid-serveren står alltid her.
4. **Legg til** lagrer lagringsstedet.

Et nytt lagringssted brukes ennå ikke av noe domene. Velg det under **Lagret på** eller **Kopiert til** på [Domener-kortet](#domains), eller på kortet til et element for bare det elementet.

## Mapper {#folders}

Et lagringssted har én mappe per domene: `container`, `vms`, `flash`, `config`, `files` og `zfs`, de samme navnene som standardstiene for sikkerhetskopier bruker. Mappene står oppført i detaljene til lagringsstedet og kan få nytt navn der (se [Endre en adresse](#addresses)). Et domene uten mappe på et lagringssted kan ikke velge det lagringsstedet.

Når et domene bruker et lagringssted i begge roller, får den andre rollen et suffiks, og den første beholder mappen sin. Et lagringssted som allerede mottar kopiene til et domene, lagrer elementene som sendes rett dit, i `<folder>-direct`; et lagringssted som allerede lagrer et domene, mottar kopiene i `<folder>-copies`.

Noen lagringssteder er selv et restic-depot: en adresse som allerede inneholdt et depot da lagringsstedet ble lagt til, et navngitt depot fra et eksisterende oppsett, eller et kopimål i roten av en bucket. Et slikt lagringssted har ingen mapper, alle domener deler det ene depotet, og det kan ikke få en andre rolle. For å lagre mer hos samme leverandør kobler du til en annen bucket eller mappe som et eget lagringssted.

## Detaljene til et lagringssted {#details}

Hvert lagringssted er en rad med leverandøren, hva det brukes til, og siste test eller kopi. **Test** sjekker hver adresse på lagringsstedet, og **Detaljer** åpner innstillingene. Hver endring i detaljene lagres med én gang.

- **Generelt**: navnet, bryteren som slår lagringsstedet av og på, adressen og, for en enhet du drifter selv, **Hvor står enheten?** (se [Utenfor bygningen](#off-the-premises)).
- **Oppbevaring**: keep-last, daglig, ukentlig og månedlig, for hvert depot på lagringsstedet. Et nytt lagringssted starter med standardreglene; et lagringssted der hver regel står på null, trimmer aldri.
- **Beskyttelse**: bryteren **Append-only**. Motparten må håndheve append-only; med bryteren på beskjærer og sletter BombVault aldri noe der. På en rest-server med append-only på kjører **Sjekk append-only** tamper-testen mot hver domenesti, hver påslått kopi og hvert depot på lagringsstedet, og viser ett svar for hele lagringsstedet: *sletting avvist* eller *sletting godtatt* (se [Ekstern lagring og gjenoppretting](offsite-recovery.md)). Bare eksterne lagringssteder har denne delen, fordi ingenting på denne boksen kan hindre at et lokalt depot blir slettet.
- **Tilgang**: legitimasjonen og, for S3, lagringsklassen. Et lagringssted som bruker den delte legitimasjonen, får et eget sett ved første endring. Et direkte depot på lagringsstedet som den nye legitimasjonen ikke kan åpne, beholder den gamle, og svaret sier fra om det. Mappe-, SFTP- og rclone-lagringssteder har ingen slik del.
- **Grenser**: opplastings- og nedlastingshastigheten og vekstbudsjettet.
- **Mapper**: én bryter per domene, med navnet på mappen. Et domene som er slått av her, kan ikke velge lagringsstedet.

Å senke oppbevaringen spør først og sier hvor mange elementer det gjelder; å slå av append-only spør først og sier hvor mange depoter på lagringsstedet som mister beskyttelsen. Å slå av et lagringssted slår av hvert depot på det; et lagringssted som et domene er lagret på, kan ikke slås av.

## Domener-kortet {#domains}

Kortet har én rad per domene, med tidsplanen, hvor domenet er lagret, hvor det kopieres, og unntakene.

- **Lagret på**: så lenge domenets sikkerhetskopisti ikke inneholder sikkerhetskopier, blir det valgte lagringsstedet domenets hjem, og stien flyttes dit. Når den inneholder sikkerhetskopier, blir valget for containere, VM-er og filsett standarden for nye elementer, som tar det i bruk ved sin første sikkerhetskopi; elementer som allerede har sikkerhetskopier, blir der de er, fordi BombVault aldri flytter en sikkerhetskopi. For flashen og BombVaults egen konfigurasjon flyttes hjemmet, og sikkerhetskopiene som allerede er skrevet, blir liggende på det gamle lagringsstedet.
- **Kopiert til**: én chip per lagringssted som kan ta imot domenets kopier. Å huke av en chip gjør lagringsstedet til et kopimål for domenet; første gang sier BombVault på forhånd hvor mange elementer og øyeblikksbilder og hvor mye data den første kjøringen sender. Å fjerne haken stopper nye kopier: kopiene som allerede ligger der, blir værende og eldes etter lagringsstedets oppbevaring, og elementer med et eget valg fortsetter å kopiere dit. Å fjerne haken fra den siste chipen stopper alle kopier, også til lagringssteder som legges til senere, til en chip blir huket av igjen. Et avslått lagringssted vises som en nedtonet chip og kan ikke velges.
- **Unntak**: elementene med et eget valg, som en liste med lenker til kortene deres.
- **Kopier nå** kjører domenets kopier med én gang.

Et domene som er satt på pause etter en ombygging via Oppdag, viser pausen på raden sin, med **Bekreft standard** (se [Plassering per element](offsite-recovery.md#placement)).

## Endre en adresse {#addresses}

Mappen til et domene kan endres i detaljene til lagringsstedet, og det samme kan adressen til et lokalt lagringssted, for eksempel etter at et depot er flyttet til en annen disk for hånd. BombVault tester hver adresse endringen berører, og godtar den når hver nye adresse er tom og ingenting var lagret på den gamle, eller når hver nye adresse inneholder det samme restic-depotet som den gamle. Alt annet avvises, med antallet sikkerhetskopier som fortsatt ligger på den gamle adressen. Et eksternt lagringssted beholder adressen sin; for å sikkerhetskopiere et annet sted kobler du det til som et eget lagringssted.

BombVault bygger listen over lagringssteder fra sin egen database og lister aldri et eksternt depot for å fylle den; testen kjører bare når du endrer noe.

## Fjerne et lagringssted {#remove}

Et lagringssted kan bare fjernes så lenge ingenting bruker det: ingen domener er lagret der, ingen standard peker på det, ingen elementer er lagret der, og ingen direkte depoter på det inneholder elementer. Ellers lister avvisningen opp hva som holder på det. Når det fjernes, forsvinner kopimålene på det også, og dets egen legitimasjon, med mindre en hentekilde eller et annet lagringssted bruker den. Ingenting slettes i selve lagringen, og bekreftelsen sier hvor mange kopier som blir liggende igjen der.

## Uten lagringssted {#without-a-place}

En adresse som ikke passer i formen lagringssted pluss mappe, fortsetter å fungere og står oppført under **Uten lagringssted**, med adressen sin. Native `b2:`-, `gs:`- og `swift:`-adresser er blant dem. **Knytt til lagringssted** knytter en slik rad til et lagringssted, etter den samme testen som ved [endring av en adresse](#addresses). Et kopimål uten lagringssted nevnes også på domenets rad, ved siden av chipene, og fortsetter å kopiere. En ekstern rad der har sin egen **Append-only**-bryter, og å slå den av spør først og oppgir hvor mange elementer som har sikkerhetskopier på den adressen. Et direkte depot følger bryteren til målet sitt.

## Utenfor bygningen {#off-the-premises}

**Hvor står enheten?** har to svar: **Her i huset** og **På en annen lokasjon**. En kopi teller som et eget sted, for 3-2-1-linjen på kortene og for Dashboardets eksterne kontroller, bare når lagringsstedet står på en annen lokasjon. En ekstra disk eller en NAS i samme hus er en kopi til, ikke et sted til. Svaret endrer ingen kopi. Skyleverandører står alltid på en annen lokasjon og en mappe på denne Unraid-serveren alltid her, så skjemaet spør ikke om dem; for alle andre lagringssteder endrer du svaret i detaljene. Et lagringssted på en annen lokasjon har merket **Annen lokasjon** på raden sin.

## Tilkoblingstyper

### Mappe på denne Unraid-serveren eller en NAS {#kind-local}

Adressen er en sti under `/mnt`, skrevet uten `/mnt`, for eksempel `user/bombvault`, og mappen til hvert domene ligger under den: `user/bombvault/container`.

- **Mappe på denne Unraid-serveren** velger blant delingene, diskene og poolene.
- **Synology**, **QNAP**, **TrueNAS**, **En annen Unraid** og **Annen delt ressurs** velger fra `/mnt/remotes`. Monter delingen på Unraid først, for eksempel med utvidelsen Unassigned Devices. Host Data må være montert Read/Write - Slave, ellers forblir en deling som monteres etter at BombVault startet, usynlig til en omstart (se [Konfigurasjon](configuration.md)).

Mappevelgeren oppretter en mappe der den står, med **Ny mappe**. Testen sjekker at mappen er tom eller ikke finnes, og at BombVault kan skrive der.

### S3 {#kind-s3}

Adressen er `s3:https://<endpoint>/<bucket>/<path>`, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** trenger bare nøkkel-ID-en og applikasjonsnøkkelen. BombVault spør B2 hvilken bucket, hvilket S3-endepunkt og hvilken mappe nøkkelen er begrenset til, og bygger adressen ut fra det. En nøkkel som når alle buckets, tilbyr bucketene sine å velge mellom.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** og **Vultr** ber om nøkkelen og, der leverandøren trenger det, om region, konto-ID eller endepunkt. BombVault fyller inn endepunktet og lister bucketene når nøkkelen har lov til å liste dem; ellers skriver du inn navnet på bucketen.
- **Google Cloud Storage** går via S3-grensesnittet med en HMAC-nøkkel, som du oppretter i Cloud Storage-innstillingene under Interoperability. En tjenestekontofil fungerer ikke her.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** og **Annen S3-tjeneste** tar adressen til tjenesten og en nøkkel.

Lagringsklassen settes i detaljene til lagringsstedet, begrenset til nivåer en gjenoppretting kan lese uten opptining.

### rest-server {#kind-rest}

Adressen er `rest:<url>/<user>`, for eksempel `rest:https://nas.lan:8000/tower`. Skjemaet ber om adressen til serveren, en bruker og et passord. Med `--private-repos` kan en bruker bare nå stier som begynner med brukerens eget navn, så BombVault setter brukeren først med mindre du skriver inn en annen sti. Når serveren avviser en sti utenfor brukerens egen, sier feilmeldingen det.

Skjemaet for rest-server har en oppskrift, klar til å lime inn, på en rest-server i append-only-modus med én bruker for denne BombVault. **Vis oppskrift** lager et passord, som vises én gang, og gir en `docker run`-linje, en compose-fil og en Unraid-mal, hver med `htpasswd`-linjen som skal inn på serveren; brukeren og passordet går rett inn i skjemaet.

**En annen BombVault** viser, over sine egne felt, de åpne tilbudene som andre instanser har sendt via Flåte. Godtar du ett, legges det til et lagringssted som bare tar imot kopier av det tilbudte domenet, fordi et tilbud har med seg en bruker for akkurat det ene domenet. Godtar du på Flåte-siden, legges det samme lagringsstedet til.

### SFTP {#kind-sftp}

Adressen er `sftp://<user>@<host>:<port>/<path>`, for eksempel `sftp://bv@backup.lan:22/bombvault`. Skjemaet ber om vert, port og bruker og viser BombVaults offentlige nøkkel. Legg den nøkkelen til i brukerens `~/.ssh/authorized_keys` på serveren; ingenting annet må installeres der. BombVault godtar serverens vertsnøkkel ved første kontakt og sjekker den fra da av.

**Hetzner Storage Box** fyller inn `<user>.your-storagebox.de` og port 23. Installer nøkkelen på boksen med Hetzners egen kommando, som ber om passordet til boksen én gang:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Skjemaet ber om adressen til serveren, brukeren og et app-passord. Opprett app-passordet i sikkerhetsinnstillingene til kontoen, og skriv inn bruker-ID-en i stedet for en e-postadresse. BombVault bygger WebDAV-stien produktet bruker, og gir tilkoblingen videre til restic via rclones miljøvariabler, med passordet i rclones tilslørte form. Adressen lyder `rclone:bvp<id>:<path>`, der `bvp<id>` er en remote som bare finnes i det miljøet; ingenting skrives til rclone-konfigurasjonen.

### Azure Blob {#kind-azure}

Adressen er `azure:<container>:/<path>`. Skjemaet ber om lagringskontoen og tilgangsnøkkelen til den; etter **Test tilkobling** lister det kontoens beholdere å velge mellom, eller du skriver inn navnet på en beholder. BombVault gir kontoen og nøkkelen videre til restic som `AZURE_ACCOUNT_NAME` og `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Adressen er `rclone:<remote>:<path>`. Skjemaet lister remotene i BombVaults rclone-konfigurasjon å velge mellom. For å erstatte den konfigurasjonen limer du inn en hel `rclone.conf` under **rclone-konfigurasjon** og klikker **Lagre konfigurasjon**. Den lagres med én gang og gjelder for hvert rclone-lagringssted, enten vinduet deretter legger til et lagringssted eller ikke.

## Kopier mellom lagringssteder med ulik legitimasjon {#different-credentials}

Et domene som er lagret på et eksternt lagringssted, er kilden til kopiene sine. `restic copy` kjører med ett miljø, og BombVault legger kildens legitimasjon til målets når de to ikke setter den samme variabelen til ulike verdier. Et Nextcloud-lagringssted og et B2-lagringssted bruker ulike variabler, så et domene som er lagret i Nextcloud, kan kopieres til B2. To S3-kontoer eller to rest-server-brukere ville trengt de samme variablene med ulike verdier; restic kan ikke ta begge, og chipen på Domener-kortet sier at legitimasjonen ikke passer sammen.
