# Lagringssteder

Et lagringssted er et sted, hvor BombVault opbevarer sikkerhedskopier: en mappe på denne Unraid, en deling på et NAS, en bucket hos en cloududbyder, en rest-server, en SFTP-konto eller en Nextcloud. Du forbinder hvert sted én gang, på **Indstillinger, Lagring**, og dets legitimationsoplysninger, opbevaring, beskyttelse og lokation hører til stedet. Domænerne (containere, VM'er, flashen, BombVaults egen konfiguration, mappesæt og ZFS-datasæt) vælger derefter blandt stederne: hvor hvert domæne gemmes, og hvor det kopieres til. ZFS-datasættene har deres række på kortet Domæner, så længe ZFS-domænet er slået til.

## Tilføj et sted {#add-a-place}

**Tilføj sted** åbner et vindue med én flise pr. udbyder i tre grupper: cloudlagring, selvhostede tjenester samt NAS-enheder og denne server.

1. Vælg en flise, og udfyld dens formular. Øjeknappen viser en hemmelighed, du har indtastet.
2. **Test forbindelse** tjekker stedet og opretter intet. For hvert domænes mappe fortæller den, hvad den fandt: tom eller findes ikke endnu, rummer allerede et restic-arkiv, eller den fejl, der stoppede testen.
3. Giv stedet et navn; udbyderens navn er udfyldt på forhånd. For en enhed, du selv driver, besvarer du **Hvor står enheden?**. Cloududbydere er altid på en anden lokation, og en mappe på denne Unraid er altid her.
4. **Tilføj** gemmer stedet.

Et nyt sted bruges endnu ikke af noget domæne. Vælg det under **Gemt på** eller **Kopieret til** på [kortet Domæner](#domains), eller på et elements kort for kun det element.

## Mapper {#folders}

Et sted har én mappe pr. domæne: `container`, `vms`, `flash`, `config`, `files` og `zfs`, de samme navne som standardstierne til sikkerhedskopier. Mapperne står i stedets detaljer og kan omdøbes der (se [Ændring af en adresse](#addresses)). Et domæne uden mappe på et sted kan ikke vælge det sted.

Når et domæne bruger et sted i begge roller, får den anden rolle et suffiks, og den første beholder sin mappe. Et sted, der allerede modtager et domænes kopier, gemmer de elementer, der sendes direkte dertil, i `<folder>-direct`; et sted, der allerede gemmer et domæne, modtager dets kopier i `<folder>-copies`.

Nogle steder er selv et restic-arkiv: en adresse, der allerede rummede et arkiv, da stedet blev tilføjet, et navngivet arkiv fra en eksisterende opsætning eller et kopimål ved roden af en bucket. Et sådant sted har ingen mapper, alle domæner deler dets ene arkiv, og det påtager sig ingen anden rolle. Vil du gemme mere hos den samme udbyder, så forbind en anden bucket eller mappe som et sted for sig.

## Stedets detaljer {#details}

Hvert sted er en række med sin udbyder, hvad det bruges til, og sin seneste test eller kopi. **Test** tjekker alle adresser på stedet, og **Detaljer** åbner dets indstillinger. Hver ændring i detaljerne gemmes, mens du laver den.

- **Generelt**: navnet, kontakten, der slår stedet til og fra, adressen og, for en enhed du selv driver, **Hvor står enheden?** (se [Uden for bygningen](#off-the-premises)).
- **Opbevaring**: behold seneste, daglige, ugentlige og månedlige, for hvert arkiv på stedet. Et nyt sted starter med standardreglerne; et sted, hvor alle regler står på nul, beskærer aldrig noget.
- **Beskyttelse**: kontakten **Append-only**. Den anden ende skal selv håndhæve append-only; med kontakten slået til beskærer og sletter BombVault aldrig noget der. På en rest-server med append-only slået til kører **Test append-only-beskyttelse** manipulationstesten mod hver domænesti, hver slået-til kopi og hvert arkiv på stedet og viser ét svar for hele stedet, *sletninger afvist* eller *sletninger accepteret* (se [Off-site og gendannelse](offsite-recovery.md)). Kun fjerne steder har dette afsnit, fordi intet på denne maskine kan forhindre, at et lokalt arkiv bliver slettet.
- **Adgang**: legitimationsoplysningerne og, for S3, lagringsklassen. Et sted, der bruger de fælles legitimationsoplysninger, får sit eget sæt ved første ændring. Et direkte arkiv på stedet, som de nye legitimationsoplysninger ikke kan åbne, beholder de gamle, og svaret siger det. Mappe-, SFTP- og rclone-steder har ikke dette afsnit.
- **Grænser**: upload- og downloadhastigheden og vækstbudgettet.
- **Mapper**: én kontakt pr. domæne med navnet på dets mappe. Et domæne, der er slået fra her, kan ikke vælge stedet.

Sænker du opbevaringen, spørger BombVault først og siger, hvor mange elementer det berører; slår du append-only fra, spørger BombVault først og siger, hvor mange arkiver på stedet mister det. Slår du et sted fra, slås alle arkiver på det fra; et sted, som et domæne gemmes på, kan ikke slås fra.

## Kortet Domæner {#domains}

Kortet har én række pr. domæne med dets tidsplan, hvor det gemmes, hvor det kopieres til, og dets undtagelser.

- **Gemt på**: så længe domænets sikkerhedskopisti ikke rummer nogen sikkerhedskopier, bliver det valgte sted domænets hjemsted, og stien flytter dertil. Når den rummer sikkerhedskopier, bliver valget for containere, VM'er og mappesæt standarden for nye elementer, som overtager den ved deres første sikkerhedskopi; elementer, der allerede har sikkerhedskopier, bliver, hvor de er, fordi BombVault aldrig flytter en sikkerhedskopi. For flashen og BombVaults egen konfiguration flytter hjemstedet, og de sikkerhedskopier, der allerede er skrevet, bliver på det gamle sted.
- **Kopieret til**: én chip pr. sted, der kan tage imod domænets kopier. Sætter du flueben ved en chip, bliver stedet kopimål for domænet; første gang siger BombVault på forhånd, hvor mange elementer og øjebliksbilleder og hvor meget data den første kørsel sender. Fjerner du fluebenet, stopper nye kopier: de kopier, der allerede ligger der, bliver og ældes efter stedets opbevaring, og elementer med deres eget valg bliver ved med at kopiere dertil. Fjerner du fluebenet ved den sidste chip, stopper alle kopier, også til steder, der tilføjes senere, indtil der igen sættes flueben ved en chip. Et sted, der er slået fra, vises som en nedtonet chip og kan ikke vælges.
- **Undtagelser**: elementerne med deres eget valg, som en liste med links til deres kort.
- **Kopiér nu** kører domænets kopier med det samme.

Et domæne, der er sat på pause efter en genopbygning via Opdag sikkerhedskopier, viser pausen i sin række med **Bekræft standard** (se [Placering pr. element](offsite-recovery.md#placement)).

## Ændring af en adresse {#addresses}

Et domænes mappe kan ændres i stedets detaljer, og det samme kan adressen på et lokalt sted, for eksempel når et arkiv er flyttet manuelt til en anden disk. BombVault tester hver adresse, ændringen berører, og godtager den, når hver ny adresse er tom, og intet var gemt på den gamle, eller når hver ny adresse rummer det samme restic-arkiv som den gamle. Alt andet afvises med antallet af sikkerhedskopier, der stadig ligger på den gamle adresse. Et fjernt sted beholder sin adresse; vil du sikkerhedskopiere et andet sted hen, så forbind det som et sted for sig.

BombVault bygger listen over steder ud fra sin egen database og lister aldrig et fjernarkiv for at udfylde den; testen kører kun, når du ændrer noget.

## Fjern et sted {#remove}

Et sted kan kun fjernes, så længe intet bruger det: intet domæne gemmes der, ingen standard peger på det, intet element gemmes der, og intet direkte arkiv på stedet rummer elementer. Ellers lister afvisningen, hvad der holder fast i det. Fjernelsen tager stedets kopimål med og dets egne legitimationsoplysninger, medmindre en hentekilde eller et andet sted bruger dem. Intet slettes i selve lageret, og bekræftelsen siger, hvor mange kopier der bliver liggende der.

## Uden sted {#without-a-place}

En adresse, der ikke passer til formen et sted plus en mappe, fungerer fortsat og står under **Uden sted** med sin adresse. Native `b2:`-, `gs:`- og `swift:`-adresser hører til dem. **Knyt til sted** knytter en sådan række til et sted efter den samme test som ved [ændring af en adresse](#addresses). Et kopimål uden sted står også i sit domænes række ved siden af chippene og bliver ved med at kopiere. En fjern række under **Uden sted** har sin egen **Append-only**-kontakt, og slår du den fra, spørger BombVault først og nævner antallet af elementer, der har sikkerhedskopier på den adresse. Et direkte arkiv følger kontakten for sit mål.

## Uden for bygningen {#off-the-premises}

**Hvor står enheden?** har to svar: **Her i huset** og **På en anden lokation**. En kopi tæller kun som en lokation for sig, i 3-2-1-linjen på kortene og i Oversigtens off-site-tjek, når dens sted er på en anden lokation. En ekstra disk eller et NAS i samme hus er en ekstra kopi, ikke en ekstra lokation. Svaret ændrer ingen kopi. Cloududbydere er altid på en anden lokation og en mappe på denne Unraid altid her, så formularen spørger ikke om dem; for alle andre steder ændrer du svaret i stedets detaljer. Et sted på en anden lokation bærer mærket **Anden lokation** i sin række.

## Forbindelsestyper

### Mappe på denne Unraid eller et NAS {#kind-local}

Adressen er en sti under `/mnt`, skrevet uden `/mnt`, for eksempel `user/bombvault`, og hvert domænes mappe ligger under den: `user/bombvault/container`.

- **Mappe på denne Unraid** vælger blandt delinger, diske og pools.
- **Synology**, **QNAP**, **TrueNAS**, **En anden Unraid** og **Anden deling** vælger fra `/mnt/remotes`. Montér først delingen på Unraid, for eksempel med pluginnet Unassigned Devices. Host Data skal være monteret Read/Write - Slave, ellers forbliver en deling, der monteres, efter BombVault er startet, usynlig indtil en genstart (se [Konfiguration](configuration.md)).

Mappevælgeren opretter med **Ny mappe** en mappe der, hvor den står. Testen tjekker, at mappen er tom eller ikke findes, og at BombVault kan skrive der.

### S3 {#kind-s3}

Adressen er `s3:https://<endpoint>/<bucket>/<path>`, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** skal kun bruge nøgle-id'et og applikationsnøglen. BombVault spørger B2, hvilken bucket, hvilket S3-endpoint og hvilken mappe nøglen er begrænset til, og bygger adressen ud fra dem. En nøgle, der må nå alle buckets, tilbyder sine buckets at vælge imellem.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** og **Vultr** beder om nøglen og, hvor udbyderen kræver det, om regionen, konto-id'et eller endpointet. BombVault udfylder endpointet og lister bucketerne, når nøglen må liste dem; ellers skriver du bucketens navn.
- **Google Cloud Storage** går via sin S3-grænseflade med en HMAC-nøgle, som du opretter i indstillingerne for Cloud Storage under Interoperability. En fil til en tjenestekonto virker ikke her.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** og **Anden S3-tjeneste** tager tjenestens adresse og en nøgle.

Lagringsklassen sættes i stedets detaljer, begrænset til niveauer, som en gendannelse kan læse uden optøning.

### rest-server {#kind-rest}

Adressen er `rest:<url>/<user>`, for eksempel `rest:https://nas.lan:8000/tower`. Formularen beder om serverens adresse, en bruger og en adgangskode. Med `--private-repos` må en bruger kun nå stier, der begynder med brugerens eget navn, så BombVault sætter brugeren forrest, medmindre du skriver en anden sti. Afviser serveren en sti uden for brugerens egen, siger fejlen det.

Formularen til rest-server har en opskrift, klar til at indsætte, på en rest-server i append-only-tilstand med én bruger til denne BombVault. **Vis opskrift** laver en adgangskode, der vises én gang, og giver en `docker run`-linje, en compose-fil og en Unraid-skabelon, hver med den `htpasswd`-linje, der skal ind på serveren; brugeren og adgangskoden går direkte ind i formularen.

**Et andet BombVault** viser over sine egne felter de åbne tilbud, som andre instanser har sendt via Flåde. Accepterer du et, tilføjes et sted, der kun gemmer kopier af det tilbudte domæne, fordi et tilbud bærer en bruger til netop det ene domæne. Accepterer du på siden Flåde, tilføjes det samme sted.

### SFTP {#kind-sftp}

Adressen er `sftp://<user>@<host>:<port>/<path>`, for eksempel `sftp://bv@backup.lan:22/bombvault`. Formularen beder om vært, port og bruger og viser BombVaults offentlige nøgle. Tilføj den nøgle til brugerens `~/.ssh/authorized_keys` på serveren; intet andet skal installeres der. BombVault accepterer serverens værtsnøgle ved første kontakt og tjekker den derefter.

**Hetzner Storage Box** udfylder `<user>.your-storagebox.de` og port 23. Installér nøglen på boksen med Hetzners egen kommando, som beder om boksens adgangskode én gang:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Formularen beder om serverens adresse, brugeren og en app-adgangskode. Opret app-adgangskoden i kontoens sikkerhedsindstillinger, og angiv bruger-id'et frem for en e-mailadresse. BombVault bygger den WebDAV-sti, produktet bruger, og giver forbindelsen videre til restic gennem rclones miljøvariabler med adgangskoden i rclones tilslørede form. Adressen lyder `rclone:bvp<id>:<path>`, hvor `bvp<id>` er en remote, der kun findes i det miljø; intet skrives til rclone-konfigurationen.

### Azure Blob {#kind-azure}

Adressen er `azure:<container>:/<path>`. Formularen beder om lagerkontoen og dens adgangsnøgle; efter **Test forbindelse** lister den kontoens containere at vælge imellem, eller du skriver en containers navn. BombVault giver kontoen og nøglen videre til restic som `AZURE_ACCOUNT_NAME` og `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Adressen er `rclone:<remote>:<path>`. Formularen lister de remotes i BombVaults rclone-konfiguration, du kan vælge imellem. Vil du erstatte den konfiguration, så indsæt en hel `rclone.conf` under **rclone-konfiguration**, og klik på **Gem konfiguration**. Den gemmes med det samme og gælder for alle rclone-steder, uanset om vinduet derefter tilføjer et.

## Kopier mellem steder med forskellige legitimationsoplysninger {#different-credentials}

Et domæne, der gemmes på et fjernt sted, er kilden til sine kopier. `restic copy` kører med ét miljø, og BombVault lægger kildens legitimationsoplysninger til målets, når de to ikke sætter den samme variabel til forskellige værdier. Et Nextcloud-sted og et B2-sted bruger forskellige variabler, så et domæne, der gemmes i Nextcloud, kan kopieres til B2. To S3-konti eller to rest-server-brugere ville kræve de samme variabler med forskellige værdier; restic kan ikke tage begge, og chippen på kortet Domæner siger, at legitimationsoplysningerne ikke passer sammen.
